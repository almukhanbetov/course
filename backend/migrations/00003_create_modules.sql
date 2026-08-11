-- +goose Up
CREATE TABLE modules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT modules_course_position_unique UNIQUE (course_id, position)
);

CREATE INDEX idx_modules_course_id ON modules (course_id);

-- +goose Down
DROP TABLE modules;
