package courses

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires only the public, read-only catalog. Mutations
// (create/update/delete) require admin auth and are wired via
// RegisterAdminRoutes instead — see handler_admin.go. This closes a gap
// from an earlier stage where these mutation routes had no auth at all.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/courses", h.ListCourses)
	rg.GET("/courses/:id", h.GetCourse)
	rg.GET("/search/suggestions", h.SuggestCourses)
}

type courseRequest struct {
	Title        string     `json:"title"`
	Slug         string     `json:"slug"`
	Description  string     `json:"description"`
	Level        string     `json:"level"`
	ImageURL     string     `json:"image_url"`
	Published    bool       `json:"published"`
	AccessType   string     `json:"access_type"`
	CategoryID   *uuid.UUID `json:"category_id"`
	InstructorID *uuid.UUID `json:"instructor_id"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func parseCourseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course id must be a valid UUID")
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *Handler) ListCourses(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))

	params := ListCoursesParams{
		Query:        c.Query("q"),
		CategorySlug: c.Query("category"),
		Level:        c.Query("level"),
		AccessType:   c.Query("access_type"),
		Sort:         c.Query("sort"),
		Page:         page,
		Limit:        limit,
	}

	result, err := h.service.SearchCourses(c.Request.Context(), params)
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, result)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list courses")
	}
}

// SuggestCourses backs search-as-you-type (Stage 22A2): a lightweight,
// public, unauthenticated endpoint wired directly onto Service.SuggestCourses
// (Stage 22A1) — no new validation here, since the service already owns
// normalizing/bounding the raw "q" query param (trim, empty-query
// short-circuit to zero results, length cap) exactly the way ListCourses
// hands its own "q" straight to SearchCourses. A missing/empty "q" is not
// an error: it safely returns an empty list, matching a dropdown with
// nothing typed yet having nothing to suggest.
func (h *Handler) SuggestCourses(c *gin.Context) {
	suggestions, err := h.service.SuggestCourses(c.Request.Context(), c.Query("q"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load suggestions")
		return
	}
	c.JSON(http.StatusOK, suggestions)
}

func (h *Handler) GetCourse(c *gin.Context) {
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

func (h *Handler) CreateCourse(c *gin.Context) {
	var req courseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	created, err := h.service.CreateCourse(c.Request.Context(), CourseInput(req))
	h.respondCourseMutation(c, created, err, http.StatusCreated)
}

func (h *Handler) UpdateCourse(c *gin.Context) {
	id, ok := parseCourseID(c)
	if !ok {
		return
	}

	var req courseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	updated, err := h.service.UpdateCourse(c.Request.Context(), id, CourseInput(req))
	h.respondCourseMutation(c, updated, err, http.StatusOK)
}

func (h *Handler) respondCourseMutation(c *gin.Context, course *Course, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, course)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrDuplicateSlug):
		respondError(c, http.StatusConflict, "COURSE_SLUG_EXISTS", "a course with this slug already exists")
	case errors.Is(err, ErrCategoryNotFound):
		respondError(c, http.StatusBadRequest, "CATEGORY_NOT_FOUND", "category_id does not reference an existing category")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save course")
	}
}

func (h *Handler) DeleteCourse(c *gin.Context) {
	id, ok := parseCourseID(c)
	if !ok {
		return
	}

	err := h.service.DeleteCourse(c.Request.Context(), id)
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
	case errors.Is(err, ErrHasEnrollments):
		respondError(c, http.StatusConflict, "COURSE_HAS_ENROLLMENTS",
			"this course has active student enrollments; unpublish it instead of deleting")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete course")
	}
}
