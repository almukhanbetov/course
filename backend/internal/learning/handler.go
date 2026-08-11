package learning

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

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.POST("/courses/:id/enroll", requireAuth, h.Enroll)
	rg.GET("/me/courses", requireAuth, h.ListMyCourses)
	rg.GET("/me/courses/:id", requireAuth, h.GetMyCourseDetail)
	rg.PUT("/lessons/:id/progress", requireAuth, h.UpdateLessonProgress)
	rg.GET("/lessons/:id/progress", requireAuth, h.GetLessonProgress)
	rg.GET("/me/continue-learning", requireAuth, h.GetContinueLearning)
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

func parseUUIDParam(c *gin.Context, name, code, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		respondError(c, http.StatusBadRequest, code, message)
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *Handler) Enroll(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	courseID, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	enrollment, created, err := h.service.EnrollInCourse(c.Request.Context(), userID, courseID)
	switch {
	case err == nil:
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		c.JSON(status, enrollment)
	case errors.Is(err, ErrCourseNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "COURSE_ACCESS_REQUIRED", "an active subscription is required to access this course")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to enroll in course")
	}
}

func (h *Handler) ListMyCourses(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	list, err := h.service.ListMyCourses(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list enrolled courses")
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) GetContinueLearning(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	items, err := h.service.GetContinueLearning(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load continue-learning list")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) GetMyCourseDetail(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	courseID, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	detail, err := h.service.GetMyCourseDetail(c.Request.Context(), userID, courseID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, detail)
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you are not enrolled in this course")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "COURSE_ACCESS_REQUIRED", "an active subscription is required to access this course")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get course")
	}
}

type progressRequest struct {
	ProgressSeconds int  `json:"progress_seconds"`
	Completed       bool `json:"completed"`
}

func (h *Handler) UpdateLessonProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	lessonID, ok := parseUUIDParam(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}

	var req progressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	progress, err := h.service.UpdateLessonProgress(c.Request.Context(), userID, lessonID, req.ProgressSeconds, req.Completed)

	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, progress)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson not found")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you are not enrolled in the course this lesson belongs to")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "COURSE_ACCESS_REQUIRED", "an active subscription is required to access this lesson")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update lesson progress")
	}
}

func (h *Handler) GetLessonProgress(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	lessonID, ok := parseUUIDParam(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}

	progress, err := h.service.GetLessonProgress(c.Request.Context(), userID, lessonID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, progress)
	case errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson not found")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you are not enrolled in the course this lesson belongs to")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "COURSE_ACCESS_REQUIRED", "an active subscription is required to access this lesson")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get lesson progress")
	}
}
