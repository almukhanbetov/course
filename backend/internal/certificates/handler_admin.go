package certificates

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"lms-backend/internal/pagination"
)

// RegisterAdminRoutes wires a read-only list only — see ListAdmin docs.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/certificates", h.ListCertificatesAdmin)
}

func (h *Handler) ListCertificatesAdmin(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))
	search := c.Query("search")

	list, total, err := h.service.ListAdmin(c.Request.Context(), search, limit, pagination.Offset(page, limit))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list certificates")
		return
	}
	c.JSON(http.StatusOK, pagination.New(list, page, limit, total))
}
