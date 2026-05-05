// Integration test for the HTTP API. Skips automatically unless a Postgres
// database is reachable via the DATABASE_URL env var. Run with:
//
//	DATABASE_URL=postgres://gamepulse:gamepulse@localhost:5432/gamepulse?sslmode=disable \
//	    go test ./internal/api/...
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/gamepulse/backend/internal/db"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/gamepulse/backend/internal/services"
)

func setupServer(t *testing.T) *Server {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Skipf("could not connect to DATABASE_URL: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })

	// Clean test data so re-runs are idempotent.
	for _, q := range []string{
		`DELETE FROM notifications_log`,
		`DELETE FROM subscriptions`,
		`DELETE FROM users WHERE phone_number LIKE '+15555%'`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("cleanup %s: %v", q, err)
		}
	}

	r := repo.New(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	smsSender := sms.NewLogSender(logger)
	subSvc := services.NewSubscriptionService(r, smsSender, logger)
	return New(r, subSvc, smsSender, logger)
}

func TestListTeams(t *testing.T) {
	s := setupServer(t)
	req, _ := http.NewRequest("GET", "/api/v1/teams", nil)
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body struct {
		Leagues []struct {
			League string `json:"league"`
			Teams  []struct {
				ID, Name, League, ExternalID string
			} `json:"teams"`
		} `json:"leagues"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Leagues) != 4 {
		t.Errorf("expected 4 leagues, got %d", len(body.Leagues))
	}
	want := []string{"NBA", "NFL", "MLB", "NHL"}
	for i, l := range body.Leagues {
		if l.League != want[i] {
			t.Errorf("league[%d] = %s, want %s", i, l.League, want[i])
		}
		if len(l.Teams) == 0 {
			t.Errorf("league %s has no teams", l.League)
		}
	}
}

func TestCreateSubscription_HappyPath(t *testing.T) {
	s := setupServer(t)
	teamID := firstTeamID(t, s)
	body := []byte(`{"phone_number":"+15555550101","team_id":"` + teamID + `","update_type":"both","frequency":"weekly"}`)

	req, _ := http.NewRequest("POST", "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body=%s", resp.StatusCode, string(raw))
	}

	// Duplicate should 409.
	req2, _ := http.NewRequest("POST", "/api/v1/subscriptions", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := s.App.Test(req2, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != 409 {
		t.Errorf("expected 409 on duplicate, got %d", resp2.StatusCode)
	}
}

func TestCreateSubscription_ValidationErrors(t *testing.T) {
	s := setupServer(t)
	teamID := firstTeamID(t, s)
	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad phone", `{"phone_number":"4155550123","team_id":"` + teamID + `","update_type":"both","frequency":"daily"}`, 400},
		{"bad uuid", `{"phone_number":"+15555550111","team_id":"not-a-uuid","update_type":"both","frequency":"daily"}`, 400},
		{"bad update_type", `{"phone_number":"+15555550112","team_id":"` + teamID + `","update_type":"foo","frequency":"daily"}`, 400},
		{"bad frequency", `{"phone_number":"+15555550113","team_id":"` + teamID + `","update_type":"both","frequency":"hourly"}`, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/api/v1/subscriptions", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := s.App.Test(req, -1)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.want {
				t.Errorf("got %d want %d", resp.StatusCode, tc.want)
			}
		})
	}
}

func firstTeamID(t *testing.T, s *Server) string {
	t.Helper()
	req, _ := http.NewRequest("GET", "/api/v1/teams", nil)
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Leagues []struct {
			Teams []struct {
				ID string `json:"id"`
			} `json:"teams"`
		} `json:"leagues"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body.Leagues[0].Teams[0].ID
}
