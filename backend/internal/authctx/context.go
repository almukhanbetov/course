// Package authctx holds the Gin context keys shared between internal/auth
// (which sets them after verifying a JWT) and internal/users (which reads
// them to resolve the current user), without making either package depend
// on the other.
package authctx

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	userIDKey = "auth_user_id"
	roleKey   = "auth_role"
)

func SetUserID(c *gin.Context, userID string) {
	c.Set(userIDKey, userID)
}

func SetRole(c *gin.Context, role string) {
	c.Set(roleKey, role)
}

func UserID(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get(userIDKey)
	if !exists {
		return uuid.UUID{}, false
	}
	id, err := uuid.Parse(value.(string))
	if err != nil {
		return uuid.UUID{}, false
	}
	return id, true
}

func Role(c *gin.Context) (string, bool) {
	value, exists := c.Get(roleKey)
	if !exists {
		return "", false
	}
	role, ok := value.(string)
	return role, ok
}
