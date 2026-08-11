---
name: project-architect
description: Lead architecture rules for an ITVDN-like LMS built with Next.js, Go Gin, PostgreSQL 17 and Docker.
---

# Project Architect

## Goal
Keep the LMS simple, modular, scalable, and understandable.

## Core stack
- Frontend: Next.js + TypeScript + App Router
- Backend: Go + Gin
- Database: PostgreSQL 17
- DB access: pgx/pgxpool
- Migrations: Goose
- Containers: Docker / Docker Compose
- Reverse proxy in production: host-level Nginx

## Architectural rule
Start as a modular monolith. Do not introduce microservices or Kubernetes without a measurable need.

## Core domain
Speciality -> Course -> Module -> Lesson

Supporting domains:
- Users
- Auth / RBAC
- Learning progress
- Tests
- Certificates
- Payments / subscriptions
- Notifications
- Admin

## Backend layering
handler -> service -> repository -> PostgreSQL

Handlers handle HTTP.
Services handle business rules.
Repositories handle SQL and transactions.

## API
Use REST under `/api/v1`.

Good:
- GET /api/v1/courses
- GET /api/v1/courses/:id
- POST /api/v1/auth/login

Avoid verb-style URLs such as `/getCourses`.

## Development rule
Before changing code:
1. Inspect existing files.
2. Preserve working structure and naming.
3. Make the smallest safe change.
4. Add a migration for schema changes.
5. Verify backend/API.
6. Verify frontend flow.

Do not rewrite unrelated code.
