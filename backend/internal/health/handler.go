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

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/health", h.GetHealth)
}

func (h *Handler) GetHealth(c *gin.Context) {
	status := h.service.Check(c.Request.Context())

	code := http.StatusOK
	if status.Status != "ok" {
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, status)
}
