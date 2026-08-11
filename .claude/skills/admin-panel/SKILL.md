---
name: admin-panel
description: Build the LMS administration area and backend APIs for managing educational content and users.
---

# Admin Panel

## Core sections
- Users
- Courses
- Specialities
- Modules
- Lessons
- Tests
- Subscriptions
- Payments
- Certificates

## Course management
Admin should be able to:
- create/edit/publish/unpublish courses
- create/reorder modules
- create/reorder lessons
- attach video/material metadata
- manage tests
- assign courses to specialities

## Rules
- Admin authorization is checked on the backend.
- Admin UI calls normal protected backend APIs.
- Do not create uncontrolled direct DB access from frontend components.
- Destructive actions need explicit confirmation in the UI.
