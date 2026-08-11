package notifications

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

// RegisterAdminRoutes wires read-only job monitoring, a scoped retry
// action, and the course-announcement trigger. There is deliberately no way
// to create, edit, or delete a notification_jobs row through this surface
// beyond retry — see RetryJob's docs on why even retry can't touch the
// recipient or payload.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/notification-jobs", h.ListJobsAdmin)
	admin.POST("/notification-jobs/:id/retry", h.RetryJob)
	admin.POST("/courses/:id/announce", h.AnnounceCourse)
}

func (h *Handler) ListJobsAdmin(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))
	status := c.Query("status")
	channel := c.Query("channel")

	result, err := h.service.ListJobsAdmin(c.Request.Context(), status, channel, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list notification jobs")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) RetryJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_JOB_ID", "job id must be a valid UUID")
		return
	}

	err = h.service.RetryJob(c.Request.Context(), id)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrJobNotFailed):
		respondError(c, http.StatusConflict, "JOB_NOT_FAILED", "only a failed job can be retried")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retry job")
	}
}

func (h *Handler) AnnounceCourse(c *gin.Context) {
	courseID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return
	}

	count, err := h.service.AnnounceCourse(c.Request.Context(), courseID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"notified_users": count})
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found or not published")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to announce course")
	}
}
