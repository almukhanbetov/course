package recommendations

import (
	"errors"
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
	rg.POST("/recommendations/:id/feedback", requireAuth, h.SubmitFeedback)
	rg.DELETE("/recommendations/:id/feedback", requireAuth, h.RemoveFeedback)
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

type feedbackRequest struct {
	Action string `json:"action"`
}

// SubmitFeedback backs "dismiss"/"not_interested" (Stage 23A2): userID is
// read exclusively from the verified JWT via authctx, exactly like
// GetMyRecommendations above — there is no client-supplied user_id field
// anywhere in feedbackRequest for a caller to substitute another user's id
// into. Idempotent/upsert-safe by construction: Service.SubmitFeedback
// delegates straight to Stage 23A1's UpsertFeedback, which ON CONFLICT
// (user_id, course_id) DO UPDATEs the existing row rather than erroring or
// duplicating — repeated identical feedback, or switching from one action
// to the other, are both just a fresh 200 with the same effect wishlist's
// idempotent Add already established as this codebase's convention.
func (h *Handler) SubmitFeedback(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return
	}

	var req feedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	err = h.service.SubmitFeedback(c.Request.Context(), userID, courseID, req.Action)
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"course_id": courseID, "action": req.Action})
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrCourseNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save feedback")
	}
}

// RemoveFeedback is SubmitFeedback's undo — same identity guarantee, and
// idempotent the same way internal/wishlist.RemoveFromWishlist is: deleting
// feedback that doesn't exist is a normal 200, not a 404, since the caller's
// desired end state (no feedback recorded) is already true either way.
func (h *Handler) RemoveFeedback(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return
	}

	if err := h.service.RemoveFeedback(c.Request.Context(), userID, courseID); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove feedback")
		return
	}
	c.JSON(http.StatusOK, gin.H{"course_id": courseID, "has_feedback": false})
}
