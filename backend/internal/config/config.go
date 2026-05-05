package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration for the GamePulse backend. Values
// are sourced from environment variables (and optionally a .env file) so the
// service can be deployed to any environment without code changes.
type Config struct {
	HTTPAddr string
	DBURL    string

	// Provider configuration. When a key is empty the system falls back to a
	// deterministic mock implementation so the app remains runnable end-to-end
	// without any external accounts during development and testing.
	//
	// SportsProvider selects which sports.Provider implementation runs:
	// "espn" (default, no key required) or "mock" (the deterministic
	// simulator used in tests and offline demos).
	SportsProvider string
	NewsAPIKey     string
	AIAPIKey       string

	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string

	// Worker tuning knobs. Defaults are tuned for a small dev environment.
	LiveTrackerInterval    time.Duration
	NewsAggregatorInterval time.Duration
	DigestInterval         time.Duration

	// EnableWorkers lets tests boot the API without spawning the background
	// workers. Production deployments leave this on.
	EnableWorkers bool
}

// Load reads configuration from the environment, optionally loading a .env
// file from the working directory. Missing values are filled with sensible
// defaults so a developer can run the service immediately after cloning.
func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		HTTPAddr:               getEnv("HTTP_ADDR", ":8080"),
		DBURL:                  getEnv("DATABASE_URL", "postgres://gamepulse:gamepulse@localhost:5432/gamepulse?sslmode=disable"),
		SportsProvider:         getEnv("SPORTS_PROVIDER", "espn"),
		NewsAPIKey:             os.Getenv("NEWS_API_KEY"),
		AIAPIKey:               os.Getenv("AI_API_KEY"),
		TwilioAccountSID:       os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:        os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioFromNumber:       os.Getenv("TWILIO_FROM_NUMBER"),
		LiveTrackerInterval:    getDuration("LIVE_TRACKER_INTERVAL", 8*time.Second),
		NewsAggregatorInterval: getDuration("NEWS_AGGREGATOR_INTERVAL", 30*time.Minute),
		DigestInterval:         getDuration("DIGEST_INTERVAL", 5*time.Minute),
		EnableWorkers:          getBool("ENABLE_WORKERS", true),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
