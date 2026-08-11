package tests

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
	rg.GET("/tests/:id", requireAuth, h.GetTest)
	rg.POST("/tests/:id/submit", requireAuth, h.SubmitTest)
	rg.GET("/me/test-attempts", requireAuth, h.ListMyAttempts)
	rg.GET("/me/test-attempts/:id", requireAuth, h.GetMyAttemptDetail)
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

func respondTestAccessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "TEST_NOT_FOUND", "test not found")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you are not enrolled in the course this test belongs to")
	case errors.Is(err, ErrLessonsNotCompleted):
		respondError(c, http.StatusForbidden, "LESSONS_NOT_COMPLETED", "complete all lessons before taking the final test")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "COURSE_ACCESS_REQUIRED", "an active subscription is required to access this test")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to process test")
	}
}

func (h *Handler) GetTest(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	testID, ok := parseUUIDParam(c, "id", "INVALID_TEST_ID", "test id must be a valid UUID")
	if !ok {
		return
	}

	test, err := h.service.GetTest(c.Request.Context(), userID, testID)
	if err != nil {
		respondTestAccessError(c, err)
		return
	}
	c.JSON(http.StatusOK, test)
}

type submitRequest struct {
	Answers []SubmitAnswer `json:"answers"`
}

func (h *Handler) SubmitTest(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	testID, ok := parseUUIDParam(c, "id", "INVALID_TEST_ID", "test id must be a valid UUID")
	if !ok {
		return
	}

	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	result, err := h.service.SubmitTest(c.Request.Context(), userID, testID, req.Answers)

	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, result)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrInvalidAnswer):
		respondError(c, http.StatusBadRequest, "INVALID_ANSWER", ErrInvalidAnswer.Error())
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrNotEnrolled), errors.Is(err, ErrLessonsNotCompleted), errors.Is(err, ErrAccessRequired):
		respondTestAccessError(c, err)
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to submit test")
	}
}

func (h *Handler) ListMyAttempts(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	list, err := h.service.ListMyAttempts(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list attempts")
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) GetMyAttemptDetail(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	attemptID, ok := parseUUIDParam(c, "id", "INVALID_ATTEMPT_ID", "attempt id must be a valid UUID")
	if !ok {
		return
	}

	detail, err := h.service.GetMyAttemptDetail(c.Request.Context(), userID, attemptID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, detail)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "ATTEMPT_NOT_FOUND", "attempt not found")
	case errors.Is(err, ErrForbidden):
		respondError(c, http.StatusForbidden, "FORBIDDEN", ErrForbidden.Error())
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get attempt")
	}
}
