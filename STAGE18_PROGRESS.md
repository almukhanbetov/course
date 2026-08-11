# Stage 18 — Wishlist + Continue Learning + Recommendations

**Status: verification complete. Stage 18 is done.**

Tracking doc for a stage that spanned multiple sessions. Not a spec restatement — status only.

## Done

### Migration
- `backend/migrations/00036_create_course_wishlist.sql` — `course_wishlist` (user_id, course_id, unique constraint, FKs, indexes on both columns). Applied cleanly in a prior session's Docker Compose run.

### `internal/wishlist` (complete: model/repository/service/handler)
- `POST /courses/:id/wishlist`, `DELETE /courses/:id/wishlist`, `GET /me/wishlist`, `GET /me/wishlist/course-ids`.
- `Add` is idempotent (`ON CONFLICT DO NOTHING`, always 200).
- `GET /me/wishlist/course-ids` is the "user-aware enrichment" endpoint (spec item 3) — public `GET /courses` is untouched since this codebase has no optional-auth middleware; the frontend is expected to call this separately to mark `in_wishlist` on course cards.
- **Not yet wired into `cmd/api/main.go`** — `wishlist.NewRepository/NewService/NewHandler` + `RegisterRoutes` calls are not present. The package compiles and is already *used* one-directionally (see next section), but its own HTTP routes are unreachable until wired. *(RESOLVED in the verification session — see "Done (this session — verification)" below.)*

### `internal/learning` (complete)
- `CreateEnrollment` now calls `wishlist.RemoveTx(ctx, tx, userID, courseID)` inside its existing transaction — enrollment auto-clears any wishlist entry for the same course, atomically (spec item 20's preferred rule).
- **Bug found and fixed**: `ListEnrolledCourses`'s `next_lesson_id` subquery previously ordered candidate lessons by the raw `lesson_progress.completed` flag only (video progress), ignoring the assignment/coding completion factors that `completed_lessons` in the same query already correctly accounts for. Now uses the identical three-factor formula (`assignmentApprovedForLesson AND codingExerciseOK`) via an aliasing trick (subquery locally shadows `l`/`m` so the existing hardcoded-alias SQL constants resolve correctly). This was a real pre-existing inconsistency, not new Stage 18 code.
- New `GetContinueLearning` (repository/service/handler) — `GET /me/continue-learning`, wired into `RegisterRoutes` and reachable today. One query (two CTEs: `candidates` for progress/last-activity aggregates, `next_lessons` via `DISTINCT ON` for the next not-completed lesson per course), sorted by `last_activity_at DESC NULLS LAST` then progress fraction — no N+1.

### `internal/recommendations` (complete as of this session)
- `model.go` — `Candidate`, `Recommendation`, reason-code constants (`same_category`, `in_learning_path`, `in_wishlist`, `high_rating`, `popular`, `new_course`, `speciality_overlap`). Present from a prior session, verified intact, gofmt'd.
- `repository.go` — present from a prior session, verified intact; this session added `GetWishlistCourseIDs` (duplicates a lookup against `course_wishlist` rather than importing `internal/wishlist`, per the codebase's established "share schema, not code" cross-domain convention). Methods: `ListCandidatesForUser`, `GetCategoryAffinity`, `GetWishlistCourseIDs`, `GetLearningPathNextCourses`, `GetCourseContext`, `ListSimilarCandidates` — each one query, no per-candidate queries anywhere.
- `service.go` — **new this session**. Deterministic, additive scoring:
  - Baseline (every candidate, personalized or not): rating (capped +10), popularity via `log2(enrollments+1)` (capped +8), freshness linear decay over 90 days (capped +5). This baseline alone *is* the new-user fallback (spec item 16) — no separate branch, a zero-history user just never accumulates the boost terms below, so ranking collapses to top-rated/popular/newest.
  - Boosts: category affinity (touched×3 + completed×4, capped +15), wishlist (+30 flat), learning-path/roadmap-next (+45 flat — deliberately the single largest factor per spec item 15's "должен получить сильный priority").
  - Candidate exclusions: `published = true`, not already enrolled (completed *or* in-progress — in-progress belongs to Continue Learning, not here), and excludes courses the user themselves instructs.
  - Sort: score DESC, then `course_id` ASC — deterministic tie-break (spec item 12).
  - `GetSimilarCourses` (public sibling): same_category (+20), speciality overlap (+15/unit, capped +30), rating (+5 max) — no personalization signals at all; a candidate with zero signal is dropped rather than padded in.
- `handler.go` — **new this session**. `GET /me/recommendations` (authenticated, reads `userID` only from JWT via `authctx`), `GET /courses/:id/similar` (public, no auth middleware applied) — both registered from one `RegisterRoutes(rg, requireAuth)` call that applies `requireAuth` selectively per-route.
- **Wired into `cmd/api/main.go`** this session: import added, `recommendationsRepo/Service/Handler` constructed alongside the Stage 17 activity/achievements block, `recommendationsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())` registered.

### Verification performed this session
- `gofmt -l .` — clean (no files need formatting).
- `go build ./...` — clean, no errors.
- `go vet ./...` — clean, no errors.
- This codebase has **no existing `_test.go` files anywhere** (confirmed via repo-wide search) — all prior stages were verified via live Docker Compose E2E instead of Go unit tests, so there was no existing test suite to run here. Build+vet is the full "focused backend test" for this session per that established convention.
- **Not run this session** (explicitly out of scope per instructions): Docker Compose, EXPLAIN ANALYZE, any HTTP-level request against a running server, frontend, security/regression checks.

## Done (this session — frontend)

**IMPORTANT — known gap (RESOLVED — see "Done (this session — verification)" below)**: `internal/wishlist` is still not wired into `cmd/api/main.go` (see below). The wishlist frontend built this session is code-complete and calls the documented API contract, but `POST/DELETE /courses/:id/wishlist`, `GET /me/wishlist`, and `GET /me/wishlist/course-ids` will 404 until that one-line wiring gap from the previous session is closed. Continue Learning and Recommendations *are* wired end-to-end already (both were reachable before this session) and should work live.

- **`lib/api.ts`**: `WishlistItem`, `ContinueLearningItem`, `Recommendation`/`RecommendationReason` types; `getMyWishlist`, `getMyWishlistCourseIds`, `getContinueLearning`, `getMyRecommendations`, `getSimilarCourses` (public, no token) fetchers; `reasonLabel(reason, categoryName?)` — the one place a reason code becomes a Russian phrase (backend never sends pre-rendered text, matching the existing `activityDisplayText` convention from Stage 17).
- **`lib/actions.ts`**: `addToWishlistAction` / `removeFromWishlistAction` — plain async functions (not `<form action>`-bound) called directly from a client component's `onClick`, matching the Stage 16 coding-exercise Run/Submit convention. Add is idempotent on the backend so these never need to distinguish "already there" from "just added".
- **`components/WishlistButton.tsx`** (new, client) — optimistic toggle with revert-on-error; always `preventDefault`/`stopPropagation` since it sits inside or beside a card that's otherwise one big `<Link>`.
- **`components/CourseCard.tsx`** (restructured) — was a single `<Link className="course-card">`; a `<button>` nested inside an `<a>` is invalid HTML and double-fires navigation, so it's now `<div className="course-card"><WishlistButton/><Link className="course-card-link">...</Link></div>`. New optional props `showWishlist`/`initialInWishlist`, both default `false`/`false` so every other pre-existing caller keeps working unchanged. Confirmed via grep this was the *only* call site (`CourseListing.tsx`) before this session.
- **`components/CourseListing.tsx`**: fetches `getMyWishlistCourseIds` only when a session token exists (item 3's enrichment pattern — the public `getCourses()` call is completely untouched), passes `showWishlist={Boolean(token)}` to every card. Anonymous visitors see the same cards with no toggle at all. Shared by both `/courses` and `/categories/[slug]`.
- **`components/RecommendationCard.tsx`** (new) — shared by both the `/dashboard` "Рекомендуем вам" grid and the `/courses/[id]` "Похожие курсы" grid (same `Recommendation` shape from both `GET /me/recommendations` and `GET /courses/:id/similar`). Shows title/image/category/access-type/rating plus the single primary reason as a one-line explanation — never the numeric score or full reason list.
- **`components/ContinueLearningCard.tsx`** (new) — "Продолжить" CTA links straight to `/learn/{course_id}/{next_lesson_id}`, i.e. exactly the server-computed next lesson (item 6/7), never a frontend guess; falls back to the course page in the rare case `next_lesson_id` is absent.
- **`app/courses/[id]/page.tsx`**: `WishlistButton` next to the enroll CTA (visible whenever logged in, independent of enrollment state — only successful enrollment auto-clears wishlist per the backend's item-20 rule, the student can still toggle freely otherwise); "Похожие курсы" section at the bottom via `getSimilarCourses(id)` (public call, renders nothing if the list comes back empty rather than showing an empty heading).
- **`app/dashboard/wishlist/page.tsx`** (new) — full wishlist grid via `GET /me/wishlist`, each card with a remove-capable `WishlistButton` (`initialInWishlist=true`), empty state links to `/courses`.
- **`app/dashboard/page.tsx`**: added "Продолжить обучение" (`getContinueLearning`, sliced to 5 per item 7's "3–5 courses") and "Рекомендуем вам" (`getMyRecommendations`, backend already caps at 6) blocks, both only rendered when non-empty; added an "Избранное →" nav link.
- **`app/globals.css`**: `.course-card` no longer carries anchor styling directly (moved to new `.course-card-link`, since the card is now a `<div>` housing both the link and the wishlist button); `.wishlist-toggle-btn` (card variant, absolutely positioned) + `.wishlist-inline .wishlist-toggle-btn` override (static, for the course-detail CTA bar usage); `.recommendation-card` (puts anchor styling back directly, since `RecommendationCard` has no nested button so the old single-`<Link>` pattern is safe there) + `.recommendation-reason`.

### Verification performed this session
- `npx tsc --noEmit` — clean, zero errors.
- `npx eslint .` — zero errors; 4 pre-existing-pattern `<img>`-vs-`next/image` warnings on the new components (`CourseCard`, `RecommendationCard`, `ContinueLearningCard`, the wishlist page), consistent with every other image-rendering component already in this codebase — not a new warning class introduced.
- `npm run build` (production, Turbopack) — compiled successfully, all 38 routes generated including the new `/dashboard/wishlist`.
- **Not run this session** (explicitly out of scope per instructions): Docker Compose, any live HTTP request, full E2E, security/regression checks. Nothing here has been exercised against a running backend yet.

## Done (this session — verification)

Read this session's own findings before assuming anything below is still open — several items in the two "deferred" lists above are now resolved.

### Blocking issue found and fixed
- **`internal/wishlist` was not wired into `cmd/api/main.go`** (exactly as flagged) — wired this session, mirroring the `recommendations` pattern exactly (import, `NewRepository/NewService/NewHandler`, `RegisterRoutes(v1, authMiddleware.RequireAuth())`). Confirmed live: `POST/DELETE /courses/:id/wishlist`, `GET /me/wishlist`, `GET /me/wishlist/course-ids` all return 401 (not 404) without a token, proving the routes now exist.

### Bug found and fixed during IDOR/security testing
- **`wishlist.Repository.Add` didn't check `published`** — only that the course row existed. Confirmed live: a freshly created draft (unpublished) course could be successfully wishlisted (`POST` → 200, `in_wishlist: true`), even though that same course is invisible everywhere else (public listing, recommendations, similar-courses all already filtered on `published = true`). Fixed by adding `AND published = true` to the existence check; both "doesn't exist" and "exists but unpublished" now return the same `404 COURSE_NOT_FOUND`, never distinguishing the two to the caller (matches this codebase's existing 404-not-403 convention). Re-verified live after rebuild: draft → 404, published course → 200, unaffected.
- Deliberately **not** changed: `ListForUser`/`ListCourseIDsForUser` still show a wishlist entry even if the course is later unpublished after being wishlisted while published — this mirrors how `course_enrollments` already behaves (a student's enrollment/certificate history isn't retroactively hidden either), so it's an intentional non-fix, not an oversight.

### EXPLAIN ANALYZE (all four required queries)
Seeded ~60 synthetic courses, ~80 users, ~80 enrollments, ~80 wishlist rows for realistic-enough volume, then ran every query from `internal/wishlist`, `internal/learning.GetContinueLearning`, and `internal/recommendations` through `EXPLAIN ANALYZE`:
- **Wishlist** (`GET /me/wishlist`): `Bitmap Index Scan on idx_course_wishlist_user_id` — 0.42ms.
- **Continue-learning**: `Bitmap Index Scan on course_enrollments_user_course_unique`, `idx_modules_course_id`, `idx_lessons_module_id`, `lesson_progress_user_lesson_unique` — 0.26–0.50ms.
- **Recommendation candidates + category affinity + learning-path-next**: all user-scoped filters resolve via `course_enrollments_user_course_unique`; `speciality_courses`/`specialities` joins are plain seq scans over genuinely tiny tables (3 seeded rows) — 0.11–1.12ms total.
- **Similar courses**: `Bitmap Index Scan on idx_speciality_courses_course_id` (both directions of the overlap join) — 0.43ms.
- The only sequential scans anywhere are on `courses` (63 rows) and `course_enrollments` (80 rows) — at this size PostgreSQL's planner correctly prefers a seq scan over an index scan (this is optimal, not a missing index; both tables already have adequate indexes — `course_enrollments_user_course_unique` was observed being used via index scan in a different query in the same session, confirming it's real and usable). **No new indexes needed beyond the two already on `course_wishlist` from migration 00036.**

### Full E2E (spec item 29's exact scenario list — every item passed)
Ran against the live stack with two fresh students plus the seeded demo data (`Go Backend Developer` → `PostgreSQL` → `Docker` roadmap in the `Backend Developer` speciality):
1. **New student → fallback recommendations**: 6 results, reasons limited to `new_course`/`popular` only — never `same_category`/`in_learning_path`/`in_wishlist` for a user with zero history. Confirmed **deterministic**: two consecutive calls produced byte-identical JSON.
2. **Add course to wishlist**: `POST` → 200, idempotent re-add → 200 again (never a 409).
3. **Wishlist page shows it**: `GET /me/wishlist` and `GET /me/wishlist/course-ids` both reflect it immediately.
4. **Wishlisted course rises in recommendations**: appeared with `score: 35`, `reasons: ["new_course", "in_wishlist"]`.
5. **Enroll → auto-disappears from wishlist**: `POST /courses/:id/enroll` → `GET /me/wishlist` → `[]`, confirming the transactional auto-remove.
6. **Complete a course → recommendations prefer its category**: after completing `Go Backend Developer` (category "Programming"), five different synthetic Programming-category courses picked up `same_category` in their reasons.
7. **Speciality next course gets high priority**: `PostgreSQL` (position 2, required) ranked **#1** overall with `score: 52` and `in_learning_path` — clearly outranking every same-category-only candidate (scores 7–17).
8. **Continue-learning shows the correct next lesson**: verified at three points — before any lesson (→ lesson 1 "Введение в Go"), after completing lesson 1 (→ correctly advanced to lesson 2 "Переменные"), and after all 6 lessons but before the final test (→ `next_lesson_id: null`, course still listed at 100% since `completed_at` is still `NULL` pending the final test — the documented edge case in `ContinueLearningItem`'s own doc comment, working exactly as designed).
9. **Complete course → disappears from continue-learning**: after passing the final test (100%, well above the 70% pass mark), `GET /me/continue-learning` → `[]`.

### Auth / authz (item 25)
Every `/me/*` and `POST/DELETE /courses/:id/wishlist` route: 401 without a token, 401 with a garbage/malformed token (never 500). `GET /courses/:id/similar` confirmed genuinely public: 200 with no `Authorization` header at all. Student correctly blocked (403) from `/admin/*` and `/instructor/*`.

### IDOR / security (item 25/26)
- Two independent students' wishlists never cross-contaminated (each endpoint reads `userID` only from the JWT via `authctx` — there is no route or body parameter for a client-supplied user id anywhere in this domain to even attempt to smuggle one through).
- A `?user_id=...` query string on `/me/recommendations` is silently ignored (the handler never reads it) — confirmed the response is unchanged from the caller's own account.
- A student's own completed course is excluded from their own recommendations (0 matches).
- An unpublished draft course is excluded from both recommendations and similar-courses (0 matches) — and, per the bug fix above, can no longer be wishlisted either.

### Regression (item 30 — every item checked, zero regressions found)
Search/filter (`q`, `category`, `level` all 200), enroll + premium-subscription gating (403 `COURSE_ACCESS_REQUIRED` correctly blocks an unsubscribed student from a `subscription`-access course), lesson video/assignment/coding-exercise endpoints (correct 404s, never 500, for lessons with none attached), final test fetch + submission + attempt history, certificate lazy-issuance (Stage 15, still fires correctly off a Stage 18 test flow), analytics/achievements/streaks (Stage 17 — `FIRST_LESSON`/`FIRST_COURSE`/`FIRST_CERTIFICATE` all correctly earned, streak counted, notifications enqueued off the very same enrollment/completion path Stage 18 exercises), notifications inbox, instructor stats, admin stats (including the Stage 17 DAU/WAU/monthly aggregates, which correctly picked up the session's synthetic + real activity), role-gating on `/admin/*` and `/instructor/*`. Frontend smoke-checked over HTTP (home/`/courses` 200, `/dashboard` and `/dashboard/wishlist` correctly 307-redirect when unauthenticated) — no frontend code changed this session, so no deeper frontend re-verification was needed.

### Final verification
`gofmt -l .` clean, `go vet ./...` clean, `go build ./...` clean — all run once more after the wishlist fix, from a cold rebuild, against the live stack. Compose stack torn down (`down -v`, `.env` removed) at the end of the session.

## Not started / explicitly deferred (unchanged — out of scope, not required for Stage 18 completion)

- **`internal/instructor`**: `wishlist_count` field on `CourseStats` (spec item 24) — a read-only nice-to-have explicitly marked optional in the spec ("Можно добавить"), never attempted, not a blocker for anything tested this session.
