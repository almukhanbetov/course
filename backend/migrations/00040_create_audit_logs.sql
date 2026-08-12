-- +goose Up

-- audit_logs is an append-only accountability trail (Stage 25A1): who did
-- what, to which entity, and when. Deliberately no updated_at column and
-- no update/delete path anywhere in internal/audit — a log entry, once
-- written, never changes (matches the roadmap's own "no update/delete
-- endpoint... append-only by construction" security requirement, enforced
-- here at the schema/repository level even before any HTTP surface exists
-- to expose it).
--
-- actor_user_id is nullable and ON DELETE SET NULL, not CASCADE — the one
-- deliberate deviation from every other domain's FK convention in this
-- codebase. An audit trail must outlive the account it describes: deleting
-- a user should never silently erase the record of what they did, only
-- anonymize the row's own actor reference. NULL is also how a legitimate
-- system-generated event (no human actor at all) is represented, not only
-- a post-deletion artifact.
--
-- action/entity_type are plain TEXT, not a CHECK-constrained enum like
-- internal/reports' content_type/status — those are a genuinely fixed
-- 3-value set; audit actions/entity types are inherently open-ended, and
-- every future stage that instruments a new mutation will add new values
-- routinely. A closed enum would need a migration for every addition,
-- directly working against "extensible enough for future stages." Kept as
-- NOT NULL still: every event must at least categorize itself, even if
-- entity_id (the specific instance) is absent.
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID REFERENCES users (id) ON DELETE SET NULL,
    actor_role TEXT,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id UUID,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Backs a future GET /admin/audit-log's two obvious lookup shapes: "what
-- happened to this entity" and "what has this actor done, in order" —
-- exactly the roadmap's own stated index plan. Neither has a consumer yet
-- this session.
CREATE INDEX idx_audit_logs_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_audit_logs_actor_created ON audit_logs (actor_user_id, created_at);

-- +goose Down
DROP TABLE audit_logs;
