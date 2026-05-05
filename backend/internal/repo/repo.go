// Package repo holds Postgres-backed data access. We deliberately keep all
// SQL in this package so the rest of the system can stay agnostic about
// database concerns and can be unit-tested with in-memory fakes.
package repo

import (
	"context"
	"errors"
	"time"

	"github.com/gamepulse/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a row could not be located. Callers should
// translate this to the appropriate API-level status (e.g. 404).
var ErrNotFound = errors.New("not found")

type Repo struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// --- Users ---------------------------------------------------------------

// FindUserByPhone returns the user matching the phone number, or
// repo.ErrNotFound if none exists.
func (r *Repo) FindUserByPhone(ctx context.Context, phone string) (*models.User, error) {
	u := &models.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, phone_number, created_at FROM users WHERE phone_number=$1`, phone,
	).Scan(&u.ID, &u.PhoneNumber, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// DeleteSubscriptionsForUser removes every subscription belonging to the
// given user. Used by the inbound STOP webhook to honor unsubscribe
// requests immediately.
func (r *Repo) DeleteSubscriptionsForUser(ctx context.Context, userID uuid.UUID) (int, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM subscriptions WHERE user_id=$1`, userID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// UpsertUserByPhone fetches a user by phone, creating one if absent. We
// treat the phone number as the natural identifier from the user's
// perspective so they can re-subscribe without an account.
func (r *Repo) UpsertUserByPhone(ctx context.Context, phone string) (*models.User, error) {
	u := &models.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, phone_number, created_at FROM users WHERE phone_number=$1`, phone,
	).Scan(&u.ID, &u.PhoneNumber, &u.CreatedAt)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	u = &models.User{ID: uuid.New(), PhoneNumber: phone, CreatedAt: time.Now().UTC()}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO users (id, phone_number, created_at) VALUES ($1,$2,$3)`,
		u.ID, u.PhoneNumber, u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// --- Teams ---------------------------------------------------------------

func (r *Repo) ListTeams(ctx context.Context) ([]models.Team, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, league, external_id FROM teams ORDER BY league, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Team
	for rows.Next() {
		var t models.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.League, &t.ExternalID); err != nil {
			return nil, err
		}
		t.LogoURL = models.TeamLogoURL(t.League, t.ExternalID)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repo) GetTeam(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	t := &models.Team{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, league, external_id FROM teams WHERE id=$1`, id,
	).Scan(&t.ID, &t.Name, &t.League, &t.ExternalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err == nil {
		t.LogoURL = models.TeamLogoURL(t.League, t.ExternalID)
	}
	return t, err
}

// --- Subscriptions -------------------------------------------------------

// CreateSubscription inserts a new subscription. The DB enforces uniqueness on
// (user_id, team_id) so duplicate submissions return an error which the API
// layer maps to 409 Conflict.
func (r *Repo) CreateSubscription(ctx context.Context, s *models.Subscription) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO subscriptions (id, user_id, team_id, update_type, frequency, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		s.ID, s.UserID, s.TeamID, s.UpdateType, s.Frequency, s.CreatedAt)
	return err
}

func (r *Repo) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM subscriptions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SubscriptionsForTeam returns every subscription for a team along with the
// associated phone number — exactly what the dispatcher needs to fan out a
// score update.
func (r *Repo) SubscriptionsForTeam(ctx context.Context, teamID uuid.UUID, types ...models.UpdateType) ([]models.SubscriptionDetail, error) {
	if len(types) == 0 {
		types = []models.UpdateType{models.UpdateLive, models.UpdateNews, models.UpdateBoth}
	}
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.team_id, s.update_type, s.frequency, s.created_at,
		       u.phone_number, t.name, t.league
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN teams t ON t.id = s.team_id
		WHERE s.team_id = $1 AND s.update_type = ANY($2)`,
		teamID, types)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SubscriptionDetail
	for rows.Next() {
		var sd models.SubscriptionDetail
		if err := rows.Scan(&sd.ID, &sd.UserID, &sd.TeamID, &sd.UpdateType, &sd.Frequency, &sd.CreatedAt,
			&sd.PhoneNumber, &sd.TeamName, &sd.League); err != nil {
			return nil, err
		}
		out = append(out, sd)
	}
	return out, rows.Err()
}

// AllSubscriptions returns every subscription joined with team + user. Used
// by digest workers that iterate over all subscribers regardless of team.
func (r *Repo) AllSubscriptions(ctx context.Context, freq models.Frequency) ([]models.SubscriptionDetail, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.team_id, s.update_type, s.frequency, s.created_at,
		       u.phone_number, t.name, t.league
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN teams t ON t.id = s.team_id
		WHERE s.frequency = $1`, freq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SubscriptionDetail
	for rows.Next() {
		var sd models.SubscriptionDetail
		if err := rows.Scan(&sd.ID, &sd.UserID, &sd.TeamID, &sd.UpdateType, &sd.Frequency, &sd.CreatedAt,
			&sd.PhoneNumber, &sd.TeamName, &sd.League); err != nil {
			return nil, err
		}
		out = append(out, sd)
	}
	return out, rows.Err()
}

// --- Games ---------------------------------------------------------------

// UpsertGame stores or updates the game's score / status. It returns the
// previous home/away score so callers can detect score changes.
func (r *Repo) UpsertGame(ctx context.Context, g *models.Game) (prevHome, prevAway int, prevStatus models.GameStatus, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, 0, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existingID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id, home_score, away_score, status FROM games WHERE team_id=$1 AND external_id=$2`,
		g.TeamID, g.ExternalID,
	).Scan(&existingID, &prevHome, &prevAway, &prevStatus)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if g.ID == uuid.Nil {
			g.ID = uuid.New()
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO games (id, team_id, opponent, status, start_time, external_id, home_score, away_score, period)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			g.ID, g.TeamID, g.Opponent, g.Status, g.StartTime, g.ExternalID, g.HomeScore, g.AwayScore, g.Period)
		if err != nil {
			return 0, 0, "", err
		}
	case err != nil:
		return 0, 0, "", err
	default:
		g.ID = existingID
		_, err = tx.Exec(ctx, `
			UPDATE games SET opponent=$1, status=$2, start_time=$3, home_score=$4, away_score=$5, period=$6
			WHERE id=$7`,
			g.Opponent, g.Status, g.StartTime, g.HomeScore, g.AwayScore, g.Period, g.ID)
		if err != nil {
			return 0, 0, "", err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, 0, "", err
	}
	return prevHome, prevAway, prevStatus, nil
}

func (r *Repo) RecordGameEvent(ctx context.Context, e *models.GameEvent) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO game_events (id, game_id, event_type, payload, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		e.ID, e.GameID, e.EventType, e.Payload, e.CreatedAt)
	return err
}

// --- News ----------------------------------------------------------------

// InsertArticle inserts a news article, returning false if it already
// exists. Deduplication is enforced by a unique (team_id, url) index.
func (r *Repo) InsertArticle(ctx context.Context, a *models.NewsArticle) (bool, error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO news_articles (id, team_id, title, content, source, url, published_at, summary)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (team_id, url) DO NOTHING`,
		a.ID, a.TeamID, a.Title, a.Content, a.Source, a.URL, a.PublishedAt, a.Summary)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *Repo) RecentArticlesForTeam(ctx context.Context, teamID uuid.UUID, since time.Time) ([]models.NewsArticle, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, team_id, title, content, source, url, published_at, summary
		FROM news_articles
		WHERE team_id=$1 AND published_at >= $2
		ORDER BY published_at DESC`,
		teamID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.NewsArticle
	for rows.Next() {
		var a models.NewsArticle
		if err := rows.Scan(&a.ID, &a.TeamID, &a.Title, &a.Content, &a.Source, &a.URL,
			&a.PublishedAt, &a.Summary); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- Notifications -------------------------------------------------------

// LogNotification records a sent message. Idempotency is enforced by a
// unique (subscription_id, dedupe_key) index — duplicates return false.
func (r *Repo) LogNotification(ctx context.Context, n *models.NotificationLog) (bool, error) {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.SentAt.IsZero() {
		n.SentAt = time.Now().UTC()
	}
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO notifications_log (id, subscription_id, message_type, content, sent_at, dedupe_key)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (subscription_id, dedupe_key) DO NOTHING`,
		n.ID, n.SubscriptionID, n.MessageType, n.Content, n.SentAt, n.DedupeKey)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// LastDigestSentAt returns the most recent digest send time for a given
// subscription, or a zero time if none has ever been sent. The digest
// builder uses this to decide whether enough of the cadence window has
// elapsed since the last send.
func (r *Repo) LastDigestSentAt(ctx context.Context, subscriptionID uuid.UUID) (time.Time, error) {
	var t *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT MAX(sent_at) FROM notifications_log
		 WHERE subscription_id=$1 AND message_type='digest'`,
		subscriptionID,
	).Scan(&t)
	if err != nil {
		return time.Time{}, err
	}
	if t == nil {
		return time.Time{}, nil
	}
	return *t, nil
}

// PendingForDigest returns content lines previously logged with the special
// "pending" message_type so the digest builder can roll them up.
func (r *Repo) PendingForDigest(ctx context.Context, subscriptionID uuid.UUID, since time.Time) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT content FROM notifications_log
		WHERE subscription_id=$1 AND message_type='pending' AND sent_at >= $2
		ORDER BY sent_at ASC`,
		subscriptionID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClearPending marks pending entries as consumed by changing their type. We
// do this rather than deleting so we keep an audit trail.
func (r *Repo) ClearPending(ctx context.Context, subscriptionID uuid.UUID, before time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notifications_log
		SET message_type='digested'
		WHERE subscription_id=$1 AND message_type='pending' AND sent_at < $2`,
		subscriptionID, before)
	return err
}
