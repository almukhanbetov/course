package assignments

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/authctx"
	"lms-backend/internal/ownership"
	"lms-backend/internal/pagination"
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

// RegisterRoutes wires the student-facing surface: view a lesson's
// assignment, manage your own draft/submission, upload/download files.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/lessons/:id/assignment", requireAuth, h.GetLessonAssignment)
	rg.GET("/assignments/:id/submission", requireAuth, h.GetMySubmission)
	rg.PUT("/assignments/:id/submission", requireAuth, h.SaveDraft)
	rg.POST("/assignments/:id/submission/files", requireAuth, h.UploadFile)
	rg.POST("/assignments/:id/submission/submit", requireAuth, h.Submit)
	rg.GET("/submission-files/:id/download", requireAuth, h.DownloadFile)
}

// --- student: view assignment -------------------------------------------

func (h *Handler) GetLessonAssignment(c *gin.Context) {
	lessonID, ok := parseUUIDParam(c, "id", "INVALID_LESSON_ID", "lesson id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	assignment, err := h.service.GetStudentAssignment(c.Request.Context(), userID, lessonID)
	respondStudentAssignment(c, assignment, err)
}

func respondStudentAssignment(c *gin.Context, assignment *StudentAssignment, err error) {
	switch {
	case err == nil:
		c.JSON(http.StatusOK, assignment)
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrLessonUnpublished), errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "this lesson has no assignment")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you must be enrolled in this course")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "ACCESS_REQUIRED", "an active subscription is required")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load assignment")
	}
}

// --- student: draft / submission -----------------------------------------

func (h *Handler) GetMySubmission(c *gin.Context) {
	assignmentID, ok := parseUUIDParam(c, "id", "INVALID_ASSIGNMENT_ID", "assignment id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	submission, err := h.service.GetSubmission(c.Request.Context(), userID, assignmentID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "SUBMISSION_NOT_FOUND", "you have not started this assignment yet")
		return
	}
	respondSubmissionAccessError(c, err, func() { c.JSON(http.StatusOK, submission) })
}

// respondSubmissionAccessError centralizes the access-check error mapping
// shared by every /assignments/:id/submission* handler; onOK runs only when
// err is nil.
func respondSubmissionAccessError(c *gin.Context, err error, onOK func()) {
	switch {
	case err == nil:
		onOK()
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrLessonUnpublished), errors.Is(err, ErrLessonNotFound):
		respondError(c, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "assignment not found")
	case errors.Is(err, ErrNotEnrolled):
		respondError(c, http.StatusForbidden, "NOT_ENROLLED", "you must be enrolled in this course")
	case errors.Is(err, ErrAccessRequired):
		respondError(c, http.StatusForbidden, "ACCESS_REQUIRED", "an active subscription is required")
	case errors.Is(err, ErrNotEditable):
		respondError(c, http.StatusConflict, "NOT_EDITABLE", "this submission is awaiting review or already graded")
	case errors.Is(err, ErrEmptySubmission):
		respondError(c, http.StatusBadRequest, "EMPTY_SUBMISSION", "add some text or at least one file before submitting")
	case errors.Is(err, ErrTooLarge):
		respondError(c, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file exceeds the maximum upload size")
	case errors.Is(err, ErrInvalidFileType):
		respondError(c, http.StatusUnprocessableEntity, "INVALID_FILE_TYPE", "only PDF, TXT, ZIP, PNG, and JPEG files are accepted")
	default:
		var validationErr *ValidationError
		if errors.As(err, &validationErr) {
			respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
			return
		}
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "request failed")
	}
}

type draftRequest struct {
	TextContent string `json:"text_content"`
}

func (h *Handler) SaveDraft(c *gin.Context) {
	assignmentID, ok := parseUUIDParam(c, "id", "INVALID_ASSIGNMENT_ID", "assignment id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	var req draftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	submission, err := h.service.SaveDraft(c.Request.Context(), userID, assignmentID, req.TextContent)
	respondSubmissionAccessError(c, err, func() { c.JSON(http.StatusOK, submission) })
}

func (h *Handler) Submit(c *gin.Context) {
	assignmentID, ok := parseUUIDParam(c, "id", "INVALID_ASSIGNMENT_ID", "assignment id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	submission, err := h.service.Submit(c.Request.Context(), userID, assignmentID)
	respondSubmissionAccessError(c, err, func() { c.JSON(http.StatusOK, submission) })
}

func (h *Handler) UploadFile(c *gin.Context) {
	assignmentID, ok := parseUUIDParam(c, "id", "INVALID_ASSIGNMENT_ID", "assignment id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.service.MaxUploadBytes()+1<<20)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			respondError(c, http.StatusBadRequest, "INVALID_BODY", "a \"file\" field is required")
			return
		}
		respondError(c, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file exceeds the maximum upload size")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read uploaded file")
		return
	}
	defer file.Close()

	result, err := h.service.UploadFile(c.Request.Context(), userID, assignmentID, UploadInput{
		Filename:     fileHeader.Filename,
		DeclaredMIME: fileHeader.Header.Get("Content-Type"),
		SizeBytes:    fileHeader.Size,
		Body:         file,
	})
	respondSubmissionAccessError(c, err, func() { c.JSON(http.StatusCreated, result) })
}

func (h *Handler) DownloadFile(c *gin.Context) {
	fileID, ok := parseUUIDParam(c, "id", "INVALID_FILE_ID", "file id must be a valid UUID")
	if !ok {
		return
	}
	userID, role, ok := currentUser(c)
	if !ok {
		return
	}

	url, filename, err := h.service.AuthorizeDownload(c.Request.Context(), userID, fileID, func(courseID uuid.UUID) (bool, error) {
		return h.ownership.CanManageCourse(c.Request.Context(), userID, role, courseID)
	})
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "FILE_NOT_FOUND", "file not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to authorize download")
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url, "filename": filename})
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

func (h *Handler) requireAssignmentOwnership(c *gin.Context) (uuid.UUID, bool) {
	assignmentID, ok := parseUUIDParam(c, "id", "INVALID_ASSIGNMENT_ID", "assignment id must be a valid UUID")
	if !ok {
		return uuid.UUID{}, false
	}
	courseID, err := h.ownership.CourseIDForAssignment(c.Request.Context(), assignmentID)
	if errors.Is(err, ownership.ErrNotFound) {
		respondError(c, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "assignment not found")
		return uuid.UUID{}, false
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resolve assignment")
		return uuid.UUID{}, false
	}
	if !h.checkOwnership(c, courseID) {
		return uuid.UUID{}, false
	}
	return assignmentID, true
}

func (h *Handler) requireSubmissionOwnership(c *gin.Context) (uuid.UUID, bool) {
	submissionID, ok := parseUUIDParam(c, "id", "INVALID_SUBMISSION_ID", "submission id must be a valid UUID")
	if !ok {
		return uuid.UUID{}, false
	}
	courseID, err := h.ownership.CourseIDForSubmission(c.Request.Context(), submissionID)
	if errors.Is(err, ownership.ErrNotFound) {
		respondError(c, http.StatusNotFound, "SUBMISSION_NOT_FOUND", "submission not found")
		return uuid.UUID{}, false
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resolve submission")
		return uuid.UUID{}, false
	}
	if !h.checkOwnership(c, courseID) {
		return uuid.UUID{}, false
	}
	return submissionID, true
}

// RegisterInstructorRoutes wires the instructor-facing surface. The caller
// (main.go) already gates the whole group with RequireAuth +
// RequireAnyRole("instructor", "admin") — every route below additionally
// checks per-course ownership via the require*Ownership helpers, so a
// foreign instructor never reaches another instructor's lesson/assignment/
// submission (item 21's IDOR concern).
func (h *Handler) RegisterInstructorRoutes(rg *gin.RouterGroup) {
	rg.POST("/lessons/:id/assignment", h.CreateAssignment)
	rg.GET("/lessons/:id/assignment", h.GetInstructorAssignment)
	rg.PUT("/assignments/:id", h.UpdateAssignment)
	rg.DELETE("/assignments/:id", h.DeleteAssignment)
	rg.GET("/assignments/:id/submissions", h.ListAssignmentSubmissions)
	rg.GET("/submissions", h.ListMySubmissions)
	rg.GET("/submissions/:id", h.GetSubmissionDetail)
	rg.POST("/submissions/:id/review", h.ReviewSubmission)
}

type assignmentRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	Instructions string `json:"instructions"`
	Required     bool   `json:"required"`
	MaxScore     *int   `json:"max_score"`
	Published    bool   `json:"published"`
}

func respondAssignmentMutation(c *gin.Context, a *Assignment, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, a)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrDuplicateLesson):
		respondError(c, http.StatusConflict, "ASSIGNMENT_EXISTS", "this lesson already has an assignment")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "assignment not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save assignment")
	}
}

func (h *Handler) CreateAssignment(c *gin.Context) {
	lessonID, ok := h.requireLessonOwnership(c)
	if !ok {
		return
	}

	var req assignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	a, err := h.service.CreateAssignment(c.Request.Context(), lessonID, AssignmentInput(req))
	respondAssignmentMutation(c, a, err, http.StatusCreated)
}

func (h *Handler) GetInstructorAssignment(c *gin.Context) {
	lessonID, ok := h.requireLessonOwnership(c)
	if !ok {
		return
	}

	a, err := h.service.GetAssignmentByLesson(c.Request.Context(), lessonID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "this lesson has no assignment yet")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load assignment")
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *Handler) UpdateAssignment(c *gin.Context) {
	assignmentID, ok := h.requireAssignmentOwnership(c)
	if !ok {
		return
	}

	var req assignmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	a, err := h.service.UpdateAssignment(c.Request.Context(), assignmentID, AssignmentInput(req))
	respondAssignmentMutation(c, a, err, http.StatusOK)
}

func (h *Handler) DeleteAssignment(c *gin.Context) {
	assignmentID, ok := h.requireAssignmentOwnership(c)
	if !ok {
		return
	}

	err := h.service.DeleteAssignment(c.Request.Context(), assignmentID)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "ASSIGNMENT_NOT_FOUND", "assignment not found")
	case errors.Is(err, ErrHasSubmissions):
		respondError(c, http.StatusConflict, "ASSIGNMENT_HAS_SUBMISSIONS",
			"this assignment has student submissions; unpublish it instead of deleting")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete assignment")
	}
}

func (h *Handler) ListAssignmentSubmissions(c *gin.Context) {
	assignmentID, ok := h.requireAssignmentOwnership(c)
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, total, err := h.service.ListSubmissions(c.Request.Context(), ListSubmissionsParams{
		AssignmentID: &assignmentID,
		Status:       c.Query("status"),
		Page:         page,
		Limit:        limit,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list submissions")
		return
	}
	c.JSON(http.StatusOK, pagination.New(result, page, limit, total))
}

// ListMySubmissions is the global inbox (item 21/22): every submission
// across every course this instructor owns, or — for an admin caller —
// every submission platform-wide. Scoping is by role, not by a client-
// supplied flag, so an instructor can never pass course_id=<not mine> and
// see through to it (requireAssignmentOwnership-style checks don't apply
// here since this is a list, not a single resource, but the InstructorID
// scope in the query achieves the same isolation).
func (h *Handler) ListMySubmissions(c *gin.Context) {
	userID, role, ok := currentUser(c)
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	params := ListSubmissionsParams{
		Status: c.Query("status"),
		Page:   page,
		Limit:  limit,
	}
	if role != "admin" {
		params.InstructorID = &userID
	}
	if courseIDStr := c.Query("course_id"); courseIDStr != "" {
		courseID, err := uuid.Parse(courseIDStr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course_id must be a valid UUID")
			return
		}
		if role != "admin" {
			can, err := h.ownership.CanManageCourse(c.Request.Context(), userID, role, courseID)
			if err != nil {
				respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify course ownership")
				return
			}
			if !can {
				respondError(c, http.StatusForbidden, "FORBIDDEN", "you do not manage this course")
				return
			}
		}
		params.CourseID = &courseID
	}

	result, total, err := h.service.ListSubmissions(c.Request.Context(), params)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list submissions")
		return
	}
	c.JSON(http.StatusOK, pagination.New(result, page, limit, total))
}

func (h *Handler) GetSubmissionDetail(c *gin.Context) {
	submissionID, ok := h.requireSubmissionOwnership(c)
	if !ok {
		return
	}

	detail, err := h.service.GetSubmissionDetail(c.Request.Context(), submissionID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "SUBMISSION_NOT_FOUND", "submission not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load submission")
		return
	}
	c.JSON(http.StatusOK, detail)
}

type reviewRequest struct {
	Status   string `json:"status"`
	Score    *int   `json:"score"`
	Feedback string `json:"feedback"`
}

func (h *Handler) ReviewSubmission(c *gin.Context) {
	submissionID, ok := h.requireSubmissionOwnership(c)
	if !ok {
		return
	}
	reviewerID, _, ok := currentUser(c)
	if !ok {
		return
	}

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	submission, err := h.service.Review(c.Request.Context(), submissionID, reviewerID, ReviewInput(req))
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, submission)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "SUBMISSION_NOT_FOUND", "submission not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save review")
	}
}
