package tests

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/tests", h.ListTestsAdmin)
	admin.POST("/tests", h.CreateTest)
	admin.GET("/tests/:id", h.GetTestAdmin)
	admin.PUT("/tests/:id", h.UpdateTest)
	admin.DELETE("/tests/:id", h.DeleteTest)

	admin.POST("/tests/:id/questions", h.CreateQuestion)
	admin.PUT("/questions/:id", h.UpdateQuestion)
	admin.DELETE("/questions/:id", h.DeleteQuestion)

	admin.POST("/questions/:id/answers", h.CreateAnswer)
	admin.PUT("/answers/:id", h.UpdateAnswer)
	admin.DELETE("/answers/:id", h.DeleteAnswer)
}

func (h *Handler) ListTestsAdmin(c *gin.Context) {
	list, err := h.service.ListTestsAdmin(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list tests")
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) GetTestAdmin(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_TEST_ID", "test id must be a valid UUID")
	if !ok {
		return
	}

	detail, err := h.service.GetAdminTestDetail(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "TEST_NOT_FOUND", "test not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get test")
		return
	}
	c.JSON(http.StatusOK, detail)
}

type testRequest struct {
	CourseID     *string `json:"course_id"`
	ModuleID     *string `json:"module_id"`
	LessonID     *string `json:"lesson_id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	PassingScore int     `json:"passing_score"`
	Published    bool    `json:"published"`
	IsFinal      bool    `json:"is_final"`
}

func (req testRequest) toInput() (TestInput, error) {
	parseOptional := func(s *string) (*uuid.UUID, error) {
		if s == nil || *s == "" {
			return nil, nil
		}
		id, err := uuid.Parse(*s)
		if err != nil {
			return nil, err
		}
		return &id, nil
	}

	courseID, err := parseOptional(req.CourseID)
	if err != nil {
		return TestInput{}, err
	}
	moduleID, err := parseOptional(req.ModuleID)
	if err != nil {
		return TestInput{}, err
	}
	lessonID, err := parseOptional(req.LessonID)
	if err != nil {
		return TestInput{}, err
	}

	return TestInput{
		CourseID:     courseID,
		ModuleID:     moduleID,
		LessonID:     lessonID,
		Title:        req.Title,
		Description:  req.Description,
		PassingScore: req.PassingScore,
		Published:    req.Published,
		IsFinal:      req.IsFinal,
	}, nil
}

func respondTestMutation(c *gin.Context, test *Test, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, test)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrInvalidParent):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", ErrInvalidParent.Error())
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "TEST_NOT_FOUND", "test not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save test")
	}
}

func (h *Handler) CreateTest(c *gin.Context) {
	var req testRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}
	input, err := req.toInput()
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "course_id/module_id/lesson_id must be valid UUIDs")
		return
	}

	test, err := h.service.CreateTest(c.Request.Context(), input)
	respondTestMutation(c, test, err, http.StatusCreated)
}

func (h *Handler) UpdateTest(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_TEST_ID", "test id must be a valid UUID")
	if !ok {
		return
	}

	var req testRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}
	input, err := req.toInput()
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "course_id/module_id/lesson_id must be valid UUIDs")
		return
	}

	test, err := h.service.UpdateTest(c.Request.Context(), id, input)
	respondTestMutation(c, test, err, http.StatusOK)
}

func (h *Handler) DeleteTest(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_TEST_ID", "test id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteTest(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "TEST_NOT_FOUND", "test not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete test")
		return
	}
	c.Status(http.StatusNoContent)
}

type questionRequest struct {
	Text string `json:"text"`
}

func respondQuestionMutation(c *gin.Context, question *Question, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, question)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question or test not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save question")
	}
}

func (h *Handler) CreateQuestion(c *gin.Context) {
	testID, ok := parseUUIDParam(c, "id", "INVALID_TEST_ID", "test id must be a valid UUID")
	if !ok {
		return
	}

	var req questionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	question, err := h.service.CreateQuestion(c.Request.Context(), testID, QuestionInput(req))
	respondQuestionMutation(c, question, err, http.StatusCreated)
}

func (h *Handler) UpdateQuestion(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_QUESTION_ID", "question id must be a valid UUID")
	if !ok {
		return
	}

	var req questionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	question, err := h.service.UpdateQuestion(c.Request.Context(), id, QuestionInput(req))
	respondQuestionMutation(c, question, err, http.StatusOK)
}

func (h *Handler) DeleteQuestion(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_QUESTION_ID", "question id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteQuestion(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete question")
		return
	}
	c.Status(http.StatusNoContent)
}

type answerRequest struct {
	Text      string `json:"text"`
	IsCorrect bool   `json:"is_correct"`
}

func respondAnswerMutation(c *gin.Context, answer *Answer, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, answer)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "ANSWER_NOT_FOUND", "answer or question not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save answer")
	}
}

func (h *Handler) CreateAnswer(c *gin.Context) {
	questionID, ok := parseUUIDParam(c, "id", "INVALID_QUESTION_ID", "question id must be a valid UUID")
	if !ok {
		return
	}

	var req answerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	answer, err := h.service.CreateAnswer(c.Request.Context(), questionID, AnswerInput(req))
	respondAnswerMutation(c, answer, err, http.StatusCreated)
}

func (h *Handler) UpdateAnswer(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_ANSWER_ID", "answer id must be a valid UUID")
	if !ok {
		return
	}

	var req answerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	answer, err := h.service.UpdateAnswer(c.Request.Context(), id, AnswerInput(req))
	respondAnswerMutation(c, answer, err, http.StatusOK)
}

func (h *Handler) DeleteAnswer(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_ANSWER_ID", "answer id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteAnswer(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "ANSWER_NOT_FOUND", "answer not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete answer")
		return
	}
	c.Status(http.StatusNoContent)
}
