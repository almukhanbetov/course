# Stage 21 — Q&A moderation completion + notification deep-link

Tracking doc — status only, not a spec restatement.

## Stage 21A1 — Q&A notification deep-link backend payload (this session)

Scope: backend-only payload change, precisely the gap identified in `STAGE20_PROGRESS.md`'s Stage 20B3 section. No frontend, no moderation (hide/show), no migration.

### Inspection performed

Re-read `backend/internal/qa/repository.go`'s `CreateAnswer` fresh (not trusted from the Stage 20 doc). Confirmed the query immediately above the `notifications.Enqueue` call already joins `lesson_questions` → `lessons` → `courses` inside the same transaction and only selected `lq.user_id, l.title, c.title`. `lesson_id` and `course_id` are both already plain columns on `lesson_questions` (denormalized at question-creation time per Stage 20A), so both were available for free in the same row — no extra join, no new query.

Checked how other domains put UUIDs into a notification `Data` map to match the established convention exactly, rather than inventing a new shape: `internal/assignments/repository.go:127` (`recalculateCourseCompletion`) does `Data: map[string]any{"course_id": courseID, "course_title": courseTitle}` with `courseID` as a raw `uuid.UUID` (not `.String()`) — `google/uuid.UUID` implements `MarshalText`, so `encoding/json` already renders it as a plain JSON string. Followed the same pattern here rather than converting to string manually.

### Change made

`backend/internal/qa/repository.go`, `CreateAnswer`:
- The pre-`Enqueue` lookup query now also selects `lq.lesson_id, lq.course_id` (two more columns on a row already being fetched) into two new local `uuid.UUID` variables.
- The `Data` map now has two more keys: `"lesson_id"` and `"course_id"`, alongside the existing `"lesson_title"`/`"course_title"`.

Nothing else in `CreateAnswer` changed: the self-answer skip (`if askerID != userID`), `DedupeKey`, `Channels`, transaction structure, and every other query in the file are untouched.

### Files changed

- `backend/internal/qa/repository.go` — `CreateAnswer`'s asker-lookup query gains two `SELECT` columns and two `Scan` targets; `Data` map gains `lesson_id`/`course_id`.

### Verification performed (focused, backend-only, per scope)

- `gofmt -l internal/qa/repository.go` — clean, no output.
- `go build ./internal/qa/...` — OK.
- `go vet ./internal/qa/...` — OK.
- `go build ./...` (whole backend, to confirm no ripple elsewhere) — OK.

No live Docker Compose verification, no E2E, no frontend changes performed this session — explicitly out of scope for 21A1.

### Deliberately not touched this session

- `frontend/lib/api.ts`'s `notificationActionLink()` — still has no `case "question_answered"`; the payload now carries `lesson_id`/`course_id` but nothing consumes them yet. This is the direct, obvious next slice (a future Stage 21 frontend session), not attempted here per explicit scope.
- Q&A hide/show moderation (`PATCH /instructor/questions/:id` etc.) — separate Stage 21 task, not started.
- Any E2E/regression pass.

## Stage 21A2 — Q&A notification deep-link frontend (this session)

Scope: frontend-only, consuming the `lesson_id`/`course_id` payload fields Stage 21A1 added. No backend change, no UI redesign.

### Inspection performed

Read `notificationActionLink()` in `frontend/lib/api.ts` (the single allow-listed `type → URL` switch every notification type goes through) and `app/dashboard/notifications/page.tsx` (the only place it's called). Confirmed the page already does `const link = notificationActionLink(n)` and conditionally renders an "Открыть" `<Link>` only when `link` is non-null — exactly the generic mechanism documented in `STAGE20_PROGRESS.md`'s Stage 20B3 section, unchanged since. Checked `AppNotification.data` — typed as `Record<string, unknown>`, so each case must narrow with `typeof` before use, matching every existing case (`certificate_id`, `course_id` on `course_announcement`). Confirmed the lesson-page URL convention (`/learn/${courseId}/${lessonId}`) against five existing call sites (`app/learn/[courseId]/[lessonId]/page.tsx`, `ContinueLearningCard.tsx`, `lib/actions.ts`) to reuse the exact same shape rather than inventing a new one.

### Change made

`frontend/lib/api.ts`, `notificationActionLink()`: added
```ts
case "question_answered":
  return typeof data.course_id === "string" && typeof data.lesson_id === "string"
    ? `/learn/${data.course_id}/${data.lesson_id}`
    : null;
```
right after the existing `course_announcement` case, following its exact `typeof`-narrowing pattern. Falls back to `null` (no "Открыть" button, current behavior) if either field is somehow absent — e.g. for any `question_answered` notification enqueued before Stage 21A1 shipped, whose stored `Data` payload only has `lesson_title`/`course_title`.

No other file changed: `app/dashboard/notifications/page.tsx` needed nothing, since it already renders the button generically off this function's return value.

### Files changed

- `frontend/lib/api.ts` — one new `case` in `notificationActionLink()`.

### Verification performed (focused, per scope)

- `npx tsc --noEmit` (whole project) — clean.
- `npx eslint lib/api.ts app/dashboard/notifications/page.tsx` — clean, no warnings.

No E2E, no live Docker Compose check, no other domain inspected — explicitly out of scope for 21A2.

## Stage 21B1 — Q&A hide/show moderation backend (this session)

Scope: backend-only real hide/show moderation for questions and answers — the other half of Stage 20B2's documented gap ("hide/show... require real backend endpoints that don't exist"). No frontend, no delete-of-others, no migration beyond confirming none was needed.

### Inspection performed

Re-read `internal/qa/{model,repository,service,handler}.go` fresh. Confirmed `published` already exists as a real column on both `lesson_questions` and `question_answers` (Stage 20A's migration), always `true` today since nothing could flip it — so **no migration was needed**, only a repository method to actually write to that column. Confirmed `ownership.CanManageCourse(ctx, userID, role, courseID)` (`internal/ownership/service.go:36`) already implements exactly the authorization rule this task needs on its own, with no extra wrapping required: `role == "admin"` → allowed if the course exists at all (platform-wide); `role == "instructor"` → allowed only if `courses.instructor_id = userID` for that specific course; anything else (including `"student"`) → `false`. This is the identical primitive `CreateAnswer` already uses for its instructor/admin branch, so the new moderation methods call it directly rather than re-deriving the rule.

**Route collision found and avoided before writing any code**: `internal/tests` already registers `PUT /questions/:id` and `PUT /answers/:id` on both the `/instructor` and `/admin` route groups (quiz-question authoring — see `internal/instructor/handler.go` and `internal/tests/handler_admin.go`). Registering the same bare paths for Q&A moderation on those same groups would have caused Gin to panic at server startup with a route conflict. Scoped the new routes under `/qa/questions/:id` and `/qa/answers/:id` instead, on both groups — this is a real instance of the naming ambiguity `internal/qa`'s own package doc comment already flagged ("no runtime route collision... but a real naming-only ambiguity worth remembering"), now concretely relevant for the first time since Stage 20A kept everything on the bare `/api/v1` prefix specifically to avoid it.

### Change made

- **`backend/internal/qa/repository.go`**: `SetQuestionPublished(ctx, id, published) (*Question, error)` and `SetAnswerPublished(ctx, id, published) (*Answer, error)` — plain `UPDATE ... SET published = $2, updated_at = now() WHERE id = $1 RETURNING ...`, `pgx.ErrNoRows` → `ErrNotFound`. No `WHERE user_id = ...`: unlike `DeleteQuestion`/`DeleteAnswer`, authorization here is course-ownership-based, not row-ownership-based, and is fully checked in the service layer before the repository is ever called. Also added `GetAnswerCourseID(ctx, answerID) (uuid.UUID, error)` — a one-query join (`question_answers` → `lesson_questions`) to resolve an answer's course for the authorization check, since answers don't carry `course_id` directly the way questions do.
- **`backend/internal/qa/service.go`**: `SetQuestionPublished`/`SetAnswerPublished` — fetch the question (or resolve the answer's course), call `ownership.CanManageCourse`, return `ErrForbidden` if `false`, otherwise delegate to the repository. Never deletes anything; only flips the flag `ListForLesson`'s existing `published = true` filter already respects, so hidden content disappears from the student-facing view with no change needed there.
- **`backend/internal/qa/handler.go`**: `publishedRequest{Published bool}` DTO (mirrors `internal/reviews`' identical `SetReviewPublished` DTO exactly — same field, same JSON tag), `SetQuestionPublished`/`SetAnswerPublished` handlers (userID+role from `authctx`, never a body field), and two new registration methods:
  - `RegisterInstructorRoutes(rg)` → `PUT /qa/questions/:id`, `PUT /qa/answers/:id`
  - `RegisterAdminRoutes(rg)` → the same two paths, same handler functions
  Both groups call the identical handler code — there's no route-group-specific branching, since authorization is fully re-derived per-request from the verified JWT + `CanManageCourse`, exactly like `CreateAnswer`'s existing three-way rule already works regardless of which surface reaches it.
- **`backend/cmd/api/main.go`**: `qaHandler.RegisterInstructorRoutes(instructorGroup)` added next to `assignmentsHandler`/`codingHandler`'s equivalent calls; `qaHandler.RegisterAdminRoutes(adminGroup)` added next to `notificationsHandler`'s equivalent call.

### Files changed

- `backend/internal/qa/repository.go` — two new `Set*Published` methods, one new `GetAnswerCourseID` method.
- `backend/internal/qa/service.go` — two new service methods.
- `backend/internal/qa/handler.go` — `publishedRequest` DTO, two new handlers, two new route-registration methods.
- `backend/cmd/api/main.go` — two new wiring lines (`instructorGroup`, `adminGroup`).
- No frontend files touched, no migration file added (none needed — `published` already existed).

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/qa/*.go cmd/api/main.go` — clean, no output.
- `go build ./...` — OK.
- `go vet ./...` — OK.

**Live, against a rebuilt Docker Compose backend** (`docker compose up -d --build backend`) — required specifically to catch the route-conflict panic risk identified above, since `go build`/`go vet` cannot detect a Gin route-registration conflict; confirmed via `docker compose logs backend` that all four new routes (`PUT /api/v1/instructor/qa/questions/:id`, `PUT /api/v1/instructor/qa/answers/:id`, `PUT /api/v1/admin/qa/questions/:id`, `PUT /api/v1/admin/qa/answers/:id`) registered and the server started cleanly with no panic.

**Focused authorization matrix** (fresh accounts: one student, two instructors each temporarily assigned ownership of a *different* real course — "Go Backend Developer" for instructor A, "PostgreSQL" for instructor B, the same genuine cross-instructor setup Stage 20C1 used — plus the seeded admin account; course ownership reverted to `NULL` afterward):

| Case | Expected | Result |
|---|---|---|
| Student, `/instructor/qa/questions/:id` | 403 (role middleware, never reaches handler) | **403** |
| Student, `/admin/qa/questions/:id` | 403 (role middleware) | **403** |
| Non-owning instructor B hides course X's question | 403 `FORBIDDEN` | **403 `FORBIDDEN`** |
| Non-owning instructor B hides course X's answer | 403 `FORBIDDEN` | **403 `FORBIDDEN`** |
| Owning instructor A hides the question | 200, `published:false` | **200**, `published:false` |
| Public `GET /lessons/:id/questions` after hide | hidden question absent from list | **absent**, confirmed |
| Owning instructor A shows it again | 200, `published:true` | **200**, `published:true` |
| Owning instructor A hides the answer | 200 | **200** |
| Admin hides the same content via `/admin` (owns nothing personally) | 200 — admin bypasses per-course ownership | **200** |
| Admin restores question + answer | 200 / 200 | **200 / 200** |
| Nonexistent question id | 404 `QUESTION_NOT_FOUND`, not 500 | **404** |
| No auth token | 401 | **401** |

All 12 cases passed on the first attempt. Zero bugs found, zero code changes needed after the initial implementation.

**Incidental finding, not a new bug**: re-confirmed the pre-existing `PUT /admin/users/:id` behavior already documented in `STAGE20_PROGRESS.md` (blanking `first_name`/`last_name` on a role-only update) also flips `active` to `false`, which blocks login — encountered while promoting the two test instructor accounts to `role: instructor` for this session's authorization matrix. Worked around it by flipping `active` back to `true` directly in the database for those two session-created test accounts only (not shared state, not a production account); did not touch the underlying `internal/users` handler, which remains out of scope for Stage 21.

### Cleanup

Test question + answer (tagged `S21B1_TEST`) deleted via the real `DELETE /questions/:id` endpoint (204, cascading the answer) — verified zero residual `S21B1_TEST`-tagged rows afterward. Both courses' temporarily-assigned `instructor_id` reverted to `NULL`. Test accounts (student, instructor A, instructor B) left in place, consistent with every prior Stage 20 session's convention — only shared/mutated state gets reverted, not the harmless standalone accounts themselves.

### Deliberately not touched this session

- Frontend: `QAModerationSection.tsx` still has no hide/show button — Stage 20B2 built it to render the `published` badge generically for exactly this reason, but no UI control exists yet to call the new endpoints. Next obvious slice (Stage 21B2), not attempted here per explicit scope.
- Admin/instructor ability to delete another user's question/answer — still not implemented; this session added hide/show specifically because it's the non-destructive, reversible action, and deliberately did not add a destructive cross-user delete alongside it.
- Any E2E/regression pass beyond the focused authorization matrix above.

## Stage 21B2 — Q&A hide/show moderation frontend (this session)

Scope: frontend-only, wiring the Stage 21B1 backend endpoints into the existing moderation UI. No backend change, no new endpoints, no delete-based fake moderation.

### Inspection performed

Re-read `STAGE21_PROGRESS.md`'s Stage 21B1 section and `components/QAModerationSection.tsx` (the single shared component both `app/instructor/questions/page.tsx` and `app/admin/questions/page.tsx` render) fresh. Confirmed the component's own doc comment explicitly recorded the exact gap this session closes: "does NOT include a hide/show toggle — the backend has no endpoint for that yet." Confirmed the component already renders `published` as a read-only badge (`"Опубликован"`/`"Скрыт"`) per question and answer, and already has the full state-management scaffolding (`useTransition` + `pendingKey` + per-item error maps) that every other action (`answerQuestionAction`, `deleteQuestionAction`, `deleteAnswerAction`) follows — the new actions reuse that exact pattern rather than inventing a new one.

**Key design decision**: Stage 21B1 registered the moderation endpoints on *both* `/instructor/qa/...` and `/admin/qa/...`, but re-reading `backend/cmd/api/main.go`'s `instructorGroup` middleware (`RequireAnyRole("instructor", "admin")`) confirmed an admin passes that gate too — and the service-layer `ownership.CanManageCourse` check already grants an admin access to any course regardless of which route reached it. So a single frontend action hitting `/instructor/qa/...` works correctly for both the instructor and admin moderation pages, exactly mirroring how `answerQuestionAction`/`deleteQuestionAction`/`deleteAnswerAction` already work identically across both pages without any role branching in the frontend. This avoided adding a second, redundant `/admin/qa/...`-calling action pair.

### Change made

- **`frontend/lib/actions.ts`**: two new server actions, `setQuestionPublishedAction(questionId, published)` and `setAnswerPublishedAction(answerId, published)` — same shape as every existing QA action in this file (`getSessionToken` → `fetch` with `Authorization` header → `parseQAError` on failure), `PUT` to `/api/v1/instructor/qa/questions/:id` / `/api/v1/instructor/qa/answers/:id` with `{ published }` as the body.
- **`frontend/components/QAModerationSection.tsx`**:
  - Imports the two new actions.
  - New `publishErrors` state map (parallel to the existing `deleteErrors`/`answerErrors` maps).
  - `handleSetQuestionPublished`/`handleSetAnswerPublished` — same `pendingKey`/`startTransition`/local-state-update pattern as every existing handler; on success, updates the specific question's or answer's `published` field in local state directly (no refetch), so the badge and button label flip instantly.
  - A "Скрыть"/"Показать" toggle button, always rendered (not gated by `user_id === currentUserId` the way delete is — moderation is a course-ownership permission, not a content-ownership one, and the page composing `groups` already scopes an instructor to only their own courses' questions while the admin page shows every course, so the button is always meaningful in this view; the backend independently re-verifies ownership regardless). Placed in a new `<span className="qa-moderation-actions">` alongside the existing delete button, for both questions and answers.
  - Loading state: button label switches to "Скрытие..."/"Показ..." while its own request is pending (mirrors the existing "Удаление..." pattern), and is `disabled` for the duration via the same shared `isPending`.
  - Error state: a `role="alert"` line under each item, scoped per-id, same pattern as `deleteErrors`/`answerErrors`.
  - Updated the component's top-of-file doc comment, which had explicitly documented the hide/show gap as unimplemented — now describes the real behavior.
- **`frontend/app/globals.css`**: one new small rule, `.qa-moderation-actions { display: flex; gap: 0.5rem; flex-shrink: 0; }`, wrapping the two action buttons inside the existing `.qa-question-header`/`.qa-answer-header` flex row (which already does `justify-content: space-between`) — no new colors or tokens, reuses `.btn-small`/`.badge` exactly as before.

### Files changed

- `frontend/lib/actions.ts` — two new server actions.
- `frontend/components/QAModerationSection.tsx` — imports, state, two new handlers, two new buttons (question + answer), updated doc comment.
- `frontend/app/globals.css` — one new layout-only rule.
- No backend files touched.

### Verification performed (focused, per scope)

- `npx tsc --noEmit` (whole project) — clean.
- `npx eslint components/QAModerationSection.tsx lib/actions.ts` — clean, zero warnings.

No E2E/live browser check, no Docker Compose rebuild, no other domain inspected — explicitly out of scope for 21B2. The backend authorization behavior these buttons rely on (owning instructor allowed, non-owning instructor 403, admin allowed platform-wide, student never reaches this page) was already live-verified end-to-end at the API level in Stage 21B1's session; this session only had to confirm the frontend calls the right endpoint with the right method/body and updates state correctly on success/failure, which `tsc`'s type-checking of the action's return shape (`SetQuestionPublishedResult`/`SetAnswerPublishedResult` against `QAQuestion`/`QAAnswer`) plus the code-level mirroring of already-proven patterns covers for this focused pass.

## Stage 21C — focused verification + final Stage 21 report (this session)

Scope: verify 21A1+21A2 (notification deep-link) and 21B1+21B2 (hide/show) live together, fix only bugs the verification itself surfaced, close out Stage 21. No new features attempted.

### Setup

Rebuilt both `backend` and `frontend` Docker Compose services (`docker compose up -d --build backend frontend`) to run this session's actual code — the frontend container was still serving the pre-21A2/21B2 image going into this session. Confirmed both healthy (`GET /api/v1/health` → 200, `GET /` → 200) and, via `docker compose logs backend`, that route registration is panic-free. Reused the student/instructor-A/instructor-B/admin test accounts and tokens from Stage 21B1 (still valid — JWTs hadn't expired, accounts untouched by any cleanup) rather than creating new ones.

### Bug found and fixed: hidden Q&A content disappeared from the moderator's own view

**Found live, during this session's own verification** (not a hypothetical): the instructor/admin moderation pages (`app/instructor/questions/page.tsx`, `app/admin/questions/page.tsx`) compose their view by calling `getLessonQuestions`, which hits the *public* `GET /lessons/:id/questions` endpoint — the same one `ListForLesson` has always filtered to `published = true` only. After hiding a question via Stage 21B1's new endpoint, the SSR-rendered moderation page's badge/button counts dropped from 5 "Опубликован"/"Скрыть" pairs to 3, with **zero** "Скрыт"/"Показать" anywhere — the hidden question had vanished from the moderator's own page, not just the student-facing one. This made "show restores visibility" (the explicit Stage 21C acceptance criterion) unreachable through the UI: there was no button left to click, since the row itself was gone from what the page fetched.

**Root cause**: Stage 21B1 added the *write* side of moderation (hide/show) but the moderation pages' *read* side was never updated off the public, published-only list — a real gap in Stage 21B1's own scope, not something introduced this session.

**Fix** (backend + frontend, minimal, mirrors the existing route/action conventions established in 21B1/21B2):
- `backend/internal/qa/repository.go`: refactored `ListForLesson` into a shared private `listForLesson(ctx, lessonID, limit, offset, includeHidden)`, with `ListForLesson` (public, unchanged behavior, `includeHidden=false`) and a new `ListForLessonModeration` (`includeHidden=true`) as thin wrappers. Both questions and answers queries use `AND (published = true OR $N)` instead of a hard `AND published = true`, so the flag threads through both without duplicating the two-query no-N+1 structure.
- `backend/internal/qa/service.go`: new `ListForLessonModeration(ctx, userID, role, lessonID, page, limit)` — resolves the lesson's course via `ownership.CourseIDForLesson`, then authorizes with the exact same `ownership.CanManageCourse` check `SetQuestionPublished`/`SetAnswerPublished` already use, before calling the repository. A non-owning instructor or student cannot see hidden content by calling this endpoint either — verified below.
- `backend/internal/qa/handler.go`: new `ListQuestionsModeration` handler; registered as `GET /qa/lessons/:id/questions` on both `RegisterInstructorRoutes` and `RegisterAdminRoutes` (same `/qa/` prefix convention 21B1 established to avoid the `internal/tests` route collision — a bare `/lessons/:id/questions` happened not to collide with anything today, but staying consistent with the established prefix avoids relying on that being true forever).
- `frontend/lib/api.ts`: new `getLessonQuestionsModeration`, hitting `GET /instructor/qa/lessons/:id/questions` (same "instructor path also serves admin" reasoning as 21B2's `setQuestionPublishedAction`/`setAnswerPublishedAction`, since that route group's `RequireAnyRole("instructor","admin")` plus `CanManageCourse` already handle both).
- `app/instructor/questions/page.tsx` / `app/admin/questions/page.tsx`: swapped `getLessonQuestions` → `getLessonQuestionsModeration`, updated their doc comments accordingly. No other page logic changed.

Verified fixed: rebuilt both containers, re-fetched the instructor moderation page SSR — the still-hidden question now correctly showed exactly 1 "Скрыт" badge and 1 "Показать" button (badge/button counts: 4 "Опубликован"/"Скрыть" + 1 "Скрыт"/"Показать" = the expected 5 total), clicked "Показать" (via the identical `PUT` the button calls), and confirmed the full 5/5 "Опубликован"/"Скрыть" state restored on both the instructor and admin pages.

### 1. Notification deep-link — verified live

- Real flow: student asked a fresh question (`S21C_TEST`), owning instructor A answered it, worker processed after ~4s. `GET /me/notifications` showed the new `question_answered` row with `data: {"course_id": "<real id>", "lesson_id": "<real id>", "course_title": "...", "lesson_title": "..."}` — both new fields present and correct, matching Stage 21A1's claim.
- SSR-fetched `/dashboard/notifications` with the student's real session cookie (`Cookie: lms_session=<jwt>` — confirmed from `lib/session.ts` that the cookie holds the raw backend JWT with no additional signing, so this is a faithful live check, not a synthetic one): found `href="/learn/11111111-1111-1111-1111-111111111111/33333333-3333-3333-3333-333333333331"` — the correct course/lesson pair — behind the "Открыть" link. **"Открыть" navigates to the correct lesson: confirmed.**
- **Older notifications without these fields fail safely**: inserted a `question_answered` notification directly into the `notifications` table with `data` containing only `lesson_title`/`course_title` (simulating genuine pre-Stage-21A1 legacy data, which is exactly what real rows created before this deploy look like). Re-fetched the SSR page: **200**, no server error, the legacy notification's title/message rendered normally, and the count of `href="/learn/...` links in the page did **not** increase — confirming `notificationActionLink` correctly returned `null` for it and the page safely omitted the button, exactly as designed. Deleted the fabricated row afterward.

### 2. Hide/show moderation — verified live (fresh matrix, post-fix)

| Case | Expected | Result |
|---|---|---|
| Owning instructor hides question | 200, `published:false` | **200** |
| Public list excludes hidden question | absent | **absent, confirmed** |
| Owning instructor shows question again | 200, `published:true` | **200** |
| Public list includes it again | present | **present, confirmed** |
| Owning instructor hides answer | 200 | **200** |
| Public list: question still shown, hidden answer excluded from its `answers` array | question present, answer absent | **confirmed** |
| Moderation list (`GET /instructor/qa/lessons/:id/questions`) still shows the hidden answer with `published:false` | visible | **visible, `published:false`** |
| Non-owning instructor hits the moderation *list* endpoint for a course they don't own | 403 `FORBIDDEN` | **403** |
| Student hits the moderation list endpoint | 403 (role middleware) | **403** |
| Admin views the moderation list for a course they don't personally own | 200, full data | **200** |
| Owning instructor restores the answer | 200, `published:true` | **200** |

All 11 cases passed. Combined with Stage 21B1's original 12-case authorization matrix on the hide/show *write* endpoints (student/non-owning-instructor/admin/owning-instructor, all re-confirmed structurally unchanged by this session's fix — the fix only touched the *read* path), the full moderation surface (list + toggle, both questions and answers, both route groups) is now verified end-to-end.

### 3. Frontend moderation UI — verified live

- **Published/hidden state accuracy**: SSR badge/button counts (`grep`-counted "Опубликован"/"Скрыт"/"Скрыть"/"Показать" occurrences against known question/answer counts) matched real backend state at every step of the hide → show cycle above, on both `/instructor/questions` and `/admin/questions`.
- **Buttons call the real backend**: confirmed by code inspection (unchanged since 21B2 — `setQuestionPublishedAction`/`setAnswerPublishedAction` in `lib/actions.ts` `PUT` to the exact live-tested endpoints with `{ published }`) plus this session's live re-verification that those exact endpoints behave correctly.
- **No fake delete-based moderation**: confirmed by code inspection (`QAModerationSection.tsx` calls `setQuestionPublishedAction`/`setAnswerPublishedAction` for hide/show and a distinct `deleteQuestionAction`/`deleteAnswerAction` for delete — never one standing in for the other) and by a live spot-check: on instructor A's rebuilt moderation page, exactly 1 "Удалить" button rendered — matching precisely the one answer instructor A had personally authored across all their courses' Q&A, never appearing next to content they didn't author. Delete remains strictly own-content-only, hide/show is the only lever over anyone else's content.
- **Student cannot reach either moderation page at all**: `GET /instructor/questions` and `GET /admin/questions` with a student session cookie both → **307** (the pre-existing Stage 20 layout role gate, untouched by Stage 21).
- Loading/error states were not exercised via real browser interaction this session (no browser-automation tool available in this environment) — see Known limitations.

### Final checks

- `gofmt -l .` (whole backend) — clean.
- `go build ./...` — OK.
- `go vet ./...` — OK.
- `npx tsc --noEmit` (whole frontend) — clean.
- `npx eslint .` (whole frontend) — 0 errors, same 4 pre-existing unrelated `<img>` warnings every prior stage has reported, nothing new.
- Docker Compose: both `backend` and `frontend` rebuilt and live; `docker compose logs backend` confirms all 6 Q&A moderation routes (`GET`/`PUT` × `{instructor,admin}` × `{lessons/:id/questions, questions/:id, answers/:id}`) registered with no route-conflict panic.

### Security results (summary)

Every Stage 21 write and read path re-derives authorization from the verified JWT per request via `ownership.CanManageCourse` — never a client-supplied field, never trusted from which route group was called. Confirmed this session: a non-owning instructor is rejected identically (403 `FORBIDDEN`) whether hitting the hide/show toggle *or* the newly-added moderation list endpoint; a student is rejected by role middleware before reaching any handler on either; an admin succeeds on any course via both without needing personal ownership. No destructive cross-user action exists anywhere in Stage 21 — hide/show only ever flips an existing flag, delete remains strictly own-content-only (unchanged from Stage 20). The notification deep-link never trusts a raw URL from the payload — `notificationActionLink` only ever builds an allow-listed path shape from typed, narrowed fields, and fails to `null` (no button) rather than guessing when data is missing or malformed.

### E2E results (summary)

Notification deep-link: real answer → real notification → correct `course_id`/`lesson_id` → correct "Открыть" href → confirmed live. Legacy-shaped notification (missing those fields): renders safely, no crash, no broken link. Hide/show: full cycle (hide → verify excluded from public view → show → verify restored) verified for both questions and answers, from both the owning-instructor and admin angles, using the exact endpoints the real frontend actions call. Moderation UI: SSR-rendered state matches backend truth at every step; delete remains scoped to own content only.

### Files changed (all of Stage 21: 21A1 + 21A2 + 21B1 + 21B2 + 21C)

Backend:
- `backend/internal/qa/repository.go` — `CreateAnswer` payload (21A1); `SetQuestionPublished`, `SetAnswerPublished`, `GetAnswerCourseID` (21B1); `ListForLesson`/`ListForLessonModeration` refactor (21C).
- `backend/internal/qa/service.go` — `SetQuestionPublished`, `SetAnswerPublished` (21B1); `ListForLessonModeration` (21C).
- `backend/internal/qa/handler.go` — `publishedRequest`, `SetQuestionPublished`, `SetAnswerPublished`, `RegisterInstructorRoutes`, `RegisterAdminRoutes` (21B1); `ListQuestionsModeration` + route registration (21C).
- `backend/cmd/api/main.go` — `qaHandler.RegisterInstructorRoutes`/`RegisterAdminRoutes` wiring (21B1).

Frontend:
- `frontend/lib/api.ts` — `notificationActionLink`'s `question_answered` case (21A2); `getLessonQuestionsModeration` (21C).
- `frontend/lib/actions.ts` — `setQuestionPublishedAction`, `setAnswerPublishedAction` (21B2).
- `frontend/components/QAModerationSection.tsx` — hide/show buttons, state, handlers (21B2).
- `frontend/app/globals.css` — `.qa-moderation-actions` (21B2).
- `frontend/app/instructor/questions/page.tsx`, `frontend/app/admin/questions/page.tsx` — switched to `getLessonQuestionsModeration` (21C).

No migration files added at any point in Stage 21 — `published` already existed on both tables since Stage 20A.

### Known limitations (Stage 21, final)

- **No real browser interaction this session**: loading-state transitions and client-side error rendering for the hide/show buttons were verified by code inspection (same `useTransition`/`pendingKey`/error-map pattern as every other action in the same component, already exercised live for answer/delete in Stage 20C1) plus confirming the exact backend calls they make behave correctly — not by driving an actual click in a browser, since no browser-automation tool is available in this environment. Lower risk than most gaps of this shape because the mechanism is copy-identical to already-proven code paths in the same file.
- **Moderation pages remain application-level composition** (course → lesson → questions in a `Promise.all`, not a single backend aggregate query) — unchanged scope note from Stage 20B2, still accurate; the new `ListForLessonModeration` call is a drop-in replacement for the per-lesson fetch, not a structural change to that composition.
- **Admin/instructor cross-user delete still does not exist** — unchanged from Stage 20/21B1: only hide/show (non-destructive) and own-content delete exist. Still a deliberate scope boundary, not an oversight.
- **Pre-existing, unrelated**: `PUT /admin/users/:id` blanking `first_name`/`last_name` and (newly re-confirmed, first noticed in 21B1) also flipping `active` to `false` on a role-only update — encountered again this session only incidentally (reused already-promoted, already-`active`-restored test accounts from 21B1, so it didn't need re-working-around this time). Still not touched; still out of scope for Stage 21.
- No automated test suite exists anywhere in this codebase (unchanged, project-wide convention) — every verification claim above is a live, scripted check against the running Docker Compose stack.

### Final Stage 21 status

**Stage 21 is complete.** Both of Stage 20's precisely-scoped deferred items — the `question_answered` notification deep-link (21A1 backend + 21A2 frontend) and Q&A hide/show moderation (21B1 backend + 21B2 frontend) — are implemented, live-verified, and, as of this session, verified *together* including the moderation-view visibility bug this session's own testing surfaced and fixed (21C). Zero known open bugs. The three items listed under "Known limitations" above are deliberate, documented scope boundaries — not defects — consistent with how every prior stage in this project has closed out.
