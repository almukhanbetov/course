package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes wires the public, unauthenticated GET /health — deliberately
// unchanged in shape and access level since before Stage 29A3 (instruction
// 2/5): a boolean-ish ok/degraded status and nothing else, safe to expose
// to anyone.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/health", h.GetHealth)
}

// RegisterAdminRoutes wires the deep health check — same admin-auth gate
// every other admin sub-router uses (see cmd/api/main.go's adminGroup),
// never reachable without it. This is the only place the richer
// per-component breakdown (Stage 29A3's actual new capability) is ever
// served.
func (h *Handler) RegisterAdminRoutes(rg *gin.RouterGroup) {
	rg.GET("/system-health", h.GetDeepHealth)
}

func (h *Handler) GetHealth(c *gin.Context) {
	status := h.service.Check(c.Request.Context())

	code := http.StatusOK
	if status.Status != "ok" {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, status)
}

func (h *Handler) GetDeepHealth(c *gin.Context) {
	status := h.service.CheckDeep(c.Request.Context())

	code := http.StatusOK
	if status.Status != "ok" {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, status)
}
