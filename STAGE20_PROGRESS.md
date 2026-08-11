# Stage 20 — Lesson Q&A (Course Discussions)

**Status: Stage 20A (backend + student flow) complete and verified. Instructor/admin moderation UI and lesson-page frontend not started.**

Tracking doc — status only, not a spec restatement.

## Decision carried in from planning

Only enrolled students may ask questions — kept consistent with `internal/reviews`' enrollment/eligibility model (same idea, simpler: Q&A only needs enrollment, not reviews' extra "made progress" condition).

## Scope of this session (20A — backend + student-facing flow only)

New domain `internal/qa`. Student-facing ask/answer/list/delete-own only — instructor/admin moderation endpoints, the lesson-page frontend, and the instructor/admin moderation pages are explicitly Stage 20B/20C, not attempted here.

## Done

### Migration `00037_create_lesson_qa.sql`
- `lesson_questions(id, lesson_id, course_id, user_id, body, published, created_at, updated_at)` — `course_id` is denormalized from `lesson_id`'s chain at creation time (resolved once via `ownership.CourseIDForLesson`) so eligibility/ownership/moderation queries never need a three-way `lesson→module→course` join.
- `question_answers(id, question_id, user_id, body, is_instructor_answer, published, created_at, updated_at)`.
- Indexes: `lesson_questions(lesson_id)` (primary list read), `lesson_questions(course_id)` (future moderation-by-course), `lesson_questions(user_id)`, `question_answers(question_id)` (the no-N+1 list join), `question_answers(user_id)`. Both tables `ON DELETE CASCADE` from their parents, matching every other domain's FK convention.
- Applied live via the `migrate` Compose service → `goose: successfully migrated database to version: 37`.

### `internal/qa` (new domain — model/repository/service/handler)
- **model.go**: `Question`, `Answer`, `AnswerView` (adds display name, never email — mirrors `reviews.PublicReview`), `QuestionView` (question + its full answer list). Package doc comment explicitly notes this is unrelated to `internal/tests`' quiz "questions"/"answers" vocabulary — no route collision (different group prefixes: bare `/api/v1/*` here vs `/api/v1/admin/*`/`/api/v1/instructor/*` for quiz authoring) but a real naming-only ambiguity worth flagging for future readers.
- **repository.go**:
  - `IsEnrolled` — one query against `course_enrollments`, written directly here (not imported from `internal/learning`) per this codebase's established "share schema, not code" convention.
  - `ListForLesson` — **exactly two queries total, never N+1**: (1) paginated published questions for a lesson with the asker's display name and a window-function total, (2) every published answer for that page's question ids via `WHERE question_id = ANY($1)`, merged in Go keyed by question id. This mirrors the exact "merge multiple flat queries in Go" idiom Stage 19 already established twice (`GetRevenueAnalytics`, `GetPlanBreakdown`) rather than introducing `json_agg`/LATERAL, which nothing else in this codebase uses.
  - `CreateAnswer` — runs in its own transaction: insert the answer, look up the question's asker + lesson/course titles, then `notifications.Enqueue` (skipped entirely if the answerer is the asker — no self-notification).
  - `DeleteQuestion`/`DeleteAnswer` — IDOR-safe by construction: `WHERE id = $1 AND user_id = $2`; a row that exists but belongs to someone else produces the identical `RowsAffected()==0 → ErrNotFound` (404) outcome as a row that doesn't exist, exactly matching `internal/reviews.Delete`'s "never distinguish 'not yours' from 'doesn't exist'" convention.
- **service.go**:
  - `CreateQuestion` — resolves lesson→course via `ownership.CourseIDForLesson` (404 if the lesson itself doesn't exist), requires enrollment (403 otherwise), validates non-empty body.
  - `CreateAnswer` — the three-way authorization rule: admin always allowed; course-owning instructor allowed via `ownership.CanManageCourse` (the same centralized check every other instructor-facing write in this codebase uses — imported directly, not duplicated, since `ownership` is this codebase's one deliberate exception to "share schema, not code"); otherwise requires enrollment. `is_instructor_answer` is set from whichever condition matched, no extra query.
- **handler.go**: the five routes, `authctx.UserID`/`authctx.Role` for identity (never a client-supplied field), full error-code mapping (`VALIDATION_ERROR`/`LESSON_NOT_FOUND`/`NOT_ENROLLED`/`QUESTION_NOT_FOUND`/`ANSWER_NOT_FOUND`/`FORBIDDEN`).

### Notifications
- `internal/notifications/model.go`: new `TypeQuestionAnswered` constant, in-app only (same minimal-scope choice as the Stage 16 assignment-status types) — only the original asker is ever notified, never every thread participant.
- `internal/notifications/templates.go`: Russian in-app copy — "Ответ на ваш вопрос" / "На ваш вопрос по уроку «%s» (курс «%s») ответили."
- No email template added (matches the codebase's existing "email is reserved for the established minimum list" convention).

### Wiring (`cmd/api/main.go`)
- `qaRepo`/`qaService`/`qaHandler` constructed after `ownershipService` (Stage 14) and `wishlistHandler` (Stage 18), before `router := gin.Default()`.
- `qaHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())` added to the same block as `wishlistHandler`/`recommendationsHandler` — all five routes require auth.
- No `/admin` or `/instructor` routes registered this session (by design — Stage 20B/20C).

## Endpoints (all under `/api/v1`, all require auth)

| Method | Path | Behavior |
|---|---|---|
| GET | `/lessons/:id/questions` | Paginated, published-only, with nested answers |
| POST | `/lessons/:id/questions` | Requires enrollment |
| POST | `/questions/:id/answers` | Requires enrollment OR course-owning instructor OR admin |
| DELETE | `/questions/:id` | Own question only (404 otherwise) |
| DELETE | `/answers/:id` | Own answer only (404 otherwise) |

## Security verification (all live against Docker Compose)

- Enrolled student asks → **201**. Non-enrolled student asks → **403 NOT_ENROLLED**. No auth → **401**. Empty body → **400 VALIDATION_ERROR**. Ask on a nonexistent lesson → **404 LESSON_NOT_FOUND**.
- Full answer-authorization matrix tested with real accounts (a temporarily-assigned course-owning instructor, a non-owning instructor, an enrolled peer student, a non-enrolled student, and admin — course ownership reverted to its original `NULL` afterward):
  - Non-enrolled, non-owning student → **403**.
  - Non-owning instructor → **403** (confirms ownership is actually checked per-course, not just role).
  - Course-owning instructor → **201**, `is_instructor_answer: true`.
  - Enrolled peer student → **201**, `is_instructor_answer: false`.
  - Admin → **201**, `is_instructor_answer: true`.
- IDOR: cross-user delete of another student's question → **404** (not 403 — never confirms the row exists for a caller who doesn't own it). Cross-user delete of another user's answer → **404**. Delete of a nonexistent question → **404**, same code path.
- Identity: every check reads `userID`/`role` from `authctx` (verified JWT), never from a request body field — there is no `user_id` field anywhere in `questionRequest`/`answerRequest`.
- Cascade correctness: deleting a question's a live check, not just a migration assumption — confirmed the lesson's question list actually goes to zero after the owner deletes their question, and that the question's own answers disappear with it (`ON DELETE CASCADE`).
- Notification correctness: the original asker received exactly 3 notifications (one per instructor/peer/admin answer), each in Russian with the correct lesson/course title interpolated. Self-answer explicitly tested: a student who asks and then answers their own question generates **zero** notifications (before/after count identical) — confirms the "do not notify yourself" guard and, by construction (one `Enqueue` call per `CreateAnswer`, keyed by the new answer's own id), that no answer ever produces more than one notification.

## Performance findings

Seeded 100 questions + 200 answers on one real lesson (isolated by a `STAGE20A_SYNTHETIC_SEED` body-prefix marker for exact cleanup) and ran `EXPLAIN ANALYZE` on both queries `ListForLesson` issues:

| Query | Plan | Execution time |
|---|---|---|
| Paginated questions for a lesson | Bitmap Index Scan on `idx_lesson_questions_lesson_id` + per-row `users` pkey lookup + window agg | 0.548 ms |
| Answers for that page's question ids (`= ANY($1)`) | Bitmap Index Scan on `idx_question_answers_question_id` + hash join to `users` | 0.358 ms |

Both indexes confirmed actually used by the planner (not just present). **Exactly two queries total** regardless of page size or answer count — verified both by code inspection (repository.go issues exactly two `pool.Query` calls in `ListForLesson`) and by the query plans above (no per-row subquery, no loop). No further index needed at this stage.

Synthetic seed cleaned up precisely by its body-prefix marker (`DELETE ... WHERE body LIKE 'STAGE20A_SYNTHETIC_SEED question%'` and the matching answer-body literal) — verified table counts afterward reflect only genuine test data from the live flow above, not leftover synthetic rows.

## Regression

`GET /lessons/:id/progress`, `GET /courses/:id`, `GET /courses/:id/reviews`, `GET /admin/ping`, `GET /me/notifications` all re-checked live — all still 200, unaffected by this session's changes.

## Files changed

- `backend/migrations/00037_create_lesson_qa.sql` — new.
- `backend/internal/qa/{model,repository,service,handler}.go` — new domain.
- `backend/internal/notifications/model.go` — `TypeQuestionAnswered` constant.
- `backend/internal/notifications/templates.go` — in-app render case.
- `backend/cmd/api/main.go` — import, construction, route registration (11 lines).
- No frontend files touched. No `/admin` or `/instructor` route files touched.

## Known issues / observations

- **Naming ambiguity, not a bug**: `internal/tests` already uses the words "questions"/"answers" for quiz authoring, registered under `/admin/*` and `/instructor/*`. This domain's `/questions/:id`, `/questions/:id/answers`, `/answers/:id` live on the bare `/api/v1/*` prefix — no runtime route collision (different Gin route groups), but worth keeping in mind when reading route lists or writing further docs.
- **Unrelated pre-existing behavior observed, not touched**: `PUT /admin/users/:id` (in `internal/users`, a domain this session did not modify) appears to overwrite `first_name`/`last_name` to empty strings when a role-only update body is sent — observed live while setting up a test instructor account (registration itself correctly returned `first_name: "I", last_name: "Owner"`; after the role-promotion call, the DB row had empty strings). This did not affect any Stage 20A correctness check (authorization logic is entirely role/ownership/enrollment-based, never name-based) but is flagged here since it's a real, reproducible observation outside this session's scope to fix.
- No automated test suite exists in this codebase (established convention) — verification is `gofmt`/`go vet`/`go build` + live scripted checks, consistent with every prior stage.
- Refunded-style "hide" moderation doesn't exist yet for questions/answers — only owner-delete. Instructor/admin moderation (hide/show) is explicitly Stage 20B/20C.

## Remaining (explicitly out of scope, not attempted this session)

- Instructor moderation endpoints (`/instructor/questions`, hide/show).
- Admin moderation endpoints (`/admin/questions`, hide/show) + admin nav item.
- Lesson-page frontend (question list, ask form, answer form).
- Notification-bell frontend dispatch for `question_answered`.
- Full-platform regression beyond the lesson/course/admin/notification spot-checks above.
