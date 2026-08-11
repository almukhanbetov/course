-- +goose Up
CREATE TABLE questions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    test_id UUID NOT NULL REFERENCES tests (id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT questions_test_position_unique UNIQUE (test_id, position)
);

CREATE INDEX idx_questions_test_id ON questions (test_id);

-- +goose Down
DROP TABLE questions;
