-- +goose Up
-- lessons.video_url (legacy, from Stage 2) is left untouched — the new
-- object-storage flow lives entirely in this table so existing data/behavior
-- can't regress. See internal/videos for how it's used.
CREATE TABLE lesson_videos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id UUID NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    storage_provider TEXT NOT NULL DEFAULT 's3',
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    duration_seconds INTEGER,
    status TEXT NOT NULL DEFAULT 'uploading' CHECK (status IN ('uploading', 'ready', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lesson_videos_lesson_id_unique UNIQUE (lesson_id)
);

-- +goose Down
DROP TABLE lesson_videos;
