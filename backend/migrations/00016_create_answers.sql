-- +goose Up
CREATE TABLE answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES questions (id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT false,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT answers_question_position_unique UNIQUE (question_id, position)
);

CREATE INDEX idx_answers_question_id ON answers (question_id);

-- +goose Down
DROP TABLE answers;
