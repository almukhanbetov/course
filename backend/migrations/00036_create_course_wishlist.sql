-- +goose Up

-- course_wishlist is a pure intent signal (Stage 18 item 20's doc comment
-- applies here too): a student "wants to learn this" — never itself a
-- learning_activity event (item 22), never itself worth a notification
-- (item 21), and automatically cleared the moment enrollment actually
-- happens (see internal/learning.Repository.CreateEnrollment, which
-- deletes the matching row in the same transaction as the enrollment
-- insert).
CREATE TABLE course_wishlist (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT course_wishlist_user_course_unique UNIQUE (user_id, course_id)
);

-- Backs "my wishlist" (user_id) and the admin/instructor per-course
-- wishlist_count aggregate (course_id) — see internal/wishlist and
-- internal/instructor's CourseStats extension.
CREATE INDEX idx_course_wishlist_user_id ON course_wishlist (user_id);
CREATE INDEX idx_course_wishlist_course_id ON course_wishlist (course_id);

-- +goose Down
DROP TABLE course_wishlist;
