# COURSE LMS

COURSE — современная LMS-платформа для онлайн-обучения с backend на Go + Gin, PostgreSQL 17 и frontend на Next.js.

Платформа включает курсы, уроки, прогресс обучения, wishlist, персональные рекомендации, тесты, coding exercises, сертификаты, аналитику, подписки, платежи и административные панели.

---

## Стек

### Backend

- Go
- Gin
- PostgreSQL 17
- Goose migrations
- pgx / pgxpool
- REST API

### Frontend

- Next.js
- TypeScript
- App Router

### Infrastructure

- Docker
- Docker Compose
- GitHub

---

## Основные возможности

- Регистрация и авторизация
- Роли пользователей
- Курсы и уроки
- Запись на курс
- Прогресс обучения
- Continue Learning
- Wishlist
- Персональные рекомендации курсов
- Similar Courses
- Roadmap / specialties
- Тесты и задания
- Coding exercises
- Сертификаты
- Achievements
- Streaks
- Analytics
- Notifications
- Instructor dashboard
- Admin dashboard
- Подписки и платежи
- Security / authorization checks
- IDOR protection

---

## Структура проекта

```text
COURSE/
├── backend/
├── frontend/
├── .claude/
│   └── skills/
├── docs/
│   └── screenshots/
├── docker-compose.yml
├── .env.example
├── STAGE18_PROGRESS.md
└── README.md
```

---

## Quick Start

### 1. Клонировать репозиторий

```bash
git clone https://github.com/almukhanbetov/course.git
cd course
```

### 2. Создать `.env`

```bash
cp .env.example .env
```

Заполните необходимые переменные окружения.

### 3. Запустить проект

```bash
docker compose up -d --build
```

### 4. Проверить контейнеры

```bash
docker compose ps
```

### 5. Открыть приложение

Frontend:

```text
http://localhost:3001
```

Backend API:

```text
http://localhost:8080
```

Если порты отличаются, используйте значения из `docker-compose.yml`.

---

## Database

Проект использует PostgreSQL 17.

Миграции управляются через Goose.

Пример:

```bash
goose -dir migrations postgres "$DATABASE_URL" up
```

Если миграции настроены отдельным Docker Compose service, используйте соответствующий service из `docker-compose.yml`.

---

## Backend Domains

Основные backend-домены проекта:

```text
auth
courses
learning
wishlist
recommendations
payments
subscriptions
notifications
analytics
admin
```

---

## Frontend

Основные страницы:

```text
/courses
/courses/[id]
/dashboard
/dashboard/wishlist
```

Dashboard включает:

- Continue Learning
- Personalized Recommendations
- Wishlist
- Learning Progress
- Achievements
- Notifications

---

## Stage 18

Stage 18 включает:

### Wishlist

Пользователь может:

- Добавить курс в wishlist
- Удалить курс из wishlist
- Просматривать сохранённые курсы
- Автоматически удалить курс из wishlist после enrollment

### Continue Learning

Платформа определяет:

- начатые, но не завершённые курсы
- следующий урок
- реальный server-side progress

### Personalized Recommendations

Рекомендации учитывают:

- историю обучения
- категории курсов
- roadmap position
- качество курса
- популярность
- freshness

### Similar Courses

Страница курса отображает похожие курсы.

### Security

Проверены:

- Authentication
- Authorization
- IDOR
- Wishlist isolation
- Private user data protection

### Database Performance

Критичные запросы проверены через:

```sql
EXPLAIN ANALYZE
```

### Regression Testing

После Stage 18 была выполнена проверка существующей функциональности LMS.

---

## Stage 18 Status

```text
Completed
```

Подробный отчёт:

```text
STAGE18_PROGRESS.md
```

---

## Screenshots

### Dashboard

![Dashboard](docs/screenshots/dashboard.png)

---

## Docker

Запустить:

```bash
docker compose up -d
```

Запустить с пересборкой:

```bash
docker compose up -d --build
```

Проверить контейнеры:

```bash
docker compose ps
```

Посмотреть логи:

```bash
docker compose logs -f
```

Остановить:

```bash
docker compose down
```

---

## Git workflow

Проверить состояние:

```bash
git status
```

После изменений:

```bash
git add .
git commit -m "feat: describe changes"
git push
```

---

## Security Principles

Проект придерживается следующих правил:

- Authorization проверяется на backend
- User-provided IDs не используются как основание для доступа
- Private user data изолированы
- Payment provider state является источником истины для payment success
- Payment secrets остаются только на server-side
- Критичные операции используют database transactions
- Protected endpoints требуют authentication

---

## Roadmap

Следующие этапы могут включать:

- Stage 19
- Расширенные рекомендации
- Улучшенный поиск
- Instructor tools
- Расширенная аналитика
- Mobile application
- CI/CD
- Production deployment
- Monitoring
- Logging

---

## Repository

```text
https://github.com/almukhanbetov/course
```

---

## Author

Mukhtar Almukhanbetov