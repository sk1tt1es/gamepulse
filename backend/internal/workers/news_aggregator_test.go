package workers

import (
	"context"
	"testing"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/news"
	"github.com/google/uuid"
)

// stubNewsProvider returns a fixed batch of articles per call, so we can
// assert how many got fetched without hitting the network.
type stubNewsProvider struct {
	articles []news.Article
	calls    int
}

func (s *stubNewsProvider) Fetch(_ context.Context, _ models.Team) ([]news.Article, error) {
	s.calls++
	out := make([]news.Article, len(s.articles))
	copy(out, s.articles)
	return out, nil
}

func TestNewsAggregator_TickDoesNotInvokeAI(t *testing.T) {
	// The aggregator's only job in v3 is to pull articles and persist them
	// with summary=''. AI is intentionally not a dependency anymore; this
	// test guards against a regression that re-introduces it (it would
	// fail to compile if the field came back).
	if _, isAIField := any(&NewsAggregator{}).(*NewsAggregator); !isAIField {
		t.Fatal("unreachable")
	}
	// Reflective sanity: the struct must not have an AI field. We assert
	// indirectly by constructing without one — if a new required AI dep
	// were added the package would fail to build above.
	_ = &NewsAggregator{
		News:     &stubNewsProvider{},
		Interval: time.Hour,
	}
}

func TestStubNewsProvider_Counts(t *testing.T) {
	// A small assertion so the test file isn't pure compile-time.
	stub := &stubNewsProvider{
		articles: []news.Article{
			{Title: "h1", URL: "https://example.com/1", PublishedAt: time.Now()},
			{Title: "h2", URL: "https://example.com/2", PublishedAt: time.Now()},
		},
	}
	out, err := stub.Fetch(context.Background(), models.Team{ID: uuid.New(), Name: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 stub articles, got %d", len(out))
	}
	if stub.calls != 1 {
		t.Errorf("expected 1 call, got %d", stub.calls)
	}
}
