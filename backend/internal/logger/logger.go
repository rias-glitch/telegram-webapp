package logger

import (
	"context"
	"log/slog"
	"os"
)

var (
	defaultLogger *slog.Logger
)

// Init initializes the global logger
func Init(level string, json bool) {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	if json {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Get returns the default logger
func Get() *slog.Logger {
	if defaultLogger == nil {
		Init("info", false)
	}
	return defaultLogger
}

// WithContext returns a logger with context values
func WithContext(ctx context.Context) *slog.Logger {
	return Get()
}

// Info logs at info level
func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

// Debug logs at debug level
func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

// Warn logs at warn level
func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

// Error logs at error level
func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}

// Fatal logs at error level and exits
func Fatal(msg string, args ...any) {
	Get().Error(msg, args...)
	os.Exit(1)
}

// With returns a logger with the given attributes
func With(args ...any) *slog.Logger {
	return Get().With(args...)
}
