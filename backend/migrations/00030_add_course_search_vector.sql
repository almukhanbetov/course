-- +goose Up
-- A generated column, not a trigger: it's always in sync with title/
-- description by construction, and to_tsvector('russian', ...) with a
-- literal configuration name is immutable enough for Postgres to allow it
-- in a STORED generated column. title gets weight A, description weight B —
-- a match in the title ranks higher than one only in the description.
ALTER TABLE courses ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('russian', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('russian', coalesce(description, '')), 'B')
    ) STORED;

CREATE INDEX idx_courses_search_vector ON courses USING GIN (search_vector);

-- +goose Down
DROP INDEX idx_courses_search_vector;
ALTER TABLE courses DROP COLUMN search_vector;
