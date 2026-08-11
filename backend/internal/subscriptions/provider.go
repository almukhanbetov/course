package subscriptions

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// PaymentProvider abstracts whatever external payment gateway actually
// moves money. Swapping the mock implementation below for a real Kaspi or
// Stripe client later means writing one new type that satisfies this
// interface and wiring it in main.go — no changes to the subscription
// service's business logic (CreateSubscription/ConfirmPayment).
type PaymentProvider struct {
	impl providerImpl
}

type providerImpl interface {
	Name() string
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*ProviderPaymentInfo, error)
	VerifyPayment(ctx context.Context, providerPaymentID string) (*ProviderPaymentInfo, error)
}

type CreatePaymentRequest struct {
	Amount   int64
	Currency string
}

// ProviderPaymentInfo.Status is one of "pending" or "paid" — the provider's
// own vocabulary, translated by the service into this domain's payment
// status before it ever touches the database.
type ProviderPaymentInfo struct {
	ProviderPaymentID string
	Status            string
}

func (p PaymentProvider) Name() string { return p.impl.Name() }

func (p PaymentProvider) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*ProviderPaymentInfo, error) {
	return p.impl.CreatePayment(ctx, req)
}

func (p PaymentProvider) VerifyPayment(ctx context.Context, providerPaymentID string) (*ProviderPaymentInfo, error) {
	return p.impl.VerifyPayment(ctx, providerPaymentID)
}

// NewProvider selects the payment provider implementation from the
// PAYMENT_PROVIDER env var (via config). "mock" is the only option today;
// an unrecognized value also falls back to mock rather than failing
// startup, since this is a development-only switch for now.
func NewProvider(name string) PaymentProvider {
	switch name {
	case "mock":
		return PaymentProvider{impl: &mockProvider{}}
	default:
		return PaymentProvider{impl: &mockProvider{}}
	}
}

// mockProvider never talks to a real gateway: CreatePayment just mints a
// fake reference id and leaves the payment pending, and VerifyPayment always
// reports "paid" since there's no real async settlement to check — the
// actual paid/active transition happens synchronously in
// Service.ConfirmMockPayment, which is the only caller that matters for the
// mock flow. It exists so the service layer's calls to the provider
// interface are exercised the same way a real provider's would be.
type mockProvider struct{}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) CreatePayment(_ context.Context, _ CreatePaymentRequest) (*ProviderPaymentInfo, error) {
	ref, err := randomHex(8)
	if err != nil {
		return nil, err
	}
	return &ProviderPaymentInfo{ProviderPaymentID: "mock_" + ref, Status: "pending"}, nil
}

func (m *mockProvider) VerifyPayment(_ context.Context, providerPaymentID string) (*ProviderPaymentInfo, error) {
	return &ProviderPaymentInfo{ProviderPaymentID: providerPaymentID, Status: "paid"}, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func newIdempotencyKey() (string, error) {
	ref, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sub_%s", ref), nil
}
