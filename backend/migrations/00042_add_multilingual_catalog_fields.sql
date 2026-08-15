-- +goose Up
-- Multilingual catalog (Stage: RU/KK/EN public frontend). title/description/
-- name stay the canonical, always-present Russian columns; these four are
-- optional per-locale overrides — NULL means "no translation yet, fall back
-- to the Russian original", which the API and frontend both already treat
-- as a normal, non-error state (see courses.Course's doc comment).
ALTER TABLE courses ADD COLUMN title_kk TEXT;
ALTER TABLE courses ADD COLUMN title_en TEXT;
ALTER TABLE courses ADD COLUMN description_kk TEXT;
ALTER TABLE courses ADD COLUMN description_en TEXT;

ALTER TABLE specialities ADD COLUMN title_kk TEXT;
ALTER TABLE specialities ADD COLUMN title_en TEXT;
ALTER TABLE specialities ADD COLUMN description_kk TEXT;
ALTER TABLE specialities ADD COLUMN description_en TEXT;

ALTER TABLE categories ADD COLUMN name_kk TEXT;
ALTER TABLE categories ADD COLUMN name_en TEXT;

-- Seed real translations for the existing demo catalog. Category names
-- already double as their own English translation ("DevOps", "Frontend"),
-- so name_en is left NULL there on purpose — the fallback already renders
-- correctly in English.
UPDATE courses SET
    title_kk = 'Docker',
    title_en = 'Docker',
    description_kk = 'Қолданбаларды контейнерлеу: имидждер, контейнерлер, Docker Compose.',
    description_en = 'Application containerization: images, containers, Docker Compose.'
WHERE slug = 'docker';

UPDATE courses SET
    title_kk = 'PostgreSQL',
    title_en = 'PostgreSQL',
    description_kk = 'Реляциялық дерекқорлардың негіздері: SQL, индекстер және өнімділік.',
    description_en = 'Fundamentals of relational databases: SQL, indexes, and performance.'
WHERE slug = 'postgresql';

UPDATE courses SET
    title_kk = 'Go Backend әзірлеуші',
    title_en = 'Go Backend Developer',
    description_kk = 'Go тілінде backend әзірлеу бойынша практикалық курс: тіл негіздерінен Gin негізіндегі REST API-ға дейін.',
    description_en = 'A hands-on course on Go backend development: from language fundamentals to a REST API with Gin.'
WHERE slug = 'go-backend-developer';

UPDATE specialities SET
    title_kk = 'Backend әзірлеуші',
    title_en = 'Backend Developer',
    description_kk = 'Go негіздерінен өндірістік backend-ке дейінгі жол: тіл, веб-фреймворк, дерекқор және контейнерлеу.',
    description_en = 'A path from Go fundamentals to a production-ready backend: language, web framework, database, and containerization.'
WHERE slug = 'backend-developer';

UPDATE categories SET name_kk = 'Бағдарламалау' WHERE slug = 'programming';
UPDATE categories SET name_kk = 'Дерекқорлар' WHERE slug = 'databases';

-- +goose Down
ALTER TABLE categories DROP COLUMN name_kk;
ALTER TABLE categories DROP COLUMN name_en;

ALTER TABLE specialities DROP COLUMN title_kk;
ALTER TABLE specialities DROP COLUMN title_en;
ALTER TABLE specialities DROP COLUMN description_kk;
ALTER TABLE specialities DROP COLUMN description_en;

ALTER TABLE courses DROP COLUMN title_kk;
ALTER TABLE courses DROP COLUMN title_en;
ALTER TABLE courses DROP COLUMN description_kk;
ALTER TABLE courses DROP COLUMN description_en;
