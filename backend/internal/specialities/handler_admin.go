package specialities

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/specialities", h.ListSpecialitiesAdmin)
	admin.POST("/specialities", h.CreateSpeciality)
	admin.PUT("/specialities/:id", h.UpdateSpeciality)
	admin.DELETE("/specialities/:id", h.DeleteSpeciality)

	admin.POST("/specialities/:id/courses", h.AddCourseToSpeciality)
	admin.PUT("/specialities/:id/courses/reorder", h.ReorderSpecialityCourses)
	admin.DELETE("/specialities/:id/courses/:courseId", h.RemoveCourseFromSpeciality)
}

func (h *Handler) ListSpecialitiesAdmin(c *gin.Context) {
	list, err := h.service.ListAllAdmin(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list specialities")
		return
	}
	c.JSON(http.StatusOK, list)
}

type specialityRequest struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Published   bool   `json:"published"`
}

func respondSpecialityMutation(c *gin.Context, speciality *Speciality, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, speciality)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrDuplicateSlug):
		respondError(c, http.StatusConflict, "SPECIALITY_SLUG_EXISTS", "a speciality with this slug already exists")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "SPECIALITY_NOT_FOUND", "speciality not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save speciality")
	}
}

func (h *Handler) CreateSpeciality(c *gin.Context) {
	var req specialityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	created, err := h.service.CreateSpeciality(c.Request.Context(), SpecialityInput(req))
	respondSpecialityMutation(c, created, err, http.StatusCreated)
}

func (h *Handler) UpdateSpeciality(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_SPECIALITY_ID", "speciality id must be a valid UUID")
	if !ok {
		return
	}

	var req specialityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	updated, err := h.service.UpdateSpeciality(c.Request.Context(), id, SpecialityInput(req))
	respondSpecialityMutation(c, updated, err, http.StatusOK)
}

func (h *Handler) DeleteSpeciality(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_SPECIALITY_ID", "speciality id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.DeleteSpeciality(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "SPECIALITY_NOT_FOUND", "speciality not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete speciality")
		return
	}
	c.Status(http.StatusNoContent)
}

type addCourseRequest struct {
	CourseID string `json:"course_id"`
	Required bool   `json:"required"`
}

func (h *Handler) AddCourseToSpeciality(c *gin.Context) {
	specialityID, ok := parseUUIDParam(c, "id", "INVALID_SPECIALITY_ID", "speciality id must be a valid UUID")
	if !ok {
		return
	}

	var req addCourseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}
	courseID, err := uuid.Parse(req.CourseID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_COURSE_ID", "course_id must be a valid UUID")
		return
	}

	sc, err := h.service.AddCourseToSpeciality(c.Request.Context(), specialityID, courseID, req.Required)
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, sc)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "SPECIALITY_NOT_FOUND", "speciality not found")
	case errors.Is(err, ErrCourseNotFound):
		respondError(c, http.StatusNotFound, "COURSE_NOT_FOUND", "course not found")
	case errors.Is(err, ErrAlreadyInRoadmap):
		respondError(c, http.StatusConflict, "COURSE_ALREADY_IN_ROADMAP", "this course is already part of the roadmap")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to add course to speciality")
	}
}

func (h *Handler) RemoveCourseFromSpeciality(c *gin.Context) {
	specialityID, ok := parseUUIDParam(c, "id", "INVALID_SPECIALITY_ID", "speciality id must be a valid UUID")
	if !ok {
		return
	}
	courseID, ok := parseUUIDParam(c, "courseId", "INVALID_COURSE_ID", "course id must be a valid UUID")
	if !ok {
		return
	}

	err := h.service.RemoveCourseFromSpeciality(c.Request.Context(), specialityID, courseID)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "this course is not part of the roadmap")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to remove course from speciality")
		return
	}
	c.Status(http.StatusNoContent)
}

type roadmapReorderRequest struct {
	Items []struct {
		CourseID string `json:"course_id"`
		Position int    `json:"position"`
		Required bool   `json:"required"`
	} `json:"items"`
}

func (h *Handler) ReorderSpecialityCourses(c *gin.Context) {
	specialityID, ok := parseUUIDParam(c, "id", "INVALID_SPECIALITY_ID", "speciality id must be a valid UUID")
	if !ok {
		return
	}

	var req roadmapReorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	items := make([]RoadmapReorderItem, len(req.Items))
	for i, it := range req.Items {
		courseID, err := uuid.Parse(it.CourseID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_BODY", "items contain an invalid course_id")
			return
		}
		items[i] = RoadmapReorderItem{CourseID: courseID, Position: it.Position, Required: it.Required}
	}

	err := h.service.ReorderSpecialityCourses(c.Request.Context(), specialityID, items)

	var validationErr *ValidationError
	switch {
	case err == nil:
		c.Status(http.StatusNoContent)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "SPECIALITY_NOT_FOUND", "speciality not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to reorder roadmap")
	}
}
