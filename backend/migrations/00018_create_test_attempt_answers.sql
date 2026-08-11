-- +goose Up
CREATE TABLE test_attempt_answers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attempt_id UUID NOT NULL REFERENCES test_attempts (id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES questions (id) ON DELETE CASCADE,
    answer_id UUID NOT NULL REFERENCES answers (id) ON DELETE CASCADE,
    correct BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT test_attempt_answers_attempt_question_unique UNIQUE (attempt_id, question_id)
);

CREATE INDEX idx_test_attempt_answers_attempt_id ON test_attempt_answers (attempt_id);

-- +goose Down
DROP TABLE test_attempt_answers;
