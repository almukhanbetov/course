---
name: courses-learning
description: Implement course catalog, specialities, modules, lessons, enrollments and learning progress.
---

# Courses & Learning

## Hierarchy
Speciality -> ordered Courses -> ordered Modules -> ordered Lessons

Never attach lessons directly to a speciality.

## Course
Typical data:
- title
- slug
- description
- category_id
- instructor_id
- level
- image
- published

## Lesson
Typical data:
- module_id
- title
- slug
- video_url
- duration_seconds
- position
- access_type
- published

## Progress
Track by user + lesson.

Do not trust a percentage sent by the frontend as source of truth.

Course completion should be derived from required lessons and, if configured, final-test status.

## Access types
Examples:
- free
- subscription
- purchased

Access must always be checked by the backend.
