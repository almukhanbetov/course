---
name: postgres-database
description: Design PostgreSQL 17 schema, queries, indexes, migrations and transactions for the LMS.
---

# PostgreSQL Database

Use PostgreSQL 17 and Goose migrations.

## Core tables
Introduce incrementally:
- users
- roles
- categories
- courses
- modules
- lessons
- specialities
- speciality_courses
- user_courses
- lesson_progress
- tests
- questions
- answers
- test_attempts
- subscriptions
- payments
- certificates
- notifications

## Rules
- Every schema change gets a migration.
- Add foreign keys where relationships are real.
- Use unique constraints for true uniqueness.
- Index columns used heavily in joins, filters and lookups.
- Avoid storing calculated values when they can be derived reliably.
- Never store videos in PostgreSQL; store metadata/URLs only.

## Ordering
Use explicit `position` fields for:
- modules
- lessons
- speciality_courses

## Progress
Typical fields:
- user_id
- lesson_id
- progress_seconds
- completed
- completed_at
- updated_at

Use a unique constraint on `(user_id, lesson_id)`.

## Transactions
Use transactions when multiple writes form one business operation.
