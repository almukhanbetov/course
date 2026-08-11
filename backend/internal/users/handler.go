package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"lms-backend/internal/authctx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/me", requireAuth, h.Me)
	rg.PUT("/me/timezone", requireAuth, h.UpdateTimezone)
}

func (h *Handler) Me(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	user, err := h.service.GetByID(c.Request.Context(), userID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get current user")
		return
	}

	c.JSON(http.StatusOK, user.Public())
}

type updateTimezoneRequest struct {
	Timezone string `json:"timezone"`
}

func (h *Handler) UpdateTimezone(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	var req updateTimezoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	user, err := h.service.UpdateTimezone(c.Request.Context(), userID, req.Timezone)
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, user.Public())
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update timezone")
	}
}
