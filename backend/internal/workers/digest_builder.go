package workers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/repo"
)

// DigestBuilder rolls up "pending" news notifications into a single SMS at
// the cadence each subscriber chose: daily (every 24h), weekly (every 7d),
// or monthly (every 30d).
//
// We use the notifications_log table itself as a queue: the dispatcher
// appends rows with message_type = 'pending', and this worker drains them
// when enough of the cadence window has elapsed since the subscriber's
// last digest. That keeps the data model small (no separate queue table)
// while making it trivial to audit what was sent and when.
type DigestBuilder struct {
	Repo     *repo.Repo
	SMS      sms.Sender
	Log      *slog.Logger
	Interval time.Duration
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

// Tick processes every supported frequency in one pass. We iterate the
// frequencies dynamically (via models.Frequencies) so adding a new cadence
// only requires a constant update.
func (d *DigestBuilder) Tick(ctx context.Context) error {
	now := d.Now()
	for _, f := range models.Frequencies() {
		if err := d.flushFreq(ctx, f, now); err != nil {
			return err
		}
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
		// Live-only subscribers store a frequency but never receive digests
		// (no news notifications are queued for them). Skip them defensively.
		if s.UpdateType == models.UpdateLive {
			continue
		}

		last, err := d.Repo.LastDigestSentAt(ctx, s.ID)
		if err != nil {
			d.Log.Warn("last-sent lookup failed", "err", err, "sub", s.ID)
			continue
		}
		// Honor the cadence: don't send a daily digest twice in 24h, etc.
		if !last.IsZero() && now.Sub(last) < window {
			continue
		}

		// Pull every "pending" row since the last digest (or the full
		// window if there's never been one). This naturally handles the
		// first-ever digest for a subscriber.
		since := last
		if since.IsZero() {
			since = now.Add(-window)
		}
		lines, err := d.Repo.PendingForDigest(ctx, s.ID, since)
		if err != nil {
			d.Log.Warn("pending lookup failed", "err", err, "sub", s.ID)
			continue
		}
		if len(lines) == 0 {
			continue
		}

		body := buildDigestBody(s, lines)
		if len(body) > 1200 {
			// Cap to a reasonable upper bound. Most carriers concatenate
			// multi-segment SMS, but runaway messages help no one.
			body = body[:1200] + "…"
		}
		// Dedupe key includes only the cadence boundary, so rapid tick
		// repeats can't double-send within a window.
		dedupe := fmt.Sprintf("digest:%s:%d", freq, now.Unix()/int64(window.Seconds()))
		inserted, err := d.Repo.LogNotification(ctx, &models.NotificationLog{
			SubscriptionID: s.ID,
			MessageType:    models.MessageDigest,
			Content:        body,
			DedupeKey:      dedupe,
		})
		if err != nil {
			d.Log.Warn("digest log failed", "err", err)
			continue
		}
		if !inserted {
			// Already sent for this window — guard against double-runs.
			continue
		}
		if err := d.SMS.Send(ctx, s.PhoneNumber, body); err != nil {
			d.Log.Warn("digest send failed", "err", err, "sub", s.ID)
			continue
		}
		if err := d.Repo.ClearPending(ctx, s.ID, now); err != nil {
			d.Log.Warn("clear pending failed", "err", err, "sub", s.ID)
		}
	}
	return nil
}

// titleCase capitalises the first letter of `s`, leaving the rest as-is.
// Avoids `strings.Title` which is deprecated.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func buildDigestBody(s models.SubscriptionDetail, lines []string) string {
	var b strings.Builder
	freqLabel := titleCase(string(s.Frequency))
	fmt.Fprintf(&b, "%s news digest for %s:\n", freqLabel, s.TeamName)
	for i, line := range lines {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("• ")
		b.WriteString(line)
	}
	return b.String()
}
