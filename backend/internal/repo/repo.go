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
//
// Initial-news semantics:
//   - Live-only subs are stored with initial_news_sent = true and
//     initial_news_not_before = NULL — they never receive a news digest.
//   - news / both subs receive an initial digest after a short cooldown
//     (set by the caller via Subscription.InitialNewsNotBefore).
func (r *Repo) CreateSubscription(ctx context.Context, s *models.Subscription) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO subscriptions (
		     id, user_id, team_id, update_type, frequency, created_at,
		     initial_news_sent, initial_news_not_before)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		s.ID, s.UserID, s.TeamID, s.UpdateType, s.Frequency, s.CreatedAt,
		s.InitialNewsSent, s.InitialNewsNotBefore)
	return err
}

// MarkInitialNewsSent flips the initial_news_sent flag so the next
// digest tick won't re-pick the subscription for an initial send.
func (r *Repo) MarkInitialNewsSent(ctx context.Context, subscriptionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE subscriptions SET initial_news_sent = true WHERE id=$1`,
		subscriptionID)
	return err
}

// SubscriptionsDueForInitialNews returns every news/both subscription
// whose +5m cooldown has elapsed and which has not yet received its
// initial digest. The result is joined with user + team like
// AllSubscriptions so the digest builder can format the SMS without
// extra lookups.
func (r *Repo) SubscriptionsDueForInitialNews(ctx context.Context, now time.Time) ([]models.SubscriptionDetail, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.team_id, s.update_type, s.frequency, s.created_at,
		       s.initial_news_sent, s.initial_news_not_before,
		       u.phone_number, t.name, t.league
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN teams t ON t.id = s.team_id
		WHERE s.initial_news_sent = false
		  AND s.initial_news_not_before IS NOT NULL
		  AND s.initial_news_not_before <= $1
		  AND s.update_type IN ('news','both')
		ORDER BY s.initial_news_not_before ASC`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptionDetails(rows)
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
		       s.initial_news_sent, s.initial_news_not_before,
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
	return scanSubscriptionDetails(rows)
}

// AllSubscriptions returns every subscription joined with team + user. Used
// by digest workers that iterate over all subscribers regardless of team.
//
// For digest cadence purposes we only return subs that have already had
// their initial news send — initial sends are handled by a separate query
// (SubscriptionsDueForInitialNews) so the recurring path does not double
// up on the welcome digest.
func (r *Repo) AllSubscriptions(ctx context.Context, freq models.Frequency) ([]models.SubscriptionDetail, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.user_id, s.team_id, s.update_type, s.frequency, s.created_at,
		       s.initial_news_sent, s.initial_news_not_before,
		       u.phone_number, t.name, t.league
		FROM subscriptions s
		JOIN users u ON u.id = s.user_id
		JOIN teams t ON t.id = s.team_id
		WHERE s.frequency = $1
		  AND s.initial_news_sent = true`, freq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubscriptionDetails(rows)
}

// scanSubscriptionDetails consolidates the per-row scan + nullable
// timestamp handling shared across every subscription read query.
func scanSubscriptionDetails(rows pgx.Rows) ([]models.SubscriptionDetail, error) {
	var out []models.SubscriptionDetail
	for rows.Next() {
		var sd models.SubscriptionDetail
		var notBefore *time.Time
		if err := rows.Scan(&sd.ID, &sd.UserID, &sd.TeamID, &sd.UpdateType, &sd.Frequency, &sd.CreatedAt,
			&sd.InitialNewsSent, &notBefore,
			&sd.PhoneNumber, &sd.TeamName, &sd.League); err != nil {
			return nil, err
		}
		sd.InitialNewsNotBefore = notBefore
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
	return scanArticles(rows)
}

// UnsentArticlesForSubscription returns every article published after
// `since` for the subscription's team that has NOT already been recorded
// in subscription_article_sent for this subscription. This is the
// per-subscriber feed that the digest builder summarizes at send time.
func (r *Repo) UnsentArticlesForSubscription(
	ctx context.Context,
	subscriptionID, teamID uuid.UUID,
	since time.Time,
) ([]models.NewsArticle, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.team_id, a.title, a.content, a.source, a.url,
		       a.published_at, a.summary
		FROM news_articles a
		WHERE a.team_id = $1
		  AND a.published_at >= $2
		  AND NOT EXISTS (
		      SELECT 1 FROM subscription_article_sent sas
		      WHERE sas.subscription_id = $3 AND sas.article_id = a.id)
		ORDER BY a.published_at DESC`,
		teamID, since, subscriptionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanArticles(rows)
}

// MarkArticlesSent records that every supplied article was included in
// a digest SMS for the subscription. Idempotent (ON CONFLICT DO NOTHING)
// so a retry that re-runs the same batch is safe.
func (r *Repo) MarkArticlesSent(
	ctx context.Context,
	subscriptionID uuid.UUID,
	articleIDs []uuid.UUID,
	at time.Time,
) error {
	if len(articleIDs) == 0 {
		return nil
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	batch := &pgx.Batch{}
	for _, aid := range articleIDs {
		batch.Queue(`
			INSERT INTO subscription_article_sent (subscription_id, article_id, sent_at)
			VALUES ($1,$2,$3)
			ON CONFLICT (subscription_id, article_id) DO NOTHING`,
			subscriptionID, aid, at)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()
	for range articleIDs {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

// DeleteFullyConsumedArticles removes every news article that no
// remaining eligible subscriber still needs.
//
// "Eligible" = an active news/both subscription whose created_at is at or
// before the article's published_at. Subscribers that signed up AFTER an
// article was published would never have seen it in their initial digest
// (which queries from created_at) so we don't keep articles around just
// for them.
//
// Returns the number of rows deleted so the caller can emit a metric.
func (r *Repo) DeleteFullyConsumedArticles(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM news_articles a
		 WHERE NOT EXISTS (
		     SELECT 1
		       FROM subscriptions s
		      WHERE s.team_id = a.team_id
		        AND s.update_type IN ('news','both')
		        AND s.created_at <= a.published_at
		        AND NOT EXISTS (
		            SELECT 1 FROM subscription_article_sent sas
		             WHERE sas.subscription_id = s.id
		               AND sas.article_id = a.id))`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeleteArticlesOlderThan is a TTL safety net: any news article older
// than `cutoff` is deleted regardless of consumption state. Returns the
// number of rows deleted.
func (r *Repo) DeleteArticlesOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM news_articles WHERE published_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func scanArticles(rows pgx.Rows) ([]models.NewsArticle, error) {
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

// --- Game cleanup -------------------------------------------------------

// DeleteFinishedGamesBefore removes finished games whose start_time is
// strictly older than `cutoff`. Cascading FKs take care of the
// associated game_events rows. Returns the number of games deleted.
func (r *Repo) DeleteFinishedGamesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM games WHERE status = 'finished' AND start_time < $1`,
		cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
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

