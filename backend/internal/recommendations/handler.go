package recommendations

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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

// RegisterRoutes registers both the personalized (authenticated) and
// similar-courses (public) endpoints on the same public v1 group — item 25:
// GET /me/recommendations always reads userID from the JWT (authctx), never
// a client-supplied id, so there is nothing here another user's id could
// even be substituted into. GET /courses/:id/similar takes no user
// identity at all (item 18: "Не требует personalization").
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/me/recommendations", requireAuth, h.GetMyRecommendations)
	rg.GET("/courses/:id/similar", h.GetSimilarCourses)
}

func (h *Handler) GetMyRecommendations(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	recs, err := h.service.GetRecommendations(c.Request.Context(), userID, 6)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load recommendations")
		return
	}
	c.JSON(http.StatusOK, recs)
}

func (h *Handler) GetSimilarCourses(c *gin.Context) {
	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return
	}

	similar, err := h.service.GetSimilarCourses(c.Request.Context(), courseID, 4)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load similar courses")
		return
	}
	c.JSON(http.StatusOK, similar)
}
