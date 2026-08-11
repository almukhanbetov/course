-- +goose Up
CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT categories_slug_unique UNIQUE (slug)
);

CREATE INDEX idx_categories_active ON categories (active);

-- Nullable and with no default other than NULL — existing courses simply
-- have no category until an admin assigns one. Nothing about this column
-- can break a course row that predates it.
ALTER TABLE courses ADD COLUMN category_id UUID REFERENCES categories (id);
CREATE INDEX idx_courses_category_id ON courses (category_id);

-- Demo categories (item 3: seed is fine for dev/demo, not a production
-- fixture) plus assigning the three pre-existing demo courses to them so
-- search/filter/category pages have something to show without extra manual
-- admin steps.
INSERT INTO categories (id, name, slug, position) VALUES
    ('dddddddd-1111-1111-1111-111111111111', 'Programming', 'programming', 1),
    ('dddddddd-2222-2222-2222-222222222222', 'Databases', 'databases', 2),
    ('dddddddd-3333-3333-3333-333333333333', 'DevOps', 'devops', 3),
    ('dddddddd-4444-4444-4444-444444444444', 'Frontend', 'frontend', 4)
ON CONFLICT (id) DO NOTHING;

UPDATE courses SET category_id = 'dddddddd-1111-1111-1111-111111111111' WHERE slug = 'go-backend-developer';
UPDATE courses SET category_id = 'dddddddd-2222-2222-2222-222222222222' WHERE slug = 'postgresql';
UPDATE courses SET category_id = 'dddddddd-3333-3333-3333-333333333333' WHERE slug = 'docker';

-- +goose Down
ALTER TABLE courses DROP COLUMN category_id;
DROP TABLE categories;
