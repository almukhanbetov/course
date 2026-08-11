package courses

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/courses", h.ListCoursesAdmin)
	admin.POST("/courses", h.CreateCourse)
	admin.GET("/courses/:id", h.GetCourseAdmin)
	admin.PUT("/courses/:id", h.UpdateCourse)
	admin.DELETE("/courses/:id", h.DeleteCourse)

	// Course submissions: the instructor-authoring moderation queue. Kept
	// distinct from student-facing course reviews (see internal/reviews) —
	// deliberately different name so "review" never means two things here.
	admin.GET("/course-submissions", h.ListCourseSubmissions)
	admin.PUT("/course-submissions/:id", h.ReviewCourseSubmission)

	admin.POST("/courses/:id/modules", h.CreateModule)
	admin.PUT("/courses/:id/modules/reorder", h.ReorderModules)
	admin.PUT("/modules/:id", h.UpdateModule)
	admin.DELETE("/modules/:id", h.DeleteModule)

	admin.POST("/modules/:id/lessons", h.CreateLesson)
	admin.PUT("/modules/:id/lessons/reorder", h.ReorderLessons)
	admin.PUT("/lessons/:id", h.UpdateLesson)
	admin.DELETE("/lessons/:id", h.DeleteLesson)
}

func parseUUID(c *gin.Context, name, code, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		respondError(c, http.StatusBadRequest, code, message)
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *Handler) ListCoursesAdmin(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))
	search := c.Query("search")

	list, total, err := h.service.ListAllAdmin(c.Request.Context(), search, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list courses")
		return
	}
	c.JSON(http.StatusOK, pagination.New(list, page, limit, total))
}

func (h *Handler) GetCourseAdmin(c *gin.Context) {
	id, ok := parseCourseID(c)
	if !ok {
		return
	}

	detail, err := h.service.GetCourseDetail(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get course")
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *Handler) ListCourseSubmissions(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListPendingReview(c.Request.Context(), page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list course submissions")
		return
	}
	c.JSON(http.StatusOK, result)
}

type courseSubmissionReviewRequest struct {
	Status          string `json:"status"`
	RejectionReason string `json:"rejection_reason"`
}

func (h *Handler) ReviewCourseSubmission(c *gin.Context) {
	id, ok := parseCourseID(c)
	if !ok {
		return
	}

	var req courseSubmissionReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	course, err := h.service.SetPublicationStatus(c.Request.Context(), id, req.Status, req.RejectionReason)
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, course)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found or not pending review")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to review course submission")
	}
}

type moduleRequest struct {
	Title string `json:"title"`
}

func (h *Handler) CreateModule(c *gin.Context) {
	courseID, ok := parseUUID(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	var req moduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	module, err := h.service.CreateModule(c.Request.Context(), courseID, ModuleInput(req))
	respondModuleMutation(c, module, err, http.StatusCreated)
}

func (h *Handler) UpdateModule(c *gin.Context) {
	id, ok := parseUUID(c, "id", "INVALID_MODULE_ID", "module id must be a valid UUID")
	if !ok {
		return
	}

	var req moduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	module, err := h.service.UpdateModule(c.Request.Context(), id, ModuleInput(req))
	respondModuleMutation(c, module, err, http.StatusOK)
}

func respondModuleMutation(c *gin.Context, module *Module, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, module)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "MODULE_NOT_FOUND", "module or course not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save module")
	}
}

func (h *Handler) DeleteModule(c *gin.Context) {
	id, ok := parseUUID(c, "id", "INVALID_MODULE_ID", "module id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteModule(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "MODULE_NOT_FOUND", "module not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete module")
		return
	}
	c.Status(http.StatusNoContent)
}

type reorderRequest struct {
	Items []struct {
		ID       string `json:"id"`
		Position int    `json:"position"`
	} `json:"items"`
}

func (req reorderRequest) toItems() ([]ReorderItem, error) {
	items := make([]ReorderItem, len(req.Items))
	for i, it := range req.Items {
		id, err := uuid.Parse(it.ID)
		if err != nil {
			return nil, err
		}
		items[i] = ReorderItem{ID: id, Position: it.Position}
	}
	return items, nil
}

func (h *Handler) ReorderModules(c *gin.Context) {
	courseID, ok := parseUUID(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}
	items, err := req.toItems()
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "items contain an invalid id")
		return
	}

	err = h.service.ReorderModules(c.Request.Context(), courseID, items)
	respondReorderResult(c, err)
}

func (h *Handler) ReorderLessons(c *gin.Context) {
	moduleID, ok := parseUUID(c, "id", "INVALID_MODULE_ID", "module id must be a valid UUID")
	if !ok {
		return
	}

	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}
	items, err := req.toItems()
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "items contain an invalid id")
		return
	}

	err = h.service.ReorderLessons(c.Request.Context(), moduleID, items)
	respondReorderResult(c, err)
}

func respondReorderResult(c *gin.Context, err error) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "NOT_FOUND", "resource not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to reorder")
	}
}

type lessonRequest struct {
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	Description     string `json:"description"`
	VideoURL        string `json:"video_url"`
	DurationSeconds int    `json:"duration_seconds"`
	IsFree          bool   `json:"is_free"`
	Published       bool   `json:"published"`
}

func (h *Handler) CreateLesson(c *gin.Context) {
	moduleID, ok := parseUUID(c, "id", "INVALID_MODULE_ID", "module id must be a valid UUID")
	if !ok {
		return
	}

	var req lessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	lesson, err := h.service.CreateLesson(c.Request.Context(), moduleID, LessonInput(req))
	respondLessonMutation(c, lesson, err, http.StatusCreated)
}

func (h *Handler) UpdateLesson(c *gin.Context) {
	id, ok := parseUUID(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}

	var req lessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	lesson, err := h.service.UpdateLesson(c.Request.Context(), id, LessonInput(req))
	respondLessonMutation(c, lesson, err, http.StatusOK)
}

func respondLessonMutation(c *gin.Context, lesson *Lesson, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, lesson)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrDuplicateSlug):
		respondError(c, http.StatusConflict, "LESSON_SLUG_EXISTS", "a lesson with this slug already exists in this module")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson or module not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save lesson")
	}
}

func (h *Handler) DeleteLesson(c *gin.Context) {
	id, ok := parseUUID(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteLesson(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "LESSON_NOT_FOUND", "lesson not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete lesson")
		return
	}
	c.Status(http.StatusNoContent)
}
