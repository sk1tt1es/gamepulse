// Package sports abstracts the sports data provider behind a small
// interface. The default mock implementation simulates a believable stream
// of in-progress games so the live tracker, dispatcher and SMS pipeline can
// be exercised end-to-end with no third-party API key.
package sports

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gamepulse/backend/internal/models"
)

// LiveGame is the shape returned by every provider implementation.
type LiveGame struct {
	ExternalID string
	Opponent   string
	Status     models.GameStatus
	StartTime  time.Time
	HomeScore  int
	AwayScore  int
	Period     string
	// LastScorer is optional. When set, the live tracker can include a
	// human-friendly attribution in the SMS notification.
	LastScorer string
}

type Provider interface {
	// LiveGames returns the (possibly empty) set of live games for a given
	// team, identified by its `external_id`.
	LiveGames(ctx context.Context, league models.League, teamExternalID string) ([]LiveGame, error)
}

// --- Mock provider -------------------------------------------------------

// MockProvider keeps an in-memory game state per (league, team_external_id)
// and advances scores on each call. Score deltas are randomised so the
// dispatcher emits realistic update bursts.
type MockProvider struct {
	mu    sync.Mutex
	games map[string]*LiveGame
	rng   *rand.Rand
}

func NewMockProvider() *MockProvider {
	return &MockProvider{
		games: make(map[string]*LiveGame),
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// opponentsByLeague gives the mock something believable to display in SMS
// messages. Keeping this list short is fine — variety isn't critical for
// demonstrating the workflow.
var opponentsByLeague = map[models.League][]string{
	models.LeagueNBA: {"Spurs", "Knicks", "Bulls", "Hawks", "Suns"},
	models.LeagueNFL: {"Steelers", "Patriots", "Ravens", "Bengals", "Vikings"},
	models.LeagueMLB: {"Mets", "Padres", "Giants", "Phillies", "Mariners"},
	models.LeagueNHL: {"Bruins", "Flames", "Jets", "Predators", "Capitals"},
}

var scorersByLeague = map[models.League][]string{
	models.LeagueNBA: {"LeBron James", "Stephen Curry", "Jayson Tatum", "Nikola Jokic", "Giannis Antetokounmpo"},
	models.LeagueNFL: {"Patrick Mahomes", "Josh Allen", "Lamar Jackson", "Jalen Hurts", "Christian McCaffrey"},
	models.LeagueMLB: {"Aaron Judge", "Mookie Betts", "Shohei Ohtani", "Juan Soto", "Freddie Freeman"},
	models.LeagueNHL: {"Connor McDavid", "Auston Matthews", "Nathan MacKinnon", "Leon Draisaitl", "David Pastrnak"},
}

func (m *MockProvider) LiveGames(_ context.Context, league models.League, teamID string) ([]LiveGame, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := string(league) + ":" + teamID
	g, ok := m.games[key]
	if !ok {
		opponents := opponentsByLeague[league]
		opponent := "Visitors"
		if len(opponents) > 0 {
			opponent = opponents[m.rng.Intn(len(opponents))]
		}
		g = &LiveGame{
			ExternalID: fmt.Sprintf("mock-%s-%s-%d", league, teamID, time.Now().Unix()),
			Opponent:   opponent,
			Status:     models.GameLive,
			StartTime:  time.Now().Add(-30 * time.Minute),
			Period:     periodForLeague(league, 1),
		}
		m.games[key] = g
		return []LiveGame{*g}, nil
	}

	// 50% chance of a score change per poll, biased toward the home team to
	// keep messages readable. We also nudge the period forward periodically.
	if m.rng.Intn(2) == 0 {
		scorers := scorersByLeague[league]
		scorer := ""
		if len(scorers) > 0 {
			scorer = scorers[m.rng.Intn(len(scorers))]
		}
		points := scoreIncrementForLeague(league, m.rng)
		if m.rng.Intn(2) == 0 {
			g.HomeScore += points
		} else {
			g.AwayScore += points
		}
		g.LastScorer = scorer
	} else {
		g.LastScorer = ""
	}

	if m.rng.Intn(8) == 0 {
		g.Period = periodForLeague(league, m.rng.Intn(4)+1)
	}
	if m.rng.Intn(40) == 0 {
		g.Status = models.GameFinished
	}

	return []LiveGame{*g}, nil
}

func periodForLeague(l models.League, n int) string {
	switch l {
	case models.LeagueNBA:
		return fmt.Sprintf("Q%d", n)
	case models.LeagueNFL:
		return fmt.Sprintf("Q%d", n)
	case models.LeagueNHL:
		return fmt.Sprintf("P%d", n)
	case models.LeagueMLB:
		return fmt.Sprintf("Inn %d", n)
	}
	return fmt.Sprintf("Period %d", n)
}

func scoreIncrementForLeague(l models.League, rng *rand.Rand) int {
	switch l {
	case models.LeagueNBA:
		// 2 or 3 points typically.
		return 2 + rng.Intn(2)
	case models.LeagueNFL:
		// Touchdowns (7) or field goals (3) — coarse simulation.
		if rng.Intn(2) == 0 {
			return 3
		}
		return 7
	case models.LeagueNHL, models.LeagueMLB:
		return 1
	}
	return 1
}
