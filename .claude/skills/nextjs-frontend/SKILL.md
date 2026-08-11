---
name: nextjs-frontend
description: Build the LMS frontend with Next.js, TypeScript and App Router.
---

# Next.js Frontend

Use:
- Next.js
- TypeScript
- App Router
- Server Components by default
- Client Components only for interaction

## Suggested routes
Public:
- /
- /courses
- /courses/[slug]
- /specialities
- /specialities/[slug]
- /pricing
- /login
- /register

Student:
- /dashboard
- /dashboard/courses
- /dashboard/progress
- /dashboard/certificates
- /dashboard/profile
- /learn/[courseSlug]/[lessonId]

Admin:
- /admin
- /admin/users
- /admin/courses
- /admin/specialities
- /admin/tests
- /admin/subscriptions
- /admin/payments

## Rules
- Do not put the entire app in one `page.tsx`.
- Reuse components.
- Keep API access in a dedicated service/client layer.
- Never expose backend secrets to browser code.
- Backend authorization is authoritative.
- Frontend validation improves UX but does not replace backend validation.
