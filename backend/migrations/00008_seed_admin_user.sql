-- +goose Up
-- Development seed credential (local/dev environments only): admin@example.com / ChangeMe123!
-- Rotate or remove before using this seed against any real deployment.
INSERT INTO users (id, email, password_hash, first_name, last_name, role_id, active)
VALUES (
    '55555555-5555-5555-5555-555555555555',
    'admin@example.com',
    '$2a$10$r8bZWjVNrOGGz4.dLlfm2eKb3KNa2vy45HXJMkqH5sxDkmcJg04FW',
    'Admin',
    'User',
    '44444444-4444-4444-4444-444444444443',
    true
)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM users WHERE id = '55555555-5555-5555-5555-555555555555';
