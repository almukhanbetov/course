package subscriptions

import (
	"time"

	"github.com/google/uuid"
)

// Plan.PriceAmount is stored in minor currency units (1/100 of the major
// unit) — see migrations/00022_create_subscription_plans.sql.
type Plan struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Slug         string    `json:"slug"`
	Description  string    `json:"description"`
	PriceAmount  int64     `json:"price_amount"`
	Currency     string    `json:"currency"`
	DurationDays int       `json:"duration_days"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PlanInput struct {
	Name         string
	Slug         string
	Description  string
	PriceAmount  int64
	Currency     string
	DurationDays int
	Active       bool
}

type Subscription struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	PlanID     uuid.UUID  `json:"plan_id"`
	Status     string     `json:"status"`
	StartsAt   *time.Time `json:"starts_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CanceledAt *time.Time `json:"canceled_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Payment struct {
	ID                uuid.UUID  `json:"id"`
	UserID            uuid.UUID  `json:"user_id"`
	SubscriptionID    *uuid.UUID `json:"subscription_id"`
	Provider          string     `json:"provider"`
	ProviderPaymentID *string    `json:"provider_payment_id"`
	Amount            int64      `json:"amount"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	IdempotencyKey    string     `json:"idempotency_key"`
	CreatedAt         time.Time  `json:"created_at"`
	PaidAt            *time.Time `json:"paid_at"`
	FailedAt          *time.Time `json:"failed_at"`
}

// CreateSubscriptionResult is what POST /api/v1/subscriptions returns — the
// new pending subscription plus the payment information the client needs to
// proceed to checkout.
type CreateSubscriptionResult struct {
	Subscription Subscription `json:"subscription"`
	Payment      Payment      `json:"payment"`
	Plan         Plan         `json:"plan"`
}

// MySubscription is the shape returned by GET /me/subscription. Plan is nil
// when Active is false and the user has no subscription at all.
type MySubscription struct {
	Active    bool       `json:"active"`
	Plan      *Plan      `json:"plan,omitempty"`
	Status    string     `json:"status,omitempty"`
	StartsAt  *time.Time `json:"starts_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AdminSubscriptionSummary/AdminPaymentSummary annotate the row with the
// owning user's identity and (for subscriptions) the plan name, for the
// read-only admin list pages.
type AdminSubscriptionSummary struct {
	Subscription
	UserEmail string `json:"user_email"`
	UserName  string `json:"user_name"`
	PlanName  string `json:"plan_name"`
}

type AdminPaymentSummary struct {
	Payment
	UserEmail string `json:"user_email"`
	UserName  string `json:"user_name"`
}

// RevenueAnalytics is the admin-only aggregate view over payments/
// subscriptions (Stage 19A). ByCurrency is deliberately a slice of
// per-currency rows, never a single combined total — subscription_plans.
// currency is not fixed platform-wide (the schema allows a different
// currency per plan), so summing amounts across currencies would silently
// produce a meaningless number. From/To bound the flow metrics only
// (PaidAmount, PaidCount, RefundedAmount, RefundedCount, NewSubscriptions,
// CanceledSubscriptions); ActiveSubscriptions/MRR are point-in-time as of
// now, independent of the requested period.
type RevenueAnalytics struct {
	From       time.Time         `json:"from"`
	To         time.Time         `json:"to"`
	ByCurrency []CurrencyRevenue `json:"by_currency"`
}

// CurrencyRevenue.MRR is a normalized monthly-recurring-revenue estimate:
// each active subscription's plan price is scaled to a 30-day period via
// price_amount * 30 / duration_days, then summed — so a plan billed
// quarterly or weekly still contributes a comparable "per month" figure.
type CurrencyRevenue struct {
	Currency              string `json:"currency"`
	PaidAmount            int64  `json:"paid_amount"`
	PaidCount             int    `json:"paid_count"`
	RefundedAmount        int64  `json:"refunded_amount"`
	RefundedCount         int    `json:"refunded_count"`
	NewSubscriptions      int    `json:"new_subscriptions"`
	CanceledSubscriptions int    `json:"canceled_subscriptions"`
	ActiveSubscriptions   int    `json:"active_subscriptions"`
	MRR                   int64  `json:"mrr"`
}
