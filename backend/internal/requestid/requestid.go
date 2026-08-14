// Package requestid provides the Gin middleware that gives every incoming
// HTTP request a unique ID, for correlating a client-reported issue with
// the exact server-side log lines it produced.
package requestid

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/logging"
)

// HeaderName is the response header the request ID is echoed back on, so a
// client (or whoever's debugging with them) can report it.
const HeaderName = "X-Request-ID"

// Middleware generates a UUID per request, sets it as a response header,
// and attaches a logger already tagged with "request_id" to the request's
// context.Context (see internal/logging.WithLogger) — every handler and
// service call downstream retrieves it via logging.FromContext(ctx).
//
// This replaces Gin's own default access-logging middleware entirely: wire
// it onto gin.New() + gin.Recovery(), not gin.Default() (which bundles its
// own plain-text logger) — otherwise every request would produce two
// differently-formatted log lines instead of one structured, correlated
// one. It logs exactly one "request completed" line per request, after the
// handler chain finishes, with the same status/latency information Gin's
// own default logger reports, just structured and correlated.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.NewString()
		c.Header(HeaderName, id)

		reqLogger := slog.Default().With("request_id", id)
		c.Request = c.Request.WithContext(logging.WithLogger(c.Request.Context(), reqLogger))

		start := time.Now()
		c.Next()

		reqLogger.Info("request completed",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
