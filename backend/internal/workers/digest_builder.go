package workers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/ai"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/google/uuid"
)

// DigestBuilder owns the entire news-out-to-SMS pipeline.
//
// It performs three jobs every tick:
//
//  1. Initial-news send: any news/both subscription whose +5m cooldown
//     has elapsed and that has never received a digest gets one,
//     summarized at send time from the article cache. The same row is
//     also written to notifications_log with message_type='digest' so
//     the cadence clock starts ticking from this first send.
//  2. Recurring digest: subscriptions whose initial digest has gone out
//     and whose cadence window (daily/weekly/monthly) has elapsed
//     receive their next digest, summarized at send time.
//  3. Housekeeping: articles that every eligible subscriber has now
//     consumed are deleted from news_articles, and a TTL safety net
//     removes anything older than ArticleRetention.
//
// Summarization happens once per article per send, so two subs with
// different cadences on the same team each pay their own LLM cost — but
// the underlying article is fetched and stored once.
type DigestBuilder struct {
	Repo *repo.Repo
	SMS  sms.Sender
	AI   ai.Summarizer
	Log  *slog.Logger
	// News is consulted only as a fallback when an initial-news send
	// finds the cache empty for a team (so first messages aren't blank
	// while waiting for the next 6h aggregator poll). Optional.
	News *NewsAggregator

	Interval         time.Duration
	ArticleRetention time.Duration

	// MaxBodyChars caps the SMS body length. Zero falls back to 1200.
	MaxBodyChars int

	// Now allows tests to inject a fixed clock.
	Now func() time.Time
}

func (d *DigestBuilder) Run(ctx context.Context) {
	if d.Interval == 0 {
		d.Interval = 5 * time.Minute
	}
	if d.Now == nil {
		d.Now = func() time.Time { return time.Now().UTC() }
	}
	tk := time.NewTicker(d.Interval)
	defer tk.Stop()

	d.Log.Info("digest builder started", "interval", d.Interval.String())
	for {
		select {
		case <-ctx.Done():
			d.Log.Info("digest builder stopped")
			return
		case <-tk.C:
			if err := d.Tick(ctx); err != nil {
				d.Log.Warn("digest tick failed", "err", err)
			}
		}
	}
}

// Tick processes initial sends, then every supported recurring frequency,
// then runs housekeeping. We iterate frequencies dynamically (via
// models.Frequencies) so adding a new cadence is a constant change.
func (d *DigestBuilder) Tick(ctx context.Context) error {
	now := d.now()
	if err := d.flushInitial(ctx, now); err != nil {
		d.Log.Warn("initial flush failed", "err", err)
	}
	for _, f := range models.Frequencies() {
		if err := d.flushFreq(ctx, f, now); err != nil {
			d.Log.Warn("recurring flush failed", "err", err, "freq", f)
		}
	}
	d.housekeep(ctx, now)
	return nil
}

// flushInitial sends the very first digest for any news/both subscription
// whose +5m cooldown has passed.
func (d *DigestBuilder) flushInitial(ctx context.Context, now time.Time) error {
	subs, err := d.Repo.SubscriptionsDueForInitialNews(ctx, now)
	if err != nil {
		return err
	}
	for _, s := range subs {
		// Window: anything since the subscription was created. Falls
		// back to a 24h look-back if CreatedAt is somehow zero.
		since := s.CreatedAt
		if since.IsZero() {
			since = now.Add(-24 * time.Hour)
		}
		articles, err := d.Repo.UnsentArticlesForSubscription(ctx, s.ID, s.TeamID, since)
		if err != nil {
			d.Log.Warn("initial article lookup failed", "err", err, "sub", s.ID)
			continue
		}
		// First-message UX guard: if the cache is empty for this team
		// (e.g. polls haven't run yet) trigger an on-demand fetch and
		// re-query. Best-effort; if it still returns nothing we send a
		// short placeholder so the user knows the subscription works.
		if len(articles) == 0 && d.News != nil {
			if _, ferr := d.News.FetchOnDemand(ctx, models.Team{
				ID: s.TeamID, Name: s.TeamName, League: s.League,
			}); ferr == nil {
				articles, _ = d.Repo.UnsentArticlesForSubscription(ctx, s.ID, s.TeamID, since)
			}
		}

		body, included, err := d.BuildNewsDigestBody(ctx, s, articles, true)
		if err != nil {
			d.Log.Warn("initial body build failed", "err", err, "sub", s.ID)
			continue
		}
		dedupe := fmt.Sprintf("digest:initial:%s", s.ID)
		if err := d.sendDigest(ctx, s, body, dedupe, included, now); err != nil {
			d.Log.Warn("initial send failed", "err", err, "sub", s.ID)
			continue
		}
		if err := d.Repo.MarkInitialNewsSent(ctx, s.ID); err != nil {
			d.Log.Warn("flip initial flag failed", "err", err, "sub", s.ID)
		}
		d.Log.Info("digest_built",
			"kind", "initial", "sub", s.ID, "team", s.TeamName,
			"articles", len(included))
	}
	return nil
}

func (d *DigestBuilder) flushFreq(ctx context.Context, freq models.Frequency, now time.Time) error {
	subs, err := d.Repo.AllSubscriptions(ctx, freq)
	if err != nil {
		return err
	}
	window := freq.Window()
	for _, s := range subs {
		// Live-only subs store a frequency but never receive digests.
		if s.UpdateType == models.UpdateLive {
			continue
		}
		last, err := d.Repo.LastDigestSentAt(ctx, s.ID)
		if err != nil {
			d.Log.Warn("last-sent lookup failed", "err", err, "sub", s.ID)
			continue
		}
		// Honor cadence: don't send a daily digest twice in 24h. The
		// initial send always writes a 'digest' row first, so once
		// that's in place this clock keys cleanly off "first message".
		if last.IsZero() {
			// No prior digest at all — initial send hasn't fired yet.
			// Skip; the initial flush handles this case.
			continue
		}
		if now.Sub(last) < window {
			continue
		}

		articles, err := d.Repo.UnsentArticlesForSubscription(ctx, s.ID, s.TeamID, last)
		if err != nil {
			d.Log.Warn("article lookup failed", "err", err, "sub", s.ID)
			continue
		}
		if len(articles) == 0 {
			// No new articles this window — skip rather than send a
			// "nothing to report" SMS that would still consume the
			// cadence slot.
			continue
		}
		body, included, err := d.BuildNewsDigestBody(ctx, s, articles, false)
		if err != nil {
			d.Log.Warn("body build failed", "err", err, "sub", s.ID)
			continue
		}
		dedupe := fmt.Sprintf("digest:%s:%d", freq, now.Unix()/int64(window.Seconds()))
		if err := d.sendDigest(ctx, s, body, dedupe, included, now); err != nil {
			d.Log.Warn("send failed", "err", err, "sub", s.ID)
			continue
		}
		d.Log.Info("digest_built",
			"kind", "recurring", "freq", freq, "sub", s.ID,
			"team", s.TeamName, "articles", len(included))
	}
	return nil
}

// BuildNewsDigestBody is the single funnel that turns a set of stored
// articles into an SMS body. It summarizes each article on demand via
// the AI provider (falling back to ai.Truncate on failure) and formats
// them as bullets keyed off the team name.
//
// `initial` flips the header so the first-ever message reads as a
// welcome digest rather than a routine roll-up.
//
// Returns the formatted body, the slice of articles actually included
// (after the per-message size cap) and any unrecoverable error.
func (d *DigestBuilder) BuildNewsDigestBody(
	ctx context.Context,
	sub models.SubscriptionDetail,
	articles []models.NewsArticle,
	initial bool,
) (string, []models.NewsArticle, error) {
	maxBody := d.MaxBodyChars
	if maxBody <= 0 {
		maxBody = 1200
	}

	var (
		header strings.Builder
		body   strings.Builder
	)
	if initial {
		fmt.Fprintf(&header, "Welcome to GamePulse %s news!", sub.TeamName)
		if len(articles) == 0 {
			header.WriteString(" No fresh stories yet — you'll get the next one as soon as it lands.")
			return header.String(), nil, nil
		}
		header.WriteString("\n")
	} else {
		freqLabel := titleCase(string(sub.Frequency))
		fmt.Fprintf(&header, "%s news digest for %s:\n", freqLabel, sub.TeamName)
	}

	included := make([]models.NewsArticle, 0, len(articles))
	body.WriteString(header.String())
	for i, a := range articles {
		summary, err := d.summarize(ctx, a)
		if err != nil || strings.TrimSpace(summary) == "" {
			// Defensive — summarize already falls back internally, but
			// guard against a brand-new backend returning an empty body.
			summary = ai.Truncate(a.Content)
			if summary == "" {
				summary = ai.Truncate(a.Title)
			}
		}
		line := "• " + summary
		// Stop adding bullets if the next one would overflow the cap.
		// We always include at least one bullet so the user never gets
		// just a header.
		if i > 0 && body.Len()+len(line)+1 > maxBody {
			break
		}
		if i > 0 {
			body.WriteString("\n")
		}
		body.WriteString(line)
		included = append(included, a)
	}

	out := body.String()
	if len(out) > maxBody {
		// "…" is 3 bytes in UTF-8; subtract its length so the final
		// string fits within the cap exactly.
		const ellipsis = "…"
		cut := maxBody - len(ellipsis)
		if cut < 0 {
			cut = 0
		}
		out = out[:cut] + ellipsis
	}
	return out, included, nil
}

func (d *DigestBuilder) summarize(ctx context.Context, a models.NewsArticle) (string, error) {
	if d.AI == nil {
		// No AI configured — fall back to title + truncated content.
		return ai.Truncate(a.Title), nil
	}
	s, err := d.AI.Summarize(ctx, a.Title, a.Content)
	if err != nil {
		d.Log.Warn("summarize failed", "err", err, "article", a.ID)
		return ai.Truncate(a.Content), nil
	}
	return s, nil
}

// sendDigest writes the digest row to notifications_log first (so a
// double-tick can't double-send), then sends the SMS, then records
// per-subscription consumption for every included article.
func (d *DigestBuilder) sendDigest(
	ctx context.Context,
	sub models.SubscriptionDetail,
	body, dedupe string,
	included []models.NewsArticle,
	now time.Time,
) error {
	inserted, err := d.Repo.LogNotification(ctx, &models.NotificationLog{
		SubscriptionID: sub.ID,
		MessageType:    models.MessageDigest,
		Content:        body,
		DedupeKey:      dedupe,
		SentAt:         now,
	})
	if err != nil {
		return fmt.Errorf("log notification: %w", err)
	}
	if !inserted {
		// Already sent for this dedupe key — guard against double-runs.
		return nil
	}
	if err := d.SMS.Send(ctx, sub.PhoneNumber, body); err != nil {
		return fmt.Errorf("sms send: %w", err)
	}
	if len(included) > 0 {
		ids := make([]uuid.UUID, len(included))
		for i, a := range included {
			ids[i] = a.ID
		}
		if err := d.Repo.MarkArticlesSent(ctx, sub.ID, ids, now); err != nil {
			d.Log.Warn("mark consumed failed", "err", err, "sub", sub.ID)
		}
	}
	return nil
}

// housekeep deletes articles that every eligible subscriber has now
// consumed plus a TTL fallback.
func (d *DigestBuilder) housekeep(ctx context.Context, now time.Time) {
	if n, err := d.Repo.DeleteFullyConsumedArticles(ctx); err != nil {
		d.Log.Warn("delete consumed articles failed", "err", err)
	} else if n > 0 {
		d.Log.Info("articles_deleted", "kind", "consumed", "count", n)
	}

	if d.ArticleRetention > 0 {
		cutoff := now.Add(-d.ArticleRetention)
		if n, err := d.Repo.DeleteArticlesOlderThan(ctx, cutoff); err != nil {
			d.Log.Warn("delete stale articles failed", "err", err)
		} else if n > 0 {
			d.Log.Info("articles_deleted", "kind", "ttl", "count", n,
				"cutoff", cutoff.Format(time.RFC3339))
		}
	}
}

func (d *DigestBuilder) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now().UTC()
}

// titleCase capitalises the first letter of `s`, leaving the rest as-is.
// Avoids `strings.Title` which is deprecated.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
