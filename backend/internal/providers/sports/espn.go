package sports

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gamepulse/backend/internal/models"
)

// ESPN is a Provider backed by ESPN's public, unauthenticated scoreboard
// endpoint. The endpoint returns every game for a league for "today" so we
// fetch once per league per TTL and serve every team request from the
// shared cache. This keeps us well under any reasonable rate limit even
// when many teams have subscribers.
//
//	GET https://site.api.espn.com/apis/site/v2/sports/{sport}/{league}/scoreboard
//
// The endpoint is undocumented but stable — it backs ESPN's own scoreboard
// pages. Treat it as best-effort: failures (DNS, 5xx, JSON drift) are
// returned to the live tracker, which logs and retries on the next tick.
type ESPN struct {
	HTTP *http.Client
	TTL  time.Duration

	mu    sync.Mutex
	cache map[models.League]espnEntry
}

type espnEntry struct {
	fetchedAt time.Time
	// games keyed by uppercase team abbreviation so lookups are O(1).
	games map[string][]LiveGame
}

func NewESPN() *ESPN {
	return &ESPN{
		HTTP:  &http.Client{Timeout: 8 * time.Second},
		TTL:   5 * time.Second,
		cache: map[models.League]espnEntry{},
	}
}

func (e *ESPN) LiveGames(ctx context.Context, league models.League, teamExternalID string) ([]LiveGame, error) {
	games, err := e.gamesForLeague(ctx, league)
	if err != nil {
		return nil, err
	}
	return games[strings.ToUpper(teamExternalID)], nil
}

// gamesForLeague returns the per-team game map for `league`, refreshing
// the cache when the TTL has elapsed.
func (e *ESPN) gamesForLeague(ctx context.Context, league models.League) (map[string][]LiveGame, error) {
	e.mu.Lock()
	if entry, ok := e.cache[league]; ok && time.Since(entry.fetchedAt) < e.TTL {
		e.mu.Unlock()
		return entry.games, nil
	}
	e.mu.Unlock()

	fresh, err := e.fetchLeague(ctx, league)
	if err != nil {
		// On error, surface the most recent cached snapshot if we have one
		// so transient ESPN hiccups don't cause SMS gaps. Otherwise return
		// the error so the caller can log it.
		e.mu.Lock()
		entry, ok := e.cache[league]
		e.mu.Unlock()
		if ok {
			return entry.games, nil
		}
		return nil, err
	}

	e.mu.Lock()
	e.cache[league] = espnEntry{fetchedAt: time.Now(), games: fresh}
	e.mu.Unlock()
	return fresh, nil
}

// scoreboardURL maps our League constants to the ESPN URL path.
func scoreboardURL(league models.League) (string, bool) {
	switch league {
	case models.LeagueNBA:
		return "https://site.api.espn.com/apis/site/v2/sports/basketball/nba/scoreboard", true
	case models.LeagueNFL:
		return "https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard", true
	case models.LeagueMLB:
		return "https://site.api.espn.com/apis/site/v2/sports/baseball/mlb/scoreboard", true
	case models.LeagueNHL:
		return "https://site.api.espn.com/apis/site/v2/sports/hockey/nhl/scoreboard", true
	}
	return "", false
}

// espnPayload covers just the fields we read; ESPN returns a much larger
// document and we ignore everything else to stay resilient to changes.
type espnPayload struct {
	Events []struct {
		ID           string `json:"id"`
		Date         string `json:"date"`
		Competitions []struct {
			Status struct {
				Type struct {
					Name        string `json:"name"`
					Completed   bool   `json:"completed"`
					Description string `json:"description"`
					ShortDetail string `json:"shortDetail"`
					State       string `json:"state"` // "pre","in","post"
				} `json:"type"`
				Period       int    `json:"period"`
				DisplayClock string `json:"displayClock"`
			} `json:"status"`
			Competitors []struct {
				HomeAway string `json:"homeAway"`
				Score    string `json:"score"`
				Team     struct {
					Abbreviation string `json:"abbreviation"`
					DisplayName  string `json:"displayName"`
				} `json:"team"`
			} `json:"competitors"`
		} `json:"competitions"`
	} `json:"events"`
}

func (e *ESPN) fetchLeague(ctx context.Context, league models.League) (map[string][]LiveGame, error) {
	url, ok := scoreboardURL(league)
	if !ok {
		return nil, fmt.Errorf("unsupported league: %s", league)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// Polite, identifiable UA helps if ESPN ever does basic filtering.
	req.Header.Set("User-Agent", "GamePulse/1.0 (+https://gamepulse.example)")
	req.Header.Set("Accept", "application/json")

	resp, err := e.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("espn non-2xx: %d", resp.StatusCode)
	}

	var body espnPayload
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	out := map[string][]LiveGame{}
	for _, ev := range body.Events {
		for _, comp := range ev.Competitions {
			home, away := splitCompetitors(comp.Competitors)
			if home == nil || away == nil {
				continue
			}
			homeScore := atoiSafe(home.Score)
			awayScore := atoiSafe(away.Score)
			status := mapStatus(comp.Status.Type.State, comp.Status.Type.Completed)
			period := pickPeriod(comp.Status.Type.ShortDetail, comp.Status.Type.Description, comp.Status.Period, league)
			start := parseStart(ev.Date)

			homeAbbr := strings.ToUpper(home.Team.Abbreviation)
			awayAbbr := strings.ToUpper(away.Team.Abbreviation)

			// Emit one LiveGame per side so each subscriber sees their
			// team as the "home_score" in the message regardless of which
			// way ESPN booked the matchup.
			homeGame := LiveGame{
				ExternalID: ev.ID,
				Opponent:   away.Team.DisplayName,
				Status:     status,
				StartTime:  start,
				HomeScore:  homeScore,
				AwayScore:  awayScore,
				Period:     period,
			}
			awayGame := LiveGame{
				ExternalID: ev.ID,
				Opponent:   home.Team.DisplayName,
				Status:     status,
				StartTime:  start,
				HomeScore:  awayScore,
				AwayScore:  homeScore,
				Period:     period,
			}
			out[homeAbbr] = append(out[homeAbbr], homeGame)
			out[awayAbbr] = append(out[awayAbbr], awayGame)
		}
	}
	return out, nil
}

// --- helpers -------------------------------------------------------------

type espnCompetitor struct {
	HomeAway string
	Score    string
	Team     struct {
		Abbreviation string
		DisplayName  string
	}
}

func splitCompetitors(in []struct {
	HomeAway string `json:"homeAway"`
	Score    string `json:"score"`
	Team     struct {
		Abbreviation string `json:"abbreviation"`
		DisplayName  string `json:"displayName"`
	} `json:"team"`
}) (home, away *espnCompetitor) {
	for _, c := range in {
		ec := espnCompetitor{HomeAway: c.HomeAway, Score: c.Score}
		ec.Team.Abbreviation = c.Team.Abbreviation
		ec.Team.DisplayName = c.Team.DisplayName
		switch c.HomeAway {
		case "home":
			h := ec
			home = &h
		case "away":
			a := ec
			away = &a
		}
	}
	return
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func mapStatus(state string, completed bool) models.GameStatus {
	if completed {
		return models.GameFinished
	}
	switch state {
	case "in":
		return models.GameLive
	case "post":
		return models.GameFinished
	default:
		return models.GameScheduled
	}
}

// pickPeriod prefers ESPN's "shortDetail" (e.g. "Q3 5:23", "Top 7th") which
// is human-readable, falling back to a synthesized period label so the SMS
// output looks reasonable even if ESPN trims the field.
func pickPeriod(shortDetail, description string, period int, league models.League) string {
	if s := strings.TrimSpace(shortDetail); s != "" {
		return s
	}
	if s := strings.TrimSpace(description); s != "" {
		return s
	}
	if period <= 0 {
		return ""
	}
	switch league {
	case models.LeagueNBA, models.LeagueNFL:
		return fmt.Sprintf("Q%d", period)
	case models.LeagueNHL:
		return fmt.Sprintf("P%d", period)
	case models.LeagueMLB:
		return fmt.Sprintf("Inn %d", period)
	}
	return fmt.Sprintf("Period %d", period)
}

func parseStart(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
