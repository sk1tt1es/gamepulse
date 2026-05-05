package workers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/ai"
	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// flakyAI returns a fixed summary unless the title hits the trigger word,
// in which case it errors so we can exercise the fallback path. An empty
// trigger means "never error" (we explicitly avoid the
// strings.Contains-with-empty-string trap).
type flakyAI struct{ trigger string }

func (f flakyAI) Summarize(_ context.Context, title, _ string) (string, error) {
	if f.trigger != "" && strings.Contains(title, f.trigger) {
		return "", errors.New("backend down")
	}
	return "AI:" + title, nil
}

func TestBuildNewsDigestBody_Initial_NoArticles(t *testing.T) {
	d := &DigestBuilder{Log: discardLogger(), AI: ai.HeuristicSummarizer{}}
	sub := models.SubscriptionDetail{
		Subscription: models.Subscription{Frequency: models.FrequencyDaily},
		TeamName:     "Lakers",
	}

	body, included, err := d.BuildNewsDigestBody(context.Background(), sub, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(included) != 0 {
		t.Fatalf("expected zero included, got %d", len(included))
	}
	if !strings.Contains(body, "Welcome") || !strings.Contains(body, "Lakers") {
		t.Errorf("expected welcome header, got %q", body)
	}
}

func TestBuildNewsDigestBody_RecurringHeader(t *testing.T) {
	d := &DigestBuilder{Log: discardLogger(), AI: flakyAI{}}
	sub := models.SubscriptionDetail{
		Subscription: models.Subscription{Frequency: models.FrequencyWeekly},
		TeamName:     "Lakers",
	}
	arts := []models.NewsArticle{
		{ID: uuid.New(), Title: "AD returns next week", Content: "He'll suit up Tuesday."},
		{ID: uuid.New(), Title: "LeBron rests", Content: "Veteran night off."},
	}
	body, included, err := d.BuildNewsDigestBody(context.Background(), sub, arts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(included) != 2 {
		t.Errorf("expected both articles included, got %d", len(included))
	}
	if !strings.HasPrefix(body, "Weekly news digest for Lakers:") {
		t.Errorf("unexpected prefix: %q", body)
	}
	if !strings.Contains(body, "• AI:AD returns next week") {
		t.Errorf("expected AI bullet, got %q", body)
	}
}

func TestBuildNewsDigestBody_AISummarizeFailureFallsBack(t *testing.T) {
	d := &DigestBuilder{Log: discardLogger(), AI: flakyAI{trigger: "BOOM"}}
	sub := models.SubscriptionDetail{
		Subscription: models.Subscription{Frequency: models.FrequencyDaily},
		TeamName:     "Lakers",
	}
	arts := []models.NewsArticle{
		{ID: uuid.New(), Title: "BOOM headline", Content: "Some recoverable content here."},
	}
	body, included, err := d.BuildNewsDigestBody(context.Background(), sub, arts, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(included) != 1 {
		t.Fatalf("expected article still included on AI failure, got %d", len(included))
	}
	if !strings.Contains(body, "Some recoverable content here.") {
		t.Errorf("expected truncated content fallback, got %q", body)
	}
}

func TestBuildNewsDigestBody_RespectsMaxBodyChars(t *testing.T) {
	d := &DigestBuilder{Log: discardLogger(), AI: ai.HeuristicSummarizer{}, MaxBodyChars: 80}
	sub := models.SubscriptionDetail{
		Subscription: models.Subscription{Frequency: models.FrequencyDaily},
		TeamName:     "Lakers",
	}
	long := strings.Repeat("word ", 40)
	arts := []models.NewsArticle{
		{ID: uuid.New(), Title: "First", Content: long},
		{ID: uuid.New(), Title: "Second", Content: long},
		{ID: uuid.New(), Title: "Third", Content: long},
	}
	body, included, err := d.BuildNewsDigestBody(context.Background(), sub, arts, false)
	if err != nil {
		t.Fatal(err)
	}
	// At least one article must always be included so users never get
	// just a header line.
	if len(included) == 0 {
		t.Fatalf("expected at least one article included")
	}
	if len(body) > 80 {
		t.Errorf("body length %d exceeded cap 80: %q", len(body), body)
	}
}

func TestTitleCase(t *testing.T) {
	cases := map[string]string{"": "", "weekly": "Weekly", "DAILY": "DAILY"}
	for in, want := range cases {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFrequencyWindow(t *testing.T) {
	cases := map[models.Frequency]string{
		models.FrequencyDaily:   "24h0m0s",
		models.FrequencyWeekly:  "168h0m0s",
		models.FrequencyMonthly: "720h0m0s",
	}
	for f, want := range cases {
		if got := f.Window().String(); got != want {
			t.Errorf("%s.Window() = %s, want %s", f, got, want)
		}
	}
}

func TestDigestBuilder_NowFallback(t *testing.T) {
	d := &DigestBuilder{}
	got := d.now()
	if time.Since(got) > time.Second {
		t.Errorf("expected now() ~= time.Now(), got %v", got)
	}
}
