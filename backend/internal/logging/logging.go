// Package logging is the structured-logging foundation shared across every
// backend binary (Stage 29A1: cmd/api; Stage 29A2 is expected to bring
// cmd/video-worker/cmd/notification-worker/cmd/code-runner onto the same
// foundation). One JSON handler, configured once, replaces the ad-hoc
// log/fmt.Println calls that previously had no consistent shape and no way
// to correlate lines belonging to the same request.
package logging

import (
	"context"
	"log/slog"
	"os"
)

// Init configures the process-wide default logger — JSON output to stdout,
// matching what a production log aggregator expects to parse — and installs
// it as slog's own package-level default via slog.SetDefault. Call this once,
// at the very start of main(), before any other logging happens. After
// Init, plain slog.Info/slog.Warn/slog.Error calls anywhere in the binary
// (including code that runs before any HTTP request exists, like startup/
// fatal-config logging) automatically use this configuration with no extra
// plumbing required.
func Init() {
	handler := slog.NewJSONHandler(os.Stdout, nil)
	slog.SetDefault(slog.New(handler))
}

type ctxKey struct{}

// WithLogger returns a context carrying a request-scoped logger. Used by
// internal/requestid's middleware to attach a logger already tagged with
// "request_id" — every downstream call that threads ctx through (every
// service-layer function in this codebase takes context.Context, not
// *gin.Context) automatically picks up correlated logging via FromContext,
// with no need to pass the ID around explicitly or thread *gin.Context
// past the handler layer.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext returns the request-scoped logger attached by
// internal/requestid's middleware, or slog.Default() if none is present —
// safe to call from any code path, whether or not it's currently running
// inside an HTTP request.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
