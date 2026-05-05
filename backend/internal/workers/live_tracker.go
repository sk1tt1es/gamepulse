// Package workers contains the background goroutines that drive GamePulse:
// live game polling, news aggregation, and digest delivery. Each worker is
// independently startable and accepts a context for clean shutdown.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/sports"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/gamepulse/backend/internal/services"
)

// LiveTracker polls the sports provider for every team that has at least one
// live-update subscriber, persists the latest game state, and dispatches a
// notification when the score changes.
//
// It also runs a small housekeeping pass at the end of every tick to delete
// finished games (and their cascading game_events) older than
// FinishedGameRetention, so the games table doesn't grow unbounded.
type LiveTracker struct {
	Repo       *repo.Repo
	Sports     sports.Provider
	Dispatcher *services.Dispatcher
	Log        *slog.Logger
	Interval   time.Duration
	// FinishedGameRetention is how long a finished game stays in the DB
	// before it (and its game_events) are deleted. Zero disables
	// housekeeping; useful in tests.
	FinishedGameRetention time.Duration
	// Now allows tests to inject a fixed clock for housekeeping.
	Now func() time.Time
}

func (t *LiveTracker) Run(ctx context.Context) {
	if t.Interval == 0 {
		t.Interval = 8 * time.Second
	}
	tk := time.NewTicker(t.Interval)
	defer tk.Stop()

	t.Log.Info("live tracker started", "interval", t.Interval.String())
	for {
		select {
		case <-ctx.Done():
			t.Log.Info("live tracker stopped")
			return
		case <-tk.C:
			if err := t.Tick(ctx); err != nil {
				t.Log.Warn("live tracker tick failed", "err", err)
			}
		}
	}
}

// Tick is exported so tests can drive the worker deterministically.
func (t *LiveTracker) Tick(ctx context.Context) error {
	teams, err := t.Repo.ListTeams(ctx)
	if err != nil {
		return err
	}

	for _, team := range teams {
		// Skip teams nobody subscribed to — saves API quota.
		subs, err := t.Repo.SubscriptionsForTeam(ctx, team.ID, models.UpdateLive, models.UpdateBoth)
		if err != nil {
			t.Log.Warn("subs lookup failed", "err", err, "team", team.ID)
			continue
		}
		if len(subs) == 0 {
			continue
		}

		live, err := t.Sports.LiveGames(ctx, team.League, team.ExternalID)
		if err != nil {
			t.Log.Warn("sports fetch failed", "err", err, "team", team.ID)
			continue
		}
		for _, g := range live {
			t.processGame(ctx, team, g, subs)
		}
	}

	t.housekeep(ctx)
	return nil
}

// housekeep deletes finished games older than the retention window. We
// run it at the end of every Tick rather than in a separate goroutine so
// there's only one source of writes to the games table from this worker.
func (t *LiveTracker) housekeep(ctx context.Context) {
	if t.FinishedGameRetention <= 0 {
		return
	}
	now := time.Now().UTC()
	if t.Now != nil {
		now = t.Now()
	}
	cutoff := now.Add(-t.FinishedGameRetention)
	n, err := t.Repo.DeleteFinishedGamesBefore(ctx, cutoff)
	if err != nil {
		t.Log.Warn("delete finished games failed", "err", err)
		return
	}
	if n > 0 {
		t.Log.Info("games_deleted", "count", n,
			"cutoff", cutoff.Format(time.RFC3339))
	}
}

func (t *LiveTracker) processGame(ctx context.Context, team models.Team, g sports.LiveGame, subs []models.SubscriptionDetail) {
	game := &models.Game{
		TeamID:     team.ID,
		Opponent:   g.Opponent,
		Status:     g.Status,
		StartTime:  g.StartTime,
		ExternalID: g.ExternalID,
		HomeScore:  g.HomeScore,
		AwayScore:  g.AwayScore,
		Period:     g.Period,
	}
	prevHome, prevAway, prevStatus, err := t.Repo.UpsertGame(ctx, game)
	if err != nil {
		t.Log.Warn("game upsert failed", "err", err, "external", g.ExternalID)
		return
	}

	// Skip if nothing changed since the last poll.
	if prevHome == g.HomeScore && prevAway == g.AwayScore && string(prevStatus) == string(g.Status) {
		return
	}

	payload, _ := json.Marshal(map[string]any{
		"home_score":  g.HomeScore,
		"away_score":  g.AwayScore,
		"period":      g.Period,
		"status":      g.Status,
		"last_scorer": g.LastScorer,
	})
	_ = t.Repo.RecordGameEvent(ctx, &models.GameEvent{
		GameID:    game.ID,
		EventType: "score_update",
		Payload:   payload,
	})

	body := formatLiveMessage(team, g)
	dedupe := fmt.Sprintf("live:%s:%d-%d:%s", game.ID.String(), g.HomeScore, g.AwayScore, g.Period)
	t.Dispatcher.FanOut(ctx, subs, models.MessageLiveScore, body, dedupe)
}

// formatLiveMessage produces the SMS-friendly score line. We keep it short
// enough that even with the team name + period prefix we stay well within
// a single SMS segment.
func formatLiveMessage(team models.Team, g sports.LiveGame) string {
	line := fmt.Sprintf("%s Update: %s — %s %d, %s %d",
		team.Name, g.Period, team.Name, g.HomeScore, g.Opponent, g.AwayScore)
	if g.LastScorer != "" {
		line += fmt.Sprintf(" (%s scored)", g.LastScorer)
	}
	if g.Status == models.GameFinished {
		line += " — FINAL"
	}
	return line
}
