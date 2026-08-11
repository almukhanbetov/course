package achievements

import (
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

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/me/achievements", requireAuth, h.GetMyAchievements)
}

func (h *Handler) GetMyAchievements(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "UNAUTHORIZED", "message": "missing or invalid authorization header"}})
		return
	}

	view, err := h.service.GetView(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "failed to load achievements"}})
		return
	}
	c.JSON(http.StatusOK, view)
}
