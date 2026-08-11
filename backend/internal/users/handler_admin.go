package users

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/users", h.ListUsersAdmin)
	admin.GET("/users/:id", h.GetUserAdmin)
	admin.PUT("/users/:id", h.UpdateUserAdmin)
}

func (h *Handler) ListUsersAdmin(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))
	search := c.Query("search")

	list, total, err := h.service.ListAdmin(c.Request.Context(), search, limit, pagination.Offset(page, limit))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list users")
		return
	}

	items := make([]PublicUser, len(list))
	for i, u := range list {
		items[i] = u.Public()
	}

	c.JSON(http.StatusOK, pagination.New(items, page, limit, total))
}

func (h *Handler) GetUserAdmin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_USER_ID", "user id must be a valid UUID")
		return
	}

	user, err := h.service.GetByID(c.Request.Context(), id)
	if errors.Is(err, ErrNotFound) {
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get user")
		return
	}
	c.JSON(http.StatusOK, user.Public())
}

type updateUserAdminRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Active    bool   `json:"active"`
	Role      string `json:"role"`
}

func (h *Handler) UpdateUserAdmin(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_USER_ID", "user id must be a valid UUID")
		return
	}

	var req updateUserAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	updated, err := h.service.UpdateAdmin(c.Request.Context(), id, UpdateAdminInput{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Active:    req.Active,
		RoleName:  req.Role,
	})

	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, updated.Public())
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update user")
	}
}
