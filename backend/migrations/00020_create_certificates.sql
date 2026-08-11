-- +goose Up
CREATE TABLE certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    certificate_number TEXT NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT certificates_user_course_unique UNIQUE (user_id, course_id),
    CONSTRAINT certificates_number_unique UNIQUE (certificate_number)
);

CREATE INDEX idx_certificates_course_id ON certificates (course_id);

-- +goose Down
DROP TABLE certificates;
