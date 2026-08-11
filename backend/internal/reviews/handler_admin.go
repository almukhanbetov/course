package reviews

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

// RegisterAdminRoutes exposes moderation only: list every review (published
// or hidden) and flip its published flag. Admins can never touch a
// student's rating or text — see the publishedRequest DTO below, which has
// no other field to bind.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/reviews", h.ListReviewsAdmin)
	admin.PUT("/reviews/:id", h.SetReviewPublished)
}

func (h *Handler) ListReviewsAdmin(c *gin.Context) {
	params := AdminListParams{}
	params.Page, params.Limit = pagination.ParseParams(c.Query("page"), c.Query("limit"))

	if courseIDStr := c.Query("course_id"); courseIDStr != "" {
		courseID, err := uuid.Parse(courseIDStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course_id must be a valid UUID")
			return
		}
		params.CourseID = &courseID
	}

	if ratingStr := c.Query("rating"); ratingStr != "" {
		rating, err := strconv.Atoi(ratingStr)
		if err != nil || rating < 1 || rating > 5 {
			respondError(c, http.StatusBadRequest, "INVALID_RATING", "rating must be an integer between 1 and 5")
			return
		}
		params.Rating = &rating
	}

	result, err := h.service.ListAdmin(c.Request.Context(), params)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list reviews")
		return
	}
	c.JSON(http.StatusOK, result)
}

type publishedRequest struct {
	Published bool `json:"published"`
}

func (h *Handler) SetReviewPublished(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REVIEW_ID", "review id must be a valid UUID")
		return
	}

	var req publishedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	review, err := h.service.SetPublished(c.Request.Context(), id, req.Published)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "REVIEW_NOT_FOUND", "review not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update review")
		return
	}
	c.JSON(http.StatusOK, review)
}
