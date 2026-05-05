// Package sms abstracts our SMS provider. Production deployments wire in
// Twilio; local/dev environments fall back to a logging stub so the rest of
// the system can be exercised end-to-end without a Twilio account.
package sms

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Sender is the small interface every SMS backend implements.
type Sender interface {
	Send(ctx context.Context, toPhone, body string) error
}

// --- Twilio backend -------------------------------------------------------

type Twilio struct {
	AccountSID string
	AuthToken  string
	From       string
	HTTP       *http.Client
}

func NewTwilio(sid, token, from string) *Twilio {
	return &Twilio{
		AccountSID: sid, AuthToken: token, From: from,
		HTTP: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *Twilio) Send(ctx context.Context, to, body string) error {
	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.AccountSID)
	form := url.Values{}
	form.Set("To", to)
	form.Set("From", t.From)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("twilio send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("twilio non-2xx: %d", resp.StatusCode)
	}
	return nil
}

// --- Log backend (used when Twilio creds aren't configured) ---------------

// LogSender records every message both to slog and to an in-memory ring so
// tests can assert on what was sent. It is the default when running without
// Twilio credentials, which lets the demo flow work out of the box.
type LogSender struct {
	Logger *slog.Logger
	mu     sync.Mutex
	sent   []SentMessage
}

type SentMessage struct {
	To   string
	Body string
	At   time.Time
}

func NewLogSender(l *slog.Logger) *LogSender {
	return &LogSender{Logger: l}
}

func (l *LogSender) Send(_ context.Context, to, body string) error {
	l.mu.Lock()
	l.sent = append(l.sent, SentMessage{To: to, Body: body, At: time.Now().UTC()})
	if len(l.sent) > 1000 {
		l.sent = l.sent[len(l.sent)-1000:]
	}
	l.mu.Unlock()
	if l.Logger != nil {
		l.Logger.Info("sms.send", "to", to, "body", body)
	}
	return nil
}

// Sent returns a snapshot of recent messages. Useful for tests and the
// debug endpoint exposed by the API.
func (l *LogSender) Sent() []SentMessage {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]SentMessage, len(l.sent))
	copy(out, l.sent)
	return out
}
