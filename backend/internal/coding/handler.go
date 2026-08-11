package coding

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/authctx"
	"lms-backend/internal/ownership"
)

type Handler struct {
	service   *Service
	ownership *ownership.Service
}

func NewHandler(service *Service, ownershipService *ownership.Service) *Handler {
	return &Handler{service: service, ownership: ownershipService}
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func currentUser(c *gin.Context) (uuid.UUID, string, bool) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return uuid.UUID{}, "", false
	}
	role, _ := authctx.Role(c)
	return userID, role, true
}

func parseUUIDParam(c *gin.Context, name, code, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		respondError(c, http.StatusBadRequest, code, message)
		return uuid.UUID{}, false
	}
	return id, true
}

// RegisterRoutes wires the student-facing surface: view a lesson's coding
// exercise, run/submit code against it, read back one submission, and list
// your own attempt history.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/lessons/:id/coding-exercise", requireAuth, h.GetLessonExercise)
	rg.POST("/coding-exercises/:id/run", requireAuth, h.Run)
	rg.POST("/coding-exercises/:id/submit", requireAuth, h.Submit)
	rg.GET("/coding-exercises/:id/attempts", requireAuth, h.ListMyAttempts)
	rg.GET("/code-submissions/:id", requireAuth, h.GetSubmission)
}

// --- student: view exercise -----------------------------------------------

func (h *Handler) GetLessonExercise(c *gin.Context) {
	lessonID, ok := parseUUIDParam(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	view, err := h.service.GetStudentExercise(c.Request.Context(), userID, lessonID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, view)
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrLessonUnpublished), errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "EXERCISE_NOT_FOUND", "this lesson has no coding exercise")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you must be enrolled in this course")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "ACCESS_REQUIRED", "an active subscription is required")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load coding exercise")
	}
}

// --- student: run / submit -------------------------------------------------

func respondExecutionError(c *gin.Context, err error) {
	var validationErr *ValidationError
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "EXERCISE_NOT_FOUND", "coding exercise not found")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you must be enrolled in this course")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "ACCESS_REQUIRED", "an active subscription is required")
	case errors.Is(err, ErrRateLimited):
		respondError(c, http.StatusTooManyRequests, "RATE_LIMITED", "too many submissions — please wait a moment before trying again")
	case errors.Is(err, ErrTooManyExecutions):
		respondError(c, http.StatusTooManyRequests, "TOO_MANY_EXECUTIONS", "you already have a submission queued or running for this exercise")
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to queue execution")
	}
}

type executeRequest struct {
	SourceCode string `json:"source_code"`
}

func (h *Handler) Run(c *gin.Context) {
	exerciseID, ok := parseUUIDParam(c, "id", "INVALID_EXERCISE_ID", "exercise id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	submission, err := h.service.Run(c.Request.Context(), userID, exerciseID, req.SourceCode)
	if err != nil {
		respondExecutionError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, submission)
}

func (h *Handler) Submit(c *gin.Context) {
	exerciseID, ok := parseUUIDParam(c, "id", "INVALID_EXERCISE_ID", "exercise id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	submission, err := h.service.Submit(c.Request.Context(), userID, exerciseID, req.SourceCode)
	if err != nil {
		respondExecutionError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, submission)
}

func (h *Handler) ListMyAttempts(c *gin.Context) {
	exerciseID, ok := parseUUIDParam(c, "id", "INVALID_EXERCISE_ID", "exercise id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	attempts, err := h.service.ListMyAttempts(c.Request.Context(), userID, exerciseID, 20)
	if err != nil {
		respondExecutionError(c, err)
		return
	}
	c.JSON(http.StatusOK, attempts)
}

// GetSubmission is owner-only — see Service.GetSubmission's doc comment.
func (h *Handler) GetSubmission(c *gin.Context) {
	submissionID, ok := parseUUIDParam(c, "id", "INVALID_SUBMISSION_ID", "submission id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	submission, err := h.service.GetSubmission(c.Request.Context(), userID, submissionID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "SUBMISSION_NOT_FOUND", "submission not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load submission")
		return
	}
	c.JSON(http.StatusOK, submission)
}

// --- instructor ownership middleware --------------------------------------

func (h *Handler) checkOwnership(c *gin.Context, courseID uuid.UUID) bool {
	userID, role, ok := currentUser(c)
	if !ok {
		return false
	}
	can, err := h.ownership.CanManageCourse(c.Request.Context(), userID, role, courseID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify course ownership")
		return false
	}
	if !can {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "you do not manage this course")
		return false
	}
	return true
}

func (h *Handler) requireLessonOwnership(c *gin.Context) (uuid.UUID, bool) {
	lessonID, ok := parseUUIDParam(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return uuid.UUID{}, false
	}
	courseID, err := h.ownership.CourseIDForLesson(c.Request.Context(), lessonID)
	if errors.Is(err, ownership.ErrNotFound) {
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson not found")
		return uuid.UUID{}, false
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resolve lesson")
		return uuid.UUID{}, false
	}
	if !h.checkOwnership(c, courseID) {
		return uuid.UUID{}, false
	}
	return lessonID, true
}

func (h *Handler) requireExerciseOwnership(c *gin.Context) (uuid.UUID, bool) {
	exerciseID, ok := parseUUIDParam(c, "id", "INVALID_EXERCISE_ID", "exercise id must be a valid UUID")
	if !ok {
		return uuid.UUID{}, false
	}
	courseID, err := h.ownership.CourseIDForCodingExercise(c.Request.Context(), exerciseID)
	if errors.Is(err, ownership.ErrNotFound) {
		respondError(c, http.StatusNotFound, "EXERCISE_NOT_FOUND", "coding exercise not found")
		return uuid.UUID{}, false
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resolve exercise")
		return uuid.UUID{}, false
	}
	if !h.checkOwnership(c, courseID) {
		return uuid.UUID{}, false
	}
	return exerciseID, true
}

func (h *Handler) requireTestCaseOwnership(c *gin.Context) (uuid.UUID, bool) {
	testCaseID, ok := parseUUIDParam(c, "id", "INVALID_TEST_CASE_ID", "test case id must be a valid UUID")
	if !ok {
		return uuid.UUID{}, false
	}
	courseID, err := h.ownership.CourseIDForTestCase(c.Request.Context(), testCaseID)
	if errors.Is(err, ownership.ErrNotFound) {
		respondError(c, http.StatusNotFound, "TEST_CASE_NOT_FOUND", "test case not found")
		return uuid.UUID{}, false
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resolve test case")
		return uuid.UUID{}, false
	}
	if !h.checkOwnership(c, courseID) {
		return uuid.UUID{}, false
	}
	return testCaseID, true
}

// RegisterInstructorRoutes wires the instructor-facing surface. The caller
// (main.go) already gates the whole group with RequireAuth +
// RequireAnyRole("instructor", "admin") — every route below additionally
// checks per-course ownership via the require*Ownership helpers, so a
// foreign instructor never reaches another instructor's exercise/test case
// (mirrors assignments' item 21 IDOR concern exactly).
func (h *Handler) RegisterInstructorRoutes(rg *gin.RouterGroup) {
	rg.POST("/lessons/:id/coding-exercise", h.CreateExercise)
	rg.GET("/lessons/:id/coding-exercise", h.GetInstructorExercise)
	rg.PUT("/coding-exercises/:id", h.UpdateExercise)
	rg.DELETE("/coding-exercises/:id", h.DeleteExercise)
	rg.GET("/coding-exercises/:id/test-cases", h.ListTestCases)
	rg.POST("/coding-exercises/:id/test-cases", h.CreateTestCase)
	rg.PUT("/coding-test-cases/:id", h.UpdateTestCase)
	rg.DELETE("/coding-test-cases/:id", h.DeleteTestCase)
}

type exerciseRequest struct {
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Language      string  `json:"language"`
	StarterCode   string  `json:"starter_code"`
	SolutionCode  *string `json:"solution_code"`
	TimeLimitMS   int     `json:"time_limit_ms"`
	MemoryLimitMB int     `json:"memory_limit_mb"`
	Published     bool    `json:"published"`
	Required      bool    `json:"required"`
}

func respondExerciseMutation(c *gin.Context, e *Exercise, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, e)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrDuplicateLesson):
		respondError(c, http.StatusConflict, "EXERCISE_EXISTS", "this lesson already has a coding exercise")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "EXERCISE_NOT_FOUND", "coding exercise not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save coding exercise")
	}
}

func (h *Handler) CreateExercise(c *gin.Context) {
	lessonID, ok := h.requireLessonOwnership(c)
	if !ok {
		return
	}

	var req exerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	e, err := h.service.CreateExercise(c.Request.Context(), lessonID, ExerciseInput(req))
	respondExerciseMutation(c, e, err, http.StatusCreated)
}

func (h *Handler) GetInstructorExercise(c *gin.Context) {
	lessonID, ok := h.requireLessonOwnership(c)
	if !ok {
		return
	}

	e, err := h.service.GetExerciseByLesson(c.Request.Context(), lessonID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "EXERCISE_NOT_FOUND", "this lesson has no coding exercise yet")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load coding exercise")
		return
	}
	c.JSON(http.StatusOK, e)
}

func (h *Handler) UpdateExercise(c *gin.Context) {
	exerciseID, ok := h.requireExerciseOwnership(c)
	if !ok {
		return
	}

	var req exerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	e, err := h.service.UpdateExercise(c.Request.Context(), exerciseID, ExerciseInput(req))
	respondExerciseMutation(c, e, err, http.StatusOK)
}

func (h *Handler) DeleteExercise(c *gin.Context) {
	exerciseID, ok := h.requireExerciseOwnership(c)
	if !ok {
		return
	}

	err := h.service.DeleteExercise(c.Request.Context(), exerciseID)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "EXERCISE_NOT_FOUND", "coding exercise not found")
	case errors.Is(err, ErrHasSubmissions):
		respondError(c, http.StatusConflict, "EXERCISE_HAS_SUBMISSIONS",
			"this exercise has student submissions; unpublish it instead of deleting")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete coding exercise")
	}
}

type testCaseRequest struct {
	Input          *string `json:"input"`
	ExpectedOutput string  `json:"expected_output"`
	Position       int     `json:"position"`
	Hidden         bool    `json:"hidden"`
}

func respondTestCaseMutation(c *gin.Context, tc *TestCase, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, tc)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrPositionConflict):
		respondError(c, http.StatusConflict, "POSITION_CONFLICT", "a test case with this position already exists")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "TEST_CASE_NOT_FOUND", "test case not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save test case")
	}
}

func (h *Handler) ListTestCases(c *gin.Context) {
	exerciseID, ok := h.requireExerciseOwnership(c)
	if !ok {
		return
	}

	cases, err := h.service.ListTestCases(c.Request.Context(), exerciseID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list test cases")
		return
	}
	c.JSON(http.StatusOK, cases)
}

func (h *Handler) CreateTestCase(c *gin.Context) {
	exerciseID, ok := h.requireExerciseOwnership(c)
	if !ok {
		return
	}

	var req testCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	tc, err := h.service.CreateTestCase(c.Request.Context(), exerciseID, TestCaseInput(req))
	respondTestCaseMutation(c, tc, err, http.StatusCreated)
}

func (h *Handler) UpdateTestCase(c *gin.Context) {
	testCaseID, ok := h.requireTestCaseOwnership(c)
	if !ok {
		return
	}

	var req testCaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	tc, err := h.service.UpdateTestCase(c.Request.Context(), testCaseID, TestCaseInput(req))
	respondTestCaseMutation(c, tc, err, http.StatusOK)
}

func (h *Handler) DeleteTestCase(c *gin.Context) {
	testCaseID, ok := h.requireTestCaseOwnership(c)
	if !ok {
		return
	}

	err := h.service.DeleteTestCase(c.Request.Context(), testCaseID)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "TEST_CASE_NOT_FOUND", "test case not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete test case")
	}
}
