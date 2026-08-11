package qa

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

// RegisterRoutes wires the five student-facing endpoints from the Stage 20A
// plan onto the bare /api/v1 group — deliberately not /admin or /instructor,
// so these never collide with internal/tests' unrelated "questions"/
// "answers" quiz-authoring routes registered on those other groups (see the
// package doc comment).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/lessons/:id/questions", requireAuth, h.ListQuestions)
	rg.POST("/lessons/:id/questions", requireAuth, h.CreateQuestion)
	rg.POST("/questions/:id/answers", requireAuth, h.CreateAnswer)
	rg.DELETE("/questions/:id", requireAuth, h.DeleteQuestion)
	rg.DELETE("/answers/:id", requireAuth, h.DeleteAnswer)
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// currentUser always reads identity from the verified JWT via authctx,
// never from a client-supplied field — see the Stage 20 plan's explicit
// "never trust client-supplied user_id" rule.
func currentUser(c *gin.Context) (uuid.UUID, string, bool) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return uuid.UUID{}, "", false
	}
	role, _ := authctx.Role(c)
	return userID, role, true
}

func idParam(c *gin.Context, name, code, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		respondError(c, http.StatusBadRequest, code, message)
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *Handler) ListQuestions(c *gin.Context) {
	lessonID, ok := idParam(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListForLesson(c.Request.Context(), lessonID, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list questions")
		return
	}
	c.JSON(http.StatusOK, result)
}

type questionRequest struct {
	Body string `json:"body"`
}

func (h *Handler) CreateQuestion(c *gin.Context) {
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}
	lessonID, ok := idParam(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}

	var req questionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	question, err := h.service.CreateQuestion(c.Request.Context(), userID, lessonID, QuestionInput{Body: req.Body})
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, question)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrLessonMissing):
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson not found")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you must be enrolled in this course to ask a question")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create question")
	}
}

type answerRequest struct {
	Body string `json:"body"`
}

func (h *Handler) CreateAnswer(c *gin.Context) {
	userID, role, ok := currentUser(c)
	if !ok {
		return
	}
	questionID, ok := idParam(c, "id", "INVALID_QUESTION_ID", "question id must be a valid UUID")
	if !ok {
		return
	}

	var req answerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	answer, err := h.service.CreateAnswer(c.Request.Context(), userID, role, questionID, AnswerInput{Body: req.Body})
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, answer)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "QUESTION_NOT_FOUND", "question not found")
	case errors.Is(err, ErrForbidden):
		respondError(c, http.StatusForbidden, "FORBIDDEN", "you must be enrolled in this course, or its instructor, or an admin, to answer")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create answer")
	}
}

func (h *Handler) DeleteQuestion(c *gin.Context) {
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}
	questionID, ok := idParam(c, "id", "INVALID_QUESTION_ID", "question id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteQuestion(c.Request.Context(), userID, questionID)
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

func (h *Handler) DeleteAnswer(c *gin.Context) {
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}
	answerID, ok := idParam(c, "id", "INVALID_ANSWER_ID", "answer id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteAnswer(c.Request.Context(), userID, answerID)
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
