-- +goose Up
CREATE TABLE lessons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id UUID NOT NULL REFERENCES modules (id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    video_url TEXT NOT NULL DEFAULT '',
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL,
    is_free BOOLEAN NOT NULL DEFAULT false,
    published BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lessons_module_position_unique UNIQUE (module_id, position),
    CONSTRAINT lessons_module_slug_unique UNIQUE (module_id, slug)
);

CREATE INDEX idx_lessons_module_id ON lessons (module_id);

-- +goose Down
DROP TABLE lessons;
