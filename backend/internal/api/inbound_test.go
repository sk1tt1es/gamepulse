package api

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// inbound webhook tests rely on the same DATABASE_URL gate as the rest of
// the api package integration suite. They exercise the full request →
// repo → response cycle.

func TestInbound_StopRemovesSubscriptions(t *testing.T) {
	s := setupServer(t)
	teamID := firstTeamID(t, s)

	// Create a subscription, then STOP and verify it's gone.
	body := strings.NewReader(`{"phone_number":"+15555550400","team_id":"` + teamID + `","update_type":"both","frequency":"daily"}`)
	req, _ := http.NewRequest("POST", "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 201 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("create %d: %s", resp.StatusCode, raw)
	}

	form := url.Values{}
	form.Set("From", "+15555550400")
	form.Set("Body", "STOP")
	stop, _ := http.NewRequest("POST", "/sms/inbound", strings.NewReader(form.Encode()))
	stop.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	stopResp, err := s.App.Test(stop, -1)
	if err != nil {
		t.Fatal(err)
	}
	if stopResp.StatusCode != 200 {
		t.Fatalf("STOP returned %d", stopResp.StatusCode)
	}
	out, _ := io.ReadAll(stopResp.Body)
	if !strings.Contains(string(out), "unsubscribed") {
		t.Errorf("expected unsubscribe message, got %q", out)
	}
	if !strings.HasPrefix(string(out), `<?xml`) {
		t.Errorf("expected TwiML, got %q", out)
	}

	// Re-creating the same subscription should now work, proving the
	// previous one was deleted.
	again := strings.NewReader(`{"phone_number":"+15555550400","team_id":"` + teamID + `","update_type":"both","frequency":"daily"}`)
	rec, _ := http.NewRequest("POST", "/api/v1/subscriptions", again)
	rec.Header.Set("Content-Type", "application/json")
	recResp, err := s.App.Test(rec, -1)
	if err != nil {
		t.Fatal(err)
	}
	if recResp.StatusCode != 201 {
		raw, _ := io.ReadAll(recResp.Body)
		t.Errorf("re-subscribe expected 201, got %d body=%s", recResp.StatusCode, raw)
	}
}

func TestInbound_HelpReply(t *testing.T) {
	s := setupServer(t)
	form := url.Values{}
	form.Set("From", "+15555550401")
	form.Set("Body", "help")
	req, _ := http.NewRequest("POST", "/sms/inbound", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "GamePulse") || !strings.Contains(string(body), "Reply STOP") {
		t.Errorf("HELP reply missing required language: %q", body)
	}
}

func TestInbound_StopForUnknownPhoneStillSucceeds(t *testing.T) {
	s := setupServer(t)
	form := url.Values{}
	form.Set("From", "+15555550999")
	form.Set("Body", "STOP")
	req, _ := http.NewRequest("POST", "/sms/inbound", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.App.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected 200 even for unknown phone, got %d", resp.StatusCode)
	}
}
