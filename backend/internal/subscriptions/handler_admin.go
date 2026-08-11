package subscriptions

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"lms-backend/internal/pagination"
)

// RegisterAdminRoutes deliberately has no DELETE for plans and no write
// routes at all for subscriptions/payments — see plan deactivation below and
// the package docs on why admins never set payment=paid directly.
func (h *Handler) RegisterAdminRoutes(admin *gin.RouterGroup) {
	admin.GET("/plans", h.ListPlansAdmin)
	admin.POST("/plans", h.CreatePlan)
	admin.PUT("/plans/:id", h.UpdatePlan)

	admin.GET("/subscriptions", h.ListSubscriptionsAdmin)
	admin.GET("/payments", h.ListPaymentsAdmin)
	admin.GET("/analytics/revenue", h.GetRevenueAnalytics)
}

type planRequest struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	PriceAmount  int64  `json:"price_amount"`
	Currency     string `json:"currency"`
	DurationDays int    `json:"duration_days"`
	Active       bool   `json:"active"`
}

func (h *Handler) ListPlansAdmin(c *gin.Context) {
	plans, err := h.service.ListAllPlansAdmin(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list plans")
		return
	}
	c.JSON(http.StatusOK, plans)
}

func (h *Handler) CreatePlan(c *gin.Context) {
	var req planRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	plan, err := h.service.CreatePlan(c.Request.Context(), PlanInput(req))
	respondPlanMutation(c, plan, err, http.StatusCreated)
}

func (h *Handler) UpdatePlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PLAN_ID", "plan id must be a valid UUID")
		return
	}

	var req planRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_BODY", "request body is invalid")
		return
	}

	plan, err := h.service.UpdatePlan(c.Request.Context(), id, PlanInput(req))
	respondPlanMutation(c, plan, err, http.StatusOK)
}

func respondPlanMutation(c *gin.Context, plan *Plan, err error, successStatus int) {
	var validationErr *ValidationError
	switch {
	case err == nil:
		c.JSON(successStatus, plan)
	case errors.As(err, &validationErr):
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", validationErr.Message)
	case errors.Is(err, ErrDuplicateSlug):
		respondError(c, http.StatusConflict, "PLAN_SLUG_EXISTS", "a plan with this slug already exists")
	case errors.Is(err, ErrNotFound):
		respondError(c, http.StatusNotFound, "PLAN_NOT_FOUND", "plan not found")
	default:
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to save plan")
	}
}

func (h *Handler) ListSubscriptionsAdmin(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))
	status := c.Query("status")

	list, total, err := h.service.ListSubscriptionsAdmin(c.Request.Context(), status, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list subscriptions")
		return
	}
	c.JSON(http.StatusOK, pagination.New(list, page, limit, total))
}

func (h *Handler) ListPaymentsAdmin(c *gin.Context) {
	page, limit := pagination.ParseParams(c.Query("page"), c.Query("limit"))
	status := c.Query("status")

	list, total, err := h.service.ListPaymentsAdmin(c.Request.Context(), status, page, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list payments")
		return
	}
	c.JSON(http.StatusOK, pagination.New(list, page, limit, total))
}

// GetRevenueAnalytics defaults to the trailing 30 days (a "this month"-ish
// window for MRR-style reporting) when from/to are omitted — mirrors the
// from/to convention in internal/activity.GetCalendar (YYYY-MM-DD, 400 on an
// unparseable value).
func (h *Handler) GetRevenueAnalytics(c *gin.Context) {
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	to := now
	if v := c.Query("from"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_FROM", "from must be YYYY-MM-DD")
			return
		}
		from = parsed
	}
	if v := c.Query("to"); v != "" {
		parsed, err := time.Parse("2006-01-02", v)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_TO", "to must be YYYY-MM-DD")
			return
		}
		// Query params are date-only, so "to" must include the entire day
		// it names — advance to the start of the next day for the
		// exclusive upper bound the queries use ([from, to)).
		to = parsed.AddDate(0, 0, 1)
	}

	analytics, err := h.service.GetRevenueAnalytics(c.Request.Context(), from, to)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load revenue analytics")
		return
	}
	c.JSON(http.StatusOK, analytics)
}
