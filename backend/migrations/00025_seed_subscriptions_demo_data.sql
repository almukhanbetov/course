-- +goose Up
INSERT INTO subscription_plans (id, name, slug, description, price_amount, currency, duration_days, active)
VALUES (
    'dddddddd-dddd-dddd-dddd-dddddddddddd',
    'Pro',
    'pro',
    'Полный доступ ко всем курсам по подписке на 30 дней.',
    990000,
    'KZT',
    30,
    true
)
ON CONFLICT (id) DO NOTHING;

-- Docker is not required in the Backend Developer roadmap and has no final
-- test, so gating it behind a subscription doesn't disturb any existing
-- regression path (enrollment/progress/final-test/certificate all exercise
-- the still-free "Go Backend Developer" and "PostgreSQL" courses).
UPDATE courses SET access_type = 'subscription' WHERE id = '88888888-8888-8888-8888-888888888888';

-- +goose Down
UPDATE courses SET access_type = 'free' WHERE id = '88888888-8888-8888-8888-888888888888';

DELETE FROM subscription_plans WHERE id = 'dddddddd-dddd-dddd-dddd-dddddddddddd';
