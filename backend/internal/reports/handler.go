package reports

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

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

// RegisterRoutes wires the one Stage 24A2 endpoint onto the bare,
// auth-required /api/v1 group — the same pattern internal/qa's
// student-facing routes and internal/recommendations' feedback routes
// already use. One generic POST /reports, not three content-type-specific
// paths (contrast the roadmap's original POST /questions/:id/report
// sketch): content_type/content_id both travel in the body alongside
// reason, matching Stage 24A1's own polymorphic (content_type, content_id)
// storage design exactly — a single handler reaches every content type via
// the same Service.CreateReport already built for that.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.POST("/reports", requireAuth, h.CreateReport)
}

type createReportRequest struct {
	ContentType string `json:"content_type"`
	ContentID   string `json:"content_id"`
	Reason      string `json:"reason"`
}

// CreateReport backs POST /reports. reporterUserID is read exclusively
// from the verified JWT via authctx.UserID — never a body field (there is
// no reporter_user_id/user_id field anywhere in createReportRequest for a
// client to even attempt to supply one), the same "identity only from the
// verified token" rule every other authenticated write in this codebase
// follows (internal/qa, internal/recommendations, ...).
//
// content_id is bound as a plain string, not uuid.UUID, specifically so a
// malformed value fails with its own distinct 400 (INVALID_CONTENT_ID)
// rather than being indistinguishable from any other body-binding failure
// (INVALID_BODY) — binding it directly as uuid.UUID would make gin reject
// the whole request during ShouldBindJSON with one generic error, losing
// that distinction.
func (h *Handler) CreateReport(c *gin.Context) {
	userID, ok := authctx.UserID(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid authorization header")
		return
	}

	var req createReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	contentID, err := uuid.Parse(req.ContentID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_CONTENT_ID", "content_id must be a valid UUID")
		return
	}

	report, err := h.service.CreateReport(c.Request.Context(), userID, req.ContentType, contentID, req.Reason)
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, report)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrContentNotFound):
		respondError(c, http.StatusNotFound, "CONTENT_NOT_FOUND", "reported content not found")
	case errors.Is(err, ErrDuplicateActiveReport):
		respondError(c, http.StatusConflict, "DUPLICATE_REPORT", "you already have an open report for this content")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to submit report")
	}
}
