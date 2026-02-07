package logger

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

// FromContext gets logger from context, returns slog.Default() if not found
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// WithLogger adds logger to context
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}
