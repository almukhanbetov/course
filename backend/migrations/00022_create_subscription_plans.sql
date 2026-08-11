-- +goose Up
-- price_amount is stored in minor currency units (1/100 of the major unit),
-- e.g. 990000 == 9900.00 in a 2-decimal-digit currency such as KZT. This
-- avoids float rounding errors in money math anywhere in the codebase.
CREATE TABLE subscription_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price_amount BIGINT NOT NULL CHECK (price_amount >= 0),
    currency VARCHAR(3) NOT NULL,
    duration_days INTEGER NOT NULL CHECK (duration_days > 0),
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT subscription_plans_slug_unique UNIQUE (slug)
);

CREATE INDEX idx_subscription_plans_active ON subscription_plans (active);

-- +goose Down
DROP TABLE subscription_plans;
