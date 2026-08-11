package notifications

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/authctx"
	"lms-backend/internal/pagination"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/me/notifications", requireAuth, h.ListMyNotifications)
	rg.GET("/me/notifications/unread-count", requireAuth, h.UnreadCount)
	rg.PUT("/me/notifications/:id/read", requireAuth, h.MarkRead)
	rg.PUT("/me/notifications/read-all", requireAuth, h.MarkAllRead)
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return uuid.UUID{}, false
	}
	return userID, true
}

func (h *Handler) ListMyNotifications(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListMyNotifications(c.Request.Context(), userID, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list notifications")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) UnreadCount(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	count, err := h.service.UnreadCount(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get unread count")
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *Handler) MarkRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_NOTIFICATION_ID", "notification id must be a valid UUID")
		return
	}

	err = h.service.MarkRead(c.Request.Context(), userID, id)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrNotFound):
		// Deliberately the same response whether the notification doesn't
		// exist or belongs to someone else — both cases return zero rows
		// affected from the repository's user-scoped UPDATE, so there's no
		// way (or reason) to tell them apart from the outside.
		respondError(c, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "notification not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to mark notification read")
	}
}

func (h *Handler) MarkAllRead(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	if err := h.service.MarkAllRead(c.Request.Context(), userID); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to mark notifications read")
		return
	}
	c.Status(http.StatusNoContent)
}
