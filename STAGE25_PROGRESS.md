# Stage 25 — Platform audit log

Tracking doc — status only, not a spec restatement.

## Stage 25A1 — platform audit log storage (this session)

Scope: minimal persistent storage for platform audit events, migration + model + repository + service only, per this session's explicit instructions. No handler, no admin endpoint, no wiring into any existing domain's mutations, no frontend.

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 25 section fresh. Re-read `backend/internal/reports/{model,repository,service}.go` (Stage 24A1) — the most recent, closest-shaped prior work: another storage-only session building a new domain ahead of any HTTP surface, with its own bounded-vocabulary design decisions to compare against. Read `backend/internal/notifications/repository.go`'s `Enqueue` in full for the established `json.Marshal` → `[]byte` → jsonb column idiom (the only existing JSON-column pattern in this codebase) and reused it verbatim for `metadata`. Checked `backend/internal/users/handler_admin.go` just enough to confirm `UpdateUserAdmin` (the future first real call site for a `role_changed` event) exists and where it lives, without reading or modifying it. Confirmed the `roles` table's actual vocabulary (`student`, `instructor`, `admin` — a genuinely fixed, small set already hardcoded throughout `internal/auth/middleware.go`'s `RequireRole`/`RequireAnyRole` calls) to inform (but not enforce via CHECK) `actor_role`. Did not inspect any unrelated frontend.

### Design decisions

- **Field names follow this session's explicit instructions, not the roadmap's literal sketch.** The roadmap proposed `audit_logs(id, actor_id, action, entity_type, entity_id, metadata, created_at)`; this session's instructions specify `actor_user_id` (not `actor_id`) and add `actor_role` — both followed exactly, since explicit session instructions take precedence over the roadmap's own planning-stage sketch.
- **No `updated_at` column, no Update/Delete repository method, anywhere.** An audit log is append-only by construction — the roadmap's own security requirement ("no update/delete endpoint... append-only by construction") is enforced here one layer below where a future HTTP surface would live, not only there. There is structurally no way to mutate an existing row through this package.
- **`actor_user_id` is nullable with `ON DELETE SET NULL`, not `CASCADE`.** The first deliberate deviation from this codebase's otherwise-universal FK convention (every other domain cascades). An accountability trail must outlive the account it describes — deleting a user should anonymize the row's actor reference, never silently erase the record of what they did. This directly satisfies instruction 4 ("allow NULL only for legitimate system-generated events if needed"): NULL means either a genuine system event (no human actor ever existed) or a human actor whose account was later deleted; both are legitimate, and the schema can't distinguish them from `actor_user_id` alone (nor does it need to — the row's own `metadata`/`action`/timestamp already carry the real context).
- **`action`/`entity_type` are plain `TEXT NOT NULL`, not a `CHECK`-constrained enum** — a deliberate contrast with `internal/reports`' `content_type`/`status` (Stage 24A1), which *are* CHECK-constrained. The reasoning is the opposite of that stage's: reports' vocabulary is genuinely fixed and small (3 content types, 3 statuses) and unlikely to ever grow; audit actions/entity types are inherently open-ended — nearly every future stage that instruments a new mutation will add a new action value, and a closed enum would require a migration for each one, working directly against this session's own instruction 3 ("keep the model extensible enough for future stages"). Both are still `NOT NULL`: every event must at least categorize itself, even when there's no single specific instance (`entity_id` is nullable for that case).
- **A documented starting vocabulary, not an enforced whitelist.** `model.go` defines `ActionRoleChanged`, `ActionContentHidden`, `ActionReportResolved`, `ActionCertificateIssued`, etc. — anticipated from the roadmap's own Stage 25 goal — purely so future call sites reuse consistent string values (important for a queryable log) without anything in this package rejecting an unlisted value. This is the literal reading of instruction 3's "constrained... where practical, but keep the model extensible": constrained in the sense of *documented, consistent naming*, not constrained in the sense of *rejected if unrecognized*.
- **`Service.Log` validates structure only** (non-empty after trim, length-capped) — never membership. `LogInput.ActorRole` is taken as a trusted Go value (meant to be sourced from `authctx.Role` by whichever future handler calls this), not re-validated against the role vocabulary the way a public HTTP endpoint would validate raw user input — this package is never reached from an HTTP request body directly, only from other Go code that already has a verified identity in hand.
- **Metadata secrecy discipline is a call-site responsibility, documented, not enforced in code.** `Service.Log` cannot inspect an arbitrary `map[string]any` and know whether a given key holds a password or token. The roadmap's own security requirement ("explicit review of every call site to confirm metadata never captures a password, token, or payment secret") is restated in `Service.Log`'s doc comment as a standing obligation for Stage 25A2+'s real call sites, not something this session's code can guarantee by itself.

### Change made

**Migration** `backend/migrations/00040_create_audit_logs.sql`:
- `audit_logs(id, actor_user_id, actor_role, action, entity_type, entity_id, metadata, created_at)`.
- `actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL` (nullable).
- `action TEXT NOT NULL`, `entity_type TEXT NOT NULL`, `entity_id UUID` (nullable), `metadata JSONB` (nullable).
- `idx_audit_logs_entity (entity_type, entity_id)`, `idx_audit_logs_actor_created (actor_user_id, created_at)` — exactly the roadmap's own stated index plan.

**`backend/internal/audit/model.go`** (new package):
- `AuditLog` struct.
- `Action*`/`EntityType*` documented constants (see design decisions above).

**`backend/internal/audit/repository.go`**:
- `Create(ctx, entry AuditLog) (*AuditLog, error)` — `json.Marshal` metadata to `[]byte` before insert, `json.Unmarshal` back after `RETURNING`, mirroring `notifications.Enqueue`'s exact idiom.

**`backend/internal/audit/service.go`**:
- `ErrActionRequired`, `ErrEntityTypeRequired`.
- `LogInput` struct.
- `Log(ctx, input LogInput) (*AuditLog, error)` — the one internal service method requested (instruction 6): validates `action`/`entity_type` non-empty and length-bounded, delegates to the repository.

No handler, no route, no wiring into `internal/users`, `internal/qa`, `internal/reviews`, `internal/reports`, or `internal/subscriptions`. `cmd/api/main.go` untouched — there is nothing to construct or register yet, since no handler exists.

### Files changed

- `backend/migrations/00040_create_audit_logs.sql` — new.
- `backend/internal/audit/model.go` — new.
- `backend/internal/audit/repository.go` — new.
- `backend/internal/audit/service.go` — new.
- No other file touched. Confirmed via `git status` — no existing domain's handler/service/repository changed.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/audit/*.go` — clean, no output.
- `go build ./...` (whole backend) — OK. The new package compiles standalone even though nothing imports it yet (no handler wiring this session, matching instructions 7/8).
- `go vet ./...` — OK.

**Live DB behavior.** Migration applied via `docker compose up -d --build backend`, confirmed `goose: successfully migrated database to version: 40`; schema confirmed via `\d audit_logs` — all columns, both indexes, and the `ON DELETE SET NULL` FK (not `CASCADE`) present exactly as designed.

No HTTP endpoint exists yet to drive `Service.Log` through, and unlike a pure-SQL repository method, its validation logic lives in Go — so a raw `psql` mirror (the technique used for `internal/reports`/`internal/recommendations` in earlier storage-only sessions) can't exercise it. Instead, a **temporary** `internal/audit/verify_scratch_test.go` was added, run once via `go test` against the real live Docker Compose Postgres (`AUDIT_VERIFY_DSN=postgres://lms:lms_password@localhost:5434/lms?sslmode=disable go test ./internal/audit/... -run TestScratchVerifyStage25A1 -v`), and **deleted immediately after** — consistent with this codebase's "no automated test suite, live scripted verification instead" convention (confirmed again this session: `find backend -name "*_test.go"` returns nothing, before or after). Its own doc comment stated this intent up front; it was not left behind.

| Case | Result |
|---|---|
| Valid event, real actor, real entity, metadata with nested keys | **Created** — `Action`, `ActorUserID` correct; `Metadata["old_role"]`/`Metadata["new_role"]` round-tripped exactly (JSON → `[]byte` → jsonb → `[]byte` → JSON, no data loss) |
| Nil actor for a system-generated event | **Created** — `ActorUserID` and `EntityID` both correctly `nil` in the returned struct |
| Empty (whitespace-only) `action` | **Rejected** — `ErrActionRequired` |
| Empty (whitespace-only) `entity_type` | **Rejected** — `ErrEntityTypeRequired` |
| Nonexistent `actor_user_id` (a real, unassigned UUID) | **Rejected** — real Postgres `23503` foreign-key-violation surfaced through the repository, confirming the FK is live and enforced, not just present in the migration file |
| `ON DELETE SET NULL` — create a throwaway user, log an event as them, delete the user | **Row survives**, `actor_user_id` re-selected from the DB and confirmed `NULL` afterward — the deviation from every other domain's `CASCADE` convention behaves exactly as designed |

All 6 checks passed on the first attempt. Every subtest cleaned up its own rows inline (`DELETE FROM audit_logs`/`DELETE FROM users` for anything it created); confirmed `SELECT count(*) FROM audit_logs` and the throwaway-user query both return `0` after the full run. The verification file itself was deleted immediately after (`rm internal/audit/verify_scratch_test.go`), and `gofmt`/`go build`/`go vet` were re-run clean afterward to confirm its removal left the package exactly as it should be.

### Not done this session (explicitly out of scope for 25A1)

- **No wiring into any existing handler.** `internal/users`' role-change path, Stage 21's Q&A hide/show, Stage 24's report-resolve, certificate issuance, subscription/payment overrides — none of them call `Service.Log` yet. `internal/audit` is reachable only from Go code that explicitly imports it, and nothing does.
- **No admin audit-log endpoints.** `GET /admin/audit-log` doesn't exist; there is no way to read this table back over HTTP at all yet.
- **No frontend.**
- **No regression pass** — out of scope for this focused backend-storage session.

## Stage 25A2 — audit two critical admin actions (this session)

Scope: wire `internal/audit`'s `Service.Log` into exactly two existing call sites — Stage 24's content-report status update and Stage 21's Q&A hide/show moderation. No new audit call sites beyond these two, no admin read endpoints, no frontend.

### Inspection performed

Re-read `STAGE25_PROGRESS.md`'s Stage 25A1 section and `backend/internal/audit/{model,repository,service}.go` fresh. Read `backend/internal/reports/{service,handler_admin}.go` and `backend/internal/qa/{service,handler}.go` in full — the only two existing call sites this session touches. Confirmed both `qa.SetQuestionPublished`/`SetAnswerPublished` already receive `userID`/`role` as parameters (sourced from `authctx.UserID`/`authctx.Role` in `handler.go`'s shared `currentUser` helper) for their existing `ownership.CanManageCourse` authorization check — so no new identity-extraction code was needed there, only reusing what already existed. Confirmed `reports.UpdateStatus`/`UpdateReportStatus` had **no** actor parameter at all before this session (the `/admin` route group's middleware already enforces admin-only access, so the handler never previously needed to read identity for any reason) — this was the one place actor extraction genuinely needed to be added. Did not inspect any other domain.

### Design decisions

- **Audit calls happen after the real mutation's repository call has already succeeded, never before, and never inside the same transaction.** Both `qa.SetQuestionPublished`/`SetAnswerPublished` and `reports.UpdateStatus` call `s.repo.Set...`/`s.repo.UpdateStatus` first; only on a nil error does each call its own logging helper. A failed mutation (403, 404, or a genuine DB error) never reaches the audit call at all — this directly satisfies instructions 4/5 by construction, not by an extra conditional.
- **A failed audit write is logged via `log.Printf` (the established "best-effort, log but don't fail" idiom this codebase already uses in `notifications/worker.go` and `videos/worker.go`) and never propagated to the caller.** The real moderation/status-change action has already committed by the time `Service.Log` runs; if the audit insert itself fails, the function returns the already-successful mutation result unchanged. This matches the roadmap's own stated (for a "future" stage) requirement — "logging failures never block the underlying action" — applied now, at the moment these first two call sites are wired, rather than retrofitted later.
- **Actor identity is never re-derived or trusted from a request body.** For `qa`, the existing `userID`/`role` already used for authorization are reused verbatim for the audit actor — literally the same Go variables, not a second read. For `reports`, `UpdateReportStatus` now reads `authctx.UserID`/`authctx.Role` the same way every other authenticated handler in this codebase does; `updateStatusRequest` has no identity field of any kind for a client to populate. Neither handler's authorization behavior changed — the `/admin` group's middleware and `ownership.CanManageCourse` are exactly as they were.
- **One new action constant, `audit.ActionReportReopened`.** Stage 25A1 anticipated `ActionReportResolved`/`ActionReportDismissed` but not the third real transition (`open`, i.e. reopening a previously resolved/dismissed report) that Stage 24B1's frontend UI already exercises via all three `TRANSITIONS`. Adding it completes the vocabulary for the domain being wired this session — not "auditing an additional domain."
- **Metadata kept minimal per instruction 2.** Q&A hide/show: `{"course_id": ..., "published": <bool>}` — enough to know which course and the resulting state, without re-fetching the question/answer body into the log. Report status update: `{"new_status": ..., "content_type": ..., "content_id": ...}` — the new status plus enough about the underlying reported content to interpret the entry without a join (impossible anyway, since `content_reports.content_id` is polymorphic). Neither includes anything resembling a password, token, or payment secret — there is none in scope for either action.

### Change made

`backend/internal/audit/model.go`:
- Added `ActionReportReopened = "report_reopened"`.

`backend/internal/qa/service.go`:
- `Service` gained an `audit *audit.Service` field; `NewService` gained an `auditService *audit.Service` parameter.
- `SetQuestionPublished`/`SetAnswerPublished`: after their repository call succeeds, call new helper `logPublishedChange`, then return the already-obtained result.
- New `logPublishedChange(ctx, actorUserID, actorRole, entityType, entityID, courseID, published)`: picks `audit.ActionContentHidden`/`audit.ActionContentShown` from `published`, calls `s.audit.Log`, `log.Printf`s (never returns) any error.

`backend/internal/reports/service.go`:
- `Service` gained an `audit *audit.Service` field; `NewService` gained an `auditService *audit.Service` parameter.
- `UpdateStatus` gained `actorUserID uuid.UUID, actorRole string` as its first two parameters (before `id`/`status`). After a successful repository update, calls new helper `logStatusUpdate`, then returns the already-obtained result.
- New `logStatusUpdate(ctx, actorUserID, actorRole, report)`: picks `ActionReportResolved`/`ActionReportDismissed`/`ActionReportReopened` from `report.Status`, calls `s.audit.Log` with the metadata described above, `log.Printf`s (never returns) any error.

`backend/internal/reports/handler_admin.go`:
- `UpdateReportStatus` now reads `authctx.UserID`/`authctx.Role` first (mirroring every other authenticated handler's pattern) and passes both through to `service.UpdateStatus`.

`backend/cmd/api/main.go`:
- Constructed `auditRepo`/`auditService` (`internal/audit` was previously never referenced from `main.go` at all — Stage 25A1 built the package with no handler and nothing wiring it in).
- `qa.NewService(qaRepo, ownershipService, auditService)` and `reports.NewService(reportsRepo, auditService)` — both updated call sites, both now depend on `auditService`.

No changes to any other domain, no new routes, no admin read endpoint, no frontend file touched.

### Files changed

- `backend/internal/audit/model.go` — one new constant.
- `backend/internal/qa/service.go` — `audit` dependency, `logPublishedChange`, both `Set*Published` methods call it.
- `backend/internal/reports/service.go` — `audit` dependency, `UpdateStatus`'s new actor parameters, `logStatusUpdate`.
- `backend/internal/reports/handler_admin.go` — `UpdateReportStatus` reads and forwards actor identity.
- `backend/cmd/api/main.go` — `audit` import, construction, both `NewService` calls updated.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/audit/*.go internal/qa/*.go internal/reports/*.go cmd/api/main.go` — clean, no output.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**Live** (Docker Compose backend rebuilt via `docker compose up -d --build backend`, no panic; fresh student + two instructor accounts (A owning "Go Backend Developer", B owning "PostgreSQL" — a genuine cross-instructor setup) + the seeded admin; a real question+answer created via the live API; `audit_logs` confirmed empty before this session's testing began):

| Case | Result |
|---|---|
| Non-owning instructor B attempts to hide the question | **403** `FORBIDDEN` — `audit_logs` count stayed **0** |
| Student attempts to hide the question (blocked by role middleware) | **403** — `audit_logs` count stayed **0** |
| Owning instructor A hides the question | **200** — `audit_logs` count → **1**: `actor_user_id`=instructor A's real id, `actor_role`="instructor", `action`="content_hidden", `entity_type`="question", `entity_id`=the real question id, `metadata`=`{"course_id":"1111...","published":false}` — every field exact |
| Owning instructor A shows the question again | **200** — new row, `action`="content_shown", `published:true` |
| Owning instructor A hides the answer | **200** — new row, `entity_type`="answer", correct `entity_id` |
| **Admin** shows the answer again, via `/admin/qa/answers/:id`, on a course the admin does **not** personally own | **200** — new row, `actor_role`="admin", `actor_user_id`=the real admin id — confirms the admin-bypass path records the correct actor, not a fabricated one |
| Nonexistent question id (genuine 404, not an authorization failure) | **404** `QUESTION_NOT_FOUND` — `audit_logs` count unchanged |
| Student submits a report (not itself audited — out of scope) | **201** — `audit_logs` count unchanged (confirms only the *status update* call site is wired, exactly as instructed) |
| Student attempts the report status update | **403** — `audit_logs` count unchanged |
| Admin resolves the report | **200** — new row: `actor_role`="admin", `action`="report_resolved", `entity_type`="report", `entity_id`=the report's own id, `metadata`=`{"content_type":"answer","content_id":"...","new_status":"resolved"}` — exact |
| Admin submits an invalid status (`"banned"`) | **400** `VALIDATION_ERROR` — `audit_logs` count unchanged |
| Admin reopens the report (`resolved` → `open`) | **200** — new row, `action`="report_reopened" |
| Admin dismisses the report (`open` → `dismissed`) | **200** — new row, `action`="report_dismissed" |
| Nonexistent report id (genuine 404) | **404** `REPORT_NOT_FOUND` — `audit_logs` count unchanged |

All required scenarios passed on the first attempt — no code changes needed after the initial implementation. Every rejected/failed attempt (403 from non-ownership, 403 from role middleware, 404 from a nonexistent entity, 400 from an invalid status) was independently confirmed to add zero rows to `audit_logs`, and every successful action added exactly one row with the correct actor identity, action, entity reference, and metadata.

### Cleanup

Test question deleted via the real `DELETE /questions/:id` endpoint (204, cascading its answer). Test report deleted directly (no delete endpoint exists for reports; `content_reports` has no FK to `lesson_questions`/`question_answers` by Stage 24A1's own polymorphic design, so it doesn't cascade from the question's deletion). All 7 `audit_logs` rows this session produced were deleted directly — every row in the table was confirmed to be this session's own test data (`audit_logs` was empty before testing began). Both courses' `instructor_id` reverted to `NULL`. Verified via direct DB query afterward: `audit_logs`, `content_reports`, and the tagged question all return `0`. Test accounts (student, instructor A, instructor B) left in place, matching every prior stage's convention.

### Not done this session (explicitly out of scope for 25A2)

- **No other domains audited** — role changes (`internal/users`), certificate issuance, subscription/payment overrides remain unwired, exactly as instructed.
- **No admin audit-log read endpoint** — `GET /admin/audit-log` doesn't exist; the only way to see these rows is the direct DB queries used for this session's own verification.
- **No frontend.**
- **No regression pass** — out of scope for this focused wiring session.

## Stage 25A3 — admin audit-log read API (this session)

Scope: one admin-only `GET /admin/audit-log` endpoint (list, filterable, paginated) reading Stage 25A1's storage and Stage 25A2's two real call sites. No new audit-writing call sites, no frontend.

### Inspection performed

Re-read `STAGE25_PROGRESS.md`'s Stage 25A1/25A2 sections and `backend/internal/audit/{model,repository,service}.go` fresh. Read `backend/internal/reports/{model,repository,service,handler_admin}.go`'s `AdminReport`/`AdminListParams`/`ListAdmin`/`ListReportsAdmin` in full — the closest and most recent existing admin list pattern (optional filters + `pagination.Result`, a `JOIN users` for a display name) — and reused its exact shape. One deliberate difference identified before writing any code: `content_reports.reporter_user_id` is `NOT NULL` (an `INNER JOIN` to `users` is always safe there), but `audit_logs.actor_user_id` is nullable by design (Stage 25A1's `ON DELETE SET NULL` decision) — so the new query needed a `LEFT JOIN`, not an inner join, or a system-generated/actor-account-deleted row would silently vanish from the list instead of appearing with a null actor name. Did not inspect any other domain.

### Design decisions

- **`LEFT JOIN users`, not `INNER JOIN`** — the one structural difference from `reports.ListAdmin`'s otherwise-identical query shape, required by `actor_user_id` being nullable. `TRIM(u.first_name || ' ' || u.last_name)` naturally evaluates to `NULL` when the joined `u` row doesn't exist (no matching id, or `actor_user_id` itself is `NULL`), which scans cleanly into `AdminAuditLog.ActorName`'s `*string` — no special-casing needed in Go.
- **`actor_name` included, matching instruction 3** ("if it can be joined cleanly") — it can, via the same one-line `TRIM(...)` join reused verbatim from `AdminReview`/`AdminReport`'s identical convention. `email` is never selected, satisfying instruction 4 directly (there's no `email` column in the query at all, not just omitted from the JSON tags).
- **No filter is validated against a whitelist.** `action`/`entity_type` were already established in Stage 25A1/25A2 as deliberately open-ended, never whitelist-enforced anywhere in this package; extending that same reasoning, `actor_role` filtering also just narrows the `WHERE` clause — an unrecognized or misspelled filter value naturally yields zero matching rows (safe, standard filter behavior), not a 400. This keeps the read side consistent with the write side's own stated design philosophy rather than introducing a new, inconsistent validation rule only on the list endpoint.
- **Read-only, by construction, not just by omission.** `handler_admin.go`'s own doc comment states directly that `Service.Log` remains the only way any row is ever written — there is no route, method, or code path in this package that could mutate or delete a row once inserted, matching the roadmap's "no update/delete endpoint" requirement at the same schema-level enforcement Stage 25A1 already established.

### Change made

`backend/internal/audit/model.go`:
- `AdminAuditLog` struct — `ID`, `ActorUserID`, `ActorName`, `ActorRole`, `Action`, `EntityType`, `EntityID`, `Metadata`, `CreatedAt`. No `email` field.
- `AdminListParams{ActorRole, Action, EntityType, Page, Limit}`.

`backend/internal/audit/repository.go`:
- `ListAdmin(ctx, params) ([]AdminAuditLog, int, error)` — one query, `LEFT JOIN users` for `actor_name` only, three optional filters, `COUNT(*) OVER()` for the total, ordered `created_at DESC` (newest first, per instruction 7), `pagination.Offset`-based paging.

`backend/internal/audit/service.go`:
- `ListAdmin(ctx, params) (pagination.Result[AdminAuditLog], error)` — thin delegation, no additional validation (see design decisions above).

`backend/internal/audit/handler_admin.go` (new):
- `RegisterAdminRoutes(admin)` → `GET /audit-log` (i.e. `/admin/audit-log` once mounted).
- `ListAuditLog` — reads `actor_role`/`action`/`entity_type`/`page`/`limit` query params, calls `service.ListAdmin`, `500` on a genuine internal error (no validation-error branch needed, since nothing here can fail 400).

`backend/cmd/api/main.go`:
- `auditHandler := audit.NewHandler(auditService)` constructed alongside the existing `auditRepo`/`auditService`.
- `auditHandler.RegisterAdminRoutes(adminGroup)` added alongside `qaHandler`/`reportsHandler`'s equivalent calls — `adminGroup` already gates every route on it to `authMiddleware.RequireAuth()+RequireRole("admin")`, the same choke point every other admin endpoint in this codebase goes through, so non-admin/unauthenticated rejection needed no new code (instruction 8 satisfied by registration, not a new check).

No changes to `internal/qa`, `internal/reports`, or any other domain — this session only adds the read surface on top of what Stage 25A1/25A2 already built.

### Files changed

- `backend/internal/audit/model.go` — `AdminAuditLog`, `AdminListParams`.
- `backend/internal/audit/repository.go` — `ListAdmin`.
- `backend/internal/audit/service.go` — `ListAdmin`.
- `backend/internal/audit/handler_admin.go` — new.
- `backend/cmd/api/main.go` — `auditHandler` construction + one route-registration line.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/audit/*.go cmd/api/main.go` — clean, no output.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**Live** (Docker Compose backend rebuilt via `docker compose up -d --build backend`, no panic, `GET /api/v1/admin/audit-log` confirmed registered; fresh student + instructor accounts, instructor assigned ownership of a real course; the seeded admin account; 3 real audit rows generated through the actual Stage 25A2 call sites — hide, show, and a report resolution — giving a genuine mix of `actor_role` (`instructor` ×2, `admin` ×1), `action` (`content_hidden`, `content_shown`, `report_resolved`), and `entity_type` (`question` ×2, `report` ×1)):

| Case | Result |
|---|---|
| Admin list, no filters | **200**, `total: 3`, all fields present and correct, including `actor_name: "Admin User"` for the admin-authored row (correctly joined) |
| Pagination (`limit=1`, pages 1–3) | **200** each; page 1 = `report_resolved` (newest), page 2 = `content_shown`, page 3 = `content_hidden` (oldest) — confirms newest-first ordering (instruction 7) end to end across real pagination, not just a single page |
| `action` filter (`content_hidden`) | **200**, `total: 1`, correctly isolates the one matching row |
| `entity_type` filter (`report`) | **200**, `total: 1`, correctly isolates the one matching row |
| `actor_role` filter (`instructor`) | **200**, `total: 2`, both matching rows |
| `entity_id` correctness | Re-verified directly: the `question`-typed row's `entity_id` matches the real question's UUID exactly |
| No `email` field anywhere in the response | Confirmed via full-response text search — absent |
| Non-admin (student) | **403** `FORBIDDEN` |
| Unauthenticated | **401** `UNAUTHORIZED` |

All 9 cases (the 7 required from instruction 12, plus the explicit `entity_id`/no-`email` checks) passed on the first attempt — no code changes needed after the initial implementation.

**Incidental finding, not a new bug**: the test instructor account's `actor_name` rendered as an empty string in the list (rather than a real name) — re-confirmed this is the same pre-existing `PUT /admin/users/:id` behavior already documented in `STAGE20_PROGRESS.md`/`STAGE21_PROGRESS.md`/`STAGE23_PROGRESS.md` (blanking `first_name`/`last_name` on a role-only update), encountered again only because this session's test setup needed a fresh instructor account. Not caused by, or fixed in, this session's code.

### Cleanup

Test question deleted via the real `DELETE /questions/:id` endpoint (204). Test report deleted directly (no delete endpoint exists; `content_reports` has no FK to `lesson_questions` by Stage 24A1's own polymorphic design). All 3 `audit_logs` rows this session produced were deleted directly — confirmed the table was empty before this session's testing began (per Stage 25A2's own cleanup), so every row present was this session's own test data. Course ownership reverted to `NULL`. Verified via direct DB query afterward: `audit_logs`, `content_reports`, and the tagged question all return `0`. Test accounts left in place, matching every prior stage's convention.

### Not done this session (explicitly out of scope for 25A3)

- **No new audit-writing call sites** — role changes, certificate issuance, subscription/payment overrides remain unwired, exactly as instructed.
- **No frontend** — no `/admin/audit-log` page; `GET /admin/audit-log` is reachable only via direct API calls today.
- **No date-range filter** — the roadmap's original sketch mentioned filtering by date; this session's explicit instruction 5 only asked for `actor_role`/`action`/`entity_type`, so no date filter was added. `created_at` is still returned on every row and the table is indexed for chronological access (`idx_audit_logs_actor_created`), so this remains a low-cost future addition if wanted.
- **No regression pass** — out of scope for this focused endpoint session.

## Stage 25B1 — admin audit-log frontend (this session)

Scope: an `/admin/audit-log` page consuming Stage 25A3's `GET /admin/audit-log`, one new nav item, filters, pagination, loading/empty/error states. No backend changes, no new audit-writing call sites.

### Inspection performed

Re-read `STAGE25_PROGRESS.md`'s Stage 25A1–25A3 sections fresh. Read `app/admin/reports/page.tsx` and its `lib/admin-api.ts` counterpart (`AdminContentReport`/`adminListReports`) in full — the closest and most recent admin list page — for the established filter-form/pagination/error-handling shape to mirror. Checked `app/admin/layout.tsx`'s nav array and `components/shell/icons.tsx`'s existing icon set for a fitting icon before adding a new one: `IconClipboard` already exists (used today only in the *instructor* sidebar for "Submissions," not the admin one), so no new icon was created this session, unlike Stage 24B1's `IconFlag`. Did not inspect any unrelated admin page beyond `reports` (the reference) and the shared layout/icon files needed to add a nav entry.

### Design decision: no client component, unlike `AdminReportsQueue`

Stage 24B1 needed `AdminReportsQueue.tsx` (a `"use client"` component with local state) because that page has a real mutation — the status-update action — that had to update in place without a full reload. `internal/audit` has no update/delete route at all (Stage 25A3's own scope boundary: read-only, by construction), so this page has nothing to make interactive. It is a single, plain, fully server-rendered component — closer to `admin/reviews`'s original pre-Stage-24B1 shape than to the reports page it was modeled on. This is the correct, minimal match for what the backend actually offers, not an oversight.

### Design decision: `<select>` for `actor_role`, free-text `<input>` for `action`/`entity_type`

`actor_role` is a genuinely fixed, small set (`student`/`instructor`/`admin` — the same set `internal/auth/middleware.go`'s `RequireRole`/`RequireAnyRole` already hardcode throughout the backend), so a `<select>` matches `reports`'/`reviews`' own precedent for a closed enum filter. `action`/`entity_type` are the opposite: Stage 25A1's `model.go` and Stage 25A3's own doc comments both explicitly document these as deliberately open-ended, never validated against a whitelist anywhere in the backend, specifically so future call sites can add new values without a migration. A `<select>` with a hardcoded option list here would misrepresent that openness and require a frontend change every time a future stage audits a new action — free-text inputs are the honest match for what the backend actually guarantees.

### Change made

`frontend/lib/admin-api.ts`:
- `AdminAuditLogEntry` interface, mirroring `backend/internal/audit/model.go`'s `AdminAuditLog` exactly — `actor_user_id`/`actor_name`/`actor_role`/`entity_id`/`metadata` all optional, matching the backend's nullable columns.
- `adminListAuditLog(token, {page, limit, actor_role, action, entity_type})` — same shape as `adminListReports`, hitting `GET /admin/audit-log`.

`frontend/components/shell/icons.tsx`:
- No change — reused the existing `IconClipboard`.

`frontend/app/admin/layout.tsx`:
- Added `IconClipboard` to the icon imports and one new nav item, `{ href: "/admin/audit-log", label: "Audit Log", icon: <IconClipboard size={18} /> }`, in the "Сообщество" group right after "Reports" — the same group and adjacency Stage 24B1 used when it added the Reports item after Q&A.

`frontend/app/admin/audit-log/page.tsx` (new):
- Server component: session gate (redirect to `/login` if no token, matching every other admin page), reads `actor_role`/`action`/`entity_type`/`page` from `searchParams`.
- `adminListAuditLog` wrapped in `try/catch` → `loadError` state with a `role="alert"` fallback, matching `admin/reports`'/`admin/questions`' established error-handling convention.
- Filter `<form method="get">` (see design decision above), a plain `<table className="admin-table">` with columns actor name / role / action / entity type / entity id / metadata preview / created_at, an `.empty-state` when `result.items.length === 0`, and pagination controls (page/total_pages/total, Prev/Next links preserving all three filters) — the same shape as `admin/reports`'s own pagination block.
- `metadataPreview(metadata)` — a local helper, `JSON.stringify` truncated to 70 characters with an ellipsis, satisfying instruction 4's "compact metadata preview" (the full object is still in the JSON response for anyone who needs it, just not spelled out in the table).
- Reuses existing classes only: `.admin-header`, `.subtitle`, `.admin-search` (already styles both `<select>` and `<input>` children, confirmed by reading the CSS before writing the form), `.admin-table-wrapper`/`.admin-table`, `.empty-state`, `.admin-pagination` — **zero new CSS added this session**.

### Files changed

- `frontend/lib/admin-api.ts` — `AdminAuditLogEntry`, `adminListAuditLog`.
- `frontend/app/admin/layout.tsx` — nav item + icon import (no new icon).
- `frontend/app/admin/audit-log/page.tsx` — new.
- No backend files touched. No other admin page touched — `admin/reports` was read for reference, not modified.

### Verification performed (focused, per scope)

- `npx tsc --noEmit` (whole project) — clean.
- `npx eslint app/admin/audit-log/page.tsx app/admin/layout.tsx lib/admin-api.ts components/shell/icons.tsx` — clean, zero warnings.
- `npx eslint .` (whole project) — **0 errors**, still exactly the same 4 pre-existing `<img>` warnings every prior stage has reported, nothing new.

No live browser interaction, no Docker Compose rebuild, no E2E — explicitly out of scope for 25B1. The backend endpoint this page calls was already live-verified end-to-end in Stage 25A3 (list, all three filters, pagination, entity_id correctness, no-email confirmation, non-admin/unauthenticated rejection); this session's job was limited to confirming the frontend calls it correctly and renders the response as specified, which `tsc`'s type-checking of `AdminAuditLogEntry` against the page's field access plus the code-level review above covers for this focused pass.

### Not done this session (explicitly out of scope for 25B1)

- **No new audit-writing call sites** — role changes, certificate issuance, subscription/payment overrides remain unwired, exactly as instructed.
- **No live browser interaction** — same standing limitation noted in every prior C-stage's own section; no browser-automation tool is available in this environment.
- **No regression pass** — out of scope for this focused frontend session.

## Stage 25C — focused verification + final Stage 25 report (this session)

Scope: verify 25A1–25A3 (backend storage/writes/reads) and 25B1 (admin frontend) live together, fix only bugs the verification itself surfaced, close out Stage 25 for the scope actually built (two audited call sites + admin read API + admin UI). No new audit call sites, no admin audit-log write endpoints, no unrelated full-platform regression.

### Setup

Rebuilt both `backend` and `frontend` (`docker compose up -d --build backend frontend`) — frontend was still serving the pre-25B1 image going into this session. Confirmed both healthy (`GET /api/v1/health` → 200, unauthenticated `GET /admin/audit-log` → 307 on the frontend) with no route-conflict panic in `docker compose logs backend`; `GET /api/v1/admin/audit-log` confirmed registered. Registered fresh student + two instructor accounts (A promoted and assigned ownership of "Go Backend Developer," B promoted and assigned ownership of a *different* real course, "PostgreSQL" — a genuine cross-instructor setup) plus the seeded admin. Confirmed `audit_logs` empty before any testing began.

### Bug found and fixed: test-setup gap, not a product bug

Mid-session, the first "non-owning instructor" rejection check was run against an account that had been registered but **never actually promoted to `instructor`** — so it correctly got rejected, but by the `/instructor` route group's role middleware (a student-level rejection), not by the `ownership.CanManageCourse` check the test was actually meant to exercise. This was caught immediately by reading the response body (`"insufficient permissions"`, the role-middleware message, not the service-layer `"you may only moderate questions in courses you own, or as an admin"` message) rather than just the status code. **This was a gap in this session's own test setup, not a bug in Stage 25's code** — fixed by promoting the second instructor account and assigning it ownership of a genuinely different course, then re-running the check, which produced the expected service-layer `FORBIDDEN` message. No application code was changed. No other bugs were found this session.

### 1. Audit creation — verified live

| Case | Result |
|---|---|
| Non-owning instructor (real `instructor` role, owns a different real course) attempts to hide a question | **403** `FORBIDDEN` (service-layer `ownership.CanManageCourse` rejection) — `audit_logs` count stayed **0** |
| Student attempts to hide a question | **403** (role middleware) — `audit_logs` count stayed **0** |
| Owning instructor A hides the question | **200** — `audit_logs` count → **1** |
| Student attempts a report status update | **403** — `audit_logs` count unchanged |
| Student submits a report (creation itself is not an audited action) | **201** — `audit_logs` count unchanged, confirming only the *status update* call site is wired, exactly as every prior session documented |
| Admin resolves the report | **200** — `audit_logs` count → **2** |
| Admin shows the question again via `/admin/qa/questions/:id`, on a course the admin does not personally own | **200** — `audit_logs` count → **3**, confirming the admin-bypass path records the real admin actor |
| Owning instructor A hides the answer | **200** — `audit_logs` count → **4** |

All required scenarios passed. Every rejected/failed attempt added zero rows; every successful action added exactly one.

### 2. Stored audit data — verified live

Full row content re-checked directly in the DB for both call sites:

- Q&A hide: `actor_user_id` = instructor A's real id, `actor_role` = `"instructor"`, `action` = `"content_hidden"`, `entity_type` = `"question"`, `entity_id` = the real question id, `metadata` = `{"course_id": "<real course id>", "published": false}`, `created_at` populated — every field exact.
- Report resolution: `actor_user_id` = the real admin id, `actor_role` = `"admin"`, `action` = `"report_resolved"`, `entity_type` = `"report"`, `entity_id` = the report's own id, `metadata` = `{"content_type": "answer", "content_id": "<real answer id>", "new_status": "resolved"}` — every field exact.

### 3. Admin audit API — verified live

| Case | Result |
|---|---|
| Admin list, no filters | **200**, `total: 4`, all 4 real rows present with correct fields |
| Newest-first ordering | Programmatically confirmed (`created_at` values equal their own sorted-descending order) across all 4 rows, not just eyeballed |
| Pagination (`limit=1`, pages 1–4) | **200** each; the 4 single-item pages reproduce the exact same order as the unpaginated list |
| `actor_role` filter (`admin`) | **200**, `total: 2`, both rows correctly `actor_role: "admin"` |
| `action` filter (`content_hidden`) | **200**, `total: 2`, both rows correctly `action: "content_hidden"` |
| `entity_type` filter (`answer`) | **200**, `total: 1`, correctly isolates the one matching row |
| Non-admin (student) | **403** `FORBIDDEN` |
| Unauthenticated | **401** `UNAUTHORIZED` |

All 8 cases passed.

### 4. Admin UI — verified via SSR + code review (no browser-automation tool in this environment, same standing limitation as Stages 21C/22C/23C/24C)

- **`/admin/audit-log` loads**: SSR-fetched with a real admin session cookie — **200**, "Журнал действий" heading present, all three filter fields (`actor_role` `<select>`, `action`/`entity_type` `<input>`s) present.
- **Filters work live on the real page**: `?actor_role=admin` → confirmed via context inspection that the only "instructor" text on that filtered page comes from the `<select>`'s own always-rendered `<option>` list (the dropdown must list every choice regardless of which is active), not from a leaked table row — the actual data was independently re-confirmed correct via the JSON API in section 3. `?entity_type=answer` → correctly narrowed. A real zero-match combination (`?actor_role=student`, since no student-authored audit rows exist by design — students can't perform either audited action) → **200**, "Записей не найдено" rendered, no crash.
- **Metadata preview renders safely**: real `metadata` (e.g. `course_id`, `published`) confirmed present in the rendered HTML; the page renders it via plain JSX text interpolation (`{metadataPreview(entry.metadata)}`, no `dangerouslySetInnerHTML` anywhere in the file — confirmed by re-reading the full page source this session), so React's default escaping applies unconditionally. Additionally, every `metadata` value that can ever reach this table is developer-authored inside Stage 25A2's two call sites (course ids, booleans, status strings, content-type/id pairs) — never raw free-text from an end user — so there is no realistic path for attacker-controlled HTML to reach this field in the first place, safe by construction on top of React's own escaping.
- **Loading/empty/error states**: empty state live-confirmed above. The error state (`loadError` branch) was **not** reproducible against a genuine backend rejection this session, unlike Stage 24C's equivalent check on the reports page — `GET /admin/audit-log`'s filters are deliberately unvalidated by Stage 25A3's own design (any filter value, recognized or not, just narrows or fails to narrow the `WHERE` clause; there is no 400 path for this endpoint to return at all). Re-read the `try/catch`/`loadError` code path directly instead and confirmed its structure is correct (mirrors `admin/reports`' identical, already-proven pattern) — this is a code-review-only confirmation for this one sub-case, stated plainly rather than glossed over.

### Final checks

- `gofmt -l .` (whole backend) — clean.
- `go build ./...` — OK.
- `go vet ./...` — OK.
- `npx tsc --noEmit` (whole frontend) — clean.
- `npx eslint .` (whole frontend) — 0 errors, same 4 pre-existing unrelated `<img>` warnings every prior stage has reported, nothing new.
- Docker Compose: both `backend` and `frontend` rebuilt and live; `GET /api/v1/admin/audit-log` confirmed registered with no panic.

### Files changed (all of Stage 25: 25A1 + 25A2 + 25A3 + 25B1 + 25C)

Backend:
- `backend/migrations/00040_create_audit_logs.sql` — new table (25A1).
- `backend/internal/audit/model.go` — `AuditLog`, vocabularies, `AdminAuditLog`, `AdminListParams` (25A1, 25A3).
- `backend/internal/audit/repository.go` — `Create` (25A1); `ListAdmin` (25A3).
- `backend/internal/audit/service.go` — `Log` (25A1); `ListAdmin` (25A3).
- `backend/internal/audit/handler_admin.go` — `GET /admin/audit-log` (25A3).
- `backend/internal/qa/service.go` — `logPublishedChange`, both `Set*Published` methods call it (25A2).
- `backend/internal/reports/service.go` — `UpdateStatus`'s new actor parameters, `logStatusUpdate` (25A2).
- `backend/internal/reports/handler_admin.go` — `UpdateReportStatus` reads/forwards actor identity (25A2).
- `backend/cmd/api/main.go` — `audit` construction + `qa`/`reports` dependency wiring + route registration (25A2, 25A3).

Frontend:
- `frontend/lib/admin-api.ts` — `AdminAuditLogEntry`, `adminListAuditLog` (25B1).
- `frontend/app/admin/layout.tsx` — nav item (25B1, reused existing `IconClipboard`).
- `frontend/app/admin/audit-log/page.tsx` — new (25B1).

No file changed this session (25C) — verification only; the one issue found was in this session's own test setup, not in any Stage 25 code.

### Known limitations (Stage 25, final)

- **Only two of the roadmap's named call sites are audited** — Q&A hide/show (Stage 21) and report status updates (Stage 24). Role changes (`internal/users`), certificate issuance, and subscription/payment admin overrides remain unwired, exactly as every session from 25A2 onward explicitly scoped. This was a deliberate two-call-site slice, not an oversight, and is the clear next step for a future stage.
- **No date-range filter** on the admin API/UI — the roadmap's original sketch mentioned one; this session's explicit instructions only asked for `actor_role`/`action`/`entity_type`, so none was added. `created_at` is returned on every row and the table is indexed for chronological access, so this remains a low-cost future addition.
- **The audit-log page's error state could not be verified against a genuine backend rejection** — `GET /admin/audit-log` has no validation path to fail (filters are deliberately unvalidated by design), unlike the reports page Stage 24C could drive into a real 400. Verified by code review only for this one sub-case; documented rather than glossed over.
- **No real browser interaction anywhere in Stage 25.** Every purely-client-side-JS-runtime behavior was verified by code inspection plus the strongest available live substitutes (SSR HTML, direct API reproduction) rather than an actual driven interaction — no browser-automation tool is available in this environment. Same standing limitation as every prior C-stage. Lower risk here than usual: the audit-log page itself has zero client-side interactivity to miss (confirmed in 25B1 — no client component exists), so this limitation applies only to the shared layout/nav chrome, not to any logic unique to this page.
- No automated test suite exists anywhere in this codebase (unchanged, project-wide convention) — every verification claim above is a live, scripted check against the running Docker Compose stack, or a documented code-level review substituting for one where live interaction wasn't reachable.

### Final Stage 25 status

**Stage 25 is complete for the scope actually built: audit-log storage (25A1), two real audited call sites (25A2), the admin read API (25A3), and the admin UI (25B1).** All four layers are implemented and live-verified together this session — creation (including a corrected non-owning-instructor rejection check), stored-data correctness, the full admin API surface (list/order/paginate/filter/reject), and the admin UI (load/filter/empty-state, with the one inherently-unreproducible error-state sub-case explicitly documented rather than skipped silently) — with **zero product bugs found**; the single issue this session caught was in its own test setup and was corrected without touching any Stage 25 code. The items listed under "Known limitations" are deliberately deferred, documented scope boundaries (most of the roadmap's named call sites still unaudited, no date filter, one inherently-unverifiable UI sub-case) — not defects.
