package certificates

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
	rg.GET("/me/certificates", requireAuth, h.ListMyCertificates)
	rg.GET("/me/certificates/:id", requireAuth, h.GetMyCertificate)
	rg.GET("/certificates/verify/:certificateNumber", h.VerifyCertificate)
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return uuid.UUID{}, false
	}
	return userID, true
}

func (h *Handler) ListMyCertificates(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	list, err := h.service.ListMyCertificates(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list certificates")
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *Handler) GetMyCertificate(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	certID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_CERTIFICATE_ID", "certificate id must be a valid UUID")
		return
	}

	detail, err := h.service.GetMyCertificate(c.Request.Context(), userID, certID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, detail)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "CERTIFICATE_NOT_FOUND", "certificate not found")
	case errors.Is(err, ErrForbidden):
		respondError(c, http.StatusForbidden, "FORBIDDEN", ErrForbidden.Error())
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get certificate")
	}
}

// VerifyCertificate always answers 200 with a valid:true/false body — this
// is the one contract public callers need to handle, with no separate
// not-found status code to special-case.
func (h *Handler) VerifyCertificate(c *gin.Context) {
	number := c.Param("certificateNumber")

	result, err := h.service.VerifyCertificate(c.Request.Context(), number)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify certificate")
		return
	}
	c.JSON(http.StatusOK, result)
}
