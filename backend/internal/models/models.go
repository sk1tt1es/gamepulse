package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// League is the set of supported sports leagues. We use string constants
// rather than an int enum so values are readable in JSON and SQL.
type League string

const (
	LeagueNBA League = "NBA"
	LeagueNFL League = "NFL"
	LeagueMLB League = "MLB"
	LeagueNHL League = "NHL"
)

// Leagues returns every supported league. Useful for iterating in workers
// that pull data per league.
func Leagues() []League { return []League{LeagueNBA, LeagueNFL, LeagueMLB, LeagueNHL} }

// UpdateType controls which kinds of messages a subscription receives.
type UpdateType string

const (
	UpdateLive UpdateType = "live"
	UpdateNews UpdateType = "news"
	UpdateBoth UpdateType = "both"
)

// Frequency controls how often news summaries are sent. Live score updates
// always go out in realtime — they ignore this field. The dispatcher and
// digest builder use the value to decide when to roll up pending news for
// each subscriber.
type Frequency string

const (
	FrequencyDaily   Frequency = "daily"
	FrequencyWeekly  Frequency = "weekly"
	FrequencyMonthly Frequency = "monthly"
)

// Window returns how far back the digest builder should look for pending
// news (and equivalently, the minimum gap between digests for a subscriber).
func (f Frequency) Window() time.Duration {
	switch f {
	case FrequencyWeekly:
		return 7 * 24 * time.Hour
	case FrequencyMonthly:
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

// Frequencies returns every supported frequency. Useful for iteration in
// the digest builder.
func Frequencies() []Frequency {
	return []Frequency{FrequencyDaily, FrequencyWeekly, FrequencyMonthly}
}

type GameStatus string

const (
	GameScheduled GameStatus = "scheduled"
	GameLive      GameStatus = "live"
	GameFinished  GameStatus = "finished"
)

type MessageType string

const (
	MessageLiveScore MessageType = "live_score"
	MessageNews      MessageType = "news"
	MessageDigest    MessageType = "digest"
	MessageWelcome   MessageType = "welcome"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	PhoneNumber string    `json:"phone_number"`
	CreatedAt   time.Time `json:"created_at"`
}

type Team struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	League     League    `json:"league"`
	ExternalID string    `json:"external_id"`
	// LogoURL is computed at read time, not stored, so we can swap CDNs
	// without a migration. Empty when no logo is known for the team.
	LogoURL string `json:"logo_url"`
}

// TeamLogoURL builds the public CDN URL for a team's logo. We use ESPN's
// public team-logo endpoint, which is keyed on a lowercase abbreviation.
// Logos that don't resolve render a graceful fallback in the UI.
func TeamLogoURL(league League, externalID string) string {
	leagueSlug := ""
	switch league {
	case LeagueNBA:
		leagueSlug = "nba"
	case LeagueNFL:
		leagueSlug = "nfl"
	case LeagueMLB:
		leagueSlug = "mlb"
	case LeagueNHL:
		leagueSlug = "nhl"
	default:
		return ""
	}
	abbr := strings.ToLower(externalID)
	return "https://a.espncdn.com/i/teamlogos/" + leagueSlug + "/500/" + abbr + ".png"
}

type Subscription struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	TeamID     uuid.UUID  `json:"team_id"`
	UpdateType UpdateType `json:"update_type"`
	Frequency  Frequency  `json:"frequency"`
	CreatedAt  time.Time  `json:"created_at"`
	// InitialNewsSent flips to true after the +5m initial news digest
	// has been delivered for this subscription. Live-only subs are
	// created with this already true so they're never picked up by the
	// initial-send query.
	InitialNewsSent bool `json:"initial_news_sent"`
	// InitialNewsNotBefore is the earliest UTC instant at which the
	// initial news digest is allowed to fire. Nil for live-only subs.
	InitialNewsNotBefore *time.Time `json:"initial_news_not_before,omitempty"`
}

// SubscriptionDetail joins user and team info for read endpoints.
type SubscriptionDetail struct {
	Subscription
	PhoneNumber string `json:"phone_number"`
	TeamName    string `json:"team_name"`
	League      League `json:"league"`
}

type Game struct {
	ID         uuid.UUID  `json:"id"`
	TeamID     uuid.UUID  `json:"team_id"`
	Opponent   string     `json:"opponent"`
	Status     GameStatus `json:"status"`
	StartTime  time.Time  `json:"start_time"`
	ExternalID string     `json:"external_id"`
	HomeScore  int        `json:"home_score"`
	AwayScore  int        `json:"away_score"`
	Period     string     `json:"period"`
}

// GameEvent is an append-only log entry emitted by the live tracker. The
// `Payload` is intentionally JSON so we can evolve the event shape without
// migrations.
type GameEvent struct {
	ID        uuid.UUID `json:"id"`
	GameID    uuid.UUID `json:"game_id"`
	EventType string    `json:"event_type"`
	Payload   []byte    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

type NewsArticle struct {
	ID          uuid.UUID `json:"id"`
	TeamID      uuid.UUID `json:"team_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Source      string    `json:"source"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
	Summary     string    `json:"summary"`
}

type NotificationLog struct {
	ID             uuid.UUID   `json:"id"`
	SubscriptionID uuid.UUID   `json:"subscription_id"`
	MessageType    MessageType `json:"message_type"`
	Content        string      `json:"content"`
	SentAt         time.Time   `json:"sent_at"`
	// DedupeKey makes the dispatcher idempotent — the same key cannot be
	// inserted twice for a given subscription thanks to a unique index.
	DedupeKey string `json:"dedupe_key"`
}
