package news

import (
	"context"
	"testing"

	"github.com/gamepulse/backend/internal/models"
	"github.com/google/uuid"
)

func TestMockProvider_RotatesHeadlines(t *testing.T) {
	m := NewMockProvider()
	team := models.Team{ID: uuid.New(), Name: "Lakers", League: models.LeagueNBA, ExternalID: "LAL"}

	titles := map[string]bool{}
	for i := 0; i < 5; i++ {
		arts, err := m.Fetch(context.Background(), team)
		if err != nil {
			t.Fatal(err)
		}
		if len(arts) != 1 {
			t.Fatalf("expected 1 article, got %d", len(arts))
		}
		titles[arts[0].Title] = true
	}
	if len(titles) < 2 {
		t.Errorf("expected rotation to surface multiple distinct titles, got %d", len(titles))
	}
}
