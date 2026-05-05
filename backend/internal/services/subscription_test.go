package services

import (
	"testing"

	"github.com/gamepulse/backend/internal/models"
)

func TestSubscriptionInput_Validate(t *testing.T) {
	t.Run("rejects bad update_type", func(t *testing.T) {
		in := SubscriptionInput{UpdateType: "bogus", Frequency: models.FrequencyDaily}
		if err := in.Validate(); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects bad frequency for news", func(t *testing.T) {
		in := SubscriptionInput{UpdateType: models.UpdateNews, Frequency: "realtime"}
		if err := in.Validate(); err == nil {
			t.Fatal("expected error for realtime news")
		}
	})

	t.Run("live with empty frequency defaults to daily", func(t *testing.T) {
		in := SubscriptionInput{UpdateType: models.UpdateLive, Frequency: ""}
		if err := in.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if in.Frequency != models.FrequencyDaily {
			t.Errorf("expected default daily, got %s", in.Frequency)
		}
	})

	t.Run("accepts every news/both + valid frequency combo", func(t *testing.T) {
		uts := []models.UpdateType{models.UpdateNews, models.UpdateBoth}
		freqs := []models.Frequency{models.FrequencyDaily, models.FrequencyWeekly, models.FrequencyMonthly}
		for _, ut := range uts {
			for _, f := range freqs {
				in := SubscriptionInput{UpdateType: ut, Frequency: f}
				if err := in.Validate(); err != nil {
					t.Errorf("ut=%s freq=%s: unexpected error %v", ut, f, err)
				}
			}
		}
	})
}

func TestBuildConfirmation(t *testing.T) {
	team := &models.Team{Name: "Lakers"}
	cases := []struct {
		ut     models.UpdateType
		fr     models.Frequency
		needle string
	}{
		{models.UpdateLive, models.FrequencyDaily, "live score updates"},
		{models.UpdateNews, models.FrequencyWeekly, "weekly news summaries"},
		{models.UpdateBoth, models.FrequencyMonthly, "live scores and monthly news summaries"},
	}
	for _, tc := range cases {
		body := buildConfirmation(team, tc.ut, tc.fr)
		if body == "" {
			t.Errorf("empty confirmation")
		}
		if !contains(body, "Lakers") || !contains(body, tc.needle) {
			t.Errorf("body %q missing %q", body, tc.needle)
		}
	}

	// Live confirmations should NOT mention the chosen frequency, since
	// frequency only governs news.
	live := buildConfirmation(team, models.UpdateLive, models.FrequencyDaily)
	if contains(live, "daily") || contains(live, "summaries") {
		t.Errorf("live confirmation should not mention frequency: %q", live)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || stringIndex(haystack, needle) >= 0)
}

func stringIndex(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
