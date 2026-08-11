-- +goose Up

-- One assignment per lesson (UNIQUE(lesson_id)) rather than many: it keeps
-- the completion-rule integration trivial (check one required assignment
-- per lesson, not aggregate several with their own required/optional mix),
-- matches the columns the spec itself suggested (no `position` field), and
-- mirrors the real-world pattern of "one practical task per lesson". If a
-- future stage needs multiple tasks per lesson, this becomes a one-column
-- migration (drop the unique constraint, add `position`) rather than a
-- rewrite of the completion logic.
CREATE TABLE assignments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id UUID NOT NULL UNIQUE REFERENCES lessons (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL DEFAULT '',
    required BOOLEAN NOT NULL DEFAULT true,
    max_score INTEGER,
    published BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assignments_max_score_positive CHECK (max_score IS NULL OR max_score > 0)
);

CREATE TABLE assignment_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    assignment_id UUID NOT NULL REFERENCES assignments (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    text_content TEXT,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'submitted', 'needs_revision', 'approved')),
    score INTEGER,
    instructor_feedback TEXT,
    submitted_at TIMESTAMPTZ,
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assignment_submissions_user_assignment_unique UNIQUE (assignment_id, user_id),
    CONSTRAINT assignment_submissions_score_non_negative CHECK (score IS NULL OR score >= 0)
);

CREATE INDEX idx_assignment_submissions_assignment_id ON assignment_submissions (assignment_id);
CREATE INDEX idx_assignment_submissions_user_id ON assignment_submissions (user_id);
CREATE INDEX idx_assignment_submissions_status ON assignment_submissions (status);

-- Files live in the existing private object storage (same MinIO bucket the
-- video pipeline already uses), never in Postgres — this table is only
-- metadata + the object key needed to fetch/presign the real bytes.
CREATE TABLE assignment_submission_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES assignment_submissions (id) ON DELETE CASCADE,
    object_key TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_assignment_submission_files_submission_id ON assignment_submission_files (submission_id);

-- One row per instructor review action (never overwritten), so a prior
-- round's feedback survives a resubmit + re-review. assignment_submissions'
-- own status/score/instructor_feedback/reviewed_at/reviewed_by columns are
-- kept as a fast "current state" snapshot — the full history lives here.
CREATE TABLE assignment_submission_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES assignment_submissions (id) ON DELETE CASCADE,
    reviewer_id UUID NOT NULL REFERENCES users (id),
    status TEXT NOT NULL CHECK (status IN ('approved', 'needs_revision')),
    score INTEGER,
    feedback TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT assignment_submission_reviews_score_non_negative CHECK (score IS NULL OR score >= 0)
);

CREATE INDEX idx_assignment_submission_reviews_submission_id ON assignment_submission_reviews (submission_id);

-- +goose Down
DROP TABLE assignment_submission_reviews;
DROP TABLE assignment_submission_files;
DROP TABLE assignment_submissions;
DROP TABLE assignments;
