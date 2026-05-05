package services

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/repo"
)

// Dispatcher fans out a single live-score notification to every matching
// subscriber. It is responsible for idempotency (via dedupe keys).
//
// As of v3 only live-score events flow through the dispatcher. News is
// summarised + sent at digest time inside DigestBuilder.
//
// Calls are safe to invoke concurrently. SMS sends happen in goroutines so a
// slow upstream (e.g. Twilio) does not block the live tracker.
type Dispatcher struct {
	Repo *repo.Repo
	SMS  sms.Sender
	Log  *slog.Logger
}

func NewDispatcher(r *repo.Repo, s sms.Sender, l *slog.Logger) *Dispatcher {
	return &Dispatcher{Repo: r, SMS: s, Log: l}
}

// FanOut sends or queues `body` to every subscription in `subs`. The
// `dedupeKey` should uniquely identify the underlying event (e.g. game id +
// score) so retries are safe.
func (d *Dispatcher) FanOut(
	ctx context.Context,
	subs []models.SubscriptionDetail,
	mt models.MessageType,
	body, dedupeKey string,
) {
	var wg sync.WaitGroup
	for _, s := range subs {
		s := s
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.send(ctx, s, mt, body, dedupeKey)
		}()
	}
	wg.Wait()
}

func (d *Dispatcher) send(
	ctx context.Context,
	s models.SubscriptionDetail,
	mt models.MessageType,
	body, dedupeKey string,
) {
	inserted, err := d.Repo.LogNotification(ctx, &models.NotificationLog{
		SubscriptionID: s.ID,
		MessageType:    mt,
		Content:        body,
		DedupeKey:      dedupeKey,
	})
	if err != nil {
		d.Log.Warn("log notification failed", "err", err, "sub", s.ID)
		return
	}
	if !inserted {
		// We've already sent this exact message to this subscriber.
		return
	}
	if err := d.SMS.Send(ctx, s.PhoneNumber, body); err != nil {
		d.Log.Warn("sms send failed", "err", err, "sub", s.ID)
	}
}
