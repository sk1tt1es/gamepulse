package workers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/ai"
	"github.com/gamepulse/backend/internal/providers/news"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/gamepulse/backend/internal/services"
)

// NewsAggregator periodically pulls articles for each team that has news
// subscribers, summarizes them via the AI backend, persists them, and
// dispatches the summary to subscribers (respecting frequency).
type NewsAggregator struct {
	Repo       *repo.Repo
	News       news.Provider
	AI         ai.Summarizer
	Dispatcher *services.Dispatcher
	Log        *slog.Logger
	Interval   time.Duration
}

func (a *NewsAggregator) Run(ctx context.Context) {
	if a.Interval == 0 {
		a.Interval = 30 * time.Minute
	}
	tk := time.NewTicker(a.Interval)
	defer tk.Stop()

	a.Log.Info("news aggregator started", "interval", a.Interval.String())
	// Run once at startup so the system has fresh data.
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

func (a *NewsAggregator) Tick(ctx context.Context) error {
	teams, err := a.Repo.ListTeams(ctx)
	if err != nil {
		return err
	}

	for _, team := range teams {
		subs, err := a.Repo.SubscriptionsForTeam(ctx, team.ID, models.UpdateNews, models.UpdateBoth)
		if err != nil {
			a.Log.Warn("subs lookup failed", "err", err, "team", team.ID)
			continue
		}
		if len(subs) == 0 {
			continue
		}
		articles, err := a.News.Fetch(ctx, team)
		if err != nil {
			a.Log.Warn("news fetch failed", "err", err, "team", team.ID)
			continue
		}
		for _, art := range articles {
			summary, err := a.AI.Summarize(ctx, art.Title, art.Content)
			if err != nil {
				a.Log.Warn("summarize failed", "err", err, "team", team.ID)
				summary = ai.Truncate(art.Content)
			}
			rec := &models.NewsArticle{
				TeamID:      team.ID,
				Title:       art.Title,
				Content:     art.Content,
				Source:      art.Source,
				URL:         art.URL,
				PublishedAt: art.PublishedAt,
				Summary:     summary,
			}
			inserted, err := a.Repo.InsertArticle(ctx, rec)
			if err != nil {
				a.Log.Warn("insert article failed", "err", err)
				continue
			}
			if !inserted {
				// We've seen this URL already — skip to avoid spam.
				continue
			}

			body := fmt.Sprintf("%s News: %s", team.Name, summary)
			dedupe := fmt.Sprintf("news:%s", rec.ID.String())
			a.Dispatcher.FanOut(ctx, subs, models.MessageNews, body, dedupe)
		}
	}
	return nil
}
