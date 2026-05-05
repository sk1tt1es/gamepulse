package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/gamepulse/backend/internal/providers/sms"
	"github.com/gamepulse/backend/internal/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrDuplicateSubscription is surfaced when a user already has a
// subscription for the same team. The DB enforces this with a unique
// (user_id, team_id) constraint.
var ErrDuplicateSubscription = errors.New("subscription already exists for this team")

// SubscriptionInput represents the validated payload arriving at the
// `POST /subscriptions` endpoint.
type SubscriptionInput struct {
	PhoneNumber string
	TeamID      uuid.UUID
	UpdateType  models.UpdateType
	Frequency   models.Frequency
}

func (in *SubscriptionInput) Validate() error {
	switch in.UpdateType {
	case models.UpdateLive, models.UpdateNews, models.UpdateBoth:
	default:
		return fmt.Errorf("update_type must be one of live|news|both")
	}

	// Frequency only governs news summaries. Live updates are inherently
	// realtime. For live-only subscribers we accept any (or empty)
	// frequency and default to "daily" so we have a valid value to store.
	if in.UpdateType == models.UpdateLive {
		if in.Frequency == "" {
			in.Frequency = models.FrequencyDaily
			return nil
		}
	}

	switch in.Frequency {
	case models.FrequencyDaily, models.FrequencyWeekly, models.FrequencyMonthly:
	default:
		return fmt.Errorf("frequency must be one of daily|weekly|monthly")
	}
	return nil
}

// SubscriptionService coordinates user creation, subscription persistence,
// and confirmation messaging.
type SubscriptionService struct {
	Repo *repo.Repo
	SMS  sms.Sender
	Log  *slog.Logger

	// InitialNewsDelay is the cooldown between subscribe and the very
	// first news digest for news/both subs. Zero means "use default 5m".
	InitialNewsDelay time.Duration
	// Now is the clock used for stamping CreatedAt + initial-news
	// boundaries. Tests override it for determinism.
	Now func() time.Time
}

func NewSubscriptionService(r *repo.Repo, s sms.Sender, l *slog.Logger) *SubscriptionService {
	return &SubscriptionService{
		Repo: r, SMS: s, Log: l,
		InitialNewsDelay: 5 * time.Minute,
		Now:              func() time.Time { return time.Now().UTC() },
	}
}

// Create runs the full subscription happy path: validate, normalize phone,
// upsert user, look up team, persist subscription, and fire a confirmation
// SMS. SMS failures are logged but do not roll back the subscription —
// the user data is the source of truth.
func (s *SubscriptionService) Create(ctx context.Context, in SubscriptionInput) (*models.Subscription, *models.Team, error) {
	if err := in.Validate(); err != nil {
		return nil, nil, err
	}
	phone, err := NormalizePhone(in.PhoneNumber)
	if err != nil {
		return nil, nil, err
	}

	team, err := s.Repo.GetTeam(ctx, in.TeamID)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.Repo.UpsertUserByPhone(ctx, phone)
	if err != nil {
		return nil, nil, err
	}

	now := s.now()
	sub := &models.Subscription{
		UserID:     user.ID,
		TeamID:     team.ID,
		UpdateType: in.UpdateType,
		Frequency:  in.Frequency,
		CreatedAt:  now,
	}
	// News and "both" subs get a deferred initial digest. Live-only subs
	// flip the flag immediately so the digest worker never picks them up.
	if in.UpdateType == models.UpdateNews || in.UpdateType == models.UpdateBoth {
		delay := s.InitialNewsDelay
		if delay <= 0 {
			delay = 5 * time.Minute
		}
		notBefore := now.Add(delay)
		sub.InitialNewsNotBefore = &notBefore
		sub.InitialNewsSent = false
	} else {
		sub.InitialNewsSent = true
	}
	if err := s.Repo.CreateSubscription(ctx, sub); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, nil, ErrDuplicateSubscription
		}
		return nil, nil, err
	}

	body := buildConfirmation(team, in.UpdateType, in.Frequency)
	if err := s.SMS.Send(ctx, phone, body); err != nil {
		// Don't fail the subscription on SMS error — log and continue. The
		// confirmation can be retried out-of-band; the dispatcher will keep
		// sending updates regardless.
		s.Log.Warn("confirmation sms failed", "err", err, "phone", phone)
	} else {
		// Best-effort log — also captures dedupe so retries don't double-send.
		_, _ = s.Repo.LogNotification(ctx, &models.NotificationLog{
			SubscriptionID: sub.ID,
			MessageType:    models.MessageWelcome,
			Content:        body,
			DedupeKey:      "welcome",
		})
	}

	return sub, team, nil
}

func (s *SubscriptionService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// buildConfirmation describes what the subscriber just signed up for in
// human-friendly language. Live updates are always realtime; news cadence
// reflects the chosen frequency.
func buildConfirmation(team *models.Team, ut models.UpdateType, fr models.Frequency) string {
	var what string
	switch ut {
	case models.UpdateLive:
		what = "live score updates"
	case models.UpdateNews:
		what = fmt.Sprintf("%s news summaries", fr)
	case models.UpdateBoth:
		what = fmt.Sprintf("live scores and %s news summaries", fr)
	}
	return fmt.Sprintf("GamePulse: You're subscribed to %s for the %s. Reply STOP to unsubscribe.",
		what, team.Name)
}

