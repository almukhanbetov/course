-- +goose Up
CREATE TABLE speciality_courses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    speciality_id UUID NOT NULL REFERENCES specialities (id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    required BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT speciality_courses_speciality_course_unique UNIQUE (speciality_id, course_id),
    CONSTRAINT speciality_courses_speciality_position_unique UNIQUE (speciality_id, position)
);

CREATE INDEX idx_speciality_courses_course_id ON speciality_courses (course_id);

-- +goose Down
DROP TABLE speciality_courses;
