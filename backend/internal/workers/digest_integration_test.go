// Integration tests for the news → digest → cleanup pipeline. Auto-skipped
// unless DATABASE_URL points at a reachable Postgres (same convention as
// internal/api/api_integration_test.go).
package workers

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gamepulse/backend/internal/db"
	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/ai"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupRepo(t *testing.T) (*repo.Repo, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Skipf("could not connect: %v", err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	for _, q := range []string{
		`DELETE FROM subscription_article_sent`,
		`DELETE FROM news_articles`,
		`DELETE FROM notifications_log`,
		`DELETE FROM subscriptions`,
		`DELETE FROM users WHERE phone_number LIKE '+15556%'`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("cleanup %s: %v", q, err)
		}
	}
	return repo.New(pool), pool
}

// makeSub creates a subscription on the first available team.
func makeSub(t *testing.T, r *repo.Repo, phone string, ut models.UpdateType, fr models.Frequency, initialDelay time.Duration) (models.SubscriptionDetail, models.Team) {
	t.Helper()
	ctx := context.Background()
	user, err := r.UpsertUserByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	teams, err := r.ListTeams(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(teams) == 0 {
		t.Fatal("no teams seeded")
	}
	team := teams[0]
	now := time.Now().UTC()
	sub := &models.Subscription{
		UserID: user.ID, TeamID: team.ID,
		UpdateType: ut, Frequency: fr,
		CreatedAt: now,
	}
	if ut == models.UpdateLive {
		sub.InitialNewsSent = true
	} else {
		nb := now.Add(initialDelay)
		sub.InitialNewsNotBefore = &nb
	}
	if err := r.CreateSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	return models.SubscriptionDetail{
		Subscription: *sub,
		PhoneNumber:  phone,
		TeamName:     team.Name,
		League:       team.League,
	}, team
}

func TestDigestIntegration_InitialSendAndConsumption(t *testing.T) {
	r, _ := setupRepo(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sender := sms.NewLogSender(logger)

	// Same team, two news subs with different cadences. Article must
	// remain until BOTH have consumed it.
	subA, team := makeSub(t, r, "+15556000001", models.UpdateBoth, models.FrequencyDaily, -1*time.Minute)
	subB, _ := makeSub(t, r, "+15556000002", models.UpdateNews, models.FrequencyWeekly, -1*time.Minute)

	// Insert one article; published a hair after both subs were created
	// so they're both "eligible" by the consumed-article check.
	pub := time.Now().UTC().Add(time.Second)
	art := &models.NewsArticle{
		TeamID: team.ID, Title: "Roster Move", Content: "Trade announced.",
		Source: "test", URL: "https://example.com/test/" + uuid.NewString(),
		PublishedAt: pub,
	}
	if _, err := r.InsertArticle(ctx, art); err != nil {
		t.Fatal(err)
	}

	d := &DigestBuilder{
		Repo: r, SMS: sender, AI: ai.HeuristicSummarizer{}, Log: logger,
		Now: func() time.Time { return time.Now().UTC() },
	}

	// First tick: both initial sends should fire (cooldown is in the past).
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := len(sender.Sent()); got != 2 {
		t.Fatalf("expected 2 SMS after initial tick, got %d", got)
	}
	for _, s := range sender.Sent() {
		if !strings.Contains(s.Body, "Welcome") {
			t.Errorf("expected welcome header, got %q", s.Body)
		}
	}

	// Both subs consumed the article and housekeeping ran in the same
	// tick — the "fully consumed" delete should have removed it.
	remaining, err := r.RecentArticlesForTeam(ctx, team.ID, time.Now().UTC().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected article to be deleted after both subs consumed, got %d remaining", len(remaining))
	}

	// Second tick should be a no-op (initial flag flipped, no new articles).
	beforeCount := len(sender.Sent())
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.Sent()) != beforeCount {
		t.Errorf("expected no extra sends on idempotent tick, got %d new", len(sender.Sent())-beforeCount)
	}

	_ = subA
	_ = subB
}

func TestDigestIntegration_SharedArticleSurvivesUntilBothConsume(t *testing.T) {
	r, _ := setupRepo(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sender := sms.NewLogSender(logger)

	// Sub A has elapsed cooldown; sub B's cooldown is still in the future.
	// After tick 1: only A consumes; article must survive for B.
	_, team := makeSub(t, r, "+15556000003", models.UpdateNews, models.FrequencyDaily, -1*time.Minute)
	_, _ = makeSub(t, r, "+15556000004", models.UpdateNews, models.FrequencyDaily, 1*time.Hour)

	pub := time.Now().UTC().Add(time.Second)
	art := &models.NewsArticle{
		TeamID: team.ID, Title: "Big News", Content: "Stuff happened.",
		Source: "test", URL: "https://example.com/test/" + uuid.NewString(),
		PublishedAt: pub,
	}
	if _, err := r.InsertArticle(ctx, art); err != nil {
		t.Fatal(err)
	}

	d := &DigestBuilder{
		Repo: r, SMS: sender, AI: ai.HeuristicSummarizer{}, Log: logger,
		Now: func() time.Time { return time.Now().UTC() },
	}
	if err := d.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if got := len(sender.Sent()); got != 1 {
		t.Fatalf("expected exactly 1 initial SMS (only A is due), got %d", got)
	}

	remaining, err := r.RecentArticlesForTeam(ctx, team.ID, time.Now().UTC().Add(-1*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Errorf("expected article to survive (B hasn't consumed), got %d remaining", len(remaining))
	}
}

func TestRepo_DeleteFinishedGamesBefore(t *testing.T) {
	r, pool := setupRepo(t)
	ctx := context.Background()

	teams, _ := r.ListTeams(ctx)
	if len(teams) == 0 {
		t.Fatal("no teams seeded")
	}
	team := teams[0]
	now := time.Now().UTC()

	old := &models.Game{
		TeamID: team.ID, Opponent: "Old Opp",
		Status: models.GameFinished, StartTime: now.Add(-48 * time.Hour),
		ExternalID: "old-" + uuid.NewString(),
	}
	recent := &models.Game{
		TeamID: team.ID, Opponent: "New Opp",
		Status: models.GameFinished, StartTime: now.Add(-1 * time.Hour),
		ExternalID: "new-" + uuid.NewString(),
	}
	scheduled := &models.Game{
		TeamID: team.ID, Opponent: "Sched Opp",
		Status: models.GameScheduled, StartTime: now.Add(-72 * time.Hour),
		ExternalID: "sched-" + uuid.NewString(),
	}
	for _, g := range []*models.Game{old, recent, scheduled} {
		if _, _, _, err := r.UpsertGame(ctx, g); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := r.DeleteFinishedGamesBefore(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deletion (only the 48h-old finished game), got %d", deleted)
	}

	// Recent finished + scheduled should remain.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM games WHERE team_id=$1 AND external_id IN ($2,$3,$4)`,
		team.ID, old.ExternalID, recent.ExternalID, scheduled.ExternalID,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 surviving rows, got %d", count)
	}
}
