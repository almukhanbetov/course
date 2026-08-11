-- +goose Up

-- notifications is the in-app read model — what GET /me/notifications
-- serves. It is populated exclusively by notification-worker materializing
-- an "in_app" notification_jobs row; nothing in the API backend writes to
-- it directly (see internal/notifications package docs).
CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    data JSONB,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_id ON notifications (user_id);
CREATE INDEX idx_notifications_created_at ON notifications (created_at);
CREATE INDEX idx_notifications_read_at ON notifications (read_at);
-- Backs "unread count" (WHERE user_id = $1 AND read_at IS NULL) without a
-- full-table scan even as the table grows.
CREATE INDEX idx_notifications_user_unread ON notifications (user_id) WHERE read_at IS NULL;

-- notification_jobs is the transactional outbox: business operations
-- (enrollment, course completion, certificate issuance, payment
-- confirmation, ...) insert rows here in the SAME database transaction as
-- the state change they're reporting, so a crash between "state changed"
-- and "notification sent" is impossible — the job just sits pending until
-- a worker claims it. See internal/notifications.Enqueue.
CREATE TABLE notification_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    payload JSONB,
    channel TEXT NOT NULL CHECK (channel IN ('in_app', 'email')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- One (event, channel) pair fires at most once, ever — e.g.
    -- "course_completed:<user>:<course>:in_app". NULL means "no
    -- deduplication requested" (e.g. an admin course announcement fans out
    -- per-user keys instead), so it must not collide with other NULLs.
    dedupe_key TEXT
);

CREATE UNIQUE INDEX idx_notification_jobs_dedupe_key ON notification_jobs (dedupe_key) WHERE dedupe_key IS NOT NULL;
CREATE INDEX idx_notification_jobs_claimable ON notification_jobs (available_at) WHERE status = 'pending';
CREATE INDEX idx_notification_jobs_user_id ON notification_jobs (user_id);
CREATE INDEX idx_notification_jobs_status ON notification_jobs (status);

-- +goose Down
DROP TABLE notification_jobs;
DROP TABLE notifications;
