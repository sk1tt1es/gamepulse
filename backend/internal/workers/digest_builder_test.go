package workers

import (
	"strings"
	"testing"

	"github.com/gamepulse/backend/internal/models"
)

func TestBuildDigestBody(t *testing.T) {
	sd := models.SubscriptionDetail{
		Subscription: models.Subscription{Frequency: models.FrequencyWeekly},
		TeamName:     "Lakers",
	}
	body := buildDigestBody(sd, []string{"AD returns next week", "LeBron rests"})
	if !strings.HasPrefix(body, "Weekly news digest for Lakers:") {
		t.Errorf("unexpected prefix: %q", body)
	}
	if !strings.Contains(body, "• AD") || !strings.Contains(body, "• LeBron") {
		t.Errorf("missing bullets: %q", body)
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
