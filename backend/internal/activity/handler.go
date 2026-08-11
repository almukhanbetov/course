package activity

import (
	"net/http"
	"time"

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

// RegisterRoutes wires the student-facing surface — every route here reads
// only the caller's own userID out of the JWT (authctx.UserID), so there is
// no id-in-URL for another student's analytics to ever leak through (item 24).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/me/analytics", requireAuth, h.GetStats)
	rg.GET("/me/activity", requireAuth, h.GetCalendar)
	rg.GET("/me/activity/recent", requireAuth, h.ListRecent)
}

func (h *Handler) GetStats(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	stats, err := h.service.GetStats(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load analytics")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) GetCalendar(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	now := time.Now()
	from := now.AddDate(0, 0, -90)
	to := now
	if v := c.Query("from"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_FROM", "from must be YYYY-MM-DD")
			return
		}
		from = parsed
	}
	if v := c.Query("to"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_TO", "to must be YYYY-MM-DD")
			return
		}
		to = parsed
	}

	days, err := h.service.GetCalendar(c.Request.Context(), userID, from, to)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load activity calendar")
		return
	}
	c.JSON(http.StatusOK, days)
}

func (h *Handler) ListRecent(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	entries, err := h.service.ListRecent(c.Request.Context(), userID, 20)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load recent activity")
		return
	}
	c.JSON(http.StatusOK, entries)
}
