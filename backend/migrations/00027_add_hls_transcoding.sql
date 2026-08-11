-- +goose Up

-- Stage 10 rows (object_key/status only) must keep working unchanged. New
-- columns default such that ALL existing rows land in a state that means
-- "already finished, legacy MP4-only, never touched by the transcoding
-- worker" — see the UPDATE below and internal/videos/service.go.
ALTER TABLE lesson_videos
    ADD COLUMN processing_status TEXT NOT NULL DEFAULT 'uploaded'
        CHECK (processing_status IN ('uploaded', 'processing', 'ready', 'failed')),
    ADD COLUMN source_object_key TEXT,
    ADD COLUMN hls_master_object_key TEXT,
    ADD COLUMN processing_error TEXT,
    ADD COLUMN processed_at TIMESTAMPTZ,
    -- is_active distinguishes the video currently served to students from a
    -- newer replacement still being transcoded (see UNIQUE index below and
    -- Repository.ActivateVideo). Exactly one row per lesson is active at a
    -- time; a lesson with no video at all simply has zero rows.
    ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT true;

-- Every row that existed before this migration is a Stage 10 upload that
-- was already fully usable (synchronous MP4 upload, no HLS pipeline). Mark
-- it "ready" (not the default "uploaded") so the worker never enqueues it,
-- and point source_object_key at its existing object_key so the legacy
-- signed-MP4 fallback path keeps resolving the same object.
UPDATE lesson_videos SET processing_status = 'ready', source_object_key = object_key;

-- lesson_id was UNIQUE (one video per lesson) — replaced with a partial
-- unique index so at most one row per lesson may be the active one, while a
-- second, not-yet-active row is allowed to exist during a replace.
ALTER TABLE lesson_videos DROP CONSTRAINT lesson_videos_lesson_id_unique;
CREATE UNIQUE INDEX lesson_videos_active_lesson_id_unique ON lesson_videos (lesson_id) WHERE is_active;

CREATE TABLE video_renditions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_video_id UUID NOT NULL REFERENCES lesson_videos (id) ON DELETE CASCADE,
    quality TEXT NOT NULL CHECK (quality IN ('360p', '720p', '1080p')),
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    bitrate_kbps INTEGER NOT NULL,
    playlist_object_key TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT video_renditions_lesson_video_quality_unique UNIQUE (lesson_video_id, quality)
);

-- target_slot exists only because a lesson_video row IS the slot (active or
-- pending) once inserted — a job just needs to say which row it's for. Kept
-- as a plain FK to lesson_video_id per the spec's field list; the "which
-- slot" question is answered by that row's own is_active flag, not by this
-- table, so no extra column is needed here.
CREATE TABLE video_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_video_id UUID NOT NULL REFERENCES lesson_videos (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partial index over exactly the rows a worker's claim query scans.
CREATE INDEX idx_video_jobs_claimable ON video_jobs (available_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE video_jobs;
DROP TABLE video_renditions;

DROP INDEX lesson_videos_active_lesson_id_unique;
ALTER TABLE lesson_videos ADD CONSTRAINT lesson_videos_lesson_id_unique UNIQUE (lesson_id);

ALTER TABLE lesson_videos
    DROP COLUMN processing_status,
    DROP COLUMN source_object_key,
    DROP COLUMN hls_master_object_key,
    DROP COLUMN processing_error,
    DROP COLUMN processed_at,
    DROP COLUMN is_active;
