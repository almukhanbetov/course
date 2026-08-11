-- +goose Up

-- instructor_id is nullable so every existing course (which has no
-- instructor) migrates cleanly — an unassigned course is simply admin-owned,
-- same as before this stage existed.
ALTER TABLE courses ADD COLUMN instructor_id UUID REFERENCES users (id);
CREATE INDEX idx_courses_instructor_id ON courses (instructor_id);

-- publication_status is the new source of truth for the authoring workflow
-- (draft -> pending_review -> published/rejected). The existing `published`
-- boolean is kept, not dropped: every public query still filters on
-- `published = true`, so this migration cannot break the search/catalog/
-- reviews code from Stage 13 or anything upstream of it. Application code
-- (courses.Service) is responsible for keeping the two in lockstep from now
-- on — `published` becomes a generated mirror of
-- `publication_status = 'published'` rather than an independent flag.
ALTER TABLE courses ADD COLUMN publication_status TEXT NOT NULL DEFAULT 'published'
    CHECK (publication_status IN ('draft', 'pending_review', 'published', 'rejected'));

UPDATE courses SET publication_status = CASE WHEN published THEN 'published' ELSE 'draft' END;

CREATE INDEX idx_courses_publication_status ON courses (publication_status);

-- Only meaningful when publication_status = 'rejected' — surfaced back to
-- the instructor so they know what to fix before resubmitting.
ALTER TABLE courses ADD COLUMN rejection_reason TEXT;

-- +goose Down
ALTER TABLE courses DROP COLUMN rejection_reason;
DROP INDEX idx_courses_publication_status;
ALTER TABLE courses DROP COLUMN publication_status;
DROP INDEX idx_courses_instructor_id;
ALTER TABLE courses DROP COLUMN instructor_id;
