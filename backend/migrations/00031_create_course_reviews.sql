-- +goose Up
CREATE TABLE course_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 5),
    review_text TEXT,
    published BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT course_reviews_user_course_unique UNIQUE (user_id, course_id)
);

CREATE INDEX idx_course_reviews_user_id ON course_reviews (user_id);
-- Backs both the public reviews list (course_id, published=true) and the
-- rating aggregate LATERAL join used by the course catalog/detail queries.
CREATE INDEX idx_course_reviews_course_published ON course_reviews (course_id) WHERE published;
-- Backs the admin moderation list's course/rating filters over all rows
-- (published and unpublished).
CREATE INDEX idx_course_reviews_course_id ON course_reviews (course_id);

-- +goose Down
DROP TABLE course_reviews;
