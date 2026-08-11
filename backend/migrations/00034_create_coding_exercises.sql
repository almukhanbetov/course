-- +goose Up

-- One coding exercise per lesson (UNIQUE(lesson_id)), same reasoning as
-- assignments in Stage 15: matches the spec's column list, keeps the
-- completion-rule SQL a single check rather than an aggregate, and mirrors
-- "one practical coding task per lesson". Extending to many-per-lesson
-- later is a one-column migration, not a rewrite.
CREATE TABLE coding_exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id UUID NOT NULL UNIQUE REFERENCES lessons (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    language TEXT NOT NULL CHECK (language IN ('go', 'python', 'javascript')),
    starter_code TEXT NOT NULL DEFAULT '',
    -- solution_code is authoring-only (instructor/admin) — the student API
    -- (internal/coding service layer) never selects this column into any
    -- student-facing response type.
    solution_code TEXT,
    time_limit_ms INTEGER NOT NULL DEFAULT 5000,
    memory_limit_mb INTEGER NOT NULL DEFAULT 128,
    published BOOLEAN NOT NULL DEFAULT false,
    required BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT coding_exercises_time_limit_positive CHECK (time_limit_ms > 0),
    CONSTRAINT coding_exercises_memory_limit_positive CHECK (memory_limit_mb > 0)
);

CREATE TABLE coding_test_cases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coding_exercise_id UUID NOT NULL REFERENCES coding_exercises (id) ON DELETE CASCADE,
    input TEXT,
    expected_output TEXT NOT NULL,
    position INTEGER NOT NULL,
    -- hidden=true (the default) means the student API never returns this
    -- case's input/expected_output, only pass/fail — see internal/coding.
    hidden BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT coding_test_cases_exercise_position_unique UNIQUE (coding_exercise_id, position)
);

CREATE INDEX idx_coding_test_cases_exercise_id ON coding_test_cases (coding_exercise_id);

CREATE TABLE code_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    exercise_id UUID NOT NULL REFERENCES coding_exercises (id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    language TEXT NOT NULL,
    source_code TEXT NOT NULL,
    -- "run" submissions (item 18) reuse this same table/status machine —
    -- mode distinguishes a quick, ungraded run from an official, completion-
    -- eligible submit. Only mode='submit' rows are ever considered by the
    -- lesson-completion rule.
    mode TEXT NOT NULL DEFAULT 'submit' CHECK (mode IN ('run', 'submit')),
    status TEXT NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'passed', 'failed', 'compile_error', 'runtime_error', 'timeout', 'internal_error')),
    passed_tests INTEGER NOT NULL DEFAULT 0,
    total_tests INTEGER NOT NULL DEFAULT 0,
    execution_time_ms INTEGER,
    memory_used_kb INTEGER,
    -- stdout/compile_output are capped at CODE_RUNNER_MAX_OUTPUT_KB by the
    -- runner itself before being written here — see internal/coding/runner.go.
    stdout TEXT,
    compile_output TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_code_submissions_user_id ON code_submissions (user_id);
CREATE INDEX idx_code_submissions_exercise_id ON code_submissions (exercise_id);
CREATE INDEX idx_code_submissions_status ON code_submissions (status);
CREATE INDEX idx_code_submissions_created_at ON code_submissions (created_at);
-- Backs both the rate limiter (submissions by this user in the last N
-- seconds) and the queue-depth guard (this user's queued/running count).
CREATE INDEX idx_code_submissions_user_created ON code_submissions (user_id, created_at);

CREATE TABLE code_execution_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id UUID NOT NULL REFERENCES code_submissions (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Same partial index shape as idx_video_jobs_claimable — covers exactly the
-- rows the code-runner's FOR UPDATE SKIP LOCKED claim query scans.
CREATE INDEX idx_code_execution_jobs_claimable ON code_execution_jobs (available_at) WHERE status = 'pending';
CREATE INDEX idx_code_execution_jobs_submission_id ON code_execution_jobs (submission_id);

-- +goose Down
DROP TABLE code_execution_jobs;
DROP TABLE code_submissions;
DROP TABLE coding_test_cases;
DROP TABLE coding_exercises;
