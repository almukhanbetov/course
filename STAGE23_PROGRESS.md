# Stage 23 — Recommendation feedback loop

Tracking doc — status only, not a spec restatement.

## Stage 23A1 — recommendation feedback backend storage (this session)

Scope: minimal persistent storage for recommendation feedback (`dismiss`/`not_interested`), repository + service methods only, per this session's explicit instructions. No HTTP endpoint, no scoring changes, no frontend.

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 23 section fresh. Re-read `backend/internal/recommendations/{model,repository,service,handler}.go` in full — the entire domain, since this session's storage layer needed to reuse its existing conventions exactly. Confirmed the existing identity convention: `GetMyRecommendations` (handler.go) reads `userID` exclusively via `authctx.UserID(c)`, never a client-supplied field — every new method added this session takes `userID` as a plain parameter for the same reason, ready for a future handler to fill it from `authctx` the identical way. Read `backend/migrations/00036_create_course_wishlist.sql` and `internal/wishlist/repository.go`'s `Add`/`Remove` as the closest existing precedent for a user-scoped, course-referencing, duplicate-safe table: `UNIQUE(user_id, course_id)` + `ON DELETE CASCADE` from both `users` and `courses`, `ON CONFLICT ... DO NOTHING` for idempotent add, plain unconditional `DELETE` for idempotent remove. Did not inspect any other domain.

### Design decisions

- **One row per (user, course), not per (user, course, action).** The task asks for at least two actions (`dismiss`, `not_interested`); the roadmap's own migration plan specifies `UNIQUE(user_id, course_id)` with no `action` in the key. Reconciled by storing `action` as a plain column with the unique constraint only on `(user_id, course_id)`, and making the insert an **upsert** (`ON CONFLICT (user_id, course_id) DO UPDATE SET action = EXCLUDED.action, created_at = now()`) rather than `DO NOTHING`. A user can only ever have one active feedback row per course — submitting `dismiss` then later `not_interested` for the same course replaces the row rather than accumulating a second one. This directly satisfies "prevent duplicate feedback for the same user/course/action" in the strictest useful sense: no duplicate *rows* are ever possible for a given user/course pair, regardless of how many times or with which action they submit.
- **Bounded action vocabulary in the service, not just a free-text column.** `allowedFeedbackActions` (model.go) whitelists `FeedbackDismiss`/`FeedbackNotInterested`; `Service.SubmitFeedback` rejects anything else with a `ValidationError` before it ever reaches SQL — the same "validate in the service, not just trust the DB column" pattern `courses.SearchCourses` and `qa.CreateQuestion` already use for their own whitelisted inputs.
- **FK-violation mapping, not a pre-check `SELECT`.** Unlike `wishlist.Add` (which does an explicit `SELECT EXISTS(...)` because it also needs to check `published = true` specifically), `UpsertFeedback` relies on the `course_id` foreign key itself and maps a `23503` violation to `ErrCourseNotFound` — the same `isForeignKeyViolation` idiom `internal/qa`'s `CreateQuestion`/`CreateAnswer` already use. Simpler and sufficient here since feedback has no extra business rule beyond "the course must actually exist."
- **A read-back method was added (`ListFeedbackCourseIDs`) even though nothing calls it yet.** Storage that can't be read back isn't useful storage; the task's "add repository/service methods" naturally includes the retrieval side, while the explicit "do NOT modify recommendation scoring yet" instruction is honored by simply never calling this method from `GetRecommendations`/`GetSimilarCourses` this session.

### Change made

**Migration** `backend/migrations/00038_create_recommendation_feedback.sql`:
- `recommendation_feedback(id, user_id, course_id, action, created_at)`, `UNIQUE(user_id, course_id)`, `ON DELETE CASCADE` from both `users` and `courses`, indexes on `user_id` and `course_id` separately (the unique constraint's own index already covers `user_id`-leading lookups efficiently, but a dedicated `course_id` index is needed for any future per-course aggregate, mirroring `course_wishlist`'s identical two-index shape exactly).

**`backend/internal/recommendations/model.go`**:
- `Feedback` struct.
- `FeedbackDismiss`/`FeedbackNotInterested` constants + `allowedFeedbackActions` whitelist map.

**`backend/internal/recommendations/repository.go`**:
- `ErrCourseNotFound`, `isForeignKeyViolation` (new to this file — `errors`/`pgconn` imports added).
- `UpsertFeedback(ctx, userID, courseID, action) error` — the upsert described above.
- `DeleteFeedback(ctx, userID, courseID) error` — idempotent delete.
- `ListFeedbackCourseIDs(ctx, userID) ([]uuid.UUID, error)` — read-back, not yet consumed anywhere.

**`backend/internal/recommendations/service.go`**:
- `ValidationError` type (new to this package, matches `internal/qa`/`internal/courses`'s identical shape).
- `SubmitFeedback(ctx, userID, courseID, action) error` — validates the action, delegates to `UpsertFeedback`.
- `RemoveFeedback(ctx, userID, courseID) error` — delegates to `DeleteFeedback`.
- `ListFeedbackCourseIDs(ctx, userID) ([]uuid.UUID, error)` — delegates to the repository method of the same name.

No changes to `GetRecommendations`, `GetSimilarCourses`, `baselineScore`, or any scoring weight/constant. No changes to `handler.go` — zero new routes.

### Files changed

- `backend/migrations/00038_create_recommendation_feedback.sql` — new.
- `backend/internal/recommendations/model.go` — `Feedback`, action constants/whitelist.
- `backend/internal/recommendations/repository.go` — `ErrCourseNotFound`, `isForeignKeyViolation`, `UpsertFeedback`, `DeleteFeedback`, `ListFeedbackCourseIDs`.
- `backend/internal/recommendations/service.go` — `ValidationError`, `SubmitFeedback`, `RemoveFeedback`, `ListFeedbackCourseIDs`.
- No handler, no route, no frontend file touched.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/recommendations/*.go` — clean, no output.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**Live DB behavior** (migration applied via `docker compose up -d --build backend`, confirmed `goose: successfully migrated database to version: 38`; schema confirmed via `\d recommendation_feedback` — all columns, both indexes, the unique constraint, and both `ON DELETE CASCADE` foreign keys present exactly as the migration specifies). No HTTP endpoint exists yet to drive this through the API, so — consistent with how Stage 22A1 verified its own pre-endpoint repository query — the exact SQL each repository method issues was run directly via `psql` against the real seeded `users`/`courses` rows:

| Check | Result |
|---|---|
| Upsert (`dismiss`) for a real user/course | Inserted, 1 row |
| Upsert again (`not_interested`) for the *same* user/course | **Same row id**, `action` updated in place — confirmed via `count(*) = 1` for that pair afterward |
| Upsert for a second, different course | Separate row — courses are independently trackable |
| Read-back (`ListFeedbackCourseIDs` equivalent) | Returned both course ids for the user |
| Insert referencing a nonexistent `course_id` | **Rejected** — `23503 foreign key constraint "recommendation_feedback_course_id_fkey"` (confirms `isForeignKeyViolation` in the Go repository would correctly map this to `ErrCourseNotFound`) |
| Delete (undo) | Row removed, `count(*) = 0` afterward |
| Cascade delete | Created a throwaway unpublished course, added feedback against it, deleted the course → feedback row disappeared automatically with no separate delete needed |

All checks passed on the first attempt. Test data (both real-user test rows and the throwaway course) cleaned up afterward — verified `SELECT count(*) FROM recommendation_feedback` and the throwaway-course-title query both return `0`.

### Not done this session (explicitly out of scope for 23A1)

- **No HTTP endpoint** — `SubmitFeedback`/`RemoveFeedback`/`ListFeedbackCourseIDs` are reachable only from Go code today, not `POST/DELETE /recommendations/:courseId/dismiss` or any other path. Natural next slice (a future Stage 23A2), not attempted here.
- **No scoring changes** — `GetRecommendations` and `GetSimilarCourses` are byte-for-byte unchanged; `ListFeedbackCourseIDs` exists but is called from nowhere. A dismissed course will still appear in a user's recommendations until a future session wires the exclusion in.
- **No frontend** — no dismiss button, no undo toast, nothing in `frontend/`.
- **No regression pass** — out of scope for this focused backend-storage session.

## Stage 23A2 — recommendation feedback HTTP endpoint (this session)

Scope: one authenticated endpoint (plus its clean undo counterpart) wiring Stage 23A1's `SubmitFeedback`/`RemoveFeedback` into the public API. No scoring changes, no frontend.

### Inspection performed

Re-read `STAGE23_PROGRESS.md`'s Stage 23A1 section and `backend/internal/recommendations/handler.go` in full — the only file this session needed to touch. Confirmed `GetMyRecommendations`'s exact identity pattern (`authctx.UserID(c)`, 401 if absent, never a body/param field) to replicate verbatim. Checked the closest sibling feature, `internal/wishlist/handler.go`'s `AddToWishlist`/`RemoveFromWishlist`, for two conventions worth reusing rather than reinventing: (1) route shape — course-scoped path (`/courses/:id/wishlist`) rather than nesting under `/me/...`, which this session mirrored as `/recommendations/:id/feedback`; (2) response shape for an idempotent write — `200` with a small `{course_id, ...}` JSON body (not `201`/`204`), with an inline comment explaining why 200 is deliberate for an idempotent action ("already there" and "just added" are indistinguishable on purpose) — copied the same reasoning for feedback's upsert. Did not inspect any other domain.

### Design decision: one endpoint, action in the body — not two action-specific routes

The roadmap's own Stage 23 line sketched `POST /recommendations/:courseId/dismiss`, one path per action. This session used a single `POST /recommendations/:id/feedback` with `{"action": "dismiss" | "not_interested"}` in the body instead, for two concrete reasons: (1) it's a direct, zero-translation wire onto `Service.SubmitFeedback(ctx, userID, courseID, action)`'s existing signature — no per-route hardcoding of which action to pass; (2) this session's own required test matrix includes "invalid action," which only makes sense as a request the client can actually send (an unrecognized string in the body) — a hardcoded `/dismiss` route has no action parameter for a client to get wrong. `Service.SubmitFeedback`'s existing `allowedFeedbackActions` whitelist (built in 23A1) now does real validation work for the first time.

### Change made

`backend/internal/recommendations/handler.go`:
- `RegisterRoutes`: added `POST /recommendations/:id/feedback` and `DELETE /recommendations/:id/feedback`, both behind `requireAuth`, alongside the existing `GET /me/recommendations`/`GET /courses/:id/similar`.
- `feedbackRequest{Action string}` DTO.
- `SubmitFeedback(c *gin.Context)`: `authctx.UserID` (401 if missing) → parse `:id` as the course UUID (400 `INVALID_COURSE_ID` if malformed) → bind `feedbackRequest` (400 `INVALID_BODY` if malformed) → `service.SubmitFeedback`, mapping `*ValidationError` → 400 `VALIDATION_ERROR`, `ErrCourseNotFound` → 404 `COURSE_NOT_FOUND`, success → 200 with `{course_id, action}`.
- `RemoveFeedback(c *gin.Context)`: same identity/course-id handling, delegates to `service.RemoveFeedback`, success → 200 with `{course_id, has_feedback: false}` — added per this session's instruction #8 ("only if the Stage 23A1 service already supports it cleanly"), which it does: `RemoveFeedback` already existed, untouched, from 23A1.

No changes to `model.go`, `repository.go`, or `service.go` — every piece this endpoint calls already existed from Stage 23A1, exactly as instructed ("reuse Stage 23A1 service/repository"). No changes to `GetRecommendations`/`GetSimilarCourses`.

### Files changed

- `backend/internal/recommendations/handler.go` — two new routes, `feedbackRequest` DTO, `SubmitFeedback`, `RemoveFeedback` handlers.
- No other file touched this session.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/recommendations/*.go` — clean, no output.
- `go build ./...` — OK.
- `go vet ./...` — OK.

**Live** (Docker Compose backend rebuilt via `docker compose up -d --build backend`; `docker compose logs backend` confirmed both new routes registered with no panic; fresh student account registered for this session):

| Case | Request | Result |
|---|---|---|
| Authenticated valid feedback | `POST /recommendations/{courseA}/feedback {"action":"dismiss"}` | **200**, `{"action":"dismiss","course_id":"..."}` |
| Repeated same feedback | Same request again | **200**, identical body — confirmed via direct DB query afterward: still exactly 1 row for this (user, course) |
| Switching dismiss → not_interested | `POST .../feedback {"action":"not_interested"}` | **200**, `{"action":"not_interested",...}` — DB confirmed same row, `action` updated in place, still exactly 1 row |
| Invalid action | `POST .../feedback {"action":"spam_flag"}` | **400** `VALIDATION_ERROR`, "action must be one of: dismiss, not_interested" |
| Nonexistent course (valid UUID, no such row) | `POST /recommendations/00000000-.../feedback {"action":"dismiss"}` | **404** `COURSE_NOT_FOUND` |
| Malformed course id (not a UUID) | `POST /recommendations/not-a-uuid/feedback {...}` | **400** `INVALID_COURSE_ID` |
| Unauthenticated request | `POST .../feedback` with no `Authorization` header | **401** `UNAUTHORIZED` |
| Undo (`DELETE`) | `DELETE /recommendations/{courseA}/feedback` | **200**, `{"has_feedback":false,...}` — DB confirmed row count 0 afterward |
| Undo again (idempotent) | Same `DELETE` again | **200**, identical body, no error despite nothing left to delete |
| Unauthenticated `DELETE` | No `Authorization` header | **401** `UNAUTHORIZED` |

All 10 cases (the 6 required plus 4 extra: malformed-UUID, DB-level idempotency confirmation, undo, and undo's own auth/idempotency) passed on the first attempt — no code changes needed after the initial implementation. Test account left in place (harmless, matches every prior stage's convention); confirmed zero residual `recommendation_feedback` rows afterward via direct DB query.

### Not done this session (explicitly out of scope for 23A2)

- **No scoring integration** — `GetRecommendations`/`GetSimilarCourses` are still byte-for-byte unchanged since 23A1; a dismissed course still appears in that user's recommendations until a future session wires `ListFeedbackCourseIDs` into candidate exclusion.
- **No frontend** — no dismiss button, no undo affordance, nothing in `frontend/`.
- **No cross-user isolation test performed live** — not one of this session's required cases; the isolation guarantee is structural (every query is scoped by `userID` from `authctx`, the same pattern Stage 18 already verified for wishlist), not yet independently re-proven with two real accounts the way Stage 18/20/21 did for their own domains. Worth an explicit live check in a future security-focused Stage 23 session.
- **No regression pass** — out of scope for this focused endpoint session.

## Stage 23A3 — recommendation feedback scoring/filter integration (this session)

Scope: make saved feedback actually affect `GET /me/recommendations`. No new actions, no new scoring weights, no frontend, no changes to the public similar-courses path.

### Inspection performed

Re-read `STAGE23_PROGRESS.md`'s Stage 23A1/23A2 sections and `backend/internal/recommendations/{repository,service}.go` fresh. Confirmed `ListCandidatesForUser` (the sole candidate query behind `GetRecommendations`) already excludes enrolled courses via `AND NOT EXISTS (SELECT 1 FROM course_enrollments ce WHERE ce.course_id = c.id AND ce.user_id = $1)` — a ready-made template for a second, identically-shaped exclusion, in the same single query. Confirmed `GetSimilarCourses`/`ListSimilarCandidates` takes no `userID` parameter at all (`GetSimilarCourses(ctx, courseID, limit)` — the handler comment: "item 18: Не требует personalization"), so per this session's instruction 5 ("do not change public similar-courses behavior unless feedback is user-specific and already part of that authenticated path") it was correctly left untouched — there is no authenticated user identity on that path for feedback to even key off.

### Design decision: extend the existing query, not a second round trip

Two options existed: (a) fetch dismissed course ids via the already-existing `ListFeedbackCourseIDs` (23A1) as a separate query and filter candidates in Go, mirroring how `wishlistIDs`/`pathIDs` are fetched and turned into lookup sets for *scoring boosts*; or (b) add one more `NOT EXISTS` clause to `ListCandidatesForUser`'s existing SQL, exactly mirroring the enrollment exclusion already there. Chose (b): it adds zero queries (`GetRecommendations` still issues exactly the same four queries it always has — candidates, affinity, wishlist, learning-path), reuses an idiom already proven in the same query rather than introducing a new one, and is strictly more efficient than (a), which would fetch full `Candidate` rows for courses only to discard them in Go. This directly serves instructions 6/7 ("avoid N+1," "keep the query bounded and efficient"). `ListFeedbackCourseIDs` (23A1) remains unused by scoring — it still exists for a possible future "list my dismissed courses" read surface, just not this one.

Both `dismiss` and `not_interested` are treated identically by this exclusion — the `NOT EXISTS` clause checks only that *a* feedback row exists for `(user_id, course_id)`, never which `action` it holds. This matches the instruction's own behavior spec (both actions "exclude that course") and avoids inventing any action-specific scoring logic.

### Change made

`backend/internal/recommendations/repository.go`:
- `ListCandidatesForUser`: added `AND NOT EXISTS (SELECT 1 FROM recommendation_feedback rf WHERE rf.course_id = c.id AND rf.user_id = $1)` as a third exclusion clause, alongside the existing published/instructor/enrollment ones. Doc comment updated to describe the new behavior and explicitly note this stays a single query.

No other function changed. `GetRecommendations` (service.go) is untouched — the exclusion is entirely inside the query it already calls, so no service-layer wiring was needed at all. `GetSimilarCourses`/`ListSimilarCandidates` untouched, per instruction 5.

### Files changed

- `backend/internal/recommendations/repository.go` — one new `NOT EXISTS` clause in `ListCandidatesForUser`, doc comment updated.
- No other file touched this session.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/recommendations/*.go` — clean, no output.
- `go build ./...` — OK.
- `go vet ./...` — OK.

**`EXPLAIN ANALYZE`** (query shape changed materially — one more `NOT EXISTS` — so run per instruction 8; a real feedback row was seeded first so the plan reflects a genuine anti-join, not a trivially-empty-table shortcut): the planner used a `Nested Loop Anti Join` against `recommendation_feedback` via `Bitmap Index Scan` on **`idx_recommendation_feedback_user_id`** (the index Stage 23A1's migration already created) — the identical shape as the pre-existing enrollment-exclusion anti-join one level up in the same plan. **Execution time: 0.396 ms**, still sub-millisecond. **No new index was needed or added** — 23A1's `idx_recommendation_feedback_user_id` already covers this lookup exactly, confirmed by the plan actually using it rather than falling back to a sequential scan. Seed row cleaned up immediately after.

**Live, end-to-end through the real API** (Docker Compose backend rebuilt via `docker compose up -d --build backend`, no panic; fresh student account):

| Step | Action | Result |
|---|---|---|
| 1. Baseline | `GET /me/recommendations`, no feedback yet | **3 courses**: Go Backend Developer, PostgreSQL, Docker |
| 2. Submit dismiss | `POST /recommendations/{docker}/feedback {"action":"dismiss"}` | **200** |
| 3. Dismissed course disappears | `GET /me/recommendations` | **2 courses** — Docker absent, confirmed by id |
| 4. Submit not_interested | `POST /recommendations/{postgresql}/feedback {"action":"not_interested"}` | **200** |
| 5. Second course disappears | `GET /me/recommendations` | **1 course** (Go Backend Developer only) — both Docker and PostgreSQL absent |
| 6. Undo feedback | `DELETE /recommendations/{docker}/feedback` | **200**, `has_feedback:false` |
| 7. Course reappears per normal scoring | `GET /me/recommendations` | **2 courses** — Docker back, PostgreSQL still excluded (its own feedback was never undone) |

All 7 required steps passed on the first attempt. Two additional checks performed to confirm instruction 4 ("keep recommendation behavior unchanged for users with no feedback") and the isolation this implies: a second, unrelated fresh account with zero feedback still saw all 3 courses at the same point in the sequence — the exclusion is genuinely per-user, not global. The public `GET /courses/:id/similar` endpoint was spot-checked unaffected (200, no auth required, unchanged response shape). All test feedback rows deleted afterward; confirmed `SELECT count(*) FROM recommendation_feedback` → `0`.

### Not done this session (explicitly out of scope for 23A3)

- **No new scoring weights or reasons** — this is a hard pre-filter (candidates never enter the scorer at all), not a negative score adjustment; `Recommendation.Reasons`/the weight constants in `service.go` are untouched.
- **No frontend** — no dismiss button, no undo affordance, nothing in `frontend/`.
- **No change to `GetSimilarCourses`** — deliberately, per instruction 5; it has no user identity to key an exclusion off in the first place.
- **No full regression pass** — out of scope for this focused integration session.

## Stage 23B1 — recommendation feedback frontend controls (this session)

Scope: "Скрыть" (dismiss) / "Не интересно" (not_interested) buttons on personalized recommendation cards only, wired to Stage 23A2's real endpoint, immediate client-side removal, no undo UI, no backend changes.

### Inspection performed

Re-read `STAGE23_PROGRESS.md`'s Stage 23A1/23A2/23A3 sections and the existing frontend surface fresh. Confirmed `RecommendationCard.tsx` is **shared** by two call sites: `app/dashboard/page.tsx`'s personalized "Рекомендуем вам" grid and `app/courses/[id]/page.tsx`'s public "Похожие курсы" grid (the component's own doc comment says so explicitly) — so adding feedback buttons unconditionally to the shared component would have violated "no feedback controls on public Similar Courses." Checked `WishlistButton.tsx` and its three real call sites (`CourseCard.tsx`, `courses/[id]/page.tsx`, `dashboard/wishlist/page.tsx`) for the established pattern of adding an action button to a card that's otherwise one big `<Link>`: every single usage renders the button as a **sibling** of the `<Link className="course-card-link">` inside an outer `<div className="course-card">`, never nested inside the anchor — `CourseCard.tsx`'s own comment states why: "a `<button>` inside an `<a>` is both invalid HTML and would double-fire navigation on click." The pre-existing `RecommendationCard` violated this: its outer element *was* the `<Link>` itself (`<Link className="course-card recommendation-card">`), which would have made adding buttons unsafe without first fixing the structure. Checked `lib/actions.ts`'s `addToWishlistAction`/`removeFromWishlistAction` for the server-action shape to mirror for the new feedback action.

### Design decision: fix `RecommendationCard`'s structure to match the established pattern first (not a redesign)

Restructured `RecommendationCard` from `<Link className="course-card recommendation-card">…</Link>` to `<div className="course-card recommendation-card">` wrapping the same content inside `<Link className="course-card-link">`, with feedback buttons as a sibling before it — identical shape to `CourseCard`'s own div+Link+WishlistButton structure. This was necessary to add buttons safely at all (same invalid-HTML/double-navigation problem `CourseCard`'s own comment already documents), not a visual or content redesign: every field rendered, every class name on the inner elements, and the existing `.course-card:has(.course-card-link:hover)` hover-lift rule (already generic enough to cover a div+inner-Link shape, since `CourseCard` already relies on it) all carry over unchanged. Confirmed `/courses/[id]`'s usage renders identically either way, since `RecommendationCard` there is passed no `feedback` prop and the buttons block simply doesn't render (`{feedback && (...)}`).

Feedback controls are wired through an **optional `feedback` prop** (`RecommendationFeedbackProps`: `pending`, `error`, `onDismiss`, `onNotInterested`) rather than a boolean flag — when absent (the `/courses/[id]` similar-courses call site, untouched this session), zero buttons render and zero new code runs there, satisfying "do not add feedback controls to public Similar Courses" by simple absence rather than an explicit conditional at each call site.

### Change made

`frontend/lib/actions.ts`:
- `RecommendationFeedbackResult`, `submitRecommendationFeedbackAction(courseId, action)` — same shape as `addToWishlistAction`, `POST`s `{action}` to `/api/v1/recommendations/:id/feedback` with the session token, parses `{error:{message}}` on failure.

`frontend/components/RecommendationCard.tsx`:
- Restructured to `div.course-card` + sibling `feedback-actions`/`Link.course-card-link` (see above).
- New optional `feedback?: RecommendationFeedbackProps` prop: renders two `.btn-small` buttons ("Скрыть" / "Не интересно") when present, each showing `"..."` and `disabled` while `feedback.pending` is true for that specific card, plus a `role="alert"` error line below the card content when `feedback.error` is set.

`frontend/components/PersonalizedRecommendations.tsx` (new, `"use client"`):
- Owns the recommendations array as local state, initialized from the server-fetched `initialRecommendations` prop.
- `handleFeedback(courseId, action)`: per-course pending tracking (`pendingId`) and per-course error tracking (`errors` keyed by `course_id`), calls `submitRecommendationFeedbackAction`, and on success **removes that item from local state immediately** — no refetch, no page reload. On failure, sets that course's error message and leaves the card in place (the user can retry).
- Renders the same `.course-grid` wrapper `dashboard/page.tsx` used to render inline, now internal to this component so it can react to its own state.

`frontend/app/dashboard/page.tsx`:
- Swapped the inline `recommendations.map((rec) => <RecommendationCard key={...} rec={rec} />)` block for `<PersonalizedRecommendations initialRecommendations={recommendations} />`. The outer `{recommendations.length > 0 && (<section>…</section>)}` gate and its "Рекомендуем вам"/"Весь каталог →" header are untouched.

`frontend/app/courses/[id]/page.tsx`:
- **Not touched.** Still renders `<RecommendationCard key={rec.course_id} rec={rec} />` with no `feedback` prop — confirmed by inspection this session, not by omission.

`frontend/app/globals.css`:
- `.recommendation-feedback-actions` (`display:flex; gap:0.4rem; margin-bottom:0.75rem`) and `.recommendation-feedback-error` (`color: var(--danger); font-size: 0.78rem`) — both new rules reuse existing tokens only (`var(--danger)`); the buttons themselves reuse the pre-existing `.btn-small` class as-is, no new button styling.

### Files changed

- `frontend/lib/actions.ts` — `RecommendationFeedbackResult`, `submitRecommendationFeedbackAction`.
- `frontend/components/RecommendationCard.tsx` — restructured (div+Link, matching `CourseCard`'s pattern) + optional `feedback` prop and buttons.
- `frontend/components/PersonalizedRecommendations.tsx` — new client wrapper.
- `frontend/app/dashboard/page.tsx` — swapped inline map for the new wrapper.
- `frontend/app/globals.css` — two new rules.
- No backend files touched. `frontend/app/courses/[id]/page.tsx` inspected, confirmed unaffected, not edited.

### Verification performed (focused, per scope)

- `npx tsc --noEmit` (whole project) — clean.
- `npx eslint components/RecommendationCard.tsx components/PersonalizedRecommendations.tsx app/dashboard/page.tsx lib/actions.ts` and a full `npx eslint .` — **0 errors**, still exactly the same 4 pre-existing `<img>` warnings every prior stage has reported (`RecommendationCard.tsx` was already one of the 4 before this session; its warning simply persists at its new line number since the `<img>` tag itself wasn't touched — confirmed by comparing the total count, unchanged at 4).

No live browser interaction, no Docker Compose rebuild, no E2E — explicitly out of scope for 23B1. The backend endpoint this component calls was already live-verified end-to-end in Stage 23A2/23A3 (valid/repeated/switched feedback, invalid action, nonexistent course, unauthenticated, and — in 23A3 — the actual exclusion-from-results effect); this session's job was limited to confirming the frontend calls it correctly and updates state as specified, which `tsc`'s type-checking of `RecommendationFeedbackResult`/`RecommendationFeedbackProps` against the component wiring plus the code-level review above covers for this focused pass.

### Known limitations (Stage 23B1)

- **No undo UI** — explicitly out of scope per this session's instructions ("do NOT add undo UI yet unless it already exists naturally"); it doesn't, so none was added. `Service.RemoveFeedback`/`DELETE /recommendations/:id/feedback` (23A1/23A2) remain unused by the frontend.
- **Dismissing every recommendation leaves an empty section header.** `dashboard/page.tsx`'s `{recommendations.length > 0 && (<section>…)}` gate is evaluated server-side from the initial fetch and doesn't re-run when the client-side list empties out via dismissal — so a user who dismisses all recommendations in one visit sees the "Рекомендуем вам" heading and "Весь каталог →" link remain with an empty grid below (`PersonalizedRecommendations` returns `null` when its list is empty), not a fully-cleaned-up section. Not fixed this session: doing so would mean lifting the section header itself into the client component, which starts to reach into `dashboard/page.tsx`'s layout beyond "add feedback controls to cards." Documented here rather than silently left for a future session to rediscover.
- **No real browser interaction** — same standing limitation noted in Stages 21C/22C's own sections; no browser-automation tool is available in this environment.

## Stage 23C — focused verification + final Stage 23 report (this session)

Scope: verify 23A1–23A3 (backend) and 23B1 (frontend) live together, fix only bugs the verification itself surfaced, close out Stage 23. No new features attempted.

### Setup

Rebuilt both `backend` and `frontend` (`docker compose up -d --build backend frontend`) — frontend was still serving the pre-23B1 image going into this session. Confirmed both healthy (`GET /api/v1/health` → 200, `GET /` → 200) with no route-conflict panic in `docker compose logs backend`; migration already at version 38 (no-op). Registered two fresh accounts (user A, user B) for this session's isolation checks.

### Bugs found

**None.** Every check below passed on the first attempt; no code changes were made this session.

### 1. E2E — verified live (backend, real API calls)

| Case | Result |
|---|---|
| Baseline personalized recommendations (user A, no feedback) | 3 courses: Go Backend Developer, PostgreSQL, Docker |
| Dismiss Docker | 200; Docker excluded from next `GET /me/recommendations` (2 remain) |
| not_interested on PostgreSQL | 200; PostgreSQL also excluded (1 remains) |
| Repeated feedback (dismiss Docker again) | 200; DB confirmed still exactly 1 row for (user A, Docker) |
| Switching action (Docker: dismiss → not_interested) | 200; DB confirmed same row, `action` updated in place, still exactly 1 row |
| Undo (Docker) | 200, `has_feedback:false`; Docker reappears in next `GET /me/recommendations`, PostgreSQL (never undone) stays excluded |
| Another user (B) unaffected | Fresh account, zero feedback, sees all 3 courses throughout — A's dismissals never leaked |
| Public Similar Courses unaffected | `GET /courses/{docker}/similar` (no auth) still includes PostgreSQL, which user A had marked `not_interested` — confirms the public path is correctly untouched by any user's personal feedback |

All 8 cases passed.

### 2. Frontend — verified via SSR/code (no browser-automation tool in this environment, same standing limitation as Stages 21C/22C)

- **Buttons call the real backend**: `submitRecommendationFeedbackAction` (`lib/actions.ts`) `POST`s to the exact `/api/v1/recommendations/:id/feedback` endpoint live-verified in section 1 above, with `{action}` as the body — confirmed by code inspection plus `tsc` type-checking its return shape against `PersonalizedRecommendations`' usage.
- **Buttons render correctly, and only where they should**: SSR-fetched `/dashboard` with a real session cookie (`Cookie: lms_session=<jwt>`, the same technique established in Stage 21C) — found exactly 3 "Скрыть" and 3 "Не интересно" buttons, one pair per recommendation card. SSR-fetched `/courses/{docker}` (the public similar-courses page) — **zero** occurrences of either button text, confirming `RecommendationCard`'s optional `feedback` prop correctly gates the buttons off entirely for that call site.
- **Card disappears after success / API failure does not remove card**: verified by fresh code re-read of `PersonalizedRecommendations.handleFeedback` — the `setRecommendations((prev) => prev.filter(...))` removal call is reached *only* inside the `result.ok` branch; the failure branch sets an error and `return`s before ever touching the list, so a failed request structurally cannot remove a card. No bug found on this re-review (unchanged since 23B1).
- **Loading state**: `pending: pendingId === rec.course_id` scopes the loading indicator (button text → `"..."`, both buttons `disabled`) to only the specific card being acted on — confirmed by code inspection; not runtime-triggered without a browser.
- **Empty recommendations state remains safe**: live-tested two related but distinct scenarios: (a) dismissed all 3 of user A's recommendations via the API, then did a **fresh** `GET /dashboard` SSR fetch (i.e. the server's own `getMyRecommendations` call for that request now returns `[]`) — **200**, no server error text in the response, and the entire "Рекомендуем вам" section (including its header) correctly absent, since `dashboard/page.tsx`'s own `{recommendations.length > 0 && (...)}` gate is false for a genuinely-empty server-side fetch. (b) The narrower scenario `STAGE23_PROGRESS.md`'s Stage 23B1 section already flagged as a known limitation — a user dismissing every recommendation *within an already-rendered page session* (where the section header was already committed to the DOM before any client-side dismissal) — could not be reproduced this session either, since it requires driving real sequential client clicks, and no browser-automation tool is available in this environment. Both scenarios are now distinguished explicitly rather than conflated, and the safe (a) case is now live-confirmed rather than only reasoned about.

### 3. Authorization — verified live

| Case | Result |
|---|---|
| Unauthenticated `POST` feedback | 401 `UNAUTHORIZED` |
| Unauthenticated `DELETE` feedback | 401 `UNAUTHORIZED` |
| **Explicit `user_id` spoofing attempt** — user A's JWT, body includes `"user_id": "<user B's real id>"` | 200 (request succeeds, since the endpoint has no `user_id` field to reject) — but the resulting DB row's `user_id` is confirmed via direct query to be **user A's real id**, never B's. The spoofed field is silently ignored, not merely rejected: `feedbackRequest` (handler.go) has no `user_id` field at all for a client to populate, and `authctx.UserID(c)` is the only identity source, so there was structurally nothing for the spoofed value to influence. |
| Invalid action (`"super_dismiss"`) | 400 `VALIDATION_ERROR`, "action must be one of: dismiss, not_interested" |
| Nonexistent course (valid UUID, no such row) | 404 `COURSE_NOT_FOUND` |
| Malformed course id (not a UUID) | 400 `INVALID_COURSE_ID` |
| Malformed request body | 400 `INVALID_BODY` |

All 7 cases passed, including the user_id-spoofing case explicitly designed to try to break the "user cannot submit feedback as another user" guarantee — it held.

### Final checks

- `gofmt -l .` (whole backend) — clean.
- `go build ./...` — OK.
- `go vet ./...` — OK.
- `npx tsc --noEmit` (whole frontend) — clean.
- `npx eslint .` (whole frontend) — 0 errors, same 4 pre-existing unrelated `<img>` warnings every prior stage has reported (`RecommendationCard.tsx` was already one of them before Stage 23B1), nothing new.
- Docker Compose: both `backend` and `frontend` rebuilt and live; `docker compose logs backend` confirms both feedback routes registered with no panic; migration confirmed at version 38.

### Files changed (all of Stage 23: 23A1 + 23A2 + 23A3 + 23B1 + 23C)

Backend:
- `backend/migrations/00038_create_recommendation_feedback.sql` — new table (23A1).
- `backend/internal/recommendations/model.go` — `Feedback`, action constants/whitelist (23A1).
- `backend/internal/recommendations/repository.go` — `UpsertFeedback`, `DeleteFeedback`, `ListFeedbackCourseIDs`, `ErrCourseNotFound` (23A1); `ListCandidatesForUser`'s new `NOT EXISTS` exclusion clause (23A3).
- `backend/internal/recommendations/service.go` — `ValidationError`, `SubmitFeedback`, `RemoveFeedback`, `ListFeedbackCourseIDs` (23A1).
- `backend/internal/recommendations/handler.go` — `POST`/`DELETE /recommendations/:id/feedback` routes + handlers (23A2).

Frontend:
- `frontend/lib/actions.ts` — `submitRecommendationFeedbackAction` (23B1).
- `frontend/components/RecommendationCard.tsx` — restructured (div+Link, matching `CourseCard`'s pattern) + optional `feedback` prop/buttons (23B1).
- `frontend/components/PersonalizedRecommendations.tsx` — new client wrapper (23B1).
- `frontend/app/dashboard/page.tsx` — swapped inline card map for the wrapper (23B1).
- `frontend/app/globals.css` — `.recommendation-feedback-actions`, `.recommendation-feedback-error` (23B1).

No file changed this session (23C) — verification only, zero bugs found.

### Known limitations (Stage 23, final)

- **No real browser interaction anywhere in Stage 23.** Every purely-client-side-JS-runtime behavior (actual click dispatch, actual loading-state transition, the mid-session "dismiss everything" empty-header edge case) was verified by code inspection plus the strongest available live substitutes (SSR HTML, direct API reproduction, a genuinely-empty fresh-load variant of the empty-state test) rather than an actual driven interaction — no browser-automation tool is available in this environment. Same standing limitation as Stages 21C/22C.
- **Mid-session "dismiss all recommendations" leaves an orphaned section header** — documented in Stage 23B1, re-confirmed still accurate and still not fixed this session (fixing it would mean lifting `dashboard/page.tsx`'s section header into client state, beyond "add feedback controls to cards"). Distinguished this session from the safe, live-confirmed fresh-page-load-with-zero-recommendations case, which correctly hides the whole section.
- **No undo UI** — unchanged from 23B1; `DELETE /recommendations/:id/feedback` exists and is live-verified working, but nothing in the frontend calls it. Out of scope per Stage 23B1's explicit instructions.
- **`GetSimilarCourses` has no feedback exclusion** — deliberate, per Stage 23A3's instruction 5: that endpoint has no user identity to key an exclusion off, and this session's live check confirms it correctly remains unaffected by any user's personal feedback.
- No automated test suite exists anywhere in this codebase (unchanged, project-wide convention) — every verification claim above is a live, scripted check against the running Docker Compose stack, or a documented code-level review substituting for one where live interaction wasn't reachable.

### Final Stage 23 status

**Stage 23 is complete for its scoped ambition (recommendation feedback loop).** Storage (23A1), the authenticated HTTP endpoint (23A2), scoring/filter integration (23A3), and the frontend controls wired to all of it (23B1) are all implemented and live-verified, including this session's own fresh, focused pass across every layer together — E2E, authorization (including an explicit spoofing attempt), and the frontend surface — which found **zero bugs**, unlike Stages 21C/22C, which each caught one real issue during their own closeout verification. The items listed under "Known limitations" are deliberately deferred, documented scope boundaries (no undo UI, the narrow mid-session empty-header edge case, no browser-automation tool in this environment) — not defects.
