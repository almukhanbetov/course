-- +goose Up
INSERT INTO courses (id, title, slug, description, level, image_url, published)
VALUES
    (
        '77777777-7777-7777-7777-777777777777',
        'PostgreSQL',
        'postgresql',
        'Основы реляционных баз данных: SQL, индексы и производительность.',
        'beginner',
        '',
        true
    ),
    (
        '88888888-8888-8888-8888-888888888888',
        'Docker',
        'docker',
        'Контейнеризация приложений: образы, контейнеры, Docker Compose.',
        'beginner',
        '',
        true
    )
ON CONFLICT (id) DO NOTHING;

INSERT INTO modules (id, course_id, title, position)
VALUES
    ('99999999-9999-9999-9999-999999999991', '77777777-7777-7777-7777-777777777777', 'Основы PostgreSQL', 1),
    ('99999999-9999-9999-9999-999999999992', '88888888-8888-8888-8888-888888888888', 'Основы Docker', 1)
ON CONFLICT (id) DO NOTHING;

INSERT INTO lessons (id, module_id, title, slug, position, is_free, published)
VALUES
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1', '99999999-9999-9999-9999-999999999991', 'Установка и подключение', 'install-and-connect', 1, true, true),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2', '99999999-9999-9999-9999-999999999991', 'SQL основы', 'sql-basics', 2, false, true),
    ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa3', '99999999-9999-9999-9999-999999999991', 'Индексы и производительность', 'indexes-and-performance', 3, false, true),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1', '99999999-9999-9999-9999-999999999992', 'Введение в контейнеры', 'intro-to-containers', 1, true, true),
    ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2', '99999999-9999-9999-9999-999999999992', 'Docker Compose', 'docker-compose', 2, false, true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO specialities (id, title, slug, description, image_url, published)
VALUES (
    '66666666-6666-6666-6666-666666666666',
    'Backend Developer',
    'backend-developer',
    'Путь от основ Go до продакшн-готового backend: язык, веб-фреймворк, база данных и контейнеризация.',
    '',
    true
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO speciality_courses (id, speciality_id, course_id, position, required)
VALUES
    ('cccccccc-cccc-cccc-cccc-ccccccccccc1', '66666666-6666-6666-6666-666666666666', '11111111-1111-1111-1111-111111111111', 1, true),
    ('cccccccc-cccc-cccc-cccc-ccccccccccc2', '66666666-6666-6666-6666-666666666666', '77777777-7777-7777-7777-777777777777', 2, true),
    ('cccccccc-cccc-cccc-cccc-ccccccccccc3', '66666666-6666-6666-6666-666666666666', '88888888-8888-8888-8888-888888888888', 3, false)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM speciality_courses WHERE id IN (
    'cccccccc-cccc-cccc-cccc-ccccccccccc1',
    'cccccccc-cccc-cccc-cccc-ccccccccccc2',
    'cccccccc-cccc-cccc-cccc-ccccccccccc3'
);

DELETE FROM specialities WHERE id = '66666666-6666-6666-6666-666666666666';

DELETE FROM lessons WHERE id IN (
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa1',
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa2',
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaa3',
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb1',
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbb2'
);

DELETE FROM modules WHERE id IN (
    '99999999-9999-9999-9999-999999999991',
    '99999999-9999-9999-9999-999999999992'
);

DELETE FROM courses WHERE id IN (
    '77777777-7777-7777-7777-777777777777',
    '88888888-8888-8888-8888-888888888888'
);
