# Roadmap — Stage 21 to Stage 30

Planning document only. No implementation performed. Written after reviewing `STAGE20_PROGRESS.md`, `README.md`, and the current backend domain list (`backend/internal/*`), route registrations (`backend/cmd/api/main.go`), and frontend routes, to avoid re-proposing anything already built.

## Baseline: what's already done (through Stage 20)

Confirmed present in the codebase, so **not** re-proposed below:

- Auth/RBAC, courses/modules/lessons, enrollment, learning progress, Continue Learning.
- Wishlist, personalized recommendations, similar courses, roadmap/specialties (Stage 18).
- Tests/quizzes, coding exercises, assignments with instructor grading (`internal/assignments`, submission review flow).
- Certificates **including public verification** (`GET /certificates/verify/:certificateNumber`, live at `frontend/app/certificates/verify/[certificateNumber]`) — so "certificates/public verification" from the candidate list is already shipped, not proposed again.
- Achievements, streaks, analytics (revenue/plan breakdown, course/instructor stats).
- Notifications (in-app, worker-based queue).
- Instructor dashboard: courses, modules/lessons, video, tests, students, stats, reviews, submissions, Q&A moderation view.
- Admin dashboard: users, courses, reviews (**with existing publish/unpublish moderation** — `PUT /admin/reviews/:id`), certificates, plans, subscriptions, payments, analytics, categories, specialities, tests, notifications, course-submissions, Q&A moderation view.
- Subscriptions and payments, with provider-state-as-source-of-truth discipline.
- Lesson Q&A (Stage 20): ask/answer/delete-own, three-way authorization, IDOR-safe deletes, no-N+1 list query.
- Course full-text search (`SearchCourses`, `websearch_to_tsquery` + ILIKE fallback) — present, but **no autocomplete/suggestions endpoint or UI** yet.
- Security discipline established project-wide: JWT-only identity, IDOR-safe 404-not-403 deletes, ownership checks via `internal/ownership`, `EXPLAIN ANALYZE`-verified no-N+1 queries, live Docker Compose verification (no automated test suite exists anywhere in this codebase — every stage verifies live).

Explicitly confirmed **absent** (verified by grep/search, not assumed):

- No `.github/` directory — no CI/CD pipeline at all.
- No audit log table or logging of admin/instructor mutating actions.
- No structured logging, metrics endpoint, or error-tracking integration (no zap/zerolog/Prometheus/Sentry references anywhere).
- No backup/restore tooling or documented runbook.
- No rate limiting on `/auth/login`, `/auth/register`, `/auth/refresh` (rate limiting exists only inside `internal/coding` for code execution).
- No search autocomplete/suggestions endpoint, no `pg_trgm`.
- No recommendation feedback mechanism (dismiss/not-interested) — recommendations are read-only today.
- No content-abuse reporting (flag) mechanism for Q&A or reviews.
- Two items Stage 20 itself deferred and precisely scoped: the `question_answered` notification deep-link, and Q&A hide/show moderation (today only delete-own exists).

These gaps directly shaped the stage selection below. "Mobile application" from the README's old wishlist-style roadmap section is intentionally not included — no signal in the codebase that it's still wanted, and it doesn't compose with the small-session model this roadmap follows.

## Session-sizing convention

Every stage below is written to be split the same way Stage 20 was: an **A** session (backend/migration), one or more **B** sessions (frontend, split further if the surface is large), and a **C** session (security + regression verification). "Small" fits comfortably in A+B+C as three short sessions; "Medium" typically needs a B1/B2 split; "Large" stages are written as a sequence of narrower sub-stages for the same reason Stage 20 itself was split into 20A/20B1/20B2/20B3/20C1/20C2 rather than attempted in one sitting.

---

## Stage 21 — Close Stage 20's deferred items: Q&A moderation + notification deep-link

**Goal:** Ship the two items Stage 20 explicitly scoped but declined to implement, before building anything new on top of Q&A: real hide/show moderation (today only delete-own exists) and the `question_answered` notification deep-link.

- **Backend scope:** Extend `internal/qa`'s `CreateAnswer` to also select and enqueue `lesson_id`/`course_id` in the notification `Data` payload (the query already fetches the parent row in the same transaction — precisely scoped in `STAGE20_PROGRESS.md`'s Stage 20B3 section). Add `PATCH /instructor/questions/:id`, `PATCH /instructor/answers/:id`, `PATCH /admin/questions/:id`, `PATCH /admin/answers/:id` to toggle `published`, authorized via the existing `ownership.CanManageCourse` check (admin unconditionally) — soft hide, not destructive delete of another user's content, to keep the change reversible and low-risk.
- **Frontend scope:** One new `case "question_answered"` in `notificationActionLink()` (`lib/api.ts`) building `/learn/{courseId}/{lessonId}`. Hide/show buttons in `QAModerationSection.tsx`, gated the same way "Ответить"/"Удалить" already are, driven by the real `published` field the API already returns (the UI was deliberately built in Stage 20B2 to render this generically for exactly this reason).
- **Migration needs:** None required for the notification fix. Optional: `hidden_by`/`hidden_at` nullable columns on `lesson_questions`/`question_answers` if an accountability trail is wanted alongside the toggle — small, additive, no backfill.
- **Security requirements:** Hide/show must fail 403/404 for a non-owning instructor exactly like `DeleteQuestion`/`DeleteAnswer` already do (reuse the same authorization test matrix Stage 20C1 ran — two instructors owning different real courses, cross-course attempt). Re-confirm the notification payload doesn't leak any field beyond `lesson_id`/`course_id`.
- **E2E/regression requirements:** Full Q&A moderation live test: instructor hides own-course content → student view correctly stops showing it; cross-instructor hide attempt → 403; admin hide works platform-wide. Notification deep-link: trigger a real `question_answered` notification, click "Открыть", confirm it lands on the correct lesson. Spot-regress the adjacent flows Stage 20C2 already covered (lesson page, notifications list, both moderation dashboards).
- **Dependencies:** Stage 20 (`internal/qa`, `internal/notifications`).
- **Estimated complexity:** Small.

---

## Stage 22 — Search autocomplete & suggestions

**Goal:** Turn the existing full-text course search into a real type-ahead experience — suggestions as the user types, not just a submit-and-reload results page.

- **Backend scope:** New lightweight endpoint, e.g. `GET /search/suggestions?q=`, returning a capped list (top 5–8) of published course titles/categories ranked by relevance, reusing `courses.SearchCourses`'s existing `search_vector`/ILIKE logic rather than duplicating it. Add `pg_trgm`-backed prefix/fuzzy matching for short queries where `websearch_to_tsquery` already falls back to ILIKE (per the comment in `courses/repository.go`).
- **Frontend scope:** Autocomplete dropdown on the course search input (navbar and `/courses`), debounced, keyboard-navigable (arrow keys + Enter), click-through to the course or a filtered results page. Loading/empty/error states for the dropdown itself.
- **Migration needs:** `CREATE EXTENSION IF NOT EXISTS pg_trgm;` + a GIN trigram index on `courses.title` (and optionally `description`) for fast prefix/fuzzy suggestion queries.
- **Security requirements:** Public, unauthenticated endpoint — must only ever return published courses (same filter `SearchCourses` already applies), no leakage of draft/unpublished titles. Basic input-length guard (e.g. reject/no-op on empty or absurdly long queries) to keep it cheap to call from every keystroke.
- **E2E/regression requirements:** Type Cyrillic and Latin queries, confirm suggestions match `/courses` full-search results for the same query; confirm draft courses never appear; confirm the dropdown renders suggestion text safely (no injection); regress `/courses` filtering (category/level/access_type) to confirm nothing in `SearchCourses` was disturbed.
- **Dependencies:** Stage 18's `SearchCourses`/`search_vector`.
- **Estimated complexity:** Medium.

---

## Stage 23 — Recommendation feedback loop

**Goal:** Let a user dismiss a recommended or similar course ("not interested"), and have future recommendation calls honor that signal — closing the read-only gap in an otherwise complete recommendations feature.

- **Backend scope:** New `POST /recommendations/:courseId/dismiss` (and a matching `DELETE` to undo) in `internal/recommendations`; the personalized-recommendations and similar-courses queries exclude any course the current user has dismissed.
- **Frontend scope:** A small dismiss ("×") affordance on recommendation and similar-course cards (dashboard + course detail page), optimistic removal from the list with an "Undo" toast.
- **Migration needs:** New `recommendation_feedback(user_id, course_id, created_at)` table, unique on `(user_id, course_id)`, `ON DELETE CASCADE` from `users`/`courses` matching every other domain's FK convention.
- **Security requirements:** Strictly self-scoped — a dismiss can only ever affect the acting user's own future recommendations (read from JWT, never a body field); verify one user's dismissals never affect another's results (the same isolation check Stage 18 already ran for wishlist).
- **E2E/regression requirements:** Dismiss a course → confirm it disappears from that user's next recommendations/similar-courses call; confirm a second, unrelated user still sees it; confirm undo restores it. Regress Stage 18's existing recommendation/wishlist/similar-courses flows.
- **Dependencies:** Stage 18 (`internal/recommendations`).
- **Estimated complexity:** Small/Medium.

---

## Stage 24 — Content abuse reporting + admin moderation queue

**Goal:** Give students a way to flag inappropriate Q&A content or reviews, and give admins one queue to work from instead of browsing every course looking for problems — the natural next step now that Stage 21 gives moderation a real hide action to apply.

- **Backend scope:** New `internal/reports` domain: `POST /questions/:id/report`, `POST /answers/:id/report`, `POST /courses/:id/reviews/:reviewId/report` (reason + optional note); `GET /admin/reports` (open queue, paginated) and `PATCH /admin/reports/:id` to resolve (dismiss the report, or hide the underlying content via Stage 21's/existing review-publish toggle from the same action).
- **Frontend scope:** A "Report" control on Q&A questions/answers (`QASection.tsx`) and on reviews, with a short reason picker. New `/admin/reports` page: list, filter by status/type, one-click resolve-and-hide.
- **Migration needs:** New `content_reports(id, reporter_id, content_type, content_id, reason, status, created_at, resolved_at)` table with an index on `(status, created_at)` for the queue, and a unique constraint on `(reporter_id, content_type, content_id, status)` scoped to open reports to prevent duplicate spam-reporting.
- **Security requirements:** Reporting requires auth but not enrollment (anyone with account access to the content can flag it); reporters must never see other users' reports; admin queue is admin-only (403 for instructor/student); resolving a report that hides content must re-run the exact Stage 21 authorization path, not a shortcut.
- **E2E/regression requirements:** Report → appears in admin queue; duplicate report from the same user on the same open item is rejected or no-ops (not a second row); resolve-and-hide → content actually disappears from the student-facing view; non-admin hitting `/admin/reports` → 403.
- **Dependencies:** Stage 21 (hide/show), Stage 20 (`internal/qa`), existing `internal/reviews` moderation.
- **Estimated complexity:** Medium.

---

## Stage 25 — Platform audit log

**Goal:** A durable, admin-readable trail of who did what to sensitive data — role changes, content hides/report resolutions, certificate issuance, subscription/payment overrides — for accountability and incident debugging, none of which exists today.

- **Backend scope:** New `internal/audit` domain with a single `Log(actor_id, action, entity_type, entity_id, metadata)` helper, called explicitly from the handful of highest-risk existing mutations (role changes in `internal/users`, hide/report-resolve from Stages 21/24, any manual subscription/payment admin overrides) — not a blanket middleware logging every request. `GET /admin/audit-log` (paginated, filterable by actor/entity/date).
- **Frontend scope:** New `/admin/audit-log` page — read-only table with filters, no write actions.
- **Migration needs:** New `audit_logs(id, actor_id, action, entity_type, entity_id, metadata jsonb, created_at)` table, indexed on `(entity_type, entity_id)` and `(actor_id, created_at)`.
- **Security requirements:** Admin-only read (no instructor access, even to their own courses' entries). No update/delete endpoint — log rows are append-only by construction. Explicit review of every call site to confirm `metadata` never captures a password, token, or payment secret.
- **E2E/regression requirements:** Perform one tracked action per logged domain (e.g. a role change, a Q&A hide) → confirm a correctly-attributed log row appears; confirm a non-admin gets 403 on the audit endpoint; confirm logging failures never block the underlying action itself (log write should not be able to fail a real user-facing mutation).
- **Dependencies:** Stage 21 (hide events) and Stage 24 (report-resolve events) as the first two real log producers; touches `internal/users`, `internal/subscriptions` admin paths.
- **Estimated complexity:** Medium.

---

## Stage 26 — Auth hardening: rate limiting & session security

**Goal:** Close the remaining first-order attack surface on the auth domain — brute-force login/register throttling and refresh-token hygiene — before Stage 22–24 add more public/authenticated surface area on top of it.

- **Backend scope:** Rate-limiting middleware on `/auth/login`, `/auth/register`, `/auth/refresh` (mirroring the token-bucket pattern already used in `internal/coding` for code execution, adapted to IP+account keys). Audit and, if needed, tighten refresh-token rotation/expiry in `internal/auth`.
- **Frontend scope:** User-facing error/backoff messaging only for the throttled case — no new pages.
- **Migration needs:** Likely none if rate limiting is in-process/token-bucket (matching the existing `internal/coding` approach, since there's no Redis in `docker-compose.yml` today); a `login_attempts` table only if a durable, multi-instance-safe limiter is judged necessary — decide during the session based on whether the backend ever runs more than one replica.
- **Security requirements:** This stage's deliverable *is* the security work: live scripted brute-force attempts must actually get throttled; verify no legitimate user gets false-positive locked out under normal retry behavior; confirm limiter state never leaks across accounts/IPs.
- **E2E/regression requirements:** Normal login/register/refresh continue to work; scripted abuse triggers throttling; full-platform smoke regression since this touches shared auth middleware every request passes through (spot-check enrollment, payments, video, certificates — the same breadth Stage 20's final regression covered for Q&A).
- **Dependencies:** Existing `internal/auth`; no dependency on Stages 21–25.
- **Estimated complexity:** Medium.

---

## Stage 27 — CI pipeline (build, lint, gate)

**Goal:** First production-readiness step — every push/PR automatically gets the same `gofmt`/`go vet`/`go build` and `tsc`/`eslint`/`next build` checks every prior stage has been running by hand, before any deploy automation is built on top.

- **Backend scope:** No application code changes. Add `.github/workflows/ci.yml`: backend job runs `gofmt -l .`, `go vet ./...`, `go build ./...` (plus `go test ./...` if/when any tests exist) against a GitHub-provided ephemeral Postgres service container matching the current schema via the existing goose migrations.
- **Frontend scope:** CI job runs `npx tsc --noEmit`, `npx eslint .`, `npm run build` — the exact three commands every prior stage's progress doc already reports running manually.
- **Migration needs:** None.
- **Security requirements:** No secrets committed to the workflow file; any DB URL/JWT secret used by CI is a throwaway CI-only value via GitHub Actions secrets, never the real `.env`. Document (not enforce, since that's a GitHub repo-settings action) that branch protection should require this check.
- **E2E/regression requirements:** Verify the pipeline actually fails on a deliberately broken commit pushed to a throwaway branch, then passes on a clean commit — proving the gate is real, not a no-op.
- **Dependencies:** None beyond existing tooling; no dependency on Stages 21–26.
- **Estimated complexity:** Medium.

---

## Stage 28 — CD & production deployment automation

**Goal:** Automate what's currently a manual `docker compose up -d --build` into a real deploy pipeline, building on Stage 27's gate.

- **Backend/Frontend scope:** No application logic changes. Harden both Dockerfiles (multi-stage builds, non-root user, slim final images). Add `.github/workflows/deploy.yml` triggered on merge to `main`: build+push images, SSH/deploy to the target VPS, run the `migrate` Compose service automatically as part of the rollout, then restart the stack.
- **Migration needs:** None new, but the deploy workflow must run existing goose migrations unattended as a required step, not a manual one.
- **Security requirements:** All secrets (SSH key, DB credentials, JWT secret, provider keys) live in GitHub Environments/secrets, never in the repo or baked into images. Nginx + HTTPS (Let's Encrypt/certbot) reverse proxy in front of the frontend/backend. Explicit check that no dev-only default (e.g. a placeholder JWT secret from `.env.example`) can silently reach production — fail closed if a required secret is missing.
- **E2E/regression requirements:** Post-deploy smoke test against the real deployed environment: health check, login, enroll, a payment-provider sandbox transaction, certificate verification — plus a documented, tested rollback procedure (redeploy the previous image tag).
- **Dependencies:** Stage 27 (CI gate must pass before anything deploys).
- **Estimated complexity:** Large — recommend splitting into a Dockerfile/image-hardening session, a deploy-workflow session, and an Nginx/HTTPS session, the same way Stage 20 split across sub-sessions.

---

## Stage 29 — Observability: structured logging, health, and alerting

**Goal:** Give the now-deployed production stack a way to be watched — today there's no structured logging, metrics, or error tracking anywhere in the codebase.

- **Backend scope:** Replace ad-hoc `log`/`fmt.Println` calls with structured logging (Go's standard `log/slog`, no new heavy dependency needed) including a request-id correlated with each HTTP request. Extend the existing `internal/health` check into a deeper check (DB reachable, MinIO reachable, notification-worker queue depth) rather than a bare 200. Optional: wire an error-tracking integration (e.g. Sentry) behind an env flag, off by default.
- **Frontend scope:** None required; optionally a minimal `/admin/system-health` page surfacing the deep health check for a human to glance at.
- **Migration needs:** None.
- **Security requirements:** The deep health endpoint must never expose internal details (hostnames, credentials, stack traces) to an unauthenticated caller beyond a boolean/degraded status. Explicit audit of every new log statement to confirm no token/password/payment secret is ever logged.
- **E2E/regression requirements:** Stop a dependency in the Docker Compose stack (e.g. MinIO) and confirm the health check correctly reports degraded while unrelated request flows still behave correctly (fail gracefully, not crash) — mirroring the existing documented convention that MinIO misconfiguration must not break unrelated flows (noted already in `main.go`'s comments).
- **Dependencies:** Stage 28 (a deployed environment worth monitoring); logically pairs with Stage 25's audit log as the two halves of "what happened and when."
- **Estimated complexity:** Medium/Large.

---

## Stage 30 — Backups, recovery & final production hardening

**Goal:** Close the last production-readiness gap — there is no backup/restore story today — and run the full-platform security and regression sweep Stage 20 explicitly deferred ("payments/subscriptions, video pipeline, certificates, achievements, search/recommendations were not re-tested" per `STAGE20_PROGRESS.md`), now that Stages 21–29 have added real new surface area on top of the existing platform.

- **Backend/Ops scope:** Scheduled `pg_dump` backup (cron container or a scheduled GitHub Actions workflow hitting the production DB over a secure channel) targeting object storage (reuse the existing MinIO/S3 infra already in `docker-compose.yml`), with a retention policy and a written, **tested** restore runbook.
- **Frontend scope:** None.
- **Migration needs:** None.
- **Security requirements:** Backup artifacts encrypted at rest and access-restricted (not a world-readable bucket); the full-platform security sweep re-confirms auth, IDOR, payment-provider-as-source-of-truth, video delivery, and every admin/instructor ownership boundary added or touched since Stage 20, in one consolidated pass.
- **E2E/regression requirements:** Actually perform one restore (into a scratch database) and verify row counts/integrity match the source — a backup that has never been restored is not a verified backup. Run the full-platform regression pass every prior stage's own progress doc has been deferring: enrollment, learning, payments, certificates, Q&A, search, recommendations, admin/instructor dashboards, all smoke-tested together.
- **Dependencies:** Stage 28 (deployed environment to back up), Stage 29 (observability, to detect a failed backup job).
- **Estimated complexity:** Large — recommend a backup/restore session and a separate full-platform regression session, the same split pattern as Stage 20's 20C1/20C2.

---

## Summary table

| Stage | Goal | Complexity | Depends on |
|---|---|---|---|
| 21 | Close Stage 20 deferrals: Q&A hide/show moderation + notification deep-link | Small | Stage 20 |
| 22 | Search autocomplete & suggestions | Medium | Stage 18 (search) |
| 23 | Recommendation feedback (dismiss / not-interested) | Small/Medium | Stage 18 (recommendations) |
| 24 | Content abuse reporting + admin moderation queue | Medium | Stage 21, Stage 20, reviews |
| 25 | Platform audit log | Medium | Stage 21, Stage 24 |
| 26 | Auth hardening: rate limiting & session security | Medium | — (existing auth domain) |
| 27 | CI pipeline (build/lint/gate) | Medium | — |
| 28 | CD & production deployment automation | Large | Stage 27 |
| 29 | Observability: logging, health, alerting | Medium/Large | Stage 28 |
| 30 | Backups/recovery + final full-platform hardening sweep | Large | Stage 28, Stage 29 |
