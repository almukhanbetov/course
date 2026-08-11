package reviews

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
	rg.GET("/courses/:id/reviews", h.ListReviews)
	rg.POST("/courses/:id/reviews", requireAuth, h.CreateReview)
	rg.GET("/courses/:id/reviews/me", requireAuth, h.GetMyReview)
	rg.PUT("/courses/:id/reviews/me", requireAuth, h.UpdateMyReview)
	rg.DELETE("/courses/:id/reviews/me", requireAuth, h.DeleteMyReview)
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

func courseIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return uuid.UUID{}, false
	}
	return id, true
}

type reviewRequest struct {
	Rating     int    `json:"rating"`
	ReviewText string `json:"review_text"`
}

func (h *Handler) ListReviews(c *gin.Context) {
	courseID, ok := courseIDParam(c)
	if !ok {
		return
	}
	page, limit := parsePageParams(c)

	result, err := h.service.ListPublicByCourse(c.Request.Context(), courseID, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list reviews")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateReview(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	courseID, ok := courseIDParam(c)
	if !ok {
		return
	}

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	review, err := h.service.Create(c.Request.Context(), userID, courseID, ReviewInput(req))
	respondReviewMutation(c, review, err, http.StatusCreated)
}

func (h *Handler) GetMyReview(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	courseID, ok := courseIDParam(c)
	if !ok {
		return
	}

	review, err := h.service.GetMine(c.Request.Context(), userID, courseID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "you have not reviewed this course yet")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load review")
		return
	}
	c.JSON(http.StatusOK, review)
}

func (h *Handler) UpdateMyReview(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	courseID, ok := courseIDParam(c)
	if !ok {
		return
	}

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	review, err := h.service.Update(c.Request.Context(), userID, courseID, ReviewInput(req))
	respondReviewMutation(c, review, err, http.StatusOK)
}

func (h *Handler) DeleteMyReview(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	courseID, ok := courseIDParam(c)
	if !ok {
		return
	}

	err := h.service.Delete(c.Request.Context(), userID, courseID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "you have not reviewed this course")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete review")
		return
	}
	c.Status(http.StatusNoContent)
}

func respondReviewMutation(c *gin.Context, review *Review, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, review)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotEligible):
		respondError(c, http.StatusForbidden, "NOT_ELIGIBLE", "you must be enrolled and have made progress in this course to review it")
	case errors.Is(err, ErrDuplicate):
		respondError(c, http.StatusConflict, "REVIEW_EXISTS", "you have already reviewed this course")
	case errors.Is(err, ErrCourseMissing):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "you have not reviewed this course")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save review")
	}
}

func parsePageParams(c *gin.Context) (page, limit int) {
	return pagination.ParseParams(c.Query("page"), c.Query("limit"))
}
