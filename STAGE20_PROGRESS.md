# Stage 20 — Lesson Q&A (Course Discussions)

**Status: STAGE 20 COMPLETE.** Backend (20A), student frontend (20B1), instructor/admin moderation UI (20B2), security + focused E2E (20C1), and final adjacent-flow regression (20C2) all done, zero unresolved bugs. Stage 20B3 (notifications deep-link) is the one deliberately deferred, precisely-scoped enhancement — see below.

Tracking doc — status only, not a spec restatement.

## Stage 20C2 — final regression + Stage 20 report (this session)

Final adjacent-flow regression pass before closing out Stage 20. **Zero regressions found; zero code changes made.**

### Final checks
- Backend: `gofmt -l .` clean, `go vet ./...` clean, `go build ./...` clean (unchanged since Stage 20A/20C1).
- Frontend: `npx tsc --noEmit` clean, `npx eslint .` clean (same 4 pre-existing unrelated `<img>` warnings, nothing new).
- `npm run build` (full production build) — clean, **all 41 routes** compiled successfully, including both new Stage 20 pages (`/instructor/questions`, `/admin/questions`) alongside every pre-existing route with no collisions.
- Docker Compose stack already healthy and running current code for both services (frontend last built during Stage 20C1, no source changes since) — smoke-tested `GET /api/v1/health` and `GET /` both 200, no rebuild needed.

### Regression results (adjacent flows, live against Docker Compose, fresh accounts)
- **Lesson page**: enrolled a fresh student, loaded `/learn/:courseId/:lessonId` — video placeholder, progress bar, lesson-nav sidebar, and the "Завершить урок" action all still render correctly alongside the new Q&A section. `PUT /lessons/:id/progress` still works (200). Assignment/coding-exercise endpoints correctly 404 for lessons that have none attached — confirmed this is pre-existing seed-data state (no assignments or coding exercises exist anywhere in the current database), not something Stage 20 removed; the conditional `{assignment && ...}`/`{codingExerciseView && ...}` blocks in the lesson page were never touched by Stage 20 and correctly render nothing when absent.
- **Reviews**: `GET /courses/:id/reviews` (public list) 200, `GET /courses/:id/reviews/me` correctly 404 for a student who hasn't reviewed yet (documented existing behavior, not a bug). Course detail page still renders its "Отзывы" and "Программа курса" sections plus the wishlist toggle button correctly.
- **Enrollment**: `POST /courses/:id/enroll` 201, `/dashboard/courses` 200, `GET /me/courses` 200 — unaffected.
- **Notifications**: created a real `question_answered` notification end-to-end (asked a question, had it answered, waited for the worker) and loaded `/dashboard/notifications` with it present in the list — page renders correctly, no crash from the type having no deep-link (`{link && <Link>}` correctly omits the "Открыть" button, exactly as designed), "Отметить всё прочитанным" still present, `GET /me/notifications/unread-count` still 200.
- **Instructor dashboard**: fresh instructor account, all six routes (`/instructor`, `/courses`, `/submissions`, `/students`, `/analytics`, `/questions`) → **200**. Sidebar contains all six expected nav labels including the new "Q&A" item (verified via its HTML-escaped `Q&amp;A` form and its `href`, confirming it's a real rendered link, not a false negative from the raw-text grep).
- **Admin dashboard**: all fourteen routes (`/admin`, `/courses`, `/users`, `/reviews`, `/questions`, `/certificates`, `/plans`, `/subscriptions`, `/payments`, `/analytics`, `/categories`, `/specialities`, `/tests`, `/notifications`, `/course-submissions`) → **200**. Full sidebar nav label check passed (14 pre-existing labels + the new "Q&A"). Admin dashboard's stat tiles still render (26 `.stat-tile` elements found), confirming the layout change (adding one nav item) didn't disturb the dashboard page itself.

### Bugs / regressions found
**None.** Every check passed on the first attempt; no code was modified this session.

### Cleanup
The one test question created for the notifications regression check was deleted immediately after (204), cascading its answer with it — confirmed zero residual `S20C2`-tagged rows afterward. No course ownership was mutated this session (only a normal enrollment, which is real, expected usage, not shared state needing reversion), so there was nothing else to revert.

## Stage 20 — final report

**What shipped**: a complete lesson-level Q&A feature — students ask questions on any lesson they're enrolled in; enrolled peers, the course's own instructor, or an admin can answer; askers get a single in-app notification per answer (never for their own); everyone can delete their own posts. Instructors get a moderation view scoped to their own courses, admins get one covering every course, both can answer directly from that view. Built as a new backend domain (`internal/qa`) plus two new frontend surfaces (the lesson page's Q&A section, and the two moderation pages) — five sessions total (20A/20B1/20B2/20C1/20C2), each independently verified live against Docker Compose, not just unit-tested in isolation (this codebase has no automated test suite at all — see "Known limitations" below).

**Security posture**: every mutation reads identity from the verified JWT only, never a client-supplied field. Delete is IDOR-safe by construction (404, never confirms existence to a non-owner). Answering is a real three-way authorization check (enrolled / course-owning instructor / admin), independently re-verified server-side regardless of which frontend surface calls it. Cross-instructor access was explicitly tested with two instructors owning *different* real courses, not just "an instructor who owns nothing" — confirmed 403/404 in both directions. Re-verified fresh in Stage 20C1 with brand-new accounts, not just trusted from Stage 20A's original testing.

**Performance posture**: the list-with-answers query is exactly two database round-trips regardless of question/answer count, confirmed both by code inspection and by `EXPLAIN ANALYZE` at two different points in time (Stage 20A and Stage 20C1) against freshly seeded 100-question/200-answer volumes, both times sub-millisecond with no per-row loop.

**Design discipline demonstrated across the sub-stages**: Stage 20B2 discovered mid-session that the backend has no hide/show or cross-user-delete endpoints, and chose to build a real, useful read+answer+own-delete moderation view rather than invent UI controls that would silently 404 — documented as a deliberate scope boundary, not an oversight. Stage 20B3 found that notifications already display correctly and only a small, precisely-scoped two-layer gap (a deep-link) remains, and declined to implement it unprompted, per that session's explicit instructions.

### Known limitations (final)
- **No hide/show moderation** — questions/answers can only ever be deleted by their own author; there is no way for an instructor or admin to hide someone else's content. Would need a new backend endpoint (e.g. `PATCH /instructor/questions/:id`/`PATCH /admin/questions/:id`) plus relaxing `DeleteQuestion`/`DeleteAnswer`'s ownership check for those roles specifically — a real, scoped addition, not attempted in any Stage 20 session.
- **Notification deep-link gap** — `question_answered` notifications display correctly (title, message, recipient, no duplicates) but have no "Открыть" one-click link to the lesson, since the enqueued `Data` payload only carries display-string titles, not `lesson_id`/`course_id`. Precisely scoped in the Stage 20B3 section of this document: a ~4-line backend query extension (no migration, no new endpoint) plus one frontend switch case.
- **Moderation pages are composed, not queried** — both `/instructor/questions` and `/admin/questions` fetch course → lesson → questions in application code (`Promise.all` per level) rather than a single backend aggregate query. Correct and fast at this project's demo-data scale; would need a dedicated endpoint if the number of courses/lessons ever grew large.
- **Naming ambiguity** (not a bug) — `internal/tests` already uses "questions"/"answers" for quiz authoring under `/admin/*`/`/instructor/*`; this feature's `/questions/:id` etc. live on the bare `/api/v1/*` prefix, so there's no runtime collision, just a shared English word worth remembering when reading route lists.
- **Pre-existing, unrelated**: `PUT /admin/users/:id` appears to blank `first_name`/`last_name` on a role-only update body — observed repeatedly while setting up test instructor accounts across every Stage 20 sub-session, never touched, not caused by any Stage 20 code.
- No automated test suite exists anywhere in this codebase (confirmed again this session) — every verification claim above is a live, scripted check against a running Docker Compose stack, the established convention for this entire project.

### Deferred items (all explicitly out of scope for Stage 20, candidates for a future stage)
- Hide/show moderation backend + UI.
- Admin/instructor ability to delete another user's question/answer.
- Notification-bell deep-link for `question_answered` (see above).
- Full-platform regression beyond the flows adjacent to Stage 20's own changes (i.e., payments/subscriptions, video pipeline, certificates, achievements, search/recommendations were not re-tested this stage — they share no code path with Stage 20's changes).

## Stage 20C1 — security + focused E2E verification (this session)

Independent, fresh re-verification of the whole Q&A feature (backend + both frontends) with brand-new test accounts — not a re-read of prior sessions' results. **Zero bugs found; zero code changes made.**

### Baseline
- `gofmt -l .` clean, `go vet ./...` clean, `go build ./...` clean (backend unchanged since Stage 20A).
- `npx tsc --noEmit` clean, `npx eslint .` clean (whole project — same 4 pre-existing unrelated `<img>` warnings, nothing new).
- Docker Compose stack already running current code for both services; no rebuild needed since nothing changed.

### 1. Student flow (fresh accounts: two enrolled students, one non-enrolled, live against Docker Compose)
- Enrolled student asks → **201**; question appears in `GET /lessons/:id/questions` **immediately** (checked in the same request cycle, no delay).
- Non-enrolled student asks → **403 `NOT_ENROLLED`**.
- Enrolled peer answers → **201**.
- **Notification timing finding (not a bug):** checking `GET /me/notifications` in the same instant as the answer returned showed 0 matching notifications; after a 3-second wait it correctly showed exactly 1, with the right Russian title/message and lesson/course names interpolated. This is expected async latency from the `notification_jobs` queue + `notification-worker` architecture (confirmed `notification-worker` container `healthy` throughout) — `Enqueue` inserts a job row synchronously, the worker materializes it into the readable `notifications` table on its own poll cycle. **Noting this explicitly so a future verification pass doesn't mistake worker latency for a missing notification** — check again after a few seconds, not immediately.
- Self-answer (asker answers their own question) → **201**, notification count **unchanged** even after waiting for the worker — confirms the "never notify yourself" guard still holds.
- Own answer delete → **204**. Own question delete → **204**, and the lesson's list correctly no longer contains it (cascade confirmed live, not just assumed from the migration).

### 2. IDOR / authorization matrix (fresh accounts: 2 instructors each owning a *different* real course, for a genuine cross-instructor test — not just "an instructor who owns nothing")
- Cross-user question delete (student C deletes student A's question) → **404** `QUESTION_NOT_FOUND` (never 403 — doesn't confirm the row's existence to a non-owner).
- Cross-user answer delete → **404** `ANSWER_NOT_FOUND`, same pattern.
- **Cross-instructor answer attempt**: an instructor owning course Y tries to answer a question in course X (owned by a different instructor) → **403 `FORBIDDEN`** — confirms authorization is checked per-course via `ownership.CanManageCourse`, not just "any instructor role."
- **Cross-instructor delete attempt**: same non-owning instructor tries to delete another user's question in a course they don't own → **404**, identical to the plain student IDOR case — instructors get no special bypass.
- Owning instructor answers within their own course → **201**, `is_instructor_answer: true`.
- Admin answers → **201**, `is_instructor_answer: true` — admin access confirmed working exactly as currently supported (no special endpoints, same three-way rule).
- Unauthenticated → **401** on all five endpoints, including the list endpoint (`GET /lessons/:id/questions` also requires auth — confirmed live, not just assumed from routing code).

### 3. Moderation pages (live SSR HTML fetched with real cookies, not just curl against the API)
- Instructor owning only course X visited `/instructor/questions`: page contained the course-X test marker, **did not** contain a distinguishing marker seeded on course Y (a different instructor's course) — confirmed instructor-scoping is real, not accidentally showing every course.
- Admin visited `/admin/questions`: page contained **both** markers — confirmed admin sees Q&A platform-wide.
- Grepped both rendered pages for any hide/show/скрыть/показать control text — **none found**, confirming Stage 20B2's decision not to invent a frontend-only moderation toggle still holds in the actual rendered output, not just in the source.
- Spot-checked the "Опубликован" published-status badge is present (the real field, still correctly always-true today) and that the "Удалить" button count on the instructor's page was exactly 1 — matching only the one answer that instructor account had itself posted during this session's testing, never appearing next to student content.

### 4. No-N+1 verification (fresh `EXPLAIN ANALYZE`, not reused from a prior session)
Seeded 100 questions + 200 answers on a real lesson (tagged `S20C1_PERF_SEED`, cleaned up precisely afterward by that exact marker):

| Query | Execution time | Notes |
|---|---|---|
| Paginated questions for a lesson | 0.588 ms | Planner chose a seq scan over the small `lesson_questions` table at this row count — same "seq scan on tiny tables is optimal" conclusion as every prior stage, not a missing index |
| Answers for that page's question ids (`= ANY($1)`) | 0.348 ms | — |

Confirmed **exactly two `pool.Query` calls total** in `ListForLesson` by re-reading the current source fresh (unchanged since Stage 20A) — no loop, no per-row correlated subquery. The `loops=102`/`loops=40` figures in the `EXPLAIN` output are PostgreSQL's own internal nested-loop join iteration *within a single query execution*, not separate round-trips from the application — genuine N+1 would show up as repeated `pool.Query` calls in Go, and there are none.

### Bugs found
**None.** Every check in items 1–4 passed on the first attempt with no code changes required.

### Cleanup
All synthetic seed data (`S20C1_PERF_SEED*`) and functional test content (test questions/answers created while exercising the flows above) deleted precisely by their tagged bodies; both courses' temporarily-assigned `instructor_id` reverted to `NULL`. Verified via direct DB query afterward: zero residual `S20C1`-tagged rows, both courses back to their original ownerless state. Test *accounts* (students/instructors/no special admin) were left in place, consistent with every prior Stage 20 session's convention — only shared/mutated state (course ownership, Q&A content) gets reverted, not the harmless standalone accounts themselves.

### Regression (spot-check only, not full-platform per instructions)
`GET /courses/:id`, `GET /lessons/:id/progress`, `GET /me/notifications`, `GET /admin/ping` all re-checked live post-cleanup — all still 200.

## Stage 20B3 — notifications decision check

### What was inspected
- `app/dashboard/notifications/page.tsx` — the only place `AppNotification` rows are ever rendered (the navbar bell, per `components/NavBar.tsx`, is a plain `<Link href="/dashboard/notifications">` with an unread-count badge — no dropdown, no separate rendering path to check).
- `notificationActionLink()` in `lib/api.ts` — the allow-listed `type → URL` switch that decides whether a notification gets an "Открыть" button.
- `internal/qa/repository.go`'s `CreateAnswer` — the exact `notifications.Enqueue(...)` call from Stage 20A, re-read fresh rather than trusted from memory.

### Finding
**Title and message already display correctly for `question_answered` notifications, with no frontend change needed** — `NotificationsPage` renders `n.title`/`n.message` generically for every notification type (no per-type branching for that part), and Stage 20A already confirmed the in-app copy renders correctly in Russian with the right lesson/course title interpolated, to the right recipient, exactly once per answer. `markNotificationReadAction`/`markAllNotificationsReadAction` are also fully generic (id-based), so read/unread state already works correctly for this type too. **On the literal question "does the notification appear to users" — yes, it already does, correctly.**

What's missing is narrower than that: the "Открыть" (Open) deep-link button. `notificationActionLink()` has no `case "question_answered"`, so it falls to `default: return null`, and `NotificationsPage` correctly hides the button when the link is `null` (`{link && <Link>...}`) — so today a student sees "Ответ на ваш вопрос" / "На ваш вопрос по уроку «Введение в Go» (курс «Go Backend Developer») ответили." but has no one-click way to jump to that lesson; they'd navigate there manually via "Мои курсы".

**This gap is not purely frontend.** The `Data` payload `CreateAnswer` currently enqueues only carries `lesson_title`/`course_title` (human-readable strings for the copy) — **not** `lesson_id`/`course_id`. Without an id, `notificationActionLink` has nothing to build a `/learn/{courseId}/{lessonId}` URL from; adding a switch case alone would have nowhere to point. The fix is small on both sides but spans both:
- **Backend** (`internal/qa/repository.go`, `CreateAnswer`): the query right above the `Enqueue` call already does `SELECT lq.user_id, l.title, c.title FROM lesson_questions lq JOIN lessons l ... JOIN courses c ... WHERE lq.id = $1` inside the same transaction — extending it to also `SELECT lq.lesson_id, lq.course_id` (both already columns on that same row) and adding two `Scan` targets, then two more entries in the existing `Data` map, is the entire change. No new migration, no new endpoint, no new authorization surface — the same query, two more columns.
- **Frontend** (`lib/api.ts`, `notificationActionLink`): one new `case "question_answered": return (typeof data.course_id === "string" && typeof data.lesson_id === "string") ? \`/learn/${data.course_id}/${data.lesson_id}\` : null;`, following the exact same pattern already used for `certificate_issued`/`course_announcement`.

### Decision
Per this session's explicit instructions, **not implemented**. Documented here as the precise minimal scope for a future Stage 20B3 if wanted: one small backend query extension (no migration, no new endpoint) + one switch-case addition in `lib/api.ts`. Everything else about Q&A notifications — delivery, correct recipient, correct copy, no duplicates, read/unread handling — was already confirmed working in Stage 20A/20B1 and re-confirmed by this session's fresh read of the current code.

### Files touched this session
None — investigation only, as instructed.

## Stage 20B2 — instructor/admin Q&A moderation UI

### Backend investigation (before writing any frontend code)
Re-read `internal/qa/{handler,service}.go` fresh to confirm exactly what Stage 20A actually authorizes, rather than assuming from memory:
- **No instructor/admin-specific Q&A endpoints exist.** All five Stage 20A routes are plain `/api/v1/*`, auth-only. There is no hide/show endpoint, no admin-wide or instructor-scoped "list Q&A" endpoint, and `DeleteQuestion`/`DeleteAnswer` are strictly `WHERE id = $1 AND user_id = $2` — **not** relaxed for admin or the owning instructor. An instructor/admin can only ever delete their *own* prior question/answer through the existing endpoint, never a student's.
- `GET /lessons/:id/questions` (`ListForLesson`) has **no eligibility gate beyond auth** — no enrollment check, no ownership check. Any authenticated user, including an instructor/admin, can already call it for any lesson id and get that lesson's published questions+answers. This is the one fact that made a zero-backend-change moderation view possible.
- `POST /questions/:id/answers` already authorizes course-owning instructor and admin (Stage 20A's three-way rule) — this is a real, working "moderation action" today, just reached from a new page instead of the lesson page.

**Conclusion: no backend change was made this session.** The task's own instruction ("do NOT change backend unless a tiny API gap blocks the UI") and ("add simple moderation actions only if backend already supports them" / "do not invent frontend-only moderation state") were read as decisive here — hide/show and cross-user delete are real, substantial backend gaps (not "tiny"), so rather than build UI controls that would silently 404, this session builds a genuinely useful moderation *view* plus the two actions that already work, and documents the rest as deferred.

### Completed
1. **Instructor moderation view** — `app/instructor/questions/page.tsx` (new). Composes existing endpoints in application code: `instructorListCourses` → `instructorGetCourse` per course (for its modules/lessons) → `getLessonQuestions` per lesson (the same fetcher Stage 20B1 already built and verified) — only lessons with at least one question are kept. No new backend endpoint, no new frontend fetcher; every function reused as-is.
2. **Admin moderation view** — `app/admin/questions/page.tsx` (new), identical composition using `adminListCourses`/`adminGetCourse` instead, scoped to every course platform-wide rather than one instructor's own.
3. **Permitted actions, nothing invented**:
   - *Answer* — every question shows a "Ответить" toggle; submitting reuses `answerQuestionAction` from Stage 20B1 verbatim. The backend independently re-derives whether this caller may actually answer (course-owning instructor or admin) — the frontend doesn't need its own copy of that rule.
   - *Delete* — a "Удалить" button renders **only when `item.user_id === currentUserId`**, i.e. only ever on a question/answer this specific instructor/admin previously posted themselves through this same feature — never on a student's content, since that would 404 against the real backend rule. This mirrors `QASection`'s exact same ownership check from Stage 20B1.
   - **No hide/show button exists anywhere in this UI** — deliberately, since no backend action backs it.
4. **Context shown per item**, per the task's explicit list: course title + lesson title as a group header above each lesson's questions, asker/answerer display name + timestamp, the question body, all its answers (nested, same visual treatment as the student-facing view, including the existing `is_instructor_answer` → "Преподаватель" badge), and a **published/hidden status badge** on every question and answer (`"Опубликован"` / `"Скрыт"`) driven directly by the real `published` field the API already returns — always `true` today since nothing can unpublish anything yet, but rendered generically so it would immediately reflect a real hide feature if one is added later, without any further frontend change.
5. **States**: loading (`app/instructor/questions/loading.tsx` and `app/admin/questions/loading.tsx`, same Next.js route-segment convention as Stage 19B's `admin/analytics/loading.tsx`), empty (`.empty-state`, "Пока нет вопросов по вашим курсам." / by extension the admin copy), error (a page-level `role="alert"` if the course/lesson aggregation itself fails, plus the same per-item submit/delete error handling as `QASection`), submitting (shared `useTransition` + `pendingKey`, identical pattern to Stage 20B1).
6. **Design**: reused every existing CSS class as-is (`.qa-question-list/-item/-header/-body`, `.qa-answer-list/-item/-header/-body`, `.badge-instructor`, `.admin-header`, `.subtitle`, `.empty-state`) — only two small additions, `.qa-moderation-group`/`.qa-moderation-group-header`, purely for the course/lesson grouping header, built from the same existing tokens. One new hand-rolled icon, `IconMessageCircle` (matches the existing stroke-SVG icon set exactly, no library), used for the new "Q&A" nav item added to both the instructor sidebar (after "My Courses") and the admin sidebar (in the "Сообщество" group, after "Reviews").

### Files changed
- `frontend/components/QAModerationSection.tsx` — new client component (answer/delete only, no ask form, grouped by course/lesson).
- `frontend/app/instructor/questions/page.tsx`, `frontend/app/instructor/questions/loading.tsx` — new.
- `frontend/app/admin/questions/page.tsx`, `frontend/app/admin/questions/loading.tsx` — new.
- `frontend/app/instructor/layout.tsx`, `frontend/app/admin/layout.tsx` — added the "Q&A" nav item each.
- `frontend/components/shell/icons.tsx` — `IconMessageCircle`.
- `frontend/app/globals.css` — `.qa-moderation-group`, `.qa-moderation-group-header`.
- No backend files touched (confirmed via `git status` — every changed/new file is under `frontend/`).
- No changes to `lib/actions.ts`/`lib/api.ts`/`lib/admin-api.ts`/`lib/instructor-api.ts` — every fetcher and action this page needs already existed from Stage 20B1 (`getLessonQuestions`, `answerQuestionAction`, `deleteQuestionAction`, `deleteAnswerAction`) or from before Stage 20 entirely (`instructorListCourses`, `instructorGetCourse`, `adminListCourses`, `adminGetCourse`).

### Verification performed
- `npx tsc --noEmit` (whole project) — clean.
- `npx eslint .` (whole project) — zero errors; the same 4 pre-existing `<img>` warnings from unrelated files as every prior stage, nothing new.
- `npm run build` — clean, `/instructor/questions` and `/admin/questions` both present among the generated routes.
- Docker Compose frontend rebuilt and restarted; live checks against `http://localhost:3001` (backend untouched, no rebuild needed there):
  - Unauthenticated → **307** for both pages (existing layout gate, untouched). Authenticated student → **307** for both (same gate — role check happens before either page's own code runs).
  - Instructor view: temporarily assigned a fresh instructor as the owner of the seeded "Go Backend Developer" course (reverted to `NULL` afterward, same careful precise-revert discipline as Stage 20A) — page correctly showed the course/lesson header and the one existing question with its two existing answers, all with "Опубликован" badges.
  - **Live answer flow**: opened the "Ответить" form on the moderation page, typed and submitted a real answer as the instructor → appeared instantly with the "ПРЕПОДАВАТЕЛЬ" badge (the backend correctly recognized the ownership match) and its own "Удалить" button (this instructor's own new content) — the pre-existing student content directly above it correctly showed **no** delete button throughout.
  - **Live delete flow**: clicked "Удалить" on that same instructor-authored answer → native `confirm()` fired with the expected message, accepted → answer removed instantly, sibling content unaffected.
  - Admin view: same course/question/answers visible (admin sees every course, no ownership needed) — confirmed via screenshot.
  - **Empty state**: a freshly created instructor account with zero courses → "Пока нет вопросов по вашим курсам." — screenshot-verified.
  - Final DB check after all testing: `lesson_questions`/`question_answers` row counts and content back to exactly the pre-session state (1 question, 2 answers) — the moderation testing left no residue.

### Known issues / observations
- **Deferred, not a bug**: hide/show moderation and admin/instructor delete-of-others'-content both require real backend endpoints that don't exist — this was the central finding of this session (see "Backend investigation" above) and is the natural scope for a future Stage 20C if wanted.
- **Composition, not a dedicated query**: both moderation pages fetch course → lesson → questions in application code (`Promise.all` per level), not a single backend query. Fine at this project's demo-data scale (a handful of courses/lessons across every prior stage); would need a real aggregate endpoint if the number of courses/lessons ever grew large. Explicitly not claimed to scale beyond that here.
- Re-observed the same pre-existing `internal/users` behavior noted in Stage 20A (`PUT /admin/users/:id` blanking `first_name`/`last_name` on a role-only update) while setting up test instructor accounts — the instructor's own answer in one screenshot shows an empty display name next to the "ПРЕПОДАВАТЕЛЬ" badge as a direct, expected consequence, not a new bug.
- No automated frontend test suite exists in this codebase (established convention) — verification is build/lint/typecheck + live scripted browser interaction + visual screenshots, consistent with every prior stage.

### Remaining (explicitly out of scope, not attempted this session)
- Hide/show moderation (backend endpoint + UI) — deferred, see above.
- Admin/instructor ability to delete a student's question/answer — deferred, same reason.
- Notification-bell deep-link for `question_answered` — investigated and precisely scoped in Stage 20B3 (see the top of this document); not yet implemented.
- Full-platform regression beyond the pages and flows exercised above.

## Stage 20B1 — student lesson Q&A frontend

### Completed
- Q&A section added to the existing lesson page (`app/learn/[courseId]/[lessonId]/page.tsx`), placed after the coding-exercise section in the same left column, following the exact same "fetch server-side, pass as props to a purpose-built section component" pattern the page already uses for `AssignmentSection`/`CodingExerciseSection`.
- The initial page of questions is fetched server-side via a new `getLessonQuestions` in `lib/api.ts` (mirrors `getMyNotifications`'s `PageResult<T>` fetcher shape exactly) and passed into a new client component, `components/QASection.tsx`.
- **Ask a question**: enrolled student types into a textarea and submits — no separate eligibility check needed client-side, since this entire page is only reachable once `getMyCourseDetail` confirms enrollment (the page already shows "Вы не записаны на этот курс" and stops otherwise), so every viewer of this section is already enrolled by construction.
- **Answer a question**: every question has a "Ответить" toggle revealing its own answer form; submission works identically for the enrolled student viewing this page (the only role this page renders for) — the backend independently re-enforces the full three-way rule (enrolled/owning-instructor/admin) regardless of what the frontend assumes.
- **Delete own question/answer**: a "Удалить" button renders only when `item.user_id === currentUserId` (compared client-side against the session user, never trusted from a hidden field); clicking asks for confirmation via a plain `confirm()` dialog (matches the `ConfirmButton` pattern's UX, implemented inline here since the delete isn't a `<form>` submission), then calls the delete action and removes the item from local state on success.
- **State handling**:
  - *Submitting*: a single shared `useTransition` + a `pendingKey` string identifies exactly which control is mid-request, so only that button shows "Отправка.../Удаление.../Загрузка...", while every mutating control is disabled for the duration (prevents overlapping double-submits).
  - *Empty*: zero questions renders the existing `.empty-state` block ("Пока нет вопросов к этому уроку. Будьте первым!").
  - *Error*: ask/answer/delete/load-more each have their own `role="alert"` message scoped to exactly the form or item that failed, never a page-wide error.
  - *Loading*: a "Показать ещё вопросы" button (shown only when `page < total_pages`) triggers a new server action (`loadMoreQuestionsAction`) and appends the next page to local state — this is the one operation that's genuinely asynchronous beyond a mutation, giving "loading" its own distinct visible state from "submitting".
- No page reload / redirect for any Q&A interaction — every action (ask/answer/delete/load-more) returns a plain result object (mirrors the existing `WishlistButton`/`addToWishlistAction` pattern exactly, not the `redirect()`-based review actions), and the component updates its own local array state directly from each response instead of refetching the list — verified live that the video/progress state elsewhere on the page is undisturbed by any Q&A interaction.
- Design: new `.qa-*` CSS classes added to `globals.css`, built entirely from existing design tokens (`var(--surface)`, `var(--border)`, `var(--radius-md)`, etc. — no new colors/tokens introduced) and closely mirroring the existing `.review-list`/`.review-item` shape; one new `.badge-instructor` variant (same pattern as `.badge-free`/`.badge-premium`) marks `is_instructor_answer` answers with an accent-colored "Преподаватель" badge.

### Files changed
- `frontend/components/QASection.tsx` — new client component.
- `frontend/lib/api.ts` — `QAQuestion`, `QAAnswer`, `QAQuestionView`, `QAAnswerView` types (mirror `backend/internal/qa/model.go` exactly) + `getLessonQuestions`.
- `frontend/lib/actions.ts` — `askQuestionAction`, `answerQuestionAction`, `deleteQuestionAction`, `deleteAnswerAction`, `loadMoreQuestionsAction`.
- `frontend/app/globals.css` — `.qa-section`, `.qa-form`, `.qa-question-list/-item/-header/-body`, `.qa-answer-list/-item/-header/-body`, `.badge-instructor`.
- `frontend/app/learn/[courseId]/[lessonId]/page.tsx` — fetches the initial question page + current user, renders `<QASection>`.
- No backend files touched (`git status` confirms only the 5 frontend files above).

### Verification performed
- `npx tsc --noEmit` — clean.
- `npx eslint app/learn/[courseId]/[lessonId]/page.tsx components/QASection.tsx lib/api.ts lib/actions.ts` — clean, zero warnings.
- `npm run build` (production) — compiled successfully, `/learn/[courseId]/[lessonId]` among all generated routes.
- Docker Compose frontend rebuilt and restarted; live checks against `http://localhost:3001` with a fresh enrolled student (headless Chrome via CDP, cookie-injected session, real keyboard/click simulation via `Input.insertText`/`element.click()`, not just static screenshots):
  - Existing question + nested answer render correctly on first load (SSR), including the pre-existing data from Stage 20A's own testing.
  - Typed and submitted a new question → appeared at the top of the list **instantly, no page reload**, with its own "Удалить" button (this student owns it) and no `is_instructor_answer` badge.
  - Typed and submitted an answer to another student's question → appeared nested under it instantly, with its own "Удалить" button; the peer's own answer correctly showed no delete button.
  - Clicked "Удалить" on the student's own question → the browser's native `confirm()` dialog fired with the expected message ("Удалить этот вопрос?"), accepted programmatically → question removed from the list instantly, no reload; the still-present sibling question and its answers were unaffected.
  - Empty-state screenshot-verified on a lesson with zero questions.
  - Client-side validation error screenshot-verified: submitting a blank question shows "Введите текст вопроса." inline, with no network request made (backend never even called for an empty body).
- All screenshots visually match the existing dark theme exactly — same surfaces, borders, radii, button styles as every other section on the page (video, progress, assignment).

### Known issues / observations
- "Load more" pagination (`loadMoreQuestionsAction`) was implemented and code-reviewed but not separately live-tested this session, since the current seeded lesson never had enough questions to exceed one page (20) — the underlying fetch is the same `GET /lessons/:id/questions?page=&limit=` already verified working in Stage 20A, so this is considered low-risk, but flagged as not independently exercised.
- Only one question's answer form can be open at a time (`openAnswerFormId` is a single value, not a set) — opening a second question's "Ответить" form closes whichever was already open. Deliberate simplicity for this session's scope, not a bug; answer drafts (`answerDrafts`) are still tracked per-question so switching back and forth doesn't lose unsent text.
- No automated frontend test suite exists in this codebase for any page (established convention) — verification is build/lint/typecheck + live scripted browser interaction + visual screenshots, consistent with every prior stage.

### Remaining (explicitly out of scope, not attempted this session)
- ~~Instructor moderation endpoints/UI~~ / ~~Admin moderation endpoints/UI~~ — **views done in Stage 20B2** (view + answer + delete-own; hide/show still deferred, see the top of this document).
- Notification-bell deep-link ('Открыть' button) for the `question_answered` type — the notification itself already displays correctly (title/message, correct recipient, correct copy, all confirmed again in Stage 20B3); only the one-click link to the lesson is missing, and needs a small two-layer fix (backend Data payload + one frontend switch case) precisely scoped in Stage 20B3 at the top of this document.
- Full-platform regression beyond the lesson page and its already-existing sections.

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
- Refunded-style "hide" moderation doesn't exist yet for questions/answers — only owner-delete, confirmed still true after Stage 20B2's fresh re-read of this exact code.

## Remaining (explicitly out of scope, not attempted this session)

- ~~Instructor moderation endpoints~~ / ~~Admin moderation endpoints~~ (`/instructor/questions`, `/admin/questions`) — **views done in Stage 20B2**; hide/show still has no backend endpoint (see Stage 20B2's section at the top of this document).
- ~~Lesson-page frontend (question list, ask form, answer form)~~ — **done in Stage 20B1** (see the top of this document).
- Notification-bell deep-link for `question_answered` — precisely scoped in Stage 20B3 (top of this document): needs a small backend Data-payload addition (lesson_id/course_id) plus one frontend switch case, not yet implemented.
- Full-platform regression beyond the lesson/course/admin/notification spot-checks above.
