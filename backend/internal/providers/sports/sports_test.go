package sports

import (
	"context"
	"testing"

	"github.com/gamepulse/backend/internal/models"
)

func TestMockProvider_AdvancesScore(t *testing.T) {
	m := NewMockProvider()
	ctx := context.Background()

	games, err := m.LiveGames(ctx, models.LeagueNBA, "LAL")
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(games))
	}
	g0 := games[0]
	if g0.Status != models.GameLive {
		t.Errorf("expected initial status live, got %s", g0.Status)
	}

	// After many ticks at least one score change should have occurred.
	gotChange := false
	for i := 0; i < 50; i++ {
		gs, err := m.LiveGames(ctx, models.LeagueNBA, "LAL")
		if err != nil {
			t.Fatal(err)
		}
		if gs[0].HomeScore+gs[0].AwayScore > 0 {
			gotChange = true
			break
		}
	}
	if !gotChange {
		t.Errorf("expected at least one score change after 50 ticks")
	}
}

func TestMockProvider_PerLeague(t *testing.T) {
	m := NewMockProvider()
	for _, l := range models.Leagues() {
		gs, err := m.LiveGames(context.Background(), l, "TEAM")
		if err != nil {
			t.Fatalf("league %s: %v", l, err)
		}
		if len(gs) == 0 || gs[0].Period == "" {
			t.Errorf("league %s: missing period", l)
		}
	}
}
