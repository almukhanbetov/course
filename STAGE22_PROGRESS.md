# Stage 22 — Search autocomplete & suggestions

Tracking doc — status only, not a spec restatement.

## Stage 22A1 — backend search suggestions query (this session)

Scope: repository + service query only, per `ROADMAP_STAGE_21_30.md`'s Stage 22 backend scope. No HTTP handler/route, no frontend, no migration unless proven necessary, no broad search changes.

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 22 section fresh. Re-read `backend/internal/courses/{model,repository,service}.go` — the only existing search backend, confirmed there is no separate `internal/search` domain. Confirmed `SearchCourses` is the sole existing full-text search path: published-only, category/level/access-type filters, `websearch_to_tsquery` against a GIN-indexed generated `search_vector` column for queries of 3+ characters, falling back to `ILIKE` on title for shorter queries (per the repository's own doc comment). Checked `backend/migrations/00030_add_course_search_vector.sql` — `search_vector` is a `STORED` generated column (title weight A, description weight B) with `idx_courses_search_vector` (GIN) already in place; no `pg_trgm` extension exists yet. Checked the live `courses` table — 3 rows, all published, confirming this project's demo-data scale is unchanged since every prior stage's own findings.

### Design decisions

- **No new domain.** `CourseSuggestion` and `SuggestCourses` were added directly to `internal/courses` (model/repository/service), the same package `SearchCourses` already lives in — reusing its data and conventions directly rather than duplicating course-search logic into a new package.
- **Reused query-normalization logic, not just the convention.** Extracted the inline "3+ chars → `websearch_to_tsquery`, shorter → ILIKE" branch that lived directly inside `SearchCourses` into a small shared `splitSearchQuery(q string) (tsQuery *string, ilikeQuery string)` function, now called by both `SearchCourses` and the new `SuggestCourses` — so a given input string is classified identically by both entry points, not just "conceptually the same rule" reimplemented twice.
- **Deliberately narrower result shape.** `CourseSuggestion{ID, Title, Slug, CategoryName}` — no rating aggregate, no instructor name, no description. `SearchCourses`'s `courseColumns`/`courseJoins` pay for a `LATERAL` rating join on every row; a per-keystroke suggestion query should never pay that cost for data a dropdown will never render.
- **No pagination — a hard cap instead.** `SuggestCourses` takes a `limit`, not page/offset; a suggestion dropdown has nothing to page through. The service fixes this at `suggestionLimit = 8` (top of the roadmap's "top 5–8" range) rather than letting a caller request an arbitrary count.
- **Empty query behaves differently from `SearchCourses` on purpose.** `SearchCourses`'s empty query means "browse the whole catalog" (a real, intentional feature for the `/courses` page). `SuggestCourses` treats an empty/whitespace-only query as "nothing to suggest yet" and returns `[]` without querying the database at all — matching the roadmap's "reject/no-op on empty" security requirement for a per-keystroke endpoint, implemented now at the service boundary even though the HTTP layer that will call it per-keystroke doesn't exist yet in this session.
- **Bounded input length.** `suggestionQueryMaxRunes = 100` — overlong input is truncated before it ever reaches SQL, so a pathological client can't force a needlessly expensive query string through a per-keystroke endpoint once one exists.
- **No migration.** `published = true` filtering and ranking reuse the existing `idx_courses_search_vector` GIN index exactly as `SearchCourses` does; verified via `EXPLAIN ANALYZE` below that no additional index is needed at the current catalog scale. `pg_trgm` (mentioned in the roadmap's migration-needs line) was deliberately **not** added this session — see "Not done this session."

### Change made

`backend/internal/courses/model.go`:
- New `CourseSuggestion` struct: `ID`, `Title`, `Slug`, `CategoryName *string`.

`backend/internal/courses/repository.go`:
- New unexported `splitSearchQuery(q string) (tsQuery *string, ilikeQuery string)`, extracted verbatim from `SearchCourses`'s previous inline logic.
- `SearchCourses` now calls `splitSearchQuery` instead of repeating the branch inline — behavior unchanged, confirmed by re-running its own existing query shape (see verification below).
- New `SuggestCourses(ctx, query string, limit int) ([]CourseSuggestion, error)`: published-only, same `tsquery @@ search_vector` / `title ILIKE` matching and `ts_rank` ordering (secondary sort: `title ASC`) as `SearchCourses`, against the narrow `CourseSuggestion` shape, `LIMIT`-bounded, no offset.

`backend/internal/courses/service.go`:
- New `SuggestCourses(ctx, query string) ([]CourseSuggestion, error)`: trims the query, returns `[]CourseSuggestion{}` immediately (no repository call) for an empty/whitespace-only query, truncates to `suggestionQueryMaxRunes` (100), then calls `repo.SuggestCourses` with the fixed `suggestionLimit` (8).

### Files changed

- `backend/internal/courses/model.go` — `CourseSuggestion` type.
- `backend/internal/courses/repository.go` — `splitSearchQuery` (new, and `SearchCourses` refactored to use it), `SuggestCourses` (new).
- `backend/internal/courses/service.go` — `suggestionLimit`, `suggestionQueryMaxRunes` constants, `SuggestCourses` (new).
- No handler, no route, no migration file.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/courses/*.go` — clean, no output.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**`EXPLAIN ANALYZE`** (run directly against Postgres via `docker compose exec postgres psql`, mirroring the exact SQL `SuggestCourses` issues — no HTTP endpoint exists yet this session to drive it through, per scope):

| Path | Query | Plan | Execution time |
|---|---|---|---|
| tsquery (3+ chars, e.g. `"docker"`) | `search_vector @@ websearch_to_tsquery('russian', $1)` | Seq Scan on `courses` (3 rows) + index-scan join to `categories` on PK | **0.191 ms** |
| ILIKE fallback (1-2 chars, e.g. `"go"`) | `title ILIKE '%go%'` | Same shape, `title ~~* '%go%'` filter instead | **0.236 ms** |

The planner chooses a sequential scan over `courses` in both cases rather than the GIN index — expected and correct at this table's current size (3 rows: "Go Backend Developer", "Docker", "PostgreSQL"), the identical conclusion every prior stage's own `EXPLAIN ANALYZE` verification has reached ("seq scan on tiny tables is optimal, not a missing index"). `idx_courses_search_vector` exists and remains available for the tsquery path once the catalog grows past the planner's seq-scan threshold. **No new index was needed or added.**

### Not done this session (explicitly out of scope for 22A1)

- **No HTTP handler or route** — `SuggestCourses` is reachable only from Go code/tests today, not `GET /search/suggestions` or any other path. That is the natural next slice (a future Stage 22A2), not attempted here.
- **No `pg_trgm` extension/migration** — the roadmap's Stage 22 migration line proposed this for fuzzy/prefix matching at real-world scale; the `EXPLAIN ANALYZE` results above show it isn't needed at the current 3-row catalog, and adding an unused index would be premature. Revisit if/when the catalog grows enough that the ILIKE fallback path's sequential scan actually shows up as a cost in a real query plan.
- **No frontend** — no autocomplete UI, no dropdown component, nothing in `frontend/`.
- **No broad search changes** — `SearchCourses`'s public behavior, route, and response shape are unchanged; the only modification to existing code was extracting its own query-splitting logic into a shared, identically-behaving helper.
- **No regression pass** — out of scope for this focused backend-query session.

## Stage 22A2 — search suggestions HTTP endpoint (this session)

Scope: one lightweight public endpoint wiring Stage 22A1's `Service.SuggestCourses` into the API. No new validation logic, no frontend, no changes to `/courses`'s own search behavior.

### Inspection performed

Re-read `STAGE22_PROGRESS.md`'s Stage 22A1 section and `backend/internal/courses/handler.go` fresh. Confirmed `RegisterRoutes(rg *gin.RouterGroup)` is called as `coursesHandler.RegisterRoutes(v1)` in `cmd/api/main.go` — the bare, unauthenticated `/api/v1` group, not wrapped in `authMiddleware.RequireAuth()` — so adding a route here is public by construction, matching the roadmap's "public, unauthenticated" requirement with no extra auth wiring needed. Checked `ListCourses`'s existing handler for the request-parsing convention (`c.Query("q")` passed straight through to the service, which owns all normalization/validation) and confirmed non-paginated list endpoints elsewhere in this codebase (`internal/categories`, `internal/specialities`) return the bare slice via `c.JSON(http.StatusOK, list)` rather than wrapping it in an envelope — followed that exact convention rather than inventing a `{items: [...]}` shape for a result that has no pagination metadata to carry anyway.

### Change made

`backend/internal/courses/handler.go`:
- `RegisterRoutes`: added `rg.GET("/search/suggestions", h.SuggestCourses)` alongside the existing `GET /courses`/`GET /courses/:id`.
- New `SuggestCourses(c *gin.Context)` handler: reads `c.Query("q")`, calls `h.service.SuggestCourses` (Stage 22A1) directly with no additional parsing/validation — the service already owns trimming, the empty-query short-circuit, and the length cap. Returns `200` with the raw `[]CourseSuggestion` slice (`[]` for no matches, never an error for an empty/missing `q`). A `500 INTERNAL_ERROR` only on a genuine downstream failure (matches every other handler's `default:` case in this file).

No changes to `ListCourses`, `SearchCourses`, or any other existing handler/route.

### Files changed

- `backend/internal/courses/handler.go` — one new route registration line, one new handler method.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/courses/*.go` — clean, no output.
- `go build ./...` — OK.
- `go vet ./...` — OK.

**Live** (Docker Compose backend rebuilt via `docker compose up -d --build backend`; confirmed via `docker compose logs backend` that `GET /api/v1/search/suggestions` registered with no route-conflict panic, and `GET /api/v1/health` → 200):

| Case | Request | Result |
|---|---|---|
| Empty query (no `q` param at all) | `GET /search/suggestions` | **200**, `[]` |
| Empty query (`q=` explicit) | `GET /search/suggestions?q=` | **200**, `[]` |
| Short query, ILIKE fallback path | `GET /search/suggestions?q=go` | **200**, `[{"title":"Go Backend Developer", "category_name":"Programming", ...}]` |
| Matching query, tsquery path | `GET /search/suggestions?q=docker` | **200**, `[{"title":"Docker", "category_name":"DevOps", ...}]` |
| Matching query, Cyrillic (full-text match against the Russian-language description) | `GET /search/suggestions?q=контейнер` | **200**, `[{"title":"Docker", ...}]` |
| Non-matching query | `GET /search/suggestions?q=xyznonexistentquery` | **200**, `[]` |

All 6 cases (the 4 required plus 2 extra: explicit-empty and Cyrillic) passed on the first attempt — no code changes needed after the initial implementation.

**Main course search regression spot-check** (explicit "do not modify main search behavior" requirement — this session's only change to shared code was extracting `splitSearchQuery` out of `SearchCourses`'s previous inline logic in Stage 22A1, so re-confirming that refactor is still behavior-preserving after this session's own rebuild): `GET /courses?q=docker` → `total: 1`, `["Docker"]`; `GET /courses?q=go` → `total: 1`, `["Go Backend Developer"]`; `GET /courses` (no query, full browse) → `total: 3`. All match pre-Stage-22 behavior exactly.

### Not done this session (explicitly out of scope for 22A2)

- **No frontend** — no autocomplete dropdown, no debouncing, no client code of any kind.
- **No draft-course leakage test performed live** — not one of the four required test cases this session; the query itself hardcodes `WHERE courses.published = true` (identical to `SearchCourses`, verified by code inspection in 22A1), so no draft course can appear regardless. Worth an explicit live check in a future security-focused Stage 22 session (create a draft course, confirm it never appears in suggestions) rather than assumed here.
- **No regression pass beyond the main-search spot-check above.**

## Stage 22B1 — course search autocomplete frontend (this session)

Scope: enhance the existing course search input with a suggestions dropdown, wired to Stage 22A2's `GET /search/suggestions`. No backend changes, no courses-page redesign, no E2E/regression pass.

### Inspection performed

Re-read `STAGE22_PROGRESS.md`'s Stage 22A1/22A2 sections and the frontend's existing search surface fresh. Found there is exactly one course search input in this codebase: `components/CourseFilters.tsx`'s `<input type="search" name="q">`, inside a plain `<form action={basePath} method="GET">` (used by both `/courses` and `/categories/[slug]` via `CourseListing.tsx`) — no navbar search input exists, so "the existing course search input" is unambiguous. Its own doc comment explicitly documented the pre-Stage-22 state: "no client JS, no per-keystroke fetches." Confirmed `/courses/[id]/page.tsx` and every existing course link (`CourseCard.tsx`) route by real course `id` (`/courses/${course.id}`), not slug — so suggestion clicks needed to target that same `id`-based path. Checked `lib/api.ts`'s `apiBaseUrl()` (resolves to `NEXT_PUBLIC_API_URL` in the browser vs `API_INTERNAL_URL`/`SERVER_API_URL` on the server) and confirmed existing public, unauthenticated reads (`getCourses`, `getCategories`) are plain exported functions with no `"use server"` restriction — callable directly from a `"use client"` component, unlike `lib/actions.ts`'s server actions. This is the established pattern for a client-side fetch to a public endpoint in this codebase, so a new `getCourseSuggestions` was added there rather than routing through a server action (which would add an unnecessary extra hop for a public, tokenless, per-keystroke read).

### Change made

`frontend/lib/api.ts`:
- New `CourseSuggestion` interface, mirroring the backend's JSON shape (`id`, `title`, `slug`, `category_name?`) exactly.
- New `getCourseSuggestions(query, signal?)` — `GET /api/v1/search/suggestions?q=`, no auth header (public endpoint), accepts an optional `AbortSignal` so the component can cancel a stale in-flight request.

`frontend/components/SearchAutocomplete.tsx` (new, `"use client"`):
- Renders the actual `<input type="search" name="q">` itself (replacing the one that used to live directly in `CourseFilters.tsx`), so it stays inside the same `<form>` under the same field name — a plain Enter-to-submit or button-click search still works exactly as before; autocomplete is additive.
- **Debounce**: 250ms via a `setTimeout` ref, cleared on every keystroke; **never fires for an empty/whitespace-only query** — that case short-circuits immediately (no timer, no request, dropdown closes).
- **Race-safety**: an `AbortController` per request, aborting any still-in-flight previous request before starting a new one, so a fast typist can never have an older response overwrite a newer one.
- **Dropdown contents**: course title + `category_name` (only rendered when present, since the backend field is optional) per suggestion, plus a quiet single-line "Загрузка..." while a request is in flight and "Ничего не найдено" when a completed search has zero results — both rendered as plain muted text inside the dropdown itself, not a toast/alert, per the "no noisy UI" requirement.
- **Error handling**: a failed fetch (network error or non-2xx) is caught and treated identically to zero results — no distinct error banner. Deliberate simplification: a public, read-only suggestions dropdown has no actionable recovery step for the user beyond "try again," so a separate error state would only add noise without adding value.
- **Selection**: clicking a suggestion (or pressing Enter while one is highlighted) calls `router.push(`/courses/${id}`)` using the real `id` the API returned — never a client-constructed guess.
- **Keyboard**: `ArrowDown`/`ArrowUp` cycle the highlighted index (wrapping both directions), `Enter` selects the highlighted suggestion (and only then — with nothing highlighted, Enter is left alone so the surrounding form's native submit still fires, preserving the original full-search fallback), `Escape` closes the dropdown without navigating.
- **Closing**: on selection (above), on `Escape`, and on an outside `mousedown` (a `document`-level listener checked against a wrapper `ref`). Each suggestion button uses `onMouseDown={(e) => e.preventDefault()}` — the standard technique to stop the input from blurring (and the dropdown from closing) before its own `onClick` can fire.
- Basic combobox ARIA (`role="combobox"`/`aria-expanded`/`aria-controls` on the input; `role="listbox"`/`role="option"`/`aria-selected` on the dropdown) — no new interaction pattern invented, just labeling the one already built.

`frontend/components/CourseFilters.tsx`:
- Swapped the plain `<input>` for `<SearchAutocomplete defaultValue={current.q} />`. Nothing else in the form changed — category/level/access/sort `<select>`s and the submit button are untouched.

`frontend/app/globals.css`:
- `.search-autocomplete` (positions the dropdown relative to the input, same `flex: 1 1 220px` the old input had via `.filter-bar input[type="search"]`, now applied to the wrapper instead), `.search-autocomplete-dropdown`, `.search-autocomplete-status`, `.search-autocomplete-item` (+ `:hover`/`.is-highlighted`), `.search-autocomplete-title`, `.search-autocomplete-category`. Every value reuses existing design tokens (`var(--surface)`, `var(--border)`, `var(--radius-md)`/`var(--radius-sm)`, `var(--bg-elevated)`, `var(--text)`/`var(--text-muted)`, `var(--shadow-md)` for the dropdown's drop shadow) — no new colors introduced.

### Files changed

- `frontend/lib/api.ts` — `CourseSuggestion` type, `getCourseSuggestions`.
- `frontend/components/SearchAutocomplete.tsx` — new client component.
- `frontend/components/CourseFilters.tsx` — swapped the plain input for `SearchAutocomplete`.
- `frontend/app/globals.css` — new `.search-autocomplete*` rules.
- No backend files touched.

### Verification performed (focused, per scope)

- `npx tsc --noEmit` (whole project) — clean.
- `npx eslint components/SearchAutocomplete.tsx components/CourseFilters.tsx lib/api.ts` — clean, zero warnings. (One intermediate issue caught and fixed during this session: an initial `useEffect` syncing `value` to a changed `defaultValue` prop tripped the `react-hooks/set-state-in-effect` rule — removed rather than worked around, since every real navigation path in this app (native form GET submit, or `router.push` to a different course-detail route) fully remounts this component anyway, making the sync effect dead weight, not a real requirement.)
- `app/globals.css` — the usual `eslint` "file ignored, no matching configuration" warning (CSS isn't linted by this project's ESLint config), same as every prior stage's CSS-only changes; not an error.

No live browser interaction, no Docker Compose rebuild, no E2E/regression pass — explicitly out of scope for 22B1. The backend endpoint this component calls was already live-verified in Stage 22A2 (empty/short/matching/non-matching queries, Cyrillic full-text match); this session's job was limited to confirming the frontend calls it correctly and renders/behaves as specified, which `tsc`'s type-checking of `CourseSuggestion`/`getCourseSuggestions` against the component's usage plus the code-level review above covers for this focused pass.

## Stage 22C — focused verification + final Stage 22 report (this session)

Scope: verify 22A1+22A2 (backend) and 22B1 (frontend) live together, fix only bugs the verification itself surfaced, close out Stage 22. No new features attempted.

### Setup

Rebuilt `frontend` via `docker compose up -d --build frontend` — it was still serving the pre-22B1 image going into this session (backend already current from 22A2). Confirmed both healthy (`GET /api/v1/health` → 200, `GET /courses` → 200) with no route-conflict panic in `docker compose logs backend`.

### Bug found and fixed: Escape didn't close the dropdown in its empty/loading states

**Found during this session's own code re-review** (not live-observed, since no browser-automation tool is available in this environment — see Known limitations): `handleKeyDown`'s guard was `if (!open || suggestions.length === 0) return;`, placed *before* the `switch` that handles `Escape`. Since the dropdown's "Загрузка..." (loading) and "Ничего не найдено" (empty-result) states both have `suggestions.length === 0` while `open` is `true`, this guard silently swallowed **every** keypress — including Escape — in exactly the two states where a user would most want to dismiss the dropdown with it. ArrowDown/ArrowUp/Enter correctly doing nothing in those states is fine (there's nothing to navigate), but Escape closing the dropdown doesn't depend on there being a list.

**Fix**: restructured `handleKeyDown` in `frontend/components/SearchAutocomplete.tsx` so `Escape` is checked immediately after the `open` guard (and returns), before the `suggestions.length === 0` guard that gates the list-navigation keys. Escape now closes the dropdown in every state it can be open in — loading, empty-result, and populated.

Re-ran `npx tsc --noEmit` and `npx eslint components/SearchAutocomplete.tsx` after the fix — both clean. Rebuilt both Docker Compose services and re-confirmed the SSR-rendered `/courses` page still renders the same correct `<input>` markup post-fix (no unrelated regression from the edit).

### 1. Backend suggestions endpoint — verified live (fresh, post-rebuild)

| Case | Request | Result |
|---|---|---|
| Empty query (no param) | `GET /search/suggestions` | **200**, `[]` |
| Empty query (`q=`) | `GET /search/suggestions?q=` | **200**, `[]` |
| Short query, ILIKE path | `GET /search/suggestions?q=go` | **200**, `[{"title":"Go Backend Developer","category_name":"Programming",...}]` |
| Matching query, tsquery path | `GET /search/suggestions?q=docker` | **200**, `[{"title":"Docker","category_name":"DevOps",...}]` |
| Non-matching query | `GET /search/suggestions?q=xyznonexistentquery` | **200**, `[]` |
| Bounded result count | `GET /search/suggestions?q=o` (broadest matchable query) | **200**, 3 results — the entire catalog, i.e. never more than exists |

All 6 cases passed. **Bounded-count caveat, stated plainly**: the live catalog has only 3 courses total, so no query can produce more than 3 results — the code-level cap (`suggestionLimit = 8`, hardcoded in the service, not client-controllable via any query param) cannot be observed actually truncating a result set at this data scale. This was already verified by code inspection and reasoning in Stage 22A1/22A2 (there is no `limit`/`page` parameter the handler reads at all, so a caller has no way to request more even in principle); re-confirmed unchanged this session, not re-provable live without seeding 9+ courses, which is out of scope for a verification-only session.

### 2. Frontend autocomplete — verified live where possible; rest verified by code + indirect live checks

No browser-automation tool is available in this environment (confirmed via `ToolSearch` this session — only `WebFetch`, unsuitable for local JS-driven interaction), so the purely client-side-JS-runtime behaviors below could not be driven by an actual keypress/click. Verified instead via the strongest available substitutes: SSR HTML inspection, direct reproduction of the exact HTTP calls the component makes, and a fresh line-by-line code re-read (which is what surfaced the Escape bug above).

| Requirement | How verified | Result |
|---|---|---|
| Debounce works | Code re-read: single `setTimeout(…, 250)` ref, cleared and reset on every `onChange` | Correct by construction; exact 250ms timing not runtime-measured |
| Dropdown renders correct suggestions | The exact `GET /search/suggestions?q=` calls the component makes were live-verified in Part 1 above; `getCourseSuggestions`'s response typing was `tsc`-checked against `CourseSuggestion` | Data path confirmed correct end-to-end |
| Click navigation works | Indirect live check: fetched `GET /courses/88888888-8888-8888-8888-888888888888` (the exact URL `selectSuggestion` would `router.push` to for the "Docker" suggestion) → **200**, `<h1>Docker</h1>` | Target resolves correctly; the click event dispatch itself not simulated |
| ArrowDown/ArrowUp work | Code re-read: modulo-wrapping index cycling, guarded by `open && suggestions.length > 0` | Correct by construction |
| Enter works | Code re-read: selects `suggestions[highlighted]` only when `highlighted >= 0`, else falls through to native submit (unchanged `preventDefault` behavior) | Correct by construction |
| Escape closes dropdown | **Bug found and fixed this session** (see above) — now closes in every open state | **Fixed and re-verified via `tsc`/`eslint`** |
| Outside click closes dropdown | Code re-read: `document`-level `mousedown` listener checked against `wrapperRef.contains` | Correct by construction; not runtime-triggered |
| Empty/error state is safe | Code re-read: fetch wrapped in `try/catch`, error path sets `suggestions([])` and never throws past the handler; "Ничего не найдено" and network-error land on the identical quiet UI, no alert/banner | Confirmed by inspection |
| Normal search form submit still works | **Live**: `GET /courses?q=docker` (exactly what a submitted form produces) → **200**, input `value="docker"`, only "Docker" in results; `GET /courses?q=go` → only "Go Backend Developer"; `GET /courses` → all 3 courses | Confirmed live, real HTTP requests |

### 3. Main `/courses` search behavior — verified unchanged

Live, post-rebuild: `GET /courses?q=docker` → exactly 1 result ("Docker"), neither "PostgreSQL" nor "Go Backend Developer" present. `GET /courses?q=go` → exactly 1 result ("Go Backend Developer"), neither of the other two present. `GET /courses` (no query) → all 3 courses, full browse intact. All three match Stage 22A2's own regression spot-check exactly — the `CourseFilters`/`SearchAutocomplete` swap did not disturb `SearchCourses`'s filtering/ranking in any way.

### Final checks

- `gofmt -l .` (whole backend) — clean.
- `go build ./...` — OK.
- `go vet ./...` — OK.
- `npx tsc --noEmit` (whole frontend) — clean.
- `npx eslint .` (whole frontend) — 0 errors, same 4 pre-existing unrelated `<img>` warnings every prior stage has reported, nothing new.
- Docker Compose: both `backend` and `frontend` rebuilt and live; `docker compose logs backend` confirms `GET /api/v1/search/suggestions` registered with no route-conflict panic.

### Files changed (all of Stage 22: 22A1 + 22A2 + 22B1 + 22C)

Backend:
- `backend/internal/courses/model.go` — `CourseSuggestion` type (22A1).
- `backend/internal/courses/repository.go` — `splitSearchQuery` extraction, `SuggestCourses` (22A1).
- `backend/internal/courses/service.go` — `Service.SuggestCourses`, `suggestionLimit`/`suggestionQueryMaxRunes` (22A1).
- `backend/internal/courses/handler.go` — `GET /search/suggestions` route + handler (22A2).

Frontend:
- `frontend/lib/api.ts` — `CourseSuggestion` type, `getCourseSuggestions` (22B1).
- `frontend/components/SearchAutocomplete.tsx` — new client component (22B1); `handleKeyDown` Escape-guard fix (22C).
- `frontend/components/CourseFilters.tsx` — swapped plain input for `SearchAutocomplete` (22B1).
- `frontend/app/globals.css` — `.search-autocomplete*` rules (22B1).

No migration at any point in Stage 22 — verified in 22A1 that the existing `idx_courses_search_vector` GIN index is sufficient at the current catalog scale; no `pg_trgm` added.

### Known limitations (Stage 22, final)

- **No real browser interaction anywhere in Stage 22.** Every purely-client-side-JS-runtime behavior (exact debounce timing, actual keypress/click dispatch, actual outside-click trigger) was verified by code inspection plus the strongest available live substitutes (SSR HTML, direct API reproduction, direct navigation-target checks) rather than an actual driven interaction — no browser-automation tool is available in this environment. This is the same limitation `STAGE21_PROGRESS.md`'s Stage 21C section already documented for that stage's frontend verification; it is a property of the environment, not something either stage could have closed. Lower risk here specifically because the one bug this class of limitation could plausibly hide (the Escape/empty-state guard ordering) was still caught by careful code re-reading, not missed.
- **Suggestion cap (8) unverified at truncation** — the live catalog only has 3 courses, so the hard cap the service enforces has never been observed actually cutting a result set short. Confirmed by code inspection that no request parameter can override it either way.
- **Draft/unpublished-course exclusion unverified live** — the query hardcodes `published = true` (verified by code inspection in 22A1, identical to `SearchCourses`'s own filter), but no live test created a draft course and confirmed it never appears in suggestions. Same standing gap noted at the end of Stage 22B1, still open.
- **No `pg_trgm` index** — deliberately not added; the current 3-row catalog doesn't need it (see 22A1's `EXPLAIN ANALYZE`). Revisit if the catalog grows enough for the ILIKE fallback's sequential scan to actually cost something in a real plan.
- No automated test suite exists anywhere in this codebase (unchanged, project-wide convention) — every verification claim above is a live, scripted check against the running Docker Compose stack, or a documented code-level review substituting for one where live interaction wasn't reachable.

### Final Stage 22 status

**Stage 22 is complete for its scoped ambition (search suggestions/autocomplete).** The backend suggestion query (22A1), its public HTTP endpoint (22A2), and the frontend autocomplete UI wired to it (22B1) are all implemented and live-verified, including this session's own fresh pass across both layers together — which caught and fixed one real bug (Escape not dismissing the dropdown in its empty/loading states) that no prior session's narrower-scoped verification had exercised. Zero other known bugs. The items listed under "Known limitations" are either genuine environment constraints (no browser tool) or deliberately deferred, documented scope boundaries (pg_trgm, draft-exclusion live-test, cap-truncation live-test) — not defects — consistent with how Stage 21 closed out.
