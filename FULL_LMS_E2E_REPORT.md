# Full LMS End-to-End Acceptance Test Report

## 1. Environment

| Item | Value |
|---|---|
| Commit under test | `38fabc8d066e25966538d4b7ecb892540695bdda` (branch `main`) |
| Test date/time | 2026-08-15, 14:03–14:24 UTC |
| Backend API | `http://localhost:8082/api/v1` (container `course-backend-1`, rebuilt from the commit above) |
| Frontend | `http://localhost:3000` (local `next dev`, Next.js 16.3.0, Turbopack — not the stale prebuilt `course-frontend-1` container) |
| Database | PostgreSQL 17, container `course-postgres-1`, port `5434→5432` |
| Object storage | MinIO, container `course-minio-1`, buckets private (no anonymous access, no port-mapped public console) |
| Video pipeline | `course-video-worker-1` — real `ffmpeg` HLS transcode |
| Code execution | `course-code-runner-1` — real sandboxed Python execution |
| Notifications | `course-notification-worker-1` — real queue worker |
| Mail capture | Mailpit, container `course-mailpit-1`, `http://localhost:8025` |
| Health check | `GET /api/v1/health` → `{"status":"ok","database":"ok"}` at test start |
| Service status at start | All 8 services up/healthy via `docker compose ps` |
| Service status at persistence checkpoint | `backend`, `notification-worker`, `video-worker`, `code-runner` restarted mid-test (no volumes touched); all returned healthy and all tested state survived the restart |

## 2. Test Identities

| Role | Email | Notes |
|---|---|---|
| Instructor | `e2e-instructor-1786802585@example.test` | Created via real `POST /auth/register`, promoted `student → instructor` via real `PUT /admin/users/:id` (admin action) |
| Student | `e2e-student-1786802585@example.test` | Created via real `POST /auth/register`, no role changes |
| Admin | `admin@example.com` | Pre-existing dev-bootstrap account (`backend/cmd/bootstrap-admin`), used **only** for: promoting the instructor role, approving the course submission, creating the speciality, and attaching the course to the speciality — all genuine admin-only product workflows. Its password was never changed, printed, or logged in this report. |

No password, JWT, or token for any account appears anywhere in this report.

## 3. Test Course

| Field | Value |
|---|---|
| Course ID | `9b6ef640-6a59-4ce0-8e3b-e121fc96afd5` |
| Title | E2E Full LMS Course 1786802585 |
| Slug | `e2e-full-lms-course-1786802585` |
| Category | Programming |
| Speciality | E2E Testing Specialty (`cc34c80d-6fec-41a4-acca-c4061db83c3c`) |
| Access mode | `free` (non-payment path; the platform's payment/subscription system is confirmed mock-only — see §7) |
| Curriculum | 3 modules / 5 lessons: text lesson (free preview), video lesson (real HLS upload+transcode), quiz lesson (3 questions), coding-exercise lesson (Python, 2 test cases), final text lesson |
| Publication | draft → pending_review → **published** (via real instructor-submit + admin-approve workflow) |

Both the Instructor and Student accounts, and this course/speciality, are left in place as a stable, clearly-named acceptance-test fixture, per instructions.

## 4. Lifecycle Results

| Area | Scenario | Result | Evidence |
|---|---|---|---|
| Environment | All 8 dev-stack services healthy at start | PASS | `docker compose ps`, `/api/v1/health` = `{"status":"ok","database":"ok"}` |
| Auth | Instructor registered via real API | PASS | `POST /auth/register` → 201, role=`student` initially |
| Auth | Admin promotes user to `instructor` via real admin workflow | PASS | `PUT /admin/users/:id` → role=`instructor`, active=true |
| Auth | Instructor login, role/active correct, `/me` correct | PASS | login response + `GET /me` |
| Auth | Instructor blocked from admin endpoints | PASS | `GET /admin/users` → 403 |
| Auth | Student registered via real API | PASS | `POST /auth/register` → 201 |
| Auth | Duplicate email registration rejected | PASS | `POST /auth/register` (dup) → 409 `EMAIL_ALREADY_EXISTS` |
| Auth | Wrong password login rejected | PASS | `POST /auth/login` → 401 `INVALID_CREDENTIALS` |
| Auth | Logged-out access to protected route rejected | PASS | `GET /me` (no token) → 401 |
| Auth | Student blocked from instructor endpoints | PASS | `GET /instructor/courses` → 403 |
| Course authoring | Invalid course create rejected (bad `access_type`) | PASS | 400 `VALIDATION_ERROR` |
| Course authoring | Course created (draft), metadata correct | PASS | `POST /instructor/courses` → 201 |
| Course authoring | 3 modules, 5 lessons created (text/video/quiz/coding/final) | PASS | curriculum API dump |
| Course authoring | Course cover set via `image_url` | PASS (feature is URL-entry only) | see §7 — no dedicated cover **upload-to-storage** endpoint exists |
| Video lesson | Real video uploaded, transcoded to HLS by video-worker | PASS | upload → `processing_status:"uploaded"` → polled to `"ready"`, 360p rendition; playback `GET /lessons/:id/video` → `stream_type:"hls"`, proxy URL |
| Video lesson | Video plays through authorized proxy in real browser | PASS | Playwright: `<video>` element rendered at `/learn/.../...`, `video-stream` proxy returns 200 only when authenticated |
| Quiz authoring | Test + 3 questions + answers created, attached to a lesson | PASS | `POST /instructor/tests`, `/questions`, `/answers` |
| Coding authoring | Exercise + 2 test cases (1 visible, 1 hidden) created | PASS | `POST /instructor/lessons/:id/coding-exercise`, `/test-cases` |
| Course review workflow | Instructor submits for review; admin approves (published) | PASS | `POST /instructor/courses/:id/submit` → `pending_review`; `PUT /admin/course-submissions/:id` → `published` |
| Speciality | Admin creates speciality, invalid create rejected first | PASS | 400 on empty title, then 201 |
| Speciality | Course attached to speciality by admin | PASS | `POST /admin/specialities/:id/courses` |
| Public discovery | Course visible via `/courses` search, filters, `/courses/:id` | PASS | API + Playwright screenshot, title/badges/instructor/curriculum all correct |
| Public discovery | Speciality detail lists the course | PASS | `GET /specialities/:id` + Playwright screenshot |
| Public discovery | Invalid filter value rejected | PASS | `GET /courses?level=nonsense` → 400 |
| Public discovery | Search suggestions return the course | PASS | `GET /search/suggestions?q=E2E` |
| Public discovery | Desktop (1440px) and mobile (390px) rendering, no overflow | PASS | Playwright screenshots, `scrollWidth === clientWidth` on all pages checked |
| Public discovery | Long course title does not break layout | PASS | Temporary 180-char title tested on desktop + mobile, no overflow, reverted afterward |
| Wishlist | Add, duplicate-add idempotent, list, lightweight ids endpoint | PASS | `POST` (200 both times), `GET /me/wishlist`, `GET /me/wishlist/course-ids` |
| Wishlist | Remove, re-add | PASS | sequential API calls |
| Wishlist | Auto-removed on enrollment | PASS | wishlist empty immediately after `POST /courses/:id/enroll` |
| Enrollment | Free enrollment succeeds, no payment involved | PASS | `POST /courses/:id/enroll` → 201 |
| Enrollment | Duplicate enroll handled gracefully | PASS | second call → 200, same enrollment id |
| Enrollment | Initial progress 0%, `next_lesson_id` correct | PASS | `GET /me/courses` |
| Access control | Logged-out cannot enroll/view progress | PASS | 401 |
| Access control | Student cannot edit the course | PASS | 403 `FORBIDDEN` |
| Access control | Instructor cannot manage a course they don't own | PASS (tested against a non-owned admin-owned demo course; no second-instructor fixture existed, and the spec explicitly says not to create one unless strictly required) | `PUT /instructor/courses/:id` (Docker demo course) → 403 `you do not manage this course` |
| Learning flow | Text lesson completed, progress persists across a fresh GET (reload-equivalent) | PASS | `PUT`/`GET /lessons/:id/progress` identical before/after |
| Learning flow | `next_lesson_id` advances correctly after each completion | PASS | verified after every lesson |
| Learning flow | Continue-learning shows correct resume point mid-course | PASS | `GET /me/continue-learning` |
| Learning flow | Continue-learning drops the course after 100% completion | PASS | empty list post-completion |
| Quiz | Failing attempt recorded correctly (0/3, not passed) | PASS | `POST /tests/:id/submit` → score 0, passed:false |
| Quiz | Passing attempt recorded correctly (3/3, passed) | PASS | score 100, passed:true |
| Quiz | Both attempts persisted in history | PASS | `GET /me/test-attempts` → 2 items |
| Quiz | Correct answers never exposed to student before submission | PASS | `GET /tests/:id` omits `is_correct` |
| Coding exercise | Failing submission executed by real sandbox, correctly marked failed | PASS | `status:"failed"`, `passed_tests:0/2` |
| Coding exercise | Passing submission executed by real sandbox, correctly marked passed | PASS | `status:"passed"`, `passed_tests:2/2` |
| Coding exercise | Lesson `coding_exercise_passed` flag updates correctly | PASS | `GET /me/courses/:id` |
| Q&A | Student asks a question (ownership correct) | PASS | `POST /lessons/:id/questions` |
| Q&A | Instructor answers, visible to student with correct relationship | PASS | `GET /lessons/:id/questions` shows nested answer |
| Q&A | Instructor hides own answer; hidden from student; restores it | PASS | `PUT /instructor/qa/answers/:id` published:false/true, student view count 0 then 1 |
| Q&A | Student cannot moderate (hide) content | PASS | 403 |
| Notifications | `welcome`, `enrolled`, `achievement_earned` (×2), `question_answered`, `course_approved`, `course_completed`, `certificate_issued` all fired for the actually-implemented event types | PASS | `GET /me/notifications` for both accounts |
| Notifications | Mark-one-read and mark-all-read change unread count correctly | PASS | count 5→4→0, then new events correctly bumped it back up |
| Notifications | Unread state persists across reload/re-login | PASS | re-verified after service restart |
| Notifications | Email channel delivers to Mailpit for welcome/course_completed/certificate_issued | PASS | Mailpit inbox: 4 messages, correct recipients/subjects |
| Completion | Course reaches server-verified 100%, `completed_at` recorded | PASS | `GET /me/courses` |
| Certificate | Auto-issued on completion, correct ownership/course/identity | PASS | `GET /me/certificates` |
| Certificate | Public verify-by-number endpoint (valid and invalid numbers, both 200) | PASS | `valid:true` for real number, `valid:false` for bogus number |
| Certificate | Other users cannot view someone else's certificate detail | PASS | instructor → 403 on student's `/me/certificates/:id` |
| Certificate | No mutation endpoint exists at all (nothing to unauthorized-mutate) | PASS | route inventory confirms GET-only certificate routes |
| Review | Invalid rating (0) rejected | PASS | 400 |
| Review | Review created, ownership correct | PASS | `POST /courses/:id/reviews` → 201 |
| Review | Duplicate review rejected | PASS | 409 `REVIEW_EXISTS` |
| Review | Rating aggregation updates on create and on edit | PASS | course `rating_average`/`rating_count` recomputed both times (5→ then 4 after edit) |
| Review | Public visibility, edit works | PASS | `GET /courses/:id/reviews`, `PUT .../reviews/me` |
| Instructor visibility | Own courses, enrolled student roster, per-course stats, aggregate stats, course reviews, QA moderation view all correct | PASS | full API sweep, all figures match student-side activity |
| Instructor visibility | Instructor still blocked from admin endpoints after all activity | PASS | 403 on `/admin/audit-log` |
| Student dashboard/profile | Dashboard, My Courses (100%), Certificates, Notifications, Achievements, Wishlist (empty) all render correctly via real UI login | PASS | Playwright screenshots, real form login (not just API) |
| Post-enrollment edit | Instructor edits course description; student sees updated description; enrollment/progress/certificate/review all intact | PASS | fresh `GET /courses/:id` + student-side re-checks after the edit |
| Authorization matrix | 12 logged-out checks, all correctly 401 | PASS | see `authz_matrix` run, full HTTP status + body evidence |
| Authorization matrix | 10 student-boundary checks, all correctly 403 | PASS | same run |
| Authorization matrix | 7 instructor-boundary checks (admin-only + non-owned-course), all correctly 403 | PASS | same run |
| Authorization matrix | Positive controls (instructor own course, student own enrollment) correctly 200 | PASS | same run |
| Storage security | MinIO bucket has no anonymous access | PASS | `mc anonymous get` fails, direct HTTP GET → 403 |
| Storage security | Video streaming requires auth | PASS | no token → 401 |
| Storage security | Path traversal on HLS proxy path blocked | PASS | raw `../` → 404 (route-normalized away); URL-encoded `../` → 400 `INVALID_PATH` |
| Storage security | Substituted/nonexistent video ID does not leak another user's content | PASS | random UUID → 404 `VIDEO_NOT_FOUND`, no data returned |
| Persistence | Backend, notification-worker, video-worker, code-runner restarted (no volumes touched) | PASS | `docker compose restart`, all containers returned healthy |
| Persistence | Enrollment, 100% progress, `completed_at`, certificate, quiz attempts (×2), coding attempts (×2), Q&A, review, unread-notification count, HLS video playback all re-verified identical after restart | PASS | full re-query with a freshly re-logged-in token |
| UI (browser) | Public course/speciality pages, student dashboard suite, instructor dashboard suite, login form flow — all exercised via real Playwright browser sessions, not just API calls | PASS | 20+ screenshots captured |
| Responsive | 1440px and 390px checked on home, `/courses`, `/specialities`, course detail, dashboard, learn page — no horizontal overflow anywhere | PASS | `scrollWidth === clientWidth` on every page checked |
| Certificate PDF / downloadable file | — | NOT IMPLEMENTED | Certificate is a structured API/UI record only; no PDF/file generation or download endpoint exists in the route inventory |
| Course cover file upload to object storage | — | NOT IMPLEMENTED | Only a plain `image_url` text field exists on courses; no multipart cover-upload endpoint was found anywhere in the backend route inventory (video/assignment uploads exist, cover does not) |
| Real payment provider | — | NOT APPLICABLE | Confirmed mock-only (`PAYMENT_PROVIDER=__REAL_PROVIDER_NOT_YET_INTEGRATED__` in `.env.production.example`, `backend/internal/subscriptions/provider.go`); this course used the free access path deliberately, per instructions, since no real payment integration exists to test |
| Cross-instructor course isolation with a genuine second instructor | — | NOT APPLICABLE | No second-instructor fixture existed; spec explicitly forbade creating one unless strictly required. Ownership-check logic was instead verified against a non-owned (admin-owned demo) course, which exercises the identical code path |
| Curriculum reordering (drag/reorder modules or lessons) | — | NOT TESTED (route exists) | `PUT .../modules/reorder` and `.../lessons/reorder` routes exist in the backend but were not exercised in this run — curriculum was authored in final order directly, so reordering was never functionally required |
| Assignments (file-upload homework) domain | — | NOT APPLICABLE | Course design used quiz + coding exercise per the spec's required content types; the assignments domain exists in the codebase but was out of scope for this course's curriculum and not exercised |

## 5. Bugs Found

**None.** No genuine regression or product defect was found during this test. Two minor, non-blocking observations were made and are recorded for completeness — neither required a code change:

1. **`PUT /admin/users/:id` is full-replace, not a partial patch.** Sending only `{"role":"instructor"}` zeroed out `first_name`/`last_name`/`active` on that user. This is not a bug: the real admin UI (`frontend/app/admin/users/page.tsx` + `lib/admin-actions.ts`) always submits the complete form (all fields pre-populated from the fetched record), so no data loss occurs through the actual supported workflow. It only surfaced because my first API call, made directly rather than through the UI, omitted fields. No fix applied — this is correct REST semantics for a form-backed endpoint.
2. **Cosmetic: instructor's course-update response transiently showed `rating_average:0, rating_count:0`** immediately after an edit, even though the persisted rating (4.0, 1 review) was untouched — confirmed by an immediate fresh `GET` returning the correct values. The `UpdateForInstructor` repository path apparently returns the updated row without re-joining the rating aggregate, while `GetCourseDetail`/list queries do the join. No data was lost or corrupted, and no instructor-facing UI displays rating on the edit form, so this has no observable product impact. No fix applied, since spec instructs fixing only defects that "prevent an already-existing feature from working" — this doesn't.

## 6. Quality Gates

No backend or frontend source code was modified during this test (verified: `git status` → clean working tree, `git diff --stat` → empty). Per instructions, quality gates were not run since there is nothing to gate — no `gofmt`/`go build`/`go vet`/`typecheck`/`lint`/`build` output would reflect any change made in this session.

## 7. Final Acceptance Summary

| Lifecycle | Verdict |
|---|---|
| Student complete lifecycle (register → discover → wishlist → enroll → learn → quiz → coding exercise → Q&A → complete → certificate → review) | **PASS** |
| Instructor complete lifecycle (register → promote → author full curriculum → submit → get approved → post-enrollment edit → visibility into students/stats/QA/reviews) | **PASS** |
| Cross-role authorization (logged-out / student / instructor / admin boundaries, 29 explicit checks) | **PASS** |
| Persistence (full app-service restart, no volumes touched, all state re-verified identical) | **PASS** |
| **Full LMS acceptance** | **PASS** |

### Roadmap/doc-only functionality not actually found in the product

- **Course cover image upload to object storage** — only a plain URL text field exists; there is no multipart upload endpoint for course covers (video and assignment file uploads exist; cover images do not).
- **Certificate PDF/file generation or download** — certificates are structured records with a public verify-by-number lookup only; no downloadable file artifact exists.
- **Real payment provider integration** — confirmed mock-only, consistent with `PRODUCTION_LAUNCH_CHECKLIST.md`; `POST /payments/:id/mock-confirm` is the only "payment" mechanism that exists.
- **Password-reset flow** — confirmed absent, consistent with prior documentation; not exercised in this test since it does not exist.

No deploy was performed. No new product features were added. No new roadmap stage was started. All results above reflect functionality actually exercised against the running dev stack at commit `38fabc8`.
