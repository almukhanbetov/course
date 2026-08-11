# Stage 19 — Admin Revenue & Subscription Analytics

**Status: Stage 19A (backend), 19B (frontend) and 19C (final verification + plan breakdown) all complete. Stage 19 is done.**

Tracking doc — status only, not a spec restatement.

## Stage 19C — final verification (this session)

### Completed
- **Security verification** (all live against Docker Compose):
  - Unauthenticated → 401. Garbage/empty-bearer token → 401 (never 500). Student → 403. **Instructor → 403** (freshly created test account — confirms no privilege escalation via a non-admin role, not just "student" specifically). Admin → 200.
  - Response body inspected directly: contains only `currency`/aggregated numbers/`plan_id`+`plan_name`. No `user_id`, `email`, `provider`, `provider_payment_id`, or `idempotency_key` anywhere — confirmed by grep against the raw JSON, not just code review.
  - No payment-provider secrets touched or exposed — the analytics code path never references `PaymentProvider`/`cfg.PaymentProvider` at all, only reads `payments`/`subscriptions`/`subscription_plans` columns.
  - Date-range validation probed with: malformed strings, invalid calendar dates (`2026-13-45`), a SQL-injection-shaped string, an XSS-shaped string, wrong separators (`2026/08/11`), an oversized string, and an inverted range (`from > to`). Every malformed input → 400, never 500, never reached the database (parsing fails before any query runs — `time.Parse` rejects first, and even if it reached SQL, values are always bound parameters, never interpolated). Inverted range → 200 with correctly empty results (not an error, since `paid_at >= from AND paid_at < to` with `from > to` structurally matches nothing).
- **Correctness verification** (live, with real and controlled test data):
  - **Currency isolation**: created a second plan in USD, confirmed a payment against it — response showed KZT and USD as two independent rows (990000 KZT vs 300000 USD), never summed into one figure.
  - **MRR formula**: hand-calculated against a **90-day** plan (price 300000) — `300000 * 30 / 90 = 100000` — matched the live response exactly. (Stage 19A had only verified the trivial 30-day passthrough case; this is the first test of the actual normalization arithmetic.)
  - **Active-subscription correctness under a stale status**: set a real test subscription's `expires_at` to the past while leaving `status='active'` (simulating a row the lazy-sync-on-read in `GetActiveSubscription` hasn't touched yet) — `active_subscriptions`/`mrr` correctly dropped to 0, confirming the analytics query independently re-checks `expires_at > now()` rather than trusting the stored status column.
  - **Canceled-subscription counting**: set `canceled_at = now()` on a test subscription — `canceled_subscriptions` incremented correctly, and correctly dropped back to 0 when queried with a date range excluding today (period-scoping confirmed, not just presence).
  - **Refunded amount/count**: still correctly reads 0 — no code path in the codebase sets `status='refunded'` (unchanged conclusion from 19A, re-confirmed).
  - **Date-range boundaries**: re-confirmed the `[from, to)` inclusive/exclusive behavior from 19A still holds after the 19C changes.
- **Stage 19B gap resolved — per-plan breakdown implemented.** Reviewed the `subscription_plans`/`subscriptions`/`payments` schema: confirmed a per-plan aggregate is a small additive `LEFT JOIN` query (`subscription_plans` → `subscriptions` → `payments`, `GROUP BY plan_id`) requiring no schema change and no touch to `CreateSubscription`/`ConfirmPayment`/checkout — so it was implemented rather than left deferred, per the instructions ("if it can be added cleanly... implement it"):
  - `internal/subscriptions/model.go`: new `PlanRevenue` type, `RevenueAnalytics.PlanBreakdown []PlanRevenue` field.
  - `internal/subscriptions/repository.go`: new `GetPlanBreakdown(ctx, from, to)` — one query, `LEFT JOIN`s through subscriptions to payments so plans with zero activity still appear (all-zero row) rather than being omitted; uses `COUNT(DISTINCT s.id) FILTER(...)` for the subscription-based counts to stay correct even if a future payment-retry flow ever puts more than one payment row on a subscription (today it's always 1:1, but this doesn't assume that).
  - `internal/subscriptions/service.go`: `GetRevenueAnalytics` now composes both the currency and plan aggregates.
  - No handler/route change — same `GET /admin/analytics/revenue` endpoint, richer payload.
  - **Verified live**: created a plan, a subscription, and a confirmed payment for it — `plan_breakdown` correctly attributed `active_subscriptions: 1, new_subscriptions: 1, paid_amount: 990000, paid_count: 1` to that specific plan.
  - **One documented, expected edge case**: a payment whose `subscription_id` went `NULL` (only possible if a subscription row is deleted — never happens through normal app use; it's how Stage 19A's own cleanup-script mistake happened to leave one payment orphaned) still counts toward the currency totals in `by_currency` but can no longer be attributed to any plan in `plan_breakdown`, since attribution requires the `subscription → plan` link. This was directly observed live: `by_currency` showed 3 payments/29700 KZT while `plan_breakdown` showed only 2 payments/19800 KZT for the one real plan — mathematically consistent given the one orphaned payment, not a bug. Documented both in a backend code comment and directly in the frontend UI's caption under the "By plan" table.
  - Frontend: `lib/admin-api.ts` gained `PlanRevenue` + `plan_breakdown` on `AdminRevenueAnalytics`; `app/admin/analytics/page.tsx`'s "not available yet" note was replaced with an actual `.admin-table` (Plan / Currency / Active / New / Canceled / Revenue), rendered **only when `plan_breakdown.length > 0`** (no fabricated rows, matches the instruction not to invent data) — screenshot-verified showing the real "Pro" plan row with correct numbers.
- **Performance**: re-seeded ~150 synthetic subscriptions/payments across 3 currencies (isolated under 3 dedicated synthetic plan ids this time — not the real "Pro" plan — specifically to make cleanup exact after Stage 19A's overly-broad cleanup mistake) and ran `EXPLAIN ANALYZE` on all four aggregate queries (the three from 19A plus the new plan-breakdown query):

  | Query | Plan | Execution time |
  |---|---|---|
  | Payment flow by currency | Seq scan on `payments` (152 rows) | 0.260 ms |
  | Subscription flow by currency | Seq scan on `subscriptions` (151 rows) + memoized index scan on `subscription_plans` pkey | 0.336 ms |
  | Active + MRR by currency | Seq scan on `subscriptions` (filtered) + memoized index scan on `subscription_plans` pkey | 0.266 ms |
  | **Plan breakdown (new)** | Hash Right Join × 2 (`payments`→`subscriptions`→`subscription_plans`), all seq scans | **0.812 ms** |

  All four sub-millisecond. The plan-breakdown query is the most expensive (two hash joins + a sort) but still trivial at this scale. **No index migration added** — not proven necessary by the query plan, same conclusion as 19A. One latent observation for future scale: `subscriptions.plan_id` has no dedicated index today (only `user_id`, `status`, `expires_at` do, from migration `00023`) and is the join key in all four queries; at the current table size the planner correctly prefers a full seq scan + hash join over using one, so this isn't a current problem, but it's the first column worth indexing if `subscriptions` ever grows large enough to change that plan.
  - Synthetic data cleaned up **precisely** this time — deleted by the three dedicated synthetic plan ids only (no `NOT IN`/`LIMIT` tricks), verified table counts returned to the exact pre-seed baseline (2 payments, 1 subscription, 1 plan) afterward.
- **Regression** (live, all against the actual running stack, not just code review):
  - Public `GET /plans` → 200. `GET /me/subscription` → 200 for a real student. `GET /admin/subscriptions`, `GET /admin/payments`, `GET /admin/plans` → 200, unaffected.
  - **Full checkout flow re-run end-to-end**: register → `POST /subscriptions` (201) → `POST /payments/:id/mock-confirm` (200) → `GET /me/subscription` shows `active: true` — proves `CreateSubscription`/`ConfirmMockPayment` are completely unaffected by the 19C changes.
  - Frontend: `/admin`, `/admin/subscriptions`, `/admin/payments`, `/admin/plans` all still 200 for admin. `/dashboard/subscription` (student-facing) and `/pricing` (public) both still 200.
  - Admin layout/navigation: `Sidebar.tsx`/`SidebarShell.tsx` were not touched this session (only the already-existing Analytics nav item from 19B remains); "Analytics" correctly highlights as the active item.
- **Frontend verification**: `npx tsc --noEmit` clean, `npx eslint app/admin/analytics lib/admin-api.ts` clean, `npm run build` clean (`/admin/analytics` among all generated routes). Live: unauthenticated → 307, instructor → 307 (layout redirect), admin → 200 with the new "By plan" table visible and correctly populated; screenshot-confirmed dark theme/sidebar/table styling all consistent with the rest of the admin panel, no visual regressions.

### Files changed (19C only)
- `backend/internal/subscriptions/model.go` — `PlanRevenue` type, `PlanBreakdown` field.
- `backend/internal/subscriptions/repository.go` — `GetPlanBreakdown`.
- `backend/internal/subscriptions/service.go` — composes the new query into the response.
- `frontend/lib/admin-api.ts` — `PlanRevenue` type, `plan_breakdown` field.
- `frontend/app/admin/analytics/page.tsx` — real "By plan" table replacing the "not available" note.
- No route/handler signature changed; no migration added.

### Known limitations (carried forward + new)
- `refunded_amount`/`refunded_count` still always read 0 — no refund code path exists anywhere in this codebase yet (unchanged from 19A).
- `plan_breakdown` and `by_currency` can legitimately disagree if a payment's subscription was ever deleted (orphaning it) — documented above, surfaced in the UI, not a bug in either query.
- No automated test suite exists in this codebase (established convention) — verification remains build/vet/lint/typecheck + live scripted + visual checks, consistent with every prior stage.
- `subscriptions.plan_id` has no dedicated index — not currently a performance problem (see EXPLAIN ANALYZE above) but flagged as the first candidate if `subscriptions` grows substantially.

### Remaining (explicitly out of scope, not attempted)
- Full-platform regression across all domains (Stage 1–18) — only the subscriptions/payments/admin/analytics surface was re-verified, per this session's instructions.
- Stage 20 or any further stage — not started.

## Stage 19B — frontend

### Completed
- New admin page `/admin/analytics` (Server Component, follows the exact same pattern as every other `/admin/*` page: `getSessionToken()` → redirect to `/login` if absent; the actual role gate is `app/admin/layout.tsx`, unchanged, which already redirects non-admins to `/dashboard` before this page ever renders).
- "Analytics" nav item added to the admin sidebar (`app/admin/layout.tsx`), in the existing "Биллинг" group next to Plans/Subscriptions/Payments, using the already-existing `IconBarChart`.
- Connected to the Stage 19A endpoint via a new `adminGetRevenueAnalytics(token, {from, to})` in `lib/admin-api.ts` — no backend changes, no new endpoint, no changes to `internal/subscriptions`.
- Displays, grouped by currency (one `<section>` per currency, each with its own heading and its own `stat-grid` — **values are never summed across currencies, anywhere in the JSX**): revenue (paid amount + payment count), MRR, active subscriptions, new subscriptions, canceled subscriptions, refunded amount + count, and the selected date range as text above the breakdown.
- Simple date-range controls: a plain GET `<form className="admin-search">` with two `<input type="date">` (`from`/`to`) — no client JS, fully bookmarkable/shareable URL, identical mechanism to the existing status filters on `/admin/subscriptions` and `/admin/payments`.
- States handled:
  - **Loading**: `app/admin/analytics/loading.tsx` (Next.js route-segment convention — automatically shown while the async Server Component fetches; no client-side spinner, no new pattern beyond a framework-native file Next already supports).
  - **Empty**: `by_currency.length === 0` → existing `.empty-state` block.
  - **API error**: any other fetch failure → `role="alert"` message with the error text, matching the `courses/[id]` page's existing error-paragraph convention.
  - **Unauthorized/forbidden**: `adminGetRevenueAnalytics` throws `Error("UNAUTHORIZED")`/`Error("FORBIDDEN")` specifically on 401/403 (checked before falling through to the generic error path) → distinct "session expired, log in again" / "no permission" messages. In practice the layout's server-side role redirect makes this unreachable for a normal user, but it's handled defensively for a mid-session token expiry/role change race.
- **Known, intentional gap at the time — "subscription breakdown by plan" is not implemented.** The task's display list included it, but the Stage 19A backend deliberately scoped `GetRevenueAnalytics` to currency-only aggregation, and this session's instructions were explicit: *reuse* the existing endpoint, *do not reimplement backend analytics*. Surfaced honestly in the UI (a note at the bottom of the page) rather than silently dropped. **Resolved in Stage 19C** (see the top of this document) — the backend now exposes a real per-plan breakdown and the frontend renders it.

### Files changed
- `frontend/app/admin/analytics/page.tsx` — new.
- `frontend/app/admin/analytics/loading.tsx` — new.
- `frontend/app/admin/layout.tsx` — added the Analytics nav item (2-line change).
- `frontend/lib/admin-api.ts` — added `CurrencyRevenue`, `AdminRevenueAnalytics`, `adminGetRevenueAnalytics`.
- No backend files touched this session (`git status` confirms the 4 backend files are exactly the ones from 19A, untouched further).

### Verification performed
- `npx tsc --noEmit` — clean.
- `npx eslint app/admin/analytics lib/admin-api.ts app/admin/layout.tsx` — clean, zero warnings.
- `npm run build` (production, Turbopack) — compiled successfully; `/admin/analytics` listed among all 57 generated routes.
- Docker Compose frontend rebuilt and restarted; live checks against `http://localhost:3001`:
  - Unauthenticated → **307** redirect (to `/login`).
  - Student session → **307** redirect (to `/dashboard`, via the unchanged layout gate) — never reaches the page's own forbidden-handling branch, confirming the layout is still the first line of defense exactly as before.
  - Admin session → **200**, page renders with the real KZT data from the 19A session's E2E check (`9 900,00 KZT` revenue, 1 payment) plus the "Analytics" sidebar item correctly highlighted as active.
  - Explicit `?from=2026-08-11&to=2026-08-11` → "Showing 2026-08-11 — 2026-08-11" displayed correctly (verified via the RSC payload, not just raw HTML — React SSR splits interpolated text across comment-delimited nodes, so a naive substring grep on the rendered output initially looked like a miss but the actual content is correct and was visually confirmed via screenshot).
  - Explicit `?from=2020-01-01&to=2020-01-02` (no data in range) → empty-state block rendered correctly, screenshot-verified.
  - Visual screenshots taken (headless Chrome via CDP, cookie-injected admin session) of both the populated and empty states — dark theme, sidebar, stat tiles, date inputs all match the existing admin design exactly; no visual regressions.
- Regression-lite: `/admin`, `/admin/subscriptions`, `/admin/payments` unaffected (only an additive nav-item change touched the shared layout; the sidebar `Sidebar.tsx`/`SidebarShell.tsx` components themselves were not modified).

### Known issues
- ~~Per-plan subscription/revenue breakdown not available~~ — **resolved in Stage 19C** (see the top of this document).
- `refunded_amount`/`refunded_count` will always read 0 in this demo environment since no code path in the codebase ever transitions a payment to `status='refunded'` yet (documented already in 19A) — the UI correctly displays this as zero rather than hiding the tile, so the fields are visible and ready for whenever a refund path exists.
- No automated frontend tests exist in this codebase for any page (established convention, confirmed in 19A) — verification is build/lint/typecheck + live manual/scripted checks, consistent with every prior stage.

## Scope of this session (19A — backend only)

Give admins aggregate financial visibility (revenue over time, active subscriptions, MRR, new/canceled subscriptions) built entirely on existing `subscriptions`/`payments` data from Stage 9. No new domain, no new tables — read-only aggregation only. Frontend, full-platform regression, and any index migration were explicitly out of scope unless proven necessary.

## Done

### `internal/subscriptions/model.go`
- `RevenueAnalytics{From, To, ByCurrency}` and `CurrencyRevenue{Currency, PaidAmount, PaidCount, RefundedAmount, RefundedCount, NewSubscriptions, CanceledSubscriptions, ActiveSubscriptions, MRR}`.
- **`ByCurrency` is a slice, never a single combined total.** `subscription_plans.currency` is not fixed platform-wide (schema allows a different currency per plan), so every monetary figure is scoped to one currency per row — nothing in this feature ever sums across currencies.
- `MRR` is a normalized monthly-recurring-revenue estimate: each active subscription's plan price is scaled to a 30-day period (`price_amount * 30 / duration_days`) before summing, so a plan billed on a different cadence still contributes a comparable "per month" figure.

### `internal/subscriptions/repository.go`
- `GetRevenueAnalytics(ctx, from, to time.Time) ([]CurrencyRevenue, error)` — three small `GROUP BY currency` queries, merged in Go keyed by currency:
  1. **Payment flow** `[from, to)`, bucketed by `paid_at`: paid amount/count, refunded amount/count. (No `refunded_at` column exists and no code path sets `status='refunded'` yet — those two fields correctly read zero today; bucketing by `paid_at` is still the right choice once a refund path exists, since a payment must have been paid before it can be refunded.)
  2. **Subscription flow** `[from, to)`: new subscriptions (`created_at` in range), canceled subscriptions (`canceled_at` in range) — joined to `subscription_plans` since currency lives on the plan, not the subscription.
  3. **Point-in-time** (as of `now()`, independent of `from`/`to`): active subscription count and MRR, `WHERE status = 'active' AND expires_at > now()`.
- Result is sorted by currency for deterministic output.

### `internal/subscriptions/service.go`
- `GetRevenueAnalytics(ctx, from, to time.Time) (*RevenueAnalytics, error)` — thin wrapper, no business logic beyond composing the repository result with the requested range.

### `internal/subscriptions/handler_admin.go`
- `GET /api/v1/admin/analytics/revenue` — registered inside the existing `RegisterAdminRoutes(admin *gin.RouterGroup)`, so it inherits `RequireAuth()+RequireRole("admin")` from the `adminGroup.Use(...)` call in `main.go` exactly like every other admin route. **No new middleware, no new route group.**
- `from`/`to` query params (`YYYY-MM-DD`, mirrors `internal/activity.GetCalendar`'s existing convention): default to the trailing 30 days when omitted; 400 `INVALID_FROM`/`INVALID_TO` on an unparseable value. A single-day range (`from=to=X`) correctly covers the entire day `X` (the handler advances an explicit `to` to the start of the next day for the exclusive `[from, to)` bound the queries use).

### Not touched (by design)
- `CreateSubscription`, `ConfirmMockPayment`/`ConfirmPayment`, `GetMySubscription`, `ListPlans`, checkout flow — zero changes. `git status` after this session shows exactly 4 modified files (`model.go`, `repository.go`, `service.go`, `handler_admin.go`), all additive.
- No student-facing route, middleware, or response shape changed.

## Verification performed this session

- `gofmt -l .` — clean. `go vet ./...` — clean. `go build ./...` — clean.
- Docker Compose backend rebuilt and restarted; live checks against `http://localhost:8082/api/v1`:
  - No token → **401**.
  - Student token → **403** (role-gated correctly, same as every other `/admin/*` route).
  - Admin token → **200**, correct JSON shape.
  - Garbage token → **401** (never 500).
  - Invalid `from` (`not-a-date`) → **400** `INVALID_FROM`.
  - Real E2E: created a subscription + confirmed its mock payment for a test student (990000 KZT, Pro plan) → `GET /admin/analytics/revenue` correctly showed `paid_amount: 990000, paid_count: 1, new_subscriptions: 1, active_subscriptions: 1, mrr: 990000` (`price_amount * 30/30` = exact passthrough for a 30-day plan, confirming the normalization formula).
  - Explicit `from=to=2026-08-11` (today) → included the payment; `from=to=2026-08-10` (yesterday) → `paid_amount/paid_count/new_subscriptions` correctly dropped to 0 while `active_subscriptions`/`mrr` stayed populated, confirming those two are correctly point-in-time and *not* period-scoped.
- Regression-lite on the touched domain only (not full-platform, per instructions): `GET /plans` still 200 public, `GET /me/subscription` still returns the student's active subscription correctly, `GET /admin/subscriptions` and `GET /admin/payments` (pre-existing admin list endpoints) still 200 — confirming nothing in the existing subscriptions surface regressed.

### EXPLAIN ANALYZE (all three new queries)

Seeded ~120 synthetic subscriptions/payments across 3 plans (2 currencies: KZT, USD) spread over 60 days, mixed `active`/`expired`/`canceled` statuses — then deleted afterward (see cleanup note below).

| Query | Plan | Execution time |
|---|---|---|
| Payment flow by currency | Seq scan on `payments` (121 rows) | 0.201 ms |
| Subscription flow by currency | Seq scan on `subscriptions` (121 rows) + memoized index scan on `subscription_plans` (pkey) | 0.360 ms |
| Active + MRR by currency | Seq scan on `subscriptions` (filtered to 10 active rows) + memoized index scan on `subscription_plans` (pkey) | 0.270 ms |

All three: sub-millisecond, planner correctly chooses a sequential scan over `payments`/`subscriptions` because both tables are small — identical conclusion to Stage 18's own `EXPLAIN ANALYZE` findings ("seq scan on tiny tables is optimal, not a missing index"). `subscriptions.status` and `subscriptions.expires_at` already have indexes from migration `00023` and weren't needed here either, for the same reason.

**No index migration added — not proven necessary per the instructions for this session.**

### Cleanup note
The synthetic seed (120 subscriptions + 120 payments across 2 temporary plans) was deleted after the `EXPLAIN ANALYZE` run. One cleanup query was written too broadly and also removed the single real subscription created earlier in this session's E2E check (for `stage19_stud@example.com`); its payment row survived (`ON DELETE SET NULL` on `payments.subscription_id`) and the endpoint was re-verified afterward to still return correct, non-crashing output with that now-orphaned payment (`paid_amount`/`paid_count` still correct; `new_subscriptions`/`active_subscriptions`/`mrr` correctly read 0 since the subscription itself no longer exists). This is disposable test data from this session only — no production/demo seed data was touched.

## Not started / explicitly deferred (out of scope for this session)

- **Frontend** — done in Stage 19B, extended in 19C (see above).
- **Full-platform regression** — still not run across all of Stage 1–18; only the subscriptions/payments/admin/analytics surface was re-checked across 19A/19B/19C (not requested in any of the three sessions).
- ~~Per-plan revenue breakdown~~ — **implemented in Stage 19C** (see the top of this document).
- **Index migration** — still deferred, still not proven necessary after 19C's re-check with the new plan-breakdown query included (see `EXPLAIN ANALYZE` above in both the 19A and 19C sections); revisit if/when `payments`/`subscriptions` grow to a size where the planner's choice changes.
