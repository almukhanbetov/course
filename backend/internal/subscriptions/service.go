package subscriptions

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type Service struct {
	repo            *Repository
	provider        PaymentProvider
	paymentProvider string // the configured PAYMENT_PROVIDER value, gates mock-confirm
}

func NewService(repo *Repository, provider PaymentProvider, paymentProviderName string) *Service {
	return &Service{repo: repo, provider: provider, paymentProvider: paymentProviderName}
}

// --- Plans ---------------------------------------------------------------

func (s *Service) ListActivePlans(ctx context.Context) ([]Plan, error) {
	return s.repo.ListActivePlans(ctx)
}

func (s *Service) ListAllPlansAdmin(ctx context.Context) ([]Plan, error) {
	return s.repo.ListAllPlansAdmin(ctx)
}

func (s *Service) GetPlan(ctx context.Context, id uuid.UUID) (*Plan, error) {
	return s.repo.GetPlan(ctx, id)
}

func validatePlanInput(input PlanInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return &ValidationError{Message: "name is required"}
	}
	if strings.TrimSpace(input.Slug) == "" {
		return &ValidationError{Message: "slug is required"}
	}
	if strings.TrimSpace(input.Currency) == "" || len(input.Currency) != 3 {
		return &ValidationError{Message: "currency must be a 3-letter code"}
	}
	if input.PriceAmount < 0 {
		return &ValidationError{Message: "price_amount must not be negative"}
	}
	if input.DurationDays <= 0 {
		return &ValidationError{Message: "duration_days must be positive"}
	}
	return nil
}

func (s *Service) CreatePlan(ctx context.Context, input PlanInput) (*Plan, error) {
	if err := validatePlanInput(input); err != nil {
		return nil, err
	}
	return s.repo.CreatePlan(ctx, input)
}

func (s *Service) UpdatePlan(ctx context.Context, id uuid.UUID, input PlanInput) (*Plan, error) {
	if err := validatePlanInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdatePlan(ctx, id, input)
}

// --- My subscription -------------------------------------------------------

func (s *Service) GetMySubscription(ctx context.Context, userID uuid.UUID) (*MySubscription, error) {
	sub, err := s.repo.GetActiveSubscription(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return &MySubscription{Active: false}, nil
	}
	if err != nil {
		return nil, err
	}

	plan, err := s.repo.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return nil, err
	}

	return &MySubscription{
		Active:    true,
		Plan:      plan,
		Status:    sub.Status,
		StartsAt:  sub.StartsAt,
		ExpiresAt: sub.ExpiresAt,
	}, nil
}

// --- Create + confirm ------------------------------------------------------

// CreateSubscription creates a pending subscription and a pending payment
// for it, quoting the amount from the plan row itself — the caller only
// ever supplies a plan_id, never a price, so the frontend cannot influence
// what gets charged.
func (s *Service) CreateSubscription(ctx context.Context, userID, planID uuid.UUID) (*CreateSubscriptionResult, error) {
	plan, err := s.repo.GetPlan(ctx, planID)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !plan.Active {
		return nil, ErrPlanInactive
	}

	providerInfo, err := s.provider.CreatePayment(ctx, CreatePaymentRequest{Amount: plan.PriceAmount, Currency: plan.Currency})
	if err != nil {
		return nil, err
	}

	idemKey, err := newIdempotencyKey()
	if err != nil {
		return nil, err
	}

	sub, pay, err := s.repo.CreatePendingSubscriptionAndPayment(ctx, userID, planID, plan.PriceAmount, plan.Currency, s.provider.Name(), providerInfo.ProviderPaymentID, idemKey)
	if err != nil {
		return nil, err
	}

	return &CreateSubscriptionResult{Subscription: *sub, Payment: *pay, Plan: *plan}, nil
}

// ConfirmMockPayment is only ever wired up by main.go when
// PAYMENT_PROVIDER=mock — see handler.go. It still re-checks the provider on
// the payment row itself so a payment created under a different provider can
// never be confirmed through this development-only path.
func (s *Service) ConfirmMockPayment(ctx context.Context, userID, paymentID uuid.UUID) (*Payment, *Subscription, error) {
	payment, err := s.repo.GetPayment(ctx, paymentID)
	if err != nil {
		return nil, nil, err
	}
	if payment.UserID != userID {
		return nil, nil, ErrForbidden
	}
	if payment.Provider != "mock" {
		return nil, nil, ErrWrongProvider
	}

	if payment.Status != "pending" {
		// Idempotent re-confirm: report the already-settled state instead of
		// erroring, so a retried client request doesn't look like a failure.
		if payment.Status == "paid" && payment.SubscriptionID != nil {
			sub, err := s.repo.GetSubscription(ctx, *payment.SubscriptionID)
			if err != nil {
				return nil, nil, err
			}
			return payment, sub, nil
		}
		return nil, nil, ErrAlreadyProcessed
	}

	if _, err := s.provider.VerifyPayment(ctx, *payment.ProviderPaymentID); err != nil {
		return nil, nil, err
	}

	confirmedPayment, confirmedSub, err := s.repo.ConfirmPayment(ctx, paymentID)
	if errors.Is(err, ErrAlreadyProcessed) {
		// Lost the race against a concurrent confirm of the same payment —
		// treat it the same as the sequential-retry case above.
		refetched, getErr := s.repo.GetPayment(ctx, paymentID)
		if getErr != nil {
			return nil, nil, getErr
		}
		if refetched.SubscriptionID == nil {
			return nil, nil, ErrAlreadyProcessed
		}
		sub, getErr := s.repo.GetSubscription(ctx, *refetched.SubscriptionID)
		if getErr != nil {
			return nil, nil, getErr
		}
		return refetched, sub, nil
	}
	return confirmedPayment, confirmedSub, err
}

func (s *Service) MockProviderEnabled() bool {
	return s.paymentProvider == "mock"
}

// --- Admin -----------------------------------------------------------------

func (s *Service) ListSubscriptionsAdmin(ctx context.Context, status string, page, limit int) ([]AdminSubscriptionSummary, int, error) {
	return s.repo.ListSubscriptionsAdmin(ctx, status, limit, (page-1)*limit)
}

func (s *Service) ListPaymentsAdmin(ctx context.Context, status string, page, limit int) ([]AdminPaymentSummary, int, error) {
	return s.repo.ListPaymentsAdmin(ctx, status, limit, (page-1)*limit)
}
