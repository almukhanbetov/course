-- +goose Up
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT roles_name_unique UNIQUE (name)
);

INSERT INTO roles (id, name) VALUES
    ('44444444-4444-4444-4444-444444444441', 'student'),
    ('44444444-4444-4444-4444-444444444442', 'instructor'),
    ('44444444-4444-4444-4444-444444444443', 'admin')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TABLE roles;
