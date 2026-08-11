-- +goose Up

-- users.timezone backs the streak algorithm (Stage 17 item 7): "active day"
-- is evaluated against the student's own local calendar date, not UTC.
-- Validated as a real IANA zone name at the application layer (Go's
-- time.LoadLocation) before ever being written here — this column has no
-- CHECK constraint of its own since Postgres has no IANA zone catalog to
-- validate against.
ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC';

-- learning_activity is the single append-only ledger every "meaningful"
-- learning event is recorded into (item 2's minimum list) — student
-- statistics, the activity calendar, streaks, and achievement evaluation
-- all read from this one table rather than re-deriving state from
-- lesson_progress/assignment_submissions/code_submissions/test_attempts/
-- certificates directly. Rows are written exactly once per real state
-- transition (never per video-progress tick — item 2), enforced by
-- dedupe_key wherever a caller can't otherwise guarantee at-most-once.
CREATE TABLE learning_activity (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    activity_type TEXT NOT NULL CHECK (activity_type IN (
        'lesson_completed', 'assignment_submitted', 'assignment_approved',
        'coding_exercise_passed', 'test_passed', 'course_completed', 'certificate_issued'
    )),
    entity_type TEXT,
    entity_id UUID,
    -- occurred_at is always server now() (see internal/activity.Record) —
    -- item 24/25 explicitly forbid accepting a client-supplied event date,
    -- since that's exactly how a student could otherwise fabricate a streak.
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata JSONB,
    dedupe_key TEXT
);

CREATE INDEX idx_learning_activity_user_occurred ON learning_activity (user_id, occurred_at);
CREATE INDEX idx_learning_activity_type ON learning_activity (activity_type);
-- Backs GetCounts' per-user aggregate FILTER queries and the streak local-
-- date derivation, both scoped to one user at a time.
CREATE INDEX idx_learning_activity_user_type ON learning_activity (user_id, activity_type);
CREATE UNIQUE INDEX idx_learning_activity_dedupe_key ON learning_activity (dedupe_key) WHERE dedupe_key IS NOT NULL;

CREATE TABLE achievements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    icon TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE user_achievements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    achievement_id UUID NOT NULL REFERENCES achievements (id) ON DELETE CASCADE,
    earned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT user_achievements_unique UNIQUE (user_id, achievement_id)
);

CREATE INDEX idx_user_achievements_user_id ON user_achievements (user_id);

-- Item 11's minimum achievement set. Thresholds live in Go
-- (internal/achievements/rules.go) alongside this row's code — the code is
-- the only thing the evaluator matches on, so a rule's threshold can change
-- later without a migration.
INSERT INTO achievements (code, title, description, icon) VALUES
    ('FIRST_LESSON', 'Первый урок', 'Завершите свой первый урок', '🎬'),
    ('FIRST_COURSE', 'Первый курс', 'Завершите свой первый курс', '🎓'),
    ('FIRST_CERTIFICATE', 'Первый сертификат', 'Получите свой первый сертификат', '📜'),
    ('CODE_BEGINNER', 'Первая строка кода', 'Успешно пройдите своё первое упражнение по коду', '💻'),
    ('CODE_10', 'Опытный программист', 'Успешно пройдите 10 упражнений по коду', '🚀'),
    ('STREAK_3', 'Три дня подряд', 'Занимайтесь 3 дня подряд', '🔥'),
    ('STREAK_7', 'Неделя подряд', 'Занимайтесь 7 дней подряд', '🔥'),
    ('STREAK_30', 'Месяц подряд', 'Занимайтесь 30 дней подряд', '🔥');

-- +goose Down
DROP TABLE user_achievements;
DROP TABLE achievements;
DROP TABLE learning_activity;
ALTER TABLE users DROP COLUMN timezone;
