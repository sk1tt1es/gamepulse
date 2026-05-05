package workers

import (
	"strings"
	"testing"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/sports"
)

func TestFormatLiveMessage(t *testing.T) {
	team := models.Team{Name: "Lakers"}
	g := sports.LiveGame{
		ExternalID: "x", Opponent: "Celtics",
		Status: models.GameLive, Period: "Q3",
		HomeScore: 78, AwayScore: 75,
		LastScorer: "LeBron James",
	}
	out := formatLiveMessage(team, g)
	for _, want := range []string{"Lakers", "Celtics", "Q3", "78", "75", "LeBron James"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in %q", want, out)
		}
	}

	g.Status = models.GameFinished
	g.LastScorer = ""
	final := formatLiveMessage(team, g)
	if !strings.Contains(final, "FINAL") {
		t.Errorf("expected FINAL marker, got %q", final)
	}
	if strings.Contains(final, "scored") {
		t.Errorf("did not expect scorer when LastScorer is empty: %q", final)
	}
}
