# Stage 24 — Content abuse reporting + admin moderation queue

Tracking doc — status only, not a spec restatement.

## Stage 24A1 — content abuse reporting backend storage (this session)

Scope: minimal persistent storage for content abuse reports (question/answer/review), migration + model + repository + service only. No HTTP endpoints, no admin moderation UI, no changes to existing Q&A/review behavior, no frontend.

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 24 section fresh. Re-read `backend/internal/qa/{model,repository}.go` and `backend/internal/reviews/model.go` — the three content domains this feature reports against — to confirm real table names (`lesson_questions`, `question_answers`, `course_reviews`) and their `id`/`user_id` column shapes. Confirmed all three are UUID-keyed with the same `id UUID PRIMARY KEY`/`created_at`/`updated_at` shape this session's own table follows. Checked `internal/courses/repository.go`'s `courseSortClause` for the established precedent of mapping a pre-validated Go value to a small fixed set of SQL fragments/identifiers without ever touching raw user input — reused for this session's `contentTable` helper. Checked `STAGE23_PROGRESS.md`'s Stage 23A1 section (the most recent, closest-shaped prior work: another user-scoped, duplicate-preventing, `ON DELETE CASCADE` table) for the established migration/repository conventions to match. Did not inspect `internal/admin`, `internal/users`, or any other domain beyond what identity convention (`authctx.UserID`, confirmed via `internal/qa`/`internal/recommendations`'s existing handlers, not re-read this session since no handler is being added) this session's design needed to stay consistent with.

### Design decisions

- **Polymorphic `content_type` + `content_id`, no direct foreign key on `content_id`.** A single column can't conditionally reference three different tables (`lesson_questions`/`question_answers`/`course_reviews`), so `content_id` is a plain `UUID NOT NULL` with no FK constraint — existence is validated in the service layer instead (`Repository.ContentExists`, which maps `content_type` to the right table via a small Go-side switch, the same "map a validated value to a fixed SQL fragment" idiom `courses.courseSortClause` already established). This directly matches the instruction's literal field list (`content_type`, `content_id` — not three separate nullable FK columns).
- **One row per (reporter, content) *while a report is open*, not ever.** A partial unique index — `UNIQUE (reporter_user_id, content_type, content_id) WHERE status = 'open'` — rather than a plain unique constraint over all columns. This blocks a duplicate *active* report from the same user against the same content while their earlier report is still unresolved, but deliberately allows a fresh report after the earlier one is resolved/dismissed (a new instance of abuse on the same content later should still be reportable). A plain full-column `UNIQUE` would need every terminal status value to be distinct just to avoid accidentally blocking legitimate re-reports; the partial index states the actual intent directly.
- **`content_type` and `status` are both bounded vocabularies, enforced twice: Go whitelist maps (`allowedContentTypes`/`allowedStatuses` in `model.go`) *and* DB-level `CHECK` constraints.** Every other bounded-string-enum column added so far in this codebase (`recommendation_feedback.action`, Stage 23A1) relies on Go-side validation alone, with no DB constraint. This session added the `CHECK` constraint too — a deliberate deviation, made because this session's own verification scope explicitly calls for confirming "invalid type/status rejection" as *database* behavior, not only an application-level check that happens not to have run yet (no handler exists this session to run it). The `CHECK` constraint is real defense-in-depth regardless: even a future direct-SQL path or a bug in the Go whitelist can't insert an out-of-vocabulary value.
- **`Status` values chosen ahead of any code that writes them.** Only `open` is ever written this session (`Create` relies on the column's own `DEFAULT 'open'`, never sets it explicitly). `resolved`/`dismissed` exist in both the `CHECK` constraint and `allowedStatuses` now so a future admin-resolve action (explicitly out of scope this session) has real values to write against without a second migration — mirroring the roadmap's own "resolve (dismiss the report, or hide the underlying content)" framing, which implies exactly two terminal states.
- **Duplicate detection surfaces as a typed error, not a silent no-op.** Unlike `wishlist.Add`'s `ON CONFLICT ... DO NOTHING` (fire-and-forget idempotent), `reports.Repository.Create` also uses `DO NOTHING` against the partial index but detects the resulting zero-row `RETURNING` (via `pgx.ErrNoRows`) and maps it to an explicit `ErrDuplicateActiveReport`. Chosen over silent success because this session's own verification scope treats "duplicate handling" as its own observable test case, and a future handler will likely want to tell a user "you've already reported this" rather than pretend a second report succeeded.

### Change made

**Migration** `backend/migrations/00039_create_content_reports.sql`:
- `content_reports(id, reporter_user_id, content_type, content_id, reason, status, created_at, updated_at)`.
- `reporter_user_id` → `REFERENCES users(id) ON DELETE CASCADE`.
- `CHECK (content_type IN ('question', 'answer', 'review'))`, `CHECK (status IN ('open', 'resolved', 'dismissed'))`, `status` defaults to `'open'`.
- `idx_content_reports_active_unique` — the partial unique index described above.
- `idx_content_reports_status_created_at` (open-queue-by-age, not consumed yet) and `idx_content_reports_content` (per-content-item lookup, not consumed yet) — both added now since they're free and obviously needed by the next stage, per the same reasoning Stage 23A1 used for `idx_recommendation_feedback_course_id`.

**`backend/internal/reports/model.go`** (new package):
- `Report` struct.
- `ContentTypeQuestion`/`ContentTypeAnswer`/`ContentTypeReview` + `allowedContentTypes`.
- `StatusOpen`/`StatusResolved`/`StatusDismissed` + `allowedStatuses` (the latter two values/map entries unused by any code this session, deliberately — see above).

**`backend/internal/reports/repository.go`**:
- `ErrDuplicateActiveReport`.
- `contentTable(contentType) (string, bool)` — the validated-value-to-fixed-SQL-identifier switch.
- `ContentExists(ctx, contentType, contentID) (bool, error)`.
- `Create(ctx, reporterUserID, contentType, contentID, reason) (*Report, error)`.

**`backend/internal/reports/service.go`**:
- `ValidationError` (matches `internal/qa`/`internal/courses`/`internal/recommendations`'s identical shape).
- `ErrContentNotFound`.
- `CreateReport(ctx, reporterUserID, contentType, contentID, reason) (*Report, error)` — validates `content_type` against the whitelist and `reason` non-empty, confirms the content exists, then delegates to the repository. `reporterUserID` is a plain parameter throughout, exactly like every other domain's identity-bearing methods (`qa.CreateQuestion`, `recommendations.SubmitFeedback`) — ready for a future handler to fill it from `authctx.UserID`, with no alternate identity parameter anywhere for a caller to substitute another user's id into.

No handler, no route, no admin UI, no change to `internal/qa` or `internal/reviews`.

### Files changed

- `backend/migrations/00039_create_content_reports.sql` — new.
- `backend/internal/reports/model.go` — new.
- `backend/internal/reports/repository.go` — new.
- `backend/internal/reports/service.go` — new.
- No other file touched. `internal/qa`, `internal/reviews`, `cmd/api/main.go` all unmodified (confirmed via `git status` — only the four new files above appear).

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/reports/*.go` — clean, no output.
- `go build ./...` (whole backend) — OK. The new package compiles standalone even though nothing imports it yet (no handler wiring this session).
- `go vet ./...` — OK.

**Live DB behavior** (migration applied via `docker compose up -d --build backend`, confirmed `goose: successfully migrated database to version: 39`; schema confirmed via `\d content_reports` — all columns, both `CHECK` constraints, all three indexes including the partial unique one, and the `ON DELETE CASCADE` FK present exactly as designed). No HTTP endpoint exists yet, so — consistent with how Stage 22A1/23A1 verified their own pre-endpoint repository logic — the exact SQL each repository method issues was run directly via `psql` against real seeded content (a real Q&A question and answer; a throwaway review row created for this session, deleted afterward):

| Check | Result |
|---|---|
| Create report (question) | Inserted, `status: 'open'` |
| Duplicate active report (same reporter, same content, while still open) | **Zero rows returned** — confirms `pgx.ErrNoRows` → `ErrDuplicateActiveReport` mapping in the Go repository would fire correctly; DB-level confirmed still exactly 1 row for that (reporter, content) pair afterward |
| Fresh report allowed after resolution | Simulated resolution (`status → 'resolved'`), then the same (reporter, content) pair accepted a new report — 2 rows total afterward (1 resolved, 1 open), original row's `reason` untouched |
| Create report (answer) | Inserted, `content_type: 'answer'` |
| Create report (review) | Inserted, `content_type: 'review'`, against a throwaway review row |
| Invalid `content_type` (`'spam_flag'`) | **Rejected** — `content_reports_content_type_check` violation |
| Invalid `status` (`'super_open'`) | **Rejected** — `content_reports_status_check` violation |
| Nonexistent `reporter_user_id` | **Rejected** — `content_reports_reporter_user_id_fkey` violation |
| `ContentExists` equivalent for a nonexistent question id | `false` — confirms `Service.CreateReport`'s `ErrContentNotFound` path is reachable |
| `ContentExists` equivalent for a nonexistent review id | `false` |
| Cascade delete | Created a throwaway user, reported as them, deleted the user → their report row disappeared automatically, no separate delete needed |

All 11 checks passed on the first attempt. Test data (reports, the throwaway review, the throwaway user) cleaned up afterward — verified `SELECT count(*)` on `content_reports`, the throwaway review, and the throwaway user all return `0`.

### Not done this session (explicitly out of scope for 24A1)

- **No HTTP endpoints** — `Service.CreateReport` is reachable only from Go code today, not `POST /questions/:id/report` or any other path from the roadmap. Natural next slice (a future Stage 24A2), not attempted here.
- **No admin moderation queue/UI** — `GET /admin/reports`, `PATCH /admin/reports/:id`, and everything frontend are untouched; `allowedStatuses`' `resolved`/`dismissed` values exist but nothing writes them yet.
- **No changes to `internal/qa` or `internal/reviews`** — confirmed via `git status`; existing Q&A/review behavior (including Stage 21's hide/show) is completely unaffected.
- **No frontend** — nothing in `frontend/`.
- **No regression pass** — out of scope for this focused backend-storage session.

## Stage 24A2 — content abuse report submission endpoint (this session)

Scope: one authenticated `POST /reports` endpoint wiring Stage 24A1's `Service.CreateReport` into the public API. No admin endpoints, no frontend, no changes to Q&A/review behavior.

### Inspection performed

Re-read `STAGE24_PROGRESS.md`'s Stage 24A1 section and `backend/internal/reports/{model,repository,service}.go` fresh — the only pre-existing files this session needed to build on. Checked `internal/qa/handler.go` and `internal/recommendations/handler.go` for the established `authctx.UserID` + `ValidationError`/`errors.As`/`errors.Is` switch pattern every prior stage's handler follows, reused verbatim. Checked `backend/cmd/api/main.go` for exactly where/how sibling domains (`qa`, `recommendations`) are constructed and routed, to wire `reports` into the identical spot with the identical shape. Grepped the whole backend for existing `http.StatusConflict` usage and found `internal/reviews/handler.go`'s `REVIEW_EXISTS` → 409 ("you have already reviewed this course") — the exact precedent for how this codebase already signals "you already did this," reused for `DUPLICATE_REPORT`. Did not inspect `internal/admin`, `internal/users`, or any other unrelated domain.

### Design decision: one generic `POST /reports`, not three content-type-specific routes

The roadmap's own Stage 24 sketch proposed three separate paths (`POST /questions/:id/report`, `POST /answers/:id/report`, `POST /courses/:id/reviews/:reviewId/report`). This session's explicit instructions asked for one endpoint accepting `content_type`/`content_id`/`reason` all in the body instead — which is also the more direct fit for Stage 24A1's own polymorphic storage design (a single `Service.CreateReport(ctx, reporterUserID, contentType, contentID, reason)` that already branches on `content_type` internally). Implemented as `POST /reports` on the bare, auth-required `/api/v1` group, matching how `internal/qa`'s bare `/questions/:id` etc. and `internal/recommendations`' `/recommendations/:id/feedback` already live directly on `v1` rather than nested under a content-specific prefix.

`content_id` is bound as a plain `string` field in `createReportRequest`, not `uuid.UUID` — deliberately, so a malformed value can be caught and reported as its own distinct `400 INVALID_CONTENT_ID` after `ShouldBindJSON` succeeds, rather than binding it as `uuid.UUID` directly, which would make gin reject the *entire* request during JSON binding with one generic, less specific error, collapsing "malformed content_id" and "malformed body" into the same response.

### Change made

`backend/internal/reports/handler.go` (new):
- `RegisterRoutes(rg, requireAuth)` → `POST /reports`.
- `createReportRequest{ContentType, ContentID, Reason string}` — no `reporter_user_id`/`user_id` field of any kind.
- `CreateReport(c *gin.Context)`: `authctx.UserID` (401 if missing) → bind body (400 `INVALID_BODY` if malformed) → parse `content_id` (400 `INVALID_CONTENT_ID` if not a UUID) → `service.CreateReport`, mapping `*ValidationError` → 400 `VALIDATION_ERROR`, `ErrContentNotFound` → 404 `CONTENT_NOT_FOUND`, `ErrDuplicateActiveReport` → 409 `DUPLICATE_REPORT`, success → 201 with the full `Report` JSON.

`backend/cmd/api/main.go`:
- Added the `reports` import (alphabetical, between `recommendations` and `reviews`).
- Constructed `reportsRepo`/`reportsService`/`reportsHandler` right after the `qa` block, with a comment explaining the one-generic-endpoint design.
- `reportsHandler.RegisterRoutes(v1, authMiddleware.RequireAuth())` added alongside `qaHandler.RegisterRoutes`.

No changes to `internal/reports/{model,repository,service}.go` (all reused exactly as Stage 24A1 built them), no changes to `internal/qa` or `internal/reviews`.

### Files changed

- `backend/internal/reports/handler.go` — new.
- `backend/cmd/api/main.go` — import + construction + route registration.
- No other file touched. `internal/qa`, `internal/reviews` confirmed unmodified via `git status`.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/reports/*.go cmd/api/main.go` — clean, no output.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**Live** (Docker Compose backend rebuilt via `docker compose up -d --build backend`; `docker compose logs backend` confirmed `POST /api/v1/reports` registered with no route-conflict panic; fresh student account; a throwaway `course_reviews` row created for the `review` content-type cases, since the seed data has none):

| Case | Request | Result |
|---|---|---|
| Valid report (question) | `POST /reports {content_type:"question", content_id:<real>, reason:"..."}` | **201**, full `Report` JSON, `status:"open"` |
| Valid report (answer) | Same shape, `content_type:"answer"` | **201** |
| Valid report (review) | Same shape, `content_type:"review"`, against the throwaway review | **201** |
| Duplicate active report (same question again) | Same question, new reason text | **409** `DUPLICATE_REPORT` |
| Invalid `content_type` (`"lesson"`) | — | **400** `VALIDATION_ERROR`, "content_type must be one of: question, answer, review" |
| Invalid `content_id` (`"not-a-uuid"`) | — | **400** `INVALID_CONTENT_ID` |
| Nonexistent content (valid UUID + type, no such row) | — | **404** `CONTENT_NOT_FOUND` |
| Empty `reason` | `reason:""` | **400** `VALIDATION_ERROR`, "reason is required" |
| Whitespace-only `reason` | `reason:"   "` | **400** `VALIDATION_ERROR` (same message — confirms `strings.TrimSpace` in the service catches this too, not just a literal-empty check) |
| Unauthenticated request | no `Authorization` header | **401** `UNAUTHORIZED` |
| Malformed JSON body | `not json` | **400** `INVALID_BODY` |
| **`reporter_user_id` spoofing attempt** — real student JWT, body includes `"reporter_user_id": "<admin's real id>"`, against fresh unreported content | — | **201**; response's `reporter_user_id` field confirmed to be the **real authenticated student's id**, not the injected admin id — the spoofed field has no corresponding struct field in `createReportRequest` at all, so it's silently dropped by JSON binding before ever reaching the service |

All 12 cases (the 7 required from instruction #7, plus whitespace-reason, malformed-body, and the explicit spoofing attempt) passed on the first attempt — no code changes needed after the initial implementation. Test data (all `S24A2_TEST`-tagged reports and reviews) cleaned up afterward; confirmed `SELECT count(*)` on `content_reports` and the throwaway reviews both return `0`. Test account left in place, matching every prior stage's convention.

### Not done this session (explicitly out of scope for 24A2)

- **No admin moderation endpoints** — `GET /admin/reports`, `PATCH /admin/reports/:id` don't exist; `StatusResolved`/`StatusDismissed` remain unwritten by any code path.
- **No frontend** — no "Report" button, nothing in `frontend/`.
- **No changes to `internal/qa`/`internal/reviews`** — confirmed via `git status`; Stage 21's hide/show and the existing review-publish toggle are both completely untouched.
- **No regression pass** — out of scope for this focused endpoint session.

## Stage 24A3 — admin content-report moderation queue backend API (this session)

Scope: one admin-only `GET /admin/reports` (list, filterable, paginated) and one admin-only `PATCH /admin/reports/:id` (status change only). No content hide/show, no frontend.

### Inspection performed

Re-read `STAGE24_PROGRESS.md`'s Stage 24A1/24A2 sections and `backend/internal/reports/{model,repository,service,handler}.go` fresh. Read `internal/reviews/handler_admin.go` and its repository's `ListAdmin`/`SetPublished` in full — the closest existing precedent for exactly this shape (admin list with optional filters + pagination, admin single-field status-style update via `RETURNING`) — and reused its structure directly: same `AdminListParams`-style filter struct, same `errors.As`/`errors.Is` switch in the handler, same `ON ... WHERE ... RETURNING` update pattern. Read `pagination/pagination.go` in full to confirm `Result[T]`/`New`/`ParseParams`/`Offset` is the current shared convention ("every admin list endpoint... instead of each domain rolling its own," per its own doc comment) and used it directly rather than reviews' older hand-rolled offset math. Checked `cmd/api/main.go`'s `adminGroup` definition to confirm it already gates every route registered on it to `authMiddleware.RequireAuth() + RequireRole("admin")` — the single choke point every existing admin endpoint relies on — so non-admin/unauthenticated rejection needed no new code in this package, only correct registration onto that group. Did not inspect `internal/admin`, `internal/users`, or any other unrelated domain.

### Design decisions

- **`AdminReport` is narrower than `Report`, matching this session's explicit field list exactly**: `id`, `reporter_user_id` + `reporter_name` (joined from `users`, never email — mirrors `AdminReview.UserName`'s identical identity-exposure convention), `content_type`, `content_id`, `reason`, `status`, `created_at`. No `updated_at` in the list view (present on the single-report response from `UpdateStatus`, which returns the full `Report`).
- **No join to the reported content itself** — `content_type`/`content_id` are polymorphic (Stage 24A1's own design constraint: one column can't reference three different tables), so there is no generic way to also pull in the content's own title/body/course context in this same query. The list surfaces the identifiers; resolving what they point to is left to a future session/frontend, not attempted here.
- **`UpdateStatus` never touches the reported content.** Per this session's explicit instructions 5/6, resolving a report only ever changes its own `status` column. Hiding a question/answer or unpublishing a review remains entirely Stage 21's/`internal/reviews`' existing, separate action — `UpdateStatus`'s query only ever runs `UPDATE content_reports SET status = ...`, never touches `lesson_questions`/`question_answers`/`course_reviews`. Verified live below (published flags on both a reported question and answer confirmed unchanged after status updates).
- **Filters are optional and additive, empty means "no filter."** `status`/`content_type` query params are validated against the same whitelists `CreateReport` already uses when non-empty; an absent filter matches everything, following the exact `($1 = '' OR cr.status = $1)` idiom `courses.SearchCourses` already established for optional filters.
- **Admin authorization is not re-implemented in this package at all** — `RegisterAdminRoutes` is registered onto `cmd/api/main.go`'s existing `adminGroup`, which already carries `RequireAuth()+RequireRole("admin")` for every route mounted on it. This is exactly how every other admin endpoint in this codebase (reviews, users, courses, qa, ...) is protected, so instruction 7's "non-admin users must be rejected" is satisfied by registration, not a new check.

### Change made

`backend/internal/reports/model.go`:
- `AdminReport` struct.
- `AdminListParams{Status, ContentType, Page, Limit}`.

`backend/internal/reports/repository.go`:
- `ErrNotFound` (new — `UpdateStatus`'s not-found case; `Create`'s duplicate case already had its own distinct `ErrDuplicateActiveReport`).
- `ListAdmin(ctx, params) ([]AdminReport, int, error)` — one query, `JOIN users` for `reporter_name` only, optional status/content_type filters, `COUNT(*) OVER()` for the total, ordered newest-first, `pagination.Offset`-based paging.
- `UpdateStatus(ctx, id, status) (*Report, error)` — `UPDATE ... SET status = $2, updated_at = now() ... RETURNING`, `pgx.ErrNoRows` → `ErrNotFound`.

`backend/internal/reports/service.go`:
- `ListAdmin(ctx, params) (pagination.Result[AdminReport], error)` — validates non-empty `status`/`content_type` against the existing whitelists, delegates.
- `UpdateStatus(ctx, id, status) (*Report, error)` — validates `status`, delegates.

`backend/internal/reports/handler_admin.go` (new):
- `RegisterAdminRoutes(admin)` → `GET /reports`, `PATCH /reports/:id` (both relative to the `/admin` group, so `/admin/reports` and `/admin/reports/:id`).
- `ListReportsAdmin` — reads `status`/`content_type`/`page`/`limit` query params, calls `service.ListAdmin`, maps `*ValidationError` → 400.
- `updateStatusRequest{Status string}`, `UpdateReportStatus` — parses `:id`, binds the body, calls `service.UpdateStatus`, maps `*ValidationError` → 400, `ErrNotFound` → 404 `REPORT_NOT_FOUND`.

`backend/cmd/api/main.go`:
- `reportsHandler.RegisterAdminRoutes(adminGroup)` added alongside `qaHandler.RegisterAdminRoutes`.

No changes to `internal/qa` or `internal/reviews` — confirmed via `git status` (only `internal/reports/*` and `cmd/api/main.go` changed).

### Files changed

- `backend/internal/reports/model.go` — `AdminReport`, `AdminListParams`.
- `backend/internal/reports/repository.go` — `ErrNotFound`, `ListAdmin`, `UpdateStatus`.
- `backend/internal/reports/service.go` — `ListAdmin`, `UpdateStatus`.
- `backend/internal/reports/handler_admin.go` — new.
- `backend/cmd/api/main.go` — one new route-registration line.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/reports/*.go cmd/api/main.go` — clean, no output.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**Live** (Docker Compose backend rebuilt via `docker compose up -d --build backend`; `docker compose logs backend` confirmed all three routes — `POST /reports`, `GET /admin/reports`, `PATCH /admin/reports/:id` — registered with no panic; fresh student account + the real seeded admin account; two real reports seeded via the actual Stage 24A2 submission endpoint, one on a question, one on an answer):

| Case | Result |
|---|---|
| Admin list (no filters) | **200**, `total: 2`, both seeded reports present with `reporter_name: "S24A3 Student"` and every listed field |
| Status filter (`?status=open`) | **200**, `total: 2`, both `status: "open"` (run before any update) |
| content_type filter (`?content_type=answer`) | **200**, `total: 1`, correctly only the answer report |
| Status update → `resolved` | **200**, full `Report` JSON with `status: "resolved"`, `updated_at` advanced |
| Status update → `dismissed` (the other report) | **200**, `status: "dismissed"` |
| Invalid status (`"banned"`) | **400** `VALIDATION_ERROR`, "status must be one of: open, resolved, dismissed" |
| `status=open` filter after both reports resolved | **200**, `total: 0` — confirms the filter reflects real state, not cached |
| Nonexistent report id | **404** `REPORT_NOT_FOUND` |
| Student (non-admin) → list | **403** `FORBIDDEN` |
| Student (non-admin) → status update | **403** `FORBIDDEN` |
| Unauthenticated → list | **401** `UNAUTHORIZED` |
| Unauthenticated → status update | **401** `UNAUTHORIZED` |

All 12 cases (the 7 required from instruction #10, plus the dismissed-status variant, the post-update filter re-check, and the nonexistent-id case) passed on the first attempt — no code changes needed after the initial implementation. **Confirmed no content-hiding side effect**: `lesson_questions.published` and `question_answers.published` for the two reported items were both re-checked directly in the DB after their reports were resolved/dismissed and found unchanged (`true` throughout) — `UpdateStatus` genuinely only ever touches `content_reports`. Test data (both seeded reports) cleaned up afterward; confirmed `SELECT count(*) FROM content_reports` → `0`.

### Not done this session (explicitly out of scope for 24A3)

- **No content hide/show wiring** — resolving a report is a pure status change; an admin who wants to actually hide the underlying content must still use Stage 21's existing Q&A hide/show endpoints or the review-publish toggle separately. Not connected here, per explicit instruction.
- **No physical deletion of reported content** — confirmed live, not just by omission.
- **No frontend** — no admin reports page, nothing in `frontend/`.
- **No regression pass** — out of scope for this focused endpoint session.

## Stage 24B1 — admin content-report moderation queue frontend (this session)

Scope: an `/admin/reports` page consuming Stage 24A3's `GET`/`PATCH /admin/reports`, one new "Reports" nav item, filters, and per-row status actions with no full page reload. No "Report" button on the reporting side (Q&A/reviews) — that's a separate future slice; this session is admin-side only.

### Inspection performed

Re-read `STAGE24_PROGRESS.md`'s Stage 24A1–24A3 sections fresh. Read `app/admin/reviews/page.tsx` and its `lib/admin-api.ts`/`lib/admin-actions.ts` counterparts in full — the closest existing admin moderation page (list + filter form + status-style action) — and confirmed every action in `admin-actions.ts` without exception uses `redirect(...)` after its mutation (full page reload). Checked `app/admin/layout.tsx`'s nav array and `components/shell/icons.tsx`'s hand-rolled icon set (no npm icon library, confirmed by the file's own doc comment) for the exact precedent Stage 20B2 set when Q&A needed a new nav icon (`IconMessageCircle`, added then, reused as-is now for styling reference). Did not inspect any unrelated admin page beyond `reviews` (the one genuine precedent) and the shared layout/icon files needed to add a nav entry.

### Design decision: break from `admin-actions.ts`'s redirect convention, deliberately

Every existing action in `lib/admin-actions.ts` redirects after its mutation — a full page reload. This session's explicit instruction 8 ("refresh/update the affected item without full page reload where practical") and instruction 9 ("per-item submitting state") both require the opposite: local state that updates in place. Rather than either (a) breaking the instruction to stay consistent with every prior admin action, or (b) inventing an unrelated new pattern, this session reused the **client-component-owns-state, server-action-returns-a-plain-result** approach already established for `QAModerationSection` (Stage 20B2/21B2) and `PersonalizedRecommendations` (Stage 23B1) — just applied to `lib/admin-actions.ts` for the first time, since every prior use of that pattern lived in the non-admin `lib/actions.ts`. `updateReportStatusAction` is flagged with an explicit comment explaining why it's the one exception in its file, so a future reader isn't left wondering whether it's an oversight.

Filters (`status`/`content_type`) stay a plain `<form method="get">` full-reload submission, matching `admin/reviews`'s own filter form exactly — instruction 8's no-reload requirement was scoped to the *status action*, not filtering, and a URL-driven filter (shareable, bookmarkable, works with back button) is the better fit for that specific interaction anyway.

### Change made

`frontend/lib/admin-api.ts`:
- `AdminContentReport` interface, mirroring `backend/internal/reports/model.go`'s `AdminReport` exactly.
- `adminListReports(token, {page, limit, status, content_type})` — same shape as `adminListReviews`, hitting `GET /admin/reports`.

`frontend/lib/admin-actions.ts`:
- `UpdateReportStatusResult`, `updateReportStatusAction(reportId, status)` — `PATCH`s `/admin/reports/:id`, returns `{ok, error?}` instead of redirecting (see design decision above).

`frontend/components/shell/icons.tsx`:
- `IconFlag` — new hand-rolled stroke-SVG icon (pole + notched pennant), same `base()`-wrapped shape every existing icon uses.

`frontend/app/admin/layout.tsx`:
- Added `IconFlag` to the icon imports and one new nav item, `{ href: "/admin/reports", label: "Reports", icon: <IconFlag size={18} /> }`, in the "Сообщество" group right after "Q&A" — the same group and adjacency Stage 20B2 used when it added the Q&A item after Reviews.

`frontend/components/AdminReportsQueue.tsx` (new, `"use client"`):
- Owns the reports array as local state, initialized from the server-fetched `initialReports` prop.
- Per-row status badge (`STATUS_LABEL`, Russian) and up to two transition buttons per row (`TRANSITIONS` — the two statuses the row is *not* currently in; never a redundant "mark open as open" button).
- `handleStatusChange`: per-report pending tracking (`pendingId`) and per-report error tracking (`errors`), calls `updateReportStatusAction`, and on success **updates only that row's `status` field in local state** — the row stays visible, never removed, satisfying "do not hide/delete the reported content from this page" at the report-row level too (a resolved/dismissed report remains in the list with its new status, not hidden).
- Empty state (`.empty-state`, "Жалоб не найдено.") when the list is empty.
- Reuses existing classes only: `.admin-table-wrapper`/`.admin-table` (from `reviews`), `.badge` (project-wide), `.btn-small` (project-wide), `.qa-moderation-actions` (Stage 21B2's purely structural `display:flex; gap` wrapper, reused here for the button row rather than adding new CSS) — **zero new CSS added this session**.

`frontend/app/admin/reports/page.tsx` (new):
- Server component: session gate (redirect to `/login` if no token, matching every other admin page), reads `status`/`content_type`/`page` from `searchParams`.
- `adminListReports` wrapped in `try/catch` → `loadError` state with a `role="alert"` fallback, matching `app/admin/questions/page.tsx`'s (Stage 20B2) established error-handling convention rather than `reviews`' let-it-throw approach — chosen because this session's own instruction 9 explicitly requires an "error state."
- Filter `<form method="get">` (status/content_type `<select>`s + submit), `<AdminReportsQueue>`, and pagination controls (page/total_pages/total, Prev/Next links preserving both filters) — same shape as `admin/reviews`'s own pagination block.

### Files changed

- `frontend/lib/admin-api.ts` — `AdminContentReport`, `adminListReports`.
- `frontend/lib/admin-actions.ts` — `UpdateReportStatusResult`, `updateReportStatusAction`.
- `frontend/components/shell/icons.tsx` — `IconFlag`.
- `frontend/app/admin/layout.tsx` — nav item + icon import.
- `frontend/components/AdminReportsQueue.tsx` — new.
- `frontend/app/admin/reports/page.tsx` — new.
- No backend files touched. No other admin page touched — `admin/reviews`, `admin/questions`, etc. were read for reference, not modified.

### Verification performed (focused, per scope)

- `npx tsc --noEmit` (whole project) — clean.
- `npx eslint components/AdminReportsQueue.tsx app/admin/reports/page.tsx app/admin/layout.tsx lib/admin-api.ts lib/admin-actions.ts components/shell/icons.tsx` — clean, zero warnings.
- `npx eslint .` (whole project) — **0 errors**, still exactly the same 4 pre-existing `<img>` warnings every prior stage has reported, nothing new.

No live browser interaction, no Docker Compose rebuild, no E2E — explicitly out of scope for 24B1. The backend endpoints this page calls were already live-verified end-to-end in Stage 24A3 (list, both filters, status update to every value, invalid status, non-admin/unauthenticated rejection, nonexistent report id); this session's job was limited to confirming the frontend calls them correctly and renders/updates as specified, which `tsc`'s type-checking of `AdminContentReport`/`updateReportStatusAction` against the component wiring plus the code-level review above covers for this focused pass.

### Not done this session (explicitly out of scope for 24B1)

- **No "Report" button on Q&A/reviews** — this session is the admin moderation-queue side only; the reporting side (a control on `QASection.tsx` and reviews consuming Stage 24A2's `POST /reports`) is a separate, not-yet-started slice.
- **No content hide/show wired to report resolution** — clicking "Решить"/"Отклонить" only ever changes the report's own status, matching the backend's identical Stage 24A3 boundary; an admin who wants to actually hide the underlying content still does so separately via `/admin/questions` or `/admin/reviews`.
- **No live browser interaction** — same standing limitation noted in Stages 21C/22C/23C's own sections; no browser-automation tool is available in this environment.
- **No regression pass** — out of scope for this focused frontend session.

## Stage 24C — focused verification + final Stage 24 report (this session)

Scope: verify 24A1–24A3 (backend) and 24B1 (admin frontend) live together, fix only bugs the verification itself surfaced, close out Stage 24 for the scope actually built (submission + admin moderation; the reporting-side "Report" button on Q&A/reviews was never started — see Known limitations). No new features attempted.

### Setup

Rebuilt both `backend` and `frontend` (`docker compose up -d --build backend frontend`) — frontend was still serving the pre-24B1 image going into this session. Confirmed both healthy (`GET /api/v1/health` → 200, `GET /admin/reports` unauthenticated → 307) with no route-conflict panic in `docker compose logs backend`; all three routes (`POST /reports`, `GET /admin/reports`, `PATCH /admin/reports/:id`) confirmed registered. Registered a fresh student account; used the seeded admin account. Created throwaway `course_reviews` rows as needed (seed data has none), cleaned up afterward.

### Bugs found

**None.** Every check below passed on the first attempt; no code changes were made this session. (Contrast Stage 22C, which caught a real Escape-key bug during its own fresh code re-review — this session's equivalent re-review of `AdminReportsQueue.tsx` and `app/admin/reports/page.tsx` found the row-scoping, failure-path, and error-handling logic all correct as originally written.)

### 1. Report submission — verified live

| Case | Result |
|---|---|
| Report a question | **201**, `status: "open"` |
| Report an answer | **201** |
| Report a review (throwaway row, seed data has none) | **201** |
| Duplicate active report (same question again) | **409** `DUPLICATE_REPORT` |
| Invalid `content_type` (`"comment"`) | **400** `VALIDATION_ERROR` |
| Nonexistent content (valid type/UUID, no row) | **404** `CONTENT_NOT_FOUND` |
| Invalid `content_id` (not a UUID) | **400** `INVALID_CONTENT_ID` |
| Unauthenticated request | **401** `UNAUTHORIZED` |
| **`reporter_user_id` spoofing attempt** — student JWT, body includes `"reporter_user_id": "<admin's real id>"`, against fresh unreported content | **201**; response's `reporter_user_id` confirmed to be the real authenticated student's id, never the injected admin id |

All 9 cases (the 8 required plus the explicit spoofing attempt) passed.

### 2. Admin moderation API — verified live

| Case | Result |
|---|---|
| Admin list (no filters) | **200**, all 4 seeded test reports present with every listed field |
| Status filter (`?status=open`) | **200**, all returned rows `status: "open"` |
| content_type filter (`?content_type=review`) | **200**, all returned rows `content_type: "review"`, count correct |
| Pagination (`?limit=1&page=1` then `&page=2`) | **200** both, each page returns exactly 1 item, `total_pages` reflects the real total |
| Set status → `resolved` | **200**, `status: "resolved"`, `updated_at` advanced |
| Set status → `dismissed` | **200**, `status: "dismissed"` |
| Invalid status (`"banned"`) | **400** `VALIDATION_ERROR` |
| Student (non-admin) → list | **403** `FORBIDDEN` |
| Student (non-admin) → status update | **403** `FORBIDDEN` |
| Unauthenticated → list and → status update | **401** `UNAUTHORIZED` (both) |
| **Status update does not hide/delete reported content** | Re-checked `lesson_questions.published` and `question_answers.published` for the two reported items directly in the DB after their reports were resolved/dismissed — both still `true`, confirming `UpdateStatus` genuinely only ever touches `content_reports` |

All 11 cases passed.

### 3. Admin UI — verified via SSR + code review (no browser-automation tool in this environment, same standing limitation as Stages 21C/22C/23C)

- **`/admin/reports` loads**: SSR-fetched with a real admin session cookie (`Cookie: lms_session=<jwt>`, the technique established in Stage 21C) — **200**, "Жалобы" heading present, both filter `<select>`s (`name="status"`, `name="content_type"`) present.
- **Report rows render correctly**: all 4 seeded reports' reason text found in the rendered HTML; status badges counted (1 "Решена", 1 "Отклонена", 2 "Открыта") exactly matching the 4 seeded reports' real statuses at that point.
- **Status action buttons match the `TRANSITIONS` table for every status**: counted 3× "Решить", 3× "Отклонить", 2× "Открыть заново" across the page — the exact counts `TRANSITIONS[open]=[Решить,Отклонить]` × 2 rows + `TRANSITIONS[resolved]=[Открыть заново,Отклонить]` × 1 row + `TRANSITIONS[dismissed]=[Открыть заново,Решить]` × 1 row predicts, confirming no row shows a same-status no-op button and every row shows exactly its two valid transitions.
- **Filters work live on the real page**: `?status=resolved` → only the resolved report's text present, exactly 1 "Решена" badge; `?content_type=answer` → only the answer report's text present.
- **Empty state is safe**: `?content_type=review&status=resolved` (a real combination with zero matches) → **200**, "Жалоб не найдено" rendered, no server error text.
- **Error state is safe, and verified against a real backend rejection, not a fabricated one**: `?status=bogus_status` (bypassing the `<select>`'s own constrained options via a raw URL param) → the backend correctly returned `400 VALIDATION_ERROR` for that upstream call, `adminListReports` threw, and the page's `try/catch` caught it into `loadError`, rendering "Не удалось загрузить жалобы" with **200** on the page itself and no crash.
- **Non-admin cannot reach the page**: student session cookie against `/admin/reports` → **307** (the existing layout-level role gate, untouched by this stage).
- **Client-side loading/pending state and the actual click-driven per-row update**: verified by fresh code re-review only, not a driven browser interaction — `handleStatusChange`'s `.map((r) => (r.id === reportId ? {...} : r))` scoping was re-confirmed correct (updates exactly the clicked row, never others), and the failure branch `return`s before ever calling `setReports`, so a failed request cannot appear to succeed. No bug found on this re-review.

### Final checks

- `gofmt -l .` (whole backend) — clean.
- `go build ./...` — OK.
- `go vet ./...` — OK.
- `npx tsc --noEmit` (whole frontend) — clean.
- `npx eslint .` (whole frontend) — 0 errors, same 4 pre-existing unrelated `<img>` warnings every prior stage has reported, nothing new.
- Docker Compose: both `backend` and `frontend` rebuilt and live; all three report-related routes confirmed registered with no panic.

### Files changed (all of Stage 24: 24A1 + 24A2 + 24A3 + 24B1 + 24C)

Backend:
- `backend/migrations/00039_create_content_reports.sql` — new table (24A1).
- `backend/internal/reports/model.go` — `Report`, vocabularies, `AdminReport`, `AdminListParams` (24A1, 24A3).
- `backend/internal/reports/repository.go` — `ContentExists`, `Create`, `ErrDuplicateActiveReport` (24A1); `ErrNotFound`, `ListAdmin`, `UpdateStatus` (24A3).
- `backend/internal/reports/service.go` — `CreateReport` (24A1); `ListAdmin`, `UpdateStatus` (24A3).
- `backend/internal/reports/handler.go` — `POST /reports` (24A2).
- `backend/internal/reports/handler_admin.go` — `GET`/`PATCH /admin/reports` (24A3).
- `backend/cmd/api/main.go` — construction + route registration (24A2, 24A3).

Frontend:
- `frontend/lib/admin-api.ts` — `AdminContentReport`, `adminListReports` (24B1).
- `frontend/lib/admin-actions.ts` — `updateReportStatusAction` (24B1).
- `frontend/components/shell/icons.tsx` — `IconFlag` (24B1).
- `frontend/app/admin/layout.tsx` — nav item (24B1).
- `frontend/components/AdminReportsQueue.tsx` — new (24B1).
- `frontend/app/admin/reports/page.tsx` — new (24B1).

No file changed this session (24C) — verification only, zero bugs found.

### Known limitations (Stage 24, final)

- **The reporting-side UI was never built.** There is no "Report" button anywhere in the student-facing app (`QASection.tsx`, review display) — `POST /reports` (24A2) is fully implemented and live-verified, but only reachable via direct API calls, not through any UI a real student would use. This is the single largest gap in Stage 24 as it stands: the admin queue works end-to-end, but nothing feeds it in normal use. Explicitly out of scope for every session so far (24B1 was scoped to "admin content-report moderation queue frontend" only); a future Stage 24B2 is the natural next slice.
- **No real browser interaction anywhere in Stage 24.** Every purely-client-side-JS-runtime behavior (actual click dispatch, actual loading-state transition) was verified by code inspection plus the strongest available live substitutes (SSR HTML, direct API reproduction, a real-backend-rejection-driven error-state test) rather than an actual driven interaction — no browser-automation tool is available in this environment. Same standing limitation as Stages 21C/22C/23C.
- **No content hide/show wired to report resolution** — deliberate, per every session's explicit instructions. An admin who resolves a report still separately visits `/admin/questions` or `/admin/reviews` to actually hide the underlying content if warranted.
- No automated test suite exists anywhere in this codebase (unchanged, project-wide convention) — every verification claim above is a live, scripted check against the running Docker Compose stack, or a documented code-level review substituting for one where live interaction wasn't reachable.

### Final Stage 24 status

**Stage 24 is complete for the scope actually built: content abuse report storage (24A1), submission (24A2), and admin moderation (24A3 backend + 24B1 frontend).** All four layers are implemented and live-verified together this session — submission (including an explicit identity-spoofing attempt), the admin API (list/filter/paginate/update/reject), and the admin UI (SSR-rendered, filtered, and safely handling empty/error states against real backend responses) — with **zero bugs found** in this session's own fresh pass. Stage 24's original roadmap scope also included a student-facing "Report" control on Q&A/reviews, which was never attempted in any session and remains the clear, explicitly-documented next step — Stage 24 is closed out here as "moderation queue complete, reporting-entry-point not started," not as fully covering the roadmap's original goal.
