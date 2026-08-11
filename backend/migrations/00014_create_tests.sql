-- +goose Up
CREATE TABLE tests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id UUID REFERENCES courses (id) ON DELETE CASCADE,
    module_id UUID REFERENCES modules (id) ON DELETE CASCADE,
    lesson_id UUID REFERENCES lessons (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    passing_score INTEGER NOT NULL DEFAULT 70,
    published BOOLEAN NOT NULL DEFAULT false,
    is_final BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT tests_single_parent CHECK (
        (CASE WHEN course_id IS NOT NULL THEN 1 ELSE 0 END) +
        (CASE WHEN module_id IS NOT NULL THEN 1 ELSE 0 END) +
        (CASE WHEN lesson_id IS NOT NULL THEN 1 ELSE 0 END) = 1
    )
);

CREATE INDEX idx_tests_course_id ON tests (course_id) WHERE course_id IS NOT NULL;
CREATE INDEX idx_tests_module_id ON tests (module_id) WHERE module_id IS NOT NULL;
CREATE INDEX idx_tests_lesson_id ON tests (lesson_id) WHERE lesson_id IS NOT NULL;
CREATE INDEX idx_tests_course_final ON tests (course_id, is_final) WHERE is_final = true;

-- +goose Down
DROP TABLE tests;
