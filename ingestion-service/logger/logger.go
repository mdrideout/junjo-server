package logger

import (
	"log/slog"
	"os"
	"strings"
)

// InitLogger initializes and returns a configured slog.Logger based on environment variables.
// It reads LOG_LEVEL (default: info) and LOG_FORMAT (default: json) from the environment.
// The logger is also set as the default global logger via slog.SetDefault().
func InitLogger() *slog.Logger {
	// Parse LOG_LEVEL from env (default: info)
	level := parseLogLevel(os.Getenv("LOG_LEVEL"))

	// Parse LOG_FORMAT from env (default: json)
	format := os.Getenv("LOG_FORMAT")
	if format == "" {
		format = "json"
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug, // Add source location for debug
	}

	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// parseLogLevel converts a string log level to slog.Level.
// Supports: debug, info, warn/warning, error
// Default: info
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
