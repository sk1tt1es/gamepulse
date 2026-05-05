package sports

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gamepulse/backend/internal/models"
)

const sampleNBAResponse = `{
  "events": [
    {
      "id": "401584829",
      "date": "2024-02-01T03:00Z",
      "competitions": [{
        "status": {
          "type": {"name":"STATUS_IN_PROGRESS","completed":false,"description":"3rd Quarter","shortDetail":"Q3 5:23","state":"in"},
          "period": 3,
          "displayClock": "5:23"
        },
        "competitors": [
          {"homeAway":"home","score":"78","team":{"abbreviation":"LAL","displayName":"Los Angeles Lakers"}},
          {"homeAway":"away","score":"75","team":{"abbreviation":"BOS","displayName":"Boston Celtics"}}
        ]
      }]
    },
    {
      "id": "401584830",
      "date": "2024-02-01T03:30Z",
      "competitions": [{
        "status": {
          "type": {"name":"STATUS_FINAL","completed":true,"description":"Final","shortDetail":"Final","state":"post"},
          "period": 4
        },
        "competitors": [
          {"homeAway":"home","score":"112","team":{"abbreviation":"GS","displayName":"Golden State Warriors"}},
          {"homeAway":"away","score":"108","team":{"abbreviation":"PHX","displayName":"Phoenix Suns"}}
        ]
      }]
    }
  ]
}`

func newESPNWithStub(t *testing.T, body string, hits *int32) *ESPN {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			atomic.AddInt32(hits, 1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	e := NewESPN()
	e.HTTP = srv.Client()
	// Override the URL function via a wrapper: easiest path is to swap
	// the package-level URL by re-pointing through a shim. We do this
	// by overriding the cache key path indirectly — simplest is to
	// monkey-patch via a custom RoundTripper that rewrites the host.
	e.HTTP.Transport = rewriteHost{srv.URL, http.DefaultTransport}
	return e
}

// rewriteHost is a tiny RoundTripper that redirects every outbound
// request to the test server, regardless of the URL the client built.
type rewriteHost struct {
	target string
	next   http.RoundTripper
}

func (r rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace scheme + host with the stub's; keep path so the handler
	// could differentiate per-league if it wanted to.
	u := req.URL
	u.Scheme = "http"
	u.Host = strings.TrimPrefix(r.target, "http://")
	return r.next.RoundTrip(req)
}

func TestESPN_LiveGames(t *testing.T) {
	e := newESPNWithStub(t, sampleNBAResponse, nil)
	ctx := context.Background()

	lal, err := e.LiveGames(ctx, models.LeagueNBA, "LAL")
	if err != nil {
		t.Fatal(err)
	}
	if len(lal) != 1 {
		t.Fatalf("expected 1 LAL game, got %d", len(lal))
	}
	g := lal[0]
	if g.HomeScore != 78 || g.AwayScore != 75 {
		t.Errorf("LAL scores wrong: home=%d away=%d", g.HomeScore, g.AwayScore)
	}
	if g.Opponent != "Boston Celtics" {
		t.Errorf("opponent wrong: %q", g.Opponent)
	}
	if g.Status != models.GameLive {
		t.Errorf("status wrong: %s", g.Status)
	}
	if g.Period != "Q3 5:23" {
		t.Errorf("period wrong: %q", g.Period)
	}

	// The same event is mirrored for the away team with scores flipped so
	// each subscriber sees their team as "home" in the SMS.
	bos, err := e.LiveGames(ctx, models.LeagueNBA, "BOS")
	if err != nil {
		t.Fatal(err)
	}
	if bos[0].HomeScore != 75 || bos[0].AwayScore != 78 {
		t.Errorf("BOS perspective wrong: home=%d away=%d", bos[0].HomeScore, bos[0].AwayScore)
	}
	if bos[0].Opponent != "Los Angeles Lakers" {
		t.Errorf("BOS opponent wrong: %q", bos[0].Opponent)
	}

	// Finished game should map status correctly.
	gs, err := e.LiveGames(ctx, models.LeagueNBA, "GS")
	if err != nil {
		t.Fatal(err)
	}
	if gs[0].Status != models.GameFinished {
		t.Errorf("expected finished, got %s", gs[0].Status)
	}
}

func TestESPN_CachesPerLeague(t *testing.T) {
	var hits int32
	e := newESPNWithStub(t, sampleNBAResponse, &hits)
	e.TTL = 50 * time.Millisecond
	ctx := context.Background()

	// Three lookups within TTL → one HTTP call.
	for _, abbr := range []string{"LAL", "BOS", "GS"} {
		if _, err := e.LiveGames(ctx, models.LeagueNBA, abbr); err != nil {
			t.Fatal(err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected 1 ESPN call, got %d", got)
	}

	time.Sleep(60 * time.Millisecond)
	if _, err := e.LiveGames(ctx, models.LeagueNBA, "LAL"); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("expected 2 ESPN calls after TTL, got %d", got)
	}
}

func TestESPN_UnknownTeamReturnsEmpty(t *testing.T) {
	e := newESPNWithStub(t, sampleNBAResponse, nil)
	games, err := e.LiveGames(context.Background(), models.LeagueNBA, "ZZZ")
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 0 {
		t.Errorf("expected no games for unknown team, got %d", len(games))
	}
}

func TestESPN_FallsBackToCacheOnError(t *testing.T) {
	// Stub that succeeds first, then fails.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n > 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleNBAResponse))
	}))
	t.Cleanup(srv.Close)

	e := NewESPN()
	e.HTTP = srv.Client()
	e.HTTP.Transport = rewriteHost{srv.URL, http.DefaultTransport}
	e.TTL = 1 * time.Millisecond

	ctx := context.Background()
	if _, err := e.LiveGames(ctx, models.LeagueNBA, "LAL"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond) // expire cache so next call refetches

	// Refetch fails, but we should get a non-error result populated from
	// the prior successful cache snapshot.
	games, err := e.LiveGames(ctx, models.LeagueNBA, "LAL")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if len(games) != 1 {
		t.Errorf("expected stale cache result, got %d games", len(games))
	}
}
