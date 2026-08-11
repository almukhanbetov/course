-- +goose Up
CREATE TABLE lesson_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lesson_id UUID NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    progress_seconds INTEGER NOT NULL DEFAULT 0,
    completed BOOLEAN NOT NULL DEFAULT false,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lesson_progress_user_lesson_unique UNIQUE (user_id, lesson_id)
);

CREATE INDEX idx_lesson_progress_lesson_id ON lesson_progress (lesson_id);

-- +goose Down
DROP TABLE lesson_progress;
