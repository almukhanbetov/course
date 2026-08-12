-- +goose Up

-- content_reports is a polymorphic abuse-flag (Stage 24A1): content_type +
-- content_id together identify the flagged row (a Q&A question, a Q&A
-- answer, or a course review). Deliberately NO foreign key on content_id
-- itself — a single column can't conditionally reference three different
-- tables (lesson_questions / question_answers / course_reviews), so
-- existence is validated in the service layer instead (see
-- internal/reports.Service.CreateReport / Repository.ContentExists), the
-- same "share schema, not code" convention internal/qa's IsEnrolled
-- already established for cross-domain checks that don't warrant a real FK.
--
-- content_type and status are both bounded vocabularies, validated in Go
-- (internal/reports/model.go's allowedContentTypes/allowedStatuses) AND
-- enforced here via CHECK constraints as a storage-level backstop — the
-- first bounded-string-enum column in this codebase's history to get one
-- (recommendation_feedback.action, Stage 23A1, relies on Go validation
-- alone), added here specifically because this session's own verification
-- scope calls for confirming invalid type/status rejection as DB
-- behavior, not just an application-level check.
CREATE TABLE content_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    content_type TEXT NOT NULL CHECK (content_type IN ('question', 'answer', 'review')),
    content_id UUID NOT NULL,
    reason TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'resolved', 'dismissed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Prevents a duplicate ACTIVE report: the same reporter flagging the same
-- piece of content again while their earlier report is still open is
-- blocked; once that report is resolved/dismissed (status no longer
-- 'open'), a fresh report is allowed again for a new instance of abuse.
-- A partial unique index rather than a plain UNIQUE(...) over all columns,
-- since a full-column constraint would need every terminal status to be a
-- distinct value just to avoid accidentally blocking legitimate
-- re-reports — this states the actual intent directly instead.
CREATE UNIQUE INDEX idx_content_reports_active_unique
    ON content_reports (reporter_user_id, content_type, content_id)
    WHERE status = 'open';

-- Backs a future admin queue (open reports ordered by age) and any future
-- per-content-item lookup — neither consumer exists yet this session, but
-- both are the obvious next reads and the indexes cost nothing to add now.
CREATE INDEX idx_content_reports_status_created_at ON content_reports (status, created_at);
CREATE INDEX idx_content_reports_content ON content_reports (content_type, content_id);

-- +goose Down
DROP TABLE content_reports;
