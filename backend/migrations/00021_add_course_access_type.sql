-- +goose Up
ALTER TABLE courses
    ADD COLUMN access_type TEXT NOT NULL DEFAULT 'free' CHECK (access_type IN ('free', 'subscription'));

-- +goose Down
ALTER TABLE courses DROP COLUMN access_type;
