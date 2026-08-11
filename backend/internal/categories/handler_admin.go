package categories

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RegisterAdminRoutes deliberately has no DELETE — see Repository.Update's
// docs on why deactivation (PUT with active=false) is the only supported
// way to retire a category.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/categories", h.ListCategoriesAdmin)
	admin.POST("/categories", h.CreateCategory)
	admin.PUT("/categories/:id", h.UpdateCategory)
}

type categoryRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Position    int    `json:"position"`
	Active      bool   `json:"active"`
}

func (h *Handler) ListCategoriesAdmin(c *gin.Context) {
	list, err := h.service.ListAllAdmin(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list categories")
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	category, err := h.service.Create(c.Request.Context(), CategoryInput(req))
	respondMutation(c, category, err, http.StatusCreated)
}

func (h *Handler) UpdateCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_CATEGORY_ID", "category id must be a valid UUID")
		return
	}

	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	category, err := h.service.Update(c.Request.Context(), id, CategoryInput(req))
	respondMutation(c, category, err, http.StatusOK)
}

func respondMutation(c *gin.Context, category *Category, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, category)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrDuplicateSlug):
		respondError(c, http.StatusConflict, "CATEGORY_SLUG_EXISTS", "a category with this slug already exists")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "CATEGORY_NOT_FOUND", "category not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save category")
	}
}
