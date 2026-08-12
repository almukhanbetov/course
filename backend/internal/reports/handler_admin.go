package reports

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

// RegisterAdminRoutes exposes the moderation queue only: list (with
// optional status/content_type filters) and change a report's status.
// Mounted onto the /admin group in cmd/api/main.go, which already gates
// every route on it to auth+admin-role (the single choke point every
// other admin endpoint in this codebase goes through) — non-admin/
// unauthenticated rejection is enforced there, not re-implemented here.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/reports", h.ListReportsAdmin)
	admin.PATCH("/reports/:id", h.UpdateReportStatus)
}

func (h *Handler) ListReportsAdmin(c *gin.Context) {
	params := AdminListParams{
		Status:      c.Query("status"),
		ContentType: c.Query("content_type"),
	}
	params.Page, params.Limit = pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListAdmin(c.Request.Context(), params)
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, result)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list reports")
	}
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) UpdateReportStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REPORT_ID", "report id must be a valid UUID")
		return
	}

	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	report, err := h.service.UpdateStatus(c.Request.Context(), id, req.Status)
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusOK, report)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "REPORT_NOT_FOUND", "report not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update report")
	}
}
