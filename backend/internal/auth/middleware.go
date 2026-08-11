package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"lms-backend/internal/authctx"
)

type Middleware struct {
	secret string
}

func NewMiddleware(secret string) *Middleware {
	return &Middleware{secret: secret}
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func (m *Middleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if header == "" || !ok || token == "" {
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
			c.Abort()
			return
		}

		claims, err := ParseToken(m.secret, token)
		if err != nil {
			respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid or expired token")
			c.Abort()
			return
		}

		authctx.SetUserID(c, claims.UserID)
		authctx.SetRole(c, claims.Role)
		c.Next()
	}
}

func (m *Middleware) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actual, exists := authctx.Role(c)
		if !exists || actual != role {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireAnyRole gates a route to a set of roles at once — e.g. the
// instructor API is reachable by "instructor" and "admin" alike, since an
// admin retains full access to everything an instructor can do.
func (m *Middleware) RequireAnyRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		actual, exists := authctx.Role(c)
		if !exists || !allowed[actual] {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "insufficient permissions")
			c.Abort()
			return
		}
		c.Next()
	}
}
