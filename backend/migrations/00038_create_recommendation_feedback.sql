-- +goose Up

-- recommendation_feedback is a user's explicit "not for me" signal on a
-- recommended/similar course (Stage 23A1) — the read-only gap Stage 18's
-- recommendation engine left open (STAGE18_PROGRESS.md / ROADMAP_STAGE_21_30.md's
-- Stage 23 goal). One row per (user_id, course_id): a user can only ever
-- have one active feedback action against a given course at a time, so
-- switching from "dismiss" to "not_interested" (or back) replaces the
-- existing row via upsert rather than accumulating duplicates — this is
-- what "action" being present but not part of the unique key means: the
-- constraint is on the (user, course) pair, not the (user, course, action)
-- triple, matching the roadmap's own UNIQUE(user_id, course_id) plan.
CREATE TABLE recommendation_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    action TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT recommendation_feedback_user_course_unique UNIQUE (user_id, course_id)
);

-- user_id backs "exclude this user's dismissed courses from their own
-- recommendations" (the future scoring-side consumer, not wired this
-- session); course_id backs any future per-course feedback aggregate.
CREATE INDEX idx_recommendation_feedback_user_id ON recommendation_feedback (user_id);
CREATE INDEX idx_recommendation_feedback_course_id ON recommendation_feedback (course_id);

-- +goose Down
DROP TABLE recommendation_feedback;
