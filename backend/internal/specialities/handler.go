package specialities

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
	rg.GET("/specialities", h.ListSpecialities)
	rg.GET("/specialities/:id", h.GetSpeciality)
	rg.GET("/me/specialities/:id", requireAuth, h.GetMyRoadmap)
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func parseUUIDParam(c *gin.Context, name, code, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		respondError(c, http.StatusBadRequest, code, message)
		return uuid.UUID{}, false
	}
	return id, true
}

func (h *Handler) ListSpecialities(c *gin.Context) {
	list, err := h.service.ListPublished(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list specialities")
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) GetSpeciality(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id", "INVALID_SPECIALITY_ID", "speciality id must be a valid UUID")
	if !ok {
		return
	}

	detail, err := h.service.GetSpecialityDetail(c.Request.Context(), id)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, detail)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "SPECIALITY_NOT_FOUND", "speciality not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get speciality")
	}
}

func (h *Handler) GetMyRoadmap(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	id, ok := parseUUIDParam(c, "id", "INVALID_SPECIALITY_ID", "speciality id must be a valid UUID")
	if !ok {
		return
	}

	roadmap, err := h.service.GetMyRoadmap(c.Request.Context(), userID, id)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, roadmap)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "SPECIALITY_NOT_FOUND", "speciality not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get roadmap")
	}
}
