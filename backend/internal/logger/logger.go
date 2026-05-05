package logger

import (
	"log/slog"
	"os"
)

// New returns the package-wide structured logger. We intentionally use the
// standard library `log/slog` package so we don't pull in a heavier logging
// dependency for a relatively small service.
func New() *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	return slog.New(h).With("service", "gamepulse")
}
