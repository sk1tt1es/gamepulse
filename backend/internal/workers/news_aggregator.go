package workers

import (
	"context"
	"log/slog"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/news"
	"github.com/gamepulse/backend/internal/repo"
)

// NewsAggregator periodically pulls articles for each team that has at
// least one news / both subscriber and stores them in news_articles.
//
// As of v3 this worker does NOT summarize and does NOT dispatch. It only
// fills the article cache. Summarization happens at SEND time inside the
// digest builder so we only pay the LLM cost for content that's actually
// going out, and the same article can serve multiple subscribers (with
// different cadences) from a single insert.
type NewsAggregator struct {
	Repo     *repo.Repo
	News     news.Provider
	Log      *slog.Logger
	Interval time.Duration
}

func (a *NewsAggregator) Run(ctx context.Context) {
	if a.Interval == 0 {
		a.Interval = 6 * time.Hour
	}
	tk := time.NewTicker(a.Interval)
	defer tk.Stop()

	a.Log.Info("news aggregator started", "interval", a.Interval.String())
	if err := a.Tick(ctx); err != nil {
		a.Log.Warn("news tick failed", "err", err)
	}
	for {
		select {
		case <-ctx.Done():
			a.Log.Info("news aggregator stopped")
			return
		case <-tk.C:
			if err := a.Tick(ctx); err != nil {
				a.Log.Warn("news tick failed", "err", err)
			}
		}
	}
}

// Tick fetches articles for every team with at least one news / both
// subscriber and inserts them with summary='' (the schema permits it).
// Duplicate URLs are skipped via the (team_id, url) unique index.
func (a *NewsAggregator) Tick(ctx context.Context) error {
	teams, err := a.Repo.ListTeams(ctx)
	if err != nil {
		return err
	}

	var teamsScanned, fetched, inserted int
	for _, team := range teams {
		subs, err := a.Repo.SubscriptionsForTeam(ctx, team.ID, models.UpdateNews, models.UpdateBoth)
		if err != nil {
			a.Log.Warn("subs lookup failed", "err", err, "team", team.ID)
			continue
		}
		if len(subs) == 0 {
			continue
		}
		teamsScanned++
		n, err := a.fetchAndStore(ctx, team)
		if err != nil {
			a.Log.Warn("news fetch failed", "err", err, "team", team.ID)
			continue
		}
		fetched += n.fetched
		inserted += n.inserted
	}
	a.Log.Info("news aggregator tick",
		"teams_scanned", teamsScanned,
		"articles_fetched", fetched,
		"articles_inserted", inserted)
	return nil
}

// FetchOnDemand pulls + stores fresh articles for a single team and
// returns the count inserted. Used by the digest builder when an
// initial-news send finds the cache empty for a team.
func (a *NewsAggregator) FetchOnDemand(ctx context.Context, team models.Team) (int, error) {
	c, err := a.fetchAndStore(ctx, team)
	if err != nil {
		return 0, err
	}
	return c.inserted, nil
}

type fetchCounts struct{ fetched, inserted int }

func (a *NewsAggregator) fetchAndStore(ctx context.Context, team models.Team) (fetchCounts, error) {
	articles, err := a.News.Fetch(ctx, team)
	if err != nil {
		return fetchCounts{}, err
	}
	out := fetchCounts{fetched: len(articles)}
	for _, art := range articles {
		rec := &models.NewsArticle{
			TeamID:      team.ID,
			Title:       art.Title,
			Content:     art.Content,
			Source:      art.Source,
			URL:         art.URL,
			PublishedAt: art.PublishedAt,
			// Summary deliberately blank — populated lazily at send time.
			Summary: "",
		}
		ins, err := a.Repo.InsertArticle(ctx, rec)
		if err != nil {
			a.Log.Warn("insert article failed", "err", err, "team", team.ID)
			continue
		}
		if ins {
			out.inserted++
		}
	}
	return out, nil
}
