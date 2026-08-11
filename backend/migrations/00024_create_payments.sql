-- +goose Up
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions (id) ON DELETE SET NULL,
    provider TEXT NOT NULL,
    provider_payment_id TEXT,
    amount BIGINT NOT NULL CHECK (amount >= 0),
    currency VARCHAR(3) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'failed', 'refunded')),
    idempotency_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    CONSTRAINT payments_idempotency_key_unique UNIQUE (idempotency_key)
);

CREATE INDEX idx_payments_user_id ON payments (user_id);
CREATE INDEX idx_payments_subscription_id ON payments (subscription_id);
CREATE INDEX idx_payments_status ON payments (status);

-- +goose Down
DROP TABLE payments;
