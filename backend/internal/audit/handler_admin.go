package audit

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lms-backend/internal/pagination"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// RegisterAdminRoutes exposes read-only access to the audit trail — list
// only, no create/update/delete route of any kind. The table is
// append-only (see model.go's doc comment); Service.Log is the only way
// any row is ever written, called only from other Go code (Stage 25A2's
// qa/reports call sites), never from an HTTP request body — there is
// nothing here for a client to write through even in principle.
//
// Mounted onto the /admin group in cmd/api/main.go, which already gates
// every route on it to auth+admin-role — the same choke point every other
// admin endpoint in this codebase goes through. Non-admin/unauthenticated
// rejection is enforced there, not re-implemented here.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/audit-log", h.ListAuditLog)
}

func (h *Handler) ListAuditLog(c *gin.Context) {
	params := AdminListParams{
		ActorRole:  c.Query("actor_role"),
		Action:     c.Query("action"),
		EntityType: c.Query("entity_type"),
	}
	params.Page, params.Limit = pagination.ParseParams(c.Query("page"), c.Query("limit"))

	result, err := h.service.ListAdmin(c.Request.Context(), params)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list audit log")
		return
	}
	c.JSON(http.StatusOK, result)
}
