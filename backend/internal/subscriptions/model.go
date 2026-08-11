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
