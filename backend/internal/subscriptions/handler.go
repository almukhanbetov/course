package subscriptions

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

// RegisterRoutes wires the public plans catalog and the authenticated
// subscription/payment endpoints. The mock-confirm endpoint is only
// attached when the service is actually configured for the mock provider —
// see main.go — so it does not exist at all (404, not 403) in any
// deployment using a real provider.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	rg.GET("/plans", h.ListPlans)
	rg.POST("/subscriptions", requireAuth, h.CreateSubscription)
	rg.GET("/me/subscription", requireAuth, h.GetMySubscription)

	if h.service.MockProviderEnabled() {
		rg.POST("/payments/:id/mock-confirm", requireAuth, h.MockConfirmPayment)
	}
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

func (h *Handler) ListPlans(c *gin.Context) {
	plans, err := h.service.ListActivePlans(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list plans")
		return
	}
	c.JSON(http.StatusOK, plans)
}

type createSubscriptionRequest struct {
	PlanID string `json:"plan_id"`
}

func (h *Handler) CreateSubscription(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var req createSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}
	planID, err := uuid.Parse(req.PlanID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PLAN_ID", "plan_id must be a valid UUID")
		return
	}

	result, err := h.service.CreateSubscription(c.Request.Context(), userID, planID)
	switch {
	case err == nil:
		c.JSON(http.StatusCreated, result)
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "PLAN_NOT_FOUND", "plan not found")
	case errors.Is(err, ErrPlanInactive):
		respondError(c, http.StatusConflict, "PLAN_INACTIVE", "this plan is no longer available")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create subscription")
	}
}

func (h *Handler) GetMySubscription(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	sub, err := h.service.GetMySubscription(c.Request.Context(), userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get subscription")
		return
	}
	c.JSON(http.StatusOK, sub)
}

func (h *Handler) MockConfirmPayment(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	paymentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PAYMENT_ID", "payment id must be a valid UUID")
		return
	}

	payment, subscription, err := h.service.ConfirmMockPayment(c.Request.Context(), userID, paymentID)
	switch {
	case err == nil:
		c.JSON(http.StatusOK, gin.H{"payment": payment, "subscription": subscription})
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "PAYMENT_NOT_FOUND", "payment not found")
	case errors.Is(err, ErrForbidden):
		respondError(c, http.StatusForbidden, "FORBIDDEN", "you may not confirm another user's payment")
	case errors.Is(err, ErrWrongProvider):
		respondError(c, http.StatusConflict, "WRONG_PROVIDER", "this payment was not created with the mock provider")
	case errors.Is(err, ErrAlreadyProcessed):
		respondError(c, http.StatusConflict, "ALREADY_PROCESSED", "this payment has already been processed")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to confirm payment")
	}
}
