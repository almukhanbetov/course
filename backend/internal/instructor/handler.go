package instructor

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/authctx"
	"lms-backend/internal/courses"
	"lms-backend/internal/ownership"
	"lms-backend/internal/pagination"
	"lms-backend/internal/tests"
	"lms-backend/internal/videos"
)

// Handler is a thin authorization+routing layer, not a reimplementation of
// module/lesson/video/test CRUD. Wherever the underlying write is already
// safe against an instructor caller (it never lets the caller set an
// owner/publication field it shouldn't), this delegates straight to the
// existing courses/videos/tests handlers — see RegisterRoutes for exactly
// which routes do that vs. which need their own narrow DTO.
type Handler struct {
	service        *Service
	coursesService *courses.Service
	coursesHandler *courses.Handler
	videosHandler  *videos.Handler
	testsService   *tests.Service
	testsHandler   *tests.Handler
	ownership      *ownership.Service
}

func NewHandler(
	service *Service,
	coursesService *courses.Service,
	coursesHandler *courses.Handler,
	videosHandler *videos.Handler,
	testsService *tests.Service,
	testsHandler *tests.Handler,
	ownershipService *ownership.Service,
) *Handler {
	return &Handler{
		service:        service,
		coursesService: coursesService,
		coursesHandler: coursesHandler,
		videosHandler:  videosHandler,
		testsService:   testsService,
		testsHandler:   testsHandler,
		ownership:      ownershipService,
	}
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

// checkOwnership is the one rule every write in this file ultimately
// depends on: admin manages everything, an instructor manages only their
// own courseID, a student manages nothing. See internal/ownership.
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

// --- ownership-resolving middleware --------------------------------------
// One per "what kind of id does :id name" — each resolves the owning course
// through the full chain (see internal/ownership) before checkOwnership, so
// a module/lesson/test/question/answer id can never be substituted across
// an ownership boundary (the IDOR concern item 21 calls out explicitly).

func (h *Handler) requireCourseOwnership(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		courseID, ok := parseUUIDParam(c, param, "INVALID_COURSE_ID", "course id must be a valid UUID")
		if !ok {
			return
		}
		if !h.checkOwnership(c, courseID) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) requireModuleOwnership(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		moduleID, ok := parseUUIDParam(c, param, "INVALID_MODULE_ID", "module id must be a valid UUID")
		if !ok {
			return
		}
		courseID, err := h.ownership.CourseIDForModule(c.Request.Context(), moduleID)
		if !resolveOr404(c, err, "MODULE_NOT_FOUND", "module not found") {
			return
		}
		if !h.checkOwnership(c, courseID) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) requireLessonOwnership(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		lessonID, ok := parseUUIDParam(c, param, "INVALID_LESSON_ID", "lesson id must be a valid UUID")
		if !ok {
			return
		}
		courseID, err := h.ownership.CourseIDForLesson(c.Request.Context(), lessonID)
		if !resolveOr404(c, err, "LESSON_NOT_FOUND", "lesson not found") {
			return
		}
		if !h.checkOwnership(c, courseID) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) requireTestOwnership(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		testID, ok := parseUUIDParam(c, param, "INVALID_TEST_ID", "test id must be a valid UUID")
		if !ok {
			return
		}
		courseID, err := h.ownership.CourseIDForTest(c.Request.Context(), testID)
		if !resolveOr404(c, err, "TEST_NOT_FOUND", "test not found") {
			return
		}
		if !h.checkOwnership(c, courseID) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) requireQuestionOwnership(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		questionID, ok := parseUUIDParam(c, param, "INVALID_QUESTION_ID", "question id must be a valid UUID")
		if !ok {
			return
		}
		courseID, err := h.ownership.CourseIDForQuestion(c.Request.Context(), questionID)
		if !resolveOr404(c, err, "QUESTION_NOT_FOUND", "question not found") {
			return
		}
		if !h.checkOwnership(c, courseID) {
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *Handler) requireAnswerOwnership(param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		answerID, ok := parseUUIDParam(c, param, "INVALID_ANSWER_ID", "answer id must be a valid UUID")
		if !ok {
			return
		}
		courseID, err := h.ownership.CourseIDForAnswer(c.Request.Context(), answerID)
		if !resolveOr404(c, err, "ANSWER_NOT_FOUND", "answer not found") {
			return
		}
		if !h.checkOwnership(c, courseID) {
			c.Abort()
			return
		}
		c.Next()
	}
}

// resolveOr404 centralizes the "did the chain resolver find a parent at
// all" handling shared by every require*Ownership middleware above.
func resolveOr404(c *gin.Context, err error, code, message string) bool {
	if errors.Is(err, ownership.ErrNotFound) {
		respondError(c, http.StatusNotFound, code, message)
		c.Abort()
		return false
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to resolve resource")
		c.Abort()
		return false
	}
	return true
}

// RegisterRoutes wires the full /api/v1/instructor surface. The caller
// (main.go) gates the whole group with RequireAuth + RequireAnyRole
// ("instructor", "admin") — nothing here re-checks that base role, only the
// finer-grained per-course ownership above it.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// Courses: a deliberately narrow DTO (courses.InstructorCourseInput has
	// no instructor_id/published/publication_status field at all), so this
	// can never reuse courses.Handler's admin CreateCourse/UpdateCourse.
	rg.GET("/courses", h.ListMyCourses)
	rg.POST("/courses", h.CreateMyCourse)
	rg.GET("/courses/:id", h.requireCourseOwnership("id"), h.GetMyCourse)
	rg.PUT("/courses/:id", h.requireCourseOwnership("id"), h.UpdateMyCourse)
	rg.POST("/courses/:id/submit", h.requireCourseOwnership("id"), h.SubmitCourseForReview)

	// Modules/lessons: courses.Handler's methods never touch anything an
	// instructor shouldn't be able to set, so they're reused verbatim —
	// only the ownership gate in front is new.
	rg.POST("/courses/:id/modules", h.requireCourseOwnership("id"), h.coursesHandler.CreateModule)
	rg.PUT("/courses/:id/modules/reorder", h.requireCourseOwnership("id"), h.coursesHandler.ReorderModules)
	rg.PUT("/modules/:id", h.requireModuleOwnership("id"), h.coursesHandler.UpdateModule)
	rg.DELETE("/modules/:id", h.requireModuleOwnership("id"), h.coursesHandler.DeleteModule)
	rg.POST("/modules/:id/lessons", h.requireModuleOwnership("id"), h.coursesHandler.CreateLesson)
	rg.PUT("/modules/:id/lessons/reorder", h.requireModuleOwnership("id"), h.coursesHandler.ReorderLessons)
	rg.PUT("/lessons/:id", h.requireLessonOwnership("id"), h.coursesHandler.UpdateLesson)
	rg.DELETE("/lessons/:id", h.requireLessonOwnership("id"), h.coursesHandler.DeleteLesson)

	// Video: same HLS/transcoding pipeline as Admin CMS, zero copied logic —
	// only the ownership gate is new (item 7).
	rg.POST("/lessons/:id/video", h.requireLessonOwnership("id"), h.videosHandler.UploadVideo)
	rg.GET("/lessons/:id/video", h.requireLessonOwnership("id"), h.videosHandler.GetVideoMetadata)
	rg.GET("/lessons/:id/video/status", h.requireLessonOwnership("id"), h.videosHandler.GetProcessingStatus)
	rg.DELETE("/lessons/:id/video", h.requireLessonOwnership("id"), h.videosHandler.DeleteVideo)

	// Tests: CreateTest names its parent course/module/lesson in the body,
	// not a URL param, so it's the one case that needs its own handler
	// below (see CreateTest's doc comment). Everything else identifies its
	// target by an existing id and reuses tests.Handler verbatim.
	rg.GET("/courses/:id/tests", h.requireCourseOwnership("id"), h.ListCourseTests)
	rg.POST("/tests", h.CreateTest)
	rg.GET("/tests/:id", h.requireTestOwnership("id"), h.testsHandler.GetTestAdmin)
	rg.PUT("/tests/:id", h.requireTestOwnership("id"), h.testsHandler.UpdateTest)
	rg.DELETE("/tests/:id", h.requireTestOwnership("id"), h.testsHandler.DeleteTest)
	rg.POST("/tests/:id/questions", h.requireTestOwnership("id"), h.testsHandler.CreateQuestion)
	rg.PUT("/questions/:id", h.requireQuestionOwnership("id"), h.testsHandler.UpdateQuestion)
	rg.DELETE("/questions/:id", h.requireQuestionOwnership("id"), h.testsHandler.DeleteQuestion)
	rg.POST("/questions/:id/answers", h.requireQuestionOwnership("id"), h.testsHandler.CreateAnswer)
	rg.PUT("/answers/:id", h.requireAnswerOwnership("id"), h.testsHandler.UpdateAnswer)
	rg.DELETE("/answers/:id", h.requireAnswerOwnership("id"), h.testsHandler.DeleteAnswer)

	// Students & analytics — all new, instructor-only aggregation queries.
	rg.GET("/students", h.ListStudents)
	rg.GET("/courses/:id/students", h.requireCourseOwnership("id"), h.ListCourseStudents)
	rg.GET("/stats", h.GetStats)
	rg.GET("/courses/:id/stats", h.requireCourseOwnership("id"), h.GetCourseStats)
	rg.GET("/courses/:id/reviews", h.requireCourseOwnership("id"), h.ListCourseReviews)
}

// --- courses -------------------------------------------------------------

type courseRequest struct {
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Description string     `json:"description"`
	Level       string     `json:"level"`
	ImageURL    string     `json:"image_url"`
	AccessType  string     `json:"access_type"`
	CategoryID  *uuid.UUID `json:"category_id"`
}

func (h *Handler) ListMyCourses(c *gin.Context) {
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.coursesService.ListByInstructor(c.Request.Context(), userID, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list courses")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) CreateMyCourse(c *gin.Context) {
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	var req courseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	course, err := h.coursesService.CreateForInstructor(c.Request.Context(), userID, courses.InstructorCourseInput(req))
	h.respondCourseMutation(c, course, err, http.StatusCreated)
}

func (h *Handler) GetMyCourse(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	detail, err := h.coursesService.GetCourseDetail(c.Request.Context(), id)
	if errors.Is(err, courses.ErrNotFound) {
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get course")
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *Handler) UpdateMyCourse(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	var req courseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	course, err := h.coursesService.UpdateForInstructor(c.Request.Context(), id, userID, courses.InstructorCourseInput(req))
	h.respondCourseMutation(c, course, err, http.StatusOK)
}

func (h *Handler) respondCourseMutation(c *gin.Context, course *courses.Course, err error, successStatus int) {
	var validationErr *courses.ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, course)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, courses.ErrDuplicateSlug):
		respondError(c, http.StatusConflict, "COURSE_SLUG_EXISTS", "a course with this slug already exists")
	case errors.Is(err, courses.ErrCategoryNotFound):
		respondError(c, http.StatusBadRequest, "CATEGORY_NOT_FOUND", "category_id does not reference an existing category")
	case errors.Is(err, courses.ErrNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save course")
	}
}

func (h *Handler) SubmitCourseForReview(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	course, err := h.coursesService.SubmitForReview(c.Request.Context(), id, userID)
	if errors.Is(err, courses.ErrNotFound) {
		respondError(c, http.StatusConflict, "INVALID_STATUS", "course must be a draft or rejected to submit for review")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to submit course for review")
		return
	}
	c.JSON(http.StatusOK, course)
}

// --- tests -----------------------------------------------------------

func (h *Handler) ListCourseTests(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	list, err := h.service.ListCourseTests(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list tests")
		return
	}
	c.JSON(http.StatusOK, list)
}

type instructorTestRequest struct {
	CourseID     *string `json:"course_id"`
	ModuleID     *string `json:"module_id"`
	LessonID     *string `json:"lesson_id"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	PassingScore int     `json:"passing_score"`
	Published    bool    `json:"published"`
	IsFinal      bool    `json:"is_final"`
}

// CreateTest is the one write in this package that can't be gated by a URL
// param alone — the test's parent (course, module, or lesson) is named in
// the request body. So it resolves and checks ownership of that named
// parent itself, before ever calling into tests.Service, instead of relying
// on a route-level middleware like every other handler here does.
func (h *Handler) CreateTest(c *gin.Context) {
	userID, role, ok := currentUser(c)
	if !ok {
		return
	}

	var req instructorTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	parseOptional := func(s *string) (*uuid.UUID, bool) {
		if s == nil || *s == "" {
			return nil, true
		}
		id, err := uuid.Parse(*s)
		if err != nil {
			return nil, false
		}
		return &id, true
	}

	courseIDPtr, ok1 := parseOptional(req.CourseID)
	moduleIDPtr, ok2 := parseOptional(req.ModuleID)
	lessonIDPtr, ok3 := parseOptional(req.LessonID)
	if !ok1 || !ok2 || !ok3 {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "course_id/module_id/lesson_id must be valid UUIDs")
		return
	}

	var targetCourseID uuid.UUID
	switch {
	case courseIDPtr != nil:
		targetCourseID = *courseIDPtr
	case moduleIDPtr != nil:
		resolved, err := h.ownership.CourseIDForModule(c.Request.Context(), *moduleIDPtr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_BODY", "module_id does not reference an existing module")
			return
		}
		targetCourseID = resolved
	case lessonIDPtr != nil:
		resolved, err := h.ownership.CourseIDForLesson(c.Request.Context(), *lessonIDPtr)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_BODY", "lesson_id does not reference an existing lesson")
			return
		}
		targetCourseID = resolved
	default:
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "a test must specify course_id, module_id, or lesson_id")
		return
	}

	can, err := h.ownership.CanManageCourse(c.Request.Context(), userID, role, targetCourseID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify course ownership")
		return
	}
	if !can {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "you do not manage this course")
		return
	}

	test, err := h.testsService.CreateTest(c.Request.Context(), tests.TestInput{
		CourseID:     courseIDPtr,
		ModuleID:     moduleIDPtr,
		LessonID:     lessonIDPtr,
		Title:        req.Title,
		Description:  req.Description,
		PassingScore: req.PassingScore,
		Published:    req.Published,
		IsFinal:      req.IsFinal,
	})

	var validationErr *tests.ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, test)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, tests.ErrInvalidParent):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", tests.ErrInvalidParent.Error())
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create test")
	}
}

// --- students & analytics ---------------------------------------------

func (h *Handler) ListStudents(c *gin.Context) {
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListStudents(c.Request.Context(), userID, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list students")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ListCourseStudents(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListCourseStudents(c.Request.Context(), id, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list students")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) GetStats(c *gin.Context) {
	userID, _, ok := currentUser(c)
	if !ok {
		return
	}

	stats, err := h.service.Stats(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load stats")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) GetCourseStats(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	stats, err := h.service.CourseStats(c.Request.Context(), id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load course stats")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) ListCourseReviews(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListCourseReviews(c.Request.Context(), id, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list reviews")
		return
	}
	c.JSON(http.StatusOK, result)
}
