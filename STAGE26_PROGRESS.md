# Stage 26 — Auth hardening: rate limiting & session security

Tracking doc — status only, not a spec restatement.

## Stage 26A1 — authentication rate limiting (this session)

Scope: rate limiting on `/auth/login` and `/auth/register` only. No session/JWT changes, no frontend, no Redis, no full regression.

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 26 section fresh. Read `backend/internal/auth/{handler,middleware,service}.go` in full — confirmed the exact endpoints (`POST /auth/register`, `POST /auth/login`) and that **no `/auth/refresh` endpoint exists in this codebase at all** (the roadmap's sketch mentioned one, but this backend issues a single fixed-TTL JWT with no refresh flow), so instruction 1's "starting with: login, register if appropriate" correctly covers everything that actually exists — there is nothing else to guard.

Checked the roadmap's claim that this should "mirror the token-bucket pattern already used in `internal/coding`" — read `internal/coding/service.go`'s `checkRateLimit` and found it is **not** an in-memory token bucket at all: it's two DB queries (`CountRecentSubmissions`, `CountActiveSubmissions`) counting rows in an existing, already-authenticated-user-scoped submissions table. That mechanism doesn't transfer to login/register, which are **unauthenticated** (no `userID` exists before a successful login) and would need a new table + migration just to count pre-auth attempts — the roadmap's own migration-needs line already anticipated this exact fork ("likely none if rate limiting is in-process... a table only if a durable, multi-instance-safe limiter is judged necessary"). Confirmed this backend runs as a single replica: no `replicas`/`scale` entry for the `backend` service in `docker-compose.yml`, no reverse proxy/load balancer in front of it — so per instruction 2 ("use a simple bounded in-memory limiter unless the existing architecture already has a suitable shared mechanism"), an in-process limiter is the correct choice, not a compromise.

Checked `cmd/api/main.go`'s Gin setup and found `router := gin.Default()` with **no `SetTrustedProxies` call** — the exact condition that produces Gin's own startup warning ("You trusted all proxies, this is NOT safe") seen in every prior stage's `docker compose logs backend` output. Under this configuration, `c.ClientIP()` honors an `X-Forwarded-For`/`X-Real-IP` header from **any** caller — directly relevant to instruction 3's "must not trust spoofable ... fields," since an attacker could rotate that header per request and hand the limiter a fresh "client" every time, defeating it entirely. Did not inspect any unrelated domain.

### Design decisions

- **`net.Request.RemoteAddr`, never `c.ClientIP()`, for the rate-limit key.** `RemoteAddr` is set by Go's `net/http` server from the actual TCP connection — never from client-controlled request data — so it can't be spoofed the way a header can. Trade-off stated plainly in code and here: this becomes wrong the day a real reverse proxy sits in front of this backend (every request would then show the proxy's own address, bucketing every real client behind it together); there is none today, confirmed above, so that day hasn't come. Revisiting this alongside a real `SetTrustedProxies` configuration is explicitly out of scope for this session (that's session-security/infra work, not this stage's rate-limiting slice).
- **Key = `endpoint + "|" + clientIP`, not IP alone.** This satisfies instruction 3's "IP + endpoint" combination directly and gives two independent guarantees verified live below: (a) exhausting `/auth/login`'s bucket for an IP never blocks `/auth/register` from that same IP, and (b) an unrelated endpoint (e.g. `/courses`, `/health`) is never touched by this middleware at all, since it's registered only on the two guarded routes.
- **One shared `RateLimiterConfig` (max attempts + window), not per-endpoint tunables.** Scope is "add rate limiting," not "give every endpoint its own dial" — per-endpoint isolation still holds structurally because the endpoint name is baked into the bucket key, even though both routes are configured with the same numbers.
- **Every request counts toward the window — success or failure alike.** A credential-stuffing script's occasional correct guess should throttle exactly like its failures do; this is standard practice for a login-endpoint limiter and does not conflict with instruction 6 ("do not change login/register success behavior") — a request that's still under the threshold reaches `Register`/`Login` completely untouched, with the exact same status/body/token-issuance logic as before. Only requests past the threshold are turned away, before the handler ever runs.
- **Fixed-window counter, not a leaky bucket/sliding window.** The simplest correct implementation that satisfies every stated requirement (bounded, in-memory, resets after its window, `Retry-After` is easy to compute exactly as "time left in the current window"). A sliding-window or token-bucket-with-refill algorithm would be more precise about burst smoothing but adds complexity this stage's scope doesn't call for.
- **Bounded via a periodic background sweep, not an unbounded map.** Each distinct `(endpoint, IP)` pair creates one map entry; without cleanup, long uptime with many distinct source IPs (including scanners) would grow the map indefinitely. A goroutine sweeps entries idle for more than 30 minutes every 10 minutes — generous enough that no live bucket is ever evicted while it could still matter.
- **Config via environment variables, matching the existing `internal/config` convention exactly** (`getEnvInt` + a matching `docker-compose.yml` `${VAR:-default}` entry, the same two-layer default pattern every existing `CODE_RUNNER_*`/`VIDEO_*` var already uses) — `AUTH_RATE_LIMIT_MAX_ATTEMPTS` (default 10) and `AUTH_RATE_LIMIT_WINDOW_SEC` (default 300, i.e. 5 minutes). Defaults chosen to be permissive enough that a legitimate user retrying a mistyped password a few times, or a household/NAT sharing one IP, is never caught, while still bounding a scripted burst.

### Change made

`backend/internal/auth/ratelimit.go` (new):
- `RateLimiterConfig{MaxAttempts, Window}`.
- `RateLimiter` — `sync.Mutex`-guarded `map[string]*bucket`, `NewRateLimiter` starts a background cleanup goroutine.
- `Limit(endpoint string) gin.HandlerFunc` — the middleware itself: fixed-window increment-and-check, `Retry-After` header + `429 RATE_LIMITED` JSON body when exceeded, otherwise `c.Next()` with zero effect on the real handler.
- `clientIP(c) string` — `RemoteAddr`-based, not `c.ClientIP()` (see design decisions).

`backend/internal/auth/handler.go`:
- `RegisterRoutes` gained a `rateLimiter *RateLimiter` parameter; both routes now go through `rateLimiter.Limit("register")`/`rateLimiter.Limit("login")` before `h.Register`/`h.Login`. Neither handler function's own body changed at all.

`backend/internal/config/config.go`:
- `AuthRateLimitMaxAttempts`, `AuthRateLimitWindowSec` fields, loaded via `getEnvInt` with the defaults above.

`backend/cmd/api/main.go`:
- Constructed `authRateLimiter := auth.NewRateLimiter(...)` right after `authMiddleware`, passed it into `authHandler.RegisterRoutes(v1, authRateLimiter)`.

`docker-compose.yml`:
- `AUTH_RATE_LIMIT_MAX_ATTEMPTS: ${AUTH_RATE_LIMIT_MAX_ATTEMPTS:-10}`, `AUTH_RATE_LIMIT_WINDOW_SEC: ${AUTH_RATE_LIMIT_WINDOW_SEC:-300}` added to the `backend` service's `environment:` block, matching the existing `CODE_RUNNER_*` entries' exact shape.

No changes to `internal/auth/{jwt,password,service}.go` — JWT/session logic is completely untouched, per instruction 7. No migration — per instruction 8/the roadmap's own conditional, none was needed.

### Files changed

- `backend/internal/auth/ratelimit.go` — new.
- `backend/internal/auth/handler.go` — `RegisterRoutes` signature + route wiring.
- `backend/internal/config/config.go` — two new fields.
- `backend/cmd/api/main.go` — `authRateLimiter` construction + updated `RegisterRoutes` call.
- `docker-compose.yml` — two new env entries on the `backend` service.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/auth/*.go internal/config/*.go cmd/api/main.go` — one formatting issue in the new file, fixed via `gofmt -w`; clean afterward.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**Live** (Docker Compose backend rebuilt with a temporary small override — `AUTH_RATE_LIMIT_MAX_ATTEMPTS=3 AUTH_RATE_LIMIT_WINDOW_SEC=8`, exported for one `docker compose up -d --build backend` — so the full exceed/recover cycle could be exercised in seconds rather than minutes; reverted to production defaults, 10/300, with a final rebuild once testing finished, confirmed via `docker compose exec backend printenv`):

| Case | Result |
|---|---|
| Normal login still works | Fresh account registered, logged in successfully — **200**, real token issued |
| Several valid/invalid attempts below threshold | 2 valid logins + 1 invalid-credentials attempt (3 total, at but not over the threshold) — **200, 200, 401** respectively, never 429; the 401 case in particular confirms a *failed* login still gets its normal error response, not silently swallowed by the limiter |
| Threshold exceeded → 429 | 4th rapid request in the same window — **429**, body `{"error":{"code":"RATE_LIMITED","message":"too many attempts, please try again later"}}`, `Retry-After: 7` header present |
| Limiter recovers after its window | Exhausted the limit (3 ok, 4th → 429), waited 9s (> the 8s test window), next attempt — **200** again, normal token issued |
| Unrelated endpoints are not rate-limited | With `/auth/login`'s bucket for the test IP fully exhausted: `GET /health` → **200**, `GET /courses` → **200** — completely unaffected |
| `/auth/register` is not blocked by an exhausted `/auth/login` bucket (same IP, different endpoint key) | **201** — succeeded normally even while that IP's login bucket was still exhausted |
| Separate clients/keys do not share a bucket | First attempt (sequential, `--rm` containers) was inconclusive by accident — Docker reassigned the same released IP to the second container *after* its predecessor's window had already naturally elapsed, which looked like isolation but didn't prove it. Corrected by running **two containers genuinely concurrently** (`docker run` in parallel via backgrounded shell jobs, same Docker network, real distinct IPs `192.168.192.10`/`192.168.192.11` confirmed via `hostname -i` inside each): both independently got exactly 3 successful logins followed by their own 429 on the 4th, with neither affecting the other's count — a real, concurrent, cross-client isolation proof, not a timing coincidence |

All required scenarios passed, with one internal correction (see below) made during the session's own verification, before any result was reported as final.

### One test-methodology issue caught and corrected mid-session (not a product bug)

Two separate points during this session's own live testing initially looked like anomalies and were investigated rather than either dismissed or reported as bugs:

1. An early threshold test issued 4 login requests split across two separate tool calls with real reasoning time in between; the 4th request unexpectedly succeeded. Investigated by converting timestamps: ~20 seconds of genuine wall-clock time had elapsed between the two tool calls — well past the 8-second test window — so the bucket had legitimately reset. Root cause: test pacing, not the limiter. Corrected by re-running the same scenario as one tight, single-command loop with no inter-call gaps, which then reproduced the exceed/recover behavior exactly as expected (see the table above).
2. The first cross-client isolation attempt reused the same container IP for "Client A" and "Client B" (Docker reassigns released addresses from its pool), and Client B's login succeeded without hitting Client A's exhausted count. Investigated the same way — Client A's own window had already elapsed by the time Client B's container started, so this was a second instance of the same root cause (sequential tool calls, not a tight burst), not a bucket-sharing bug. Corrected by running both clients truly concurrently (see above), which produced the real, conclusive proof of isolation.

Both are documented here explicitly rather than silently reconciled, since an anomaly that turns out to have an innocent explanation is still worth recording — it's what "live scripted verification" catching its own test-setup mistakes looks like, the same discipline every prior stage's C-session applied to its own live checks.

### Not done this session (explicitly out of scope for 26A1)

- **No `/auth/refresh` guard** — none exists in this codebase to guard.
- **No session/JWT changes** — `internal/auth/{jwt,password,service}.go` untouched, per instruction 7.
- **No `SetTrustedProxies` configuration** — the underlying reason `clientIP()` had to avoid `c.ClientIP()` in the first place; fixing that global Gin config is session-security/infra-adjacent work, not this stage's rate-limiting slice, and is noted as a real follow-up the moment a reverse proxy is introduced.
- **No frontend changes** — no backoff/error messaging added; the roadmap's own frontend scope line for this ("user-facing error/backoff messaging only") is explicitly deferred to a future sub-stage.
- **No full regression pass** — out of scope for this focused session.

## Stage 26A2 — session/JWT security hardening (this session)

Scope: review and harden the existing JWT/session behavior against instruction 2's checklist. No refresh tokens, no auth-system redesign, no frontend, no OAuth.

### Inspection performed

Re-read `STAGE26_PROGRESS.md`'s Stage 26A1 section fresh, then `backend/internal/auth/{jwt,middleware,service,handler}.go` in full — every file that issues, transmits, or validates a token. Went through instruction 2's checklist item by item against the existing code before writing anything:

| Checklist item | Finding |
|---|---|
| Token expiration | Set correctly in `GenerateToken` (`ExpiresAt`); but the library's own default is to treat a token with **no** `exp` claim as never-expiring — `WithExpirationRequired` wasn't being passed, so a hand-crafted token omitting `exp` would have been accepted as valid forever. **Gap — fixed.** |
| Signature validation | Already correct — `jwt.ParseWithClaims` verifies the signature via the keyfunc's returned secret; no code path skips this. |
| Allowed signing algorithm | The old keyfunc only checked `t.Method.(*jwt.SigningMethodHMAC)` — *any* HMAC variant (HS256/HS384/HS512), not specifically the one algorithm `GenerateToken` ever issues. Confirmed live below that an HS384-signed token (valid signature, wrong algorithm) was accepted by the *old* logic's class of check. **Gap — fixed** via `jwt.WithValidMethods`, the library's own documented defense against algorithm-confusion attacks. |
| Missing/invalid Authorization header | Already correct — `middleware.go`'s `RequireAuth` checks empty header, missing `"Bearer "` prefix, and empty token-after-prefix, all three as 401. |
| Malformed token handling | Already correct — `jwt.ParseWithClaims` errors out on structurally invalid input, mapped to `ErrInvalidToken` → 401. |
| Expired token handling | Already correct for tokens that *do* carry `exp` (the only kind `GenerateToken` ever produces) — see the "no exp claim" gap above for the one case this didn't cover. |
| Role/user claims taken only from validated token | Already correct — `authctx.SetUserID`/`SetRole` are only ever called after `ParseToken` returns successfully; nothing reads claims from an unverified decode anywhere in this codebase. |
| No trust in client-supplied role/user_id | Verified codebase-wide, not just asserted: grepped every `json:"user_id"`/`json:"role"`/`json:"reporter_user_id"`/`json:"actor_user_id"` struct tag in `internal/`. Every hit is either a **response** shape (what the server returns, e.g. `Question`, `AuditLog`, `AdminReport`) or `internal/users/handler_admin.go`'s `updateUserAdminRequest.Role` — the one legitimate case, since that's an admin (already authenticated via their own verified JWT, already gated by `RequireRole("admin")` middleware) setting a **target** user's role via `:id` in the URL, never the caller's own identity. No handler anywhere binds a `user_id`/`role` field from a request body and uses it in place of `authctx`. |

Also confirmed (relevant to the algorithm-pinning fix specifically): `cmd/api/main.go` runs `gin.Default()` with no `SetTrustedProxies` call, already documented as a known, deliberately-deferred gap in Stage 26A1's own progress notes — unrelated to this session's JWT-specific scope, not touched again here.

### Design decisions

- **`jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name})` replaces the old keyfunc type-assertion, rather than sitting alongside it.** `WithValidMethods` is checked by the parser *before* the keyfunc is even invoked, so keeping the old `*jwt.SigningMethodHMAC` type-check afterward would have been unreachable dead code for every case it used to catch (alg confusion with `"none"` or an asymmetric algorithm) — those are now rejected earlier, by the officially-recommended mechanism. The old check wasn't wrong, just superseded by a strictly stronger, more precise one; keeping both would have been confusing rather than genuinely layered defense.
- **`jwt.WithExpirationRequired()` added.** Every token `GenerateToken` issues already sets `ExpiresAt`, so this changes nothing for any legitimate token — it only closes off a hand-crafted token that omits `exp` entirely, which the library's own documented default (*"By default exp claim is optional"*) would otherwise have accepted as never-expiring.
- **JWT TTL is now configurable (`JWT_ACCESS_TOKEN_TTL_MINUTES`, default 1440 = 24h), not a package-level const.** Instruction 3's condition ("too loose OR hardcoded") was satisfied by "hardcoded" alone — the 24h *value* itself wasn't judged excessive for a system with no refresh flow, so the default is unchanged, only its hardcoding is removed. Threaded through `config.Config` → `auth.Service` (a new `accessTokenTTL time.Duration` field) → `GenerateToken`'s now-required `ttl` parameter, following the exact `getEnvInt` + `docker-compose.yml` `${VAR:-default}` pattern Stage 26A1 already established for the rate-limit config.
- **No refresh-token introduction, no `SetTrustedProxies` change, no algorithm switch away from HMAC.** All explicitly out of scope per instructions 4/5/7 and Stage 26A1's own already-documented deferral, respectively — this session is a hardening pass on what exists, not a redesign.

### Change made

`backend/internal/auth/jwt.go`:
- Removed the package-level `const AccessTokenTTL = 24 * time.Hour`.
- `GenerateToken` gained a `ttl time.Duration` parameter (was previously an implicit 24h via the removed const).
- `ParseToken`'s `jwt.ParseWithClaims` call now passes `jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name})` and `jwt.WithExpirationRequired()`; the keyfunc itself simplified to just return the secret (the algorithm check now lives entirely in `WithValidMethods`, checked earlier by the parser).

`backend/internal/auth/service.go`:
- `Service` gained an `accessTokenTTL time.Duration` field; `NewService` gained a matching parameter.
- `Login`'s `GenerateToken` call now passes `s.accessTokenTTL`.

`backend/internal/config/config.go`:
- `JWTAccessTokenTTLMinutes int` field, loaded via `getEnvInt("JWT_ACCESS_TOKEN_TTL_MINUTES", 1440)`.

`backend/cmd/api/main.go`:
- `auth.NewService(usersService, cfg.JWTSecret, time.Duration(cfg.JWTAccessTokenTTLMinutes)*time.Minute)`.

`docker-compose.yml`:
- `JWT_ACCESS_TOKEN_TTL_MINUTES: ${JWT_ACCESS_TOKEN_TTL_MINUTES:-1440}` added to the `backend` service's `environment:` block.

No changes to `internal/auth/{middleware,password,handler}.go` — `RequireAuth`'s header-parsing logic, password hashing, and the `Register`/`Login` handler bodies were all already correct and are untouched. No refresh-token code added anywhere. No frontend file touched.

### Files changed

- `backend/internal/auth/jwt.go` — algorithm pinning, required expiration, configurable TTL parameter.
- `backend/internal/auth/service.go` — `accessTokenTTL` field/parameter, `Login` passes it through.
- `backend/internal/config/config.go` — `JWTAccessTokenTTLMinutes`.
- `backend/cmd/api/main.go` — updated `auth.NewService` call.
- `docker-compose.yml` — one new env entry.

### Verification performed

**Static (focused, per scope):**
- `gofmt -l internal/auth/*.go internal/config/*.go cmd/api/main.go` — clean, no output.
- `go build ./...` (whole backend) — OK.
- `go vet ./...` — OK.

**Unit-level, precise** (a temporary `internal/auth/verify_scratch_test.go`, run once via `go test ./internal/auth/... -run TestScratchVerifyStage26A2 -v`, deleted immediately after — matching Stage 25A1's established "no automated test suite, live scripted verification instead" technique for exercising code paths hard to reach through the live HTTP surface alone):

| Case | Result |
|---|---|
| Valid token round-trips (`GenerateToken` → `ParseToken`) | **PASS** — claims match exactly |
| Expired token (`exp` in the past) | **PASS** — rejected |
| Wrong secret | **PASS** — rejected |
| Tampered signature (last 4 chars of a valid token corrupted) | **PASS** — rejected |
| HS384-signed (valid signature under HS384, wrong algorithm) | **PASS** — rejected, proving `WithValidMethods` actually does something beyond the old "any HMAC" check |
| `alg=none` (unsecured JWT, `role: "admin"` claimed) | **PASS** — rejected |
| Validly-signed, **no `exp` claim at all**, `role: "admin"` claimed | **PASS** — rejected, proving `WithExpirationRequired` actually does something (this exact token would have been accepted as never-expiring under the pre-session code) |
| Manipulated role claim, correctly re-signed with the real secret | **PASS** — accepted, and deliberately so: this sub-test documents the actual security boundary (only code holding the secret can mint claims; a client without the secret cannot, which every rejection case above demonstrates) rather than asserting a new behavior |

**Live, against the real running backend** (Docker Compose backend rebuilt via `docker compose up -d --build backend`, no panic; several of the exact forged tokens from the unit-level pass above were fed to `GET /api/v1/me`, bridging unit-level correctness with proof that the *deployed binary* enforces it, not just an isolated function):

| Case | Result |
|---|---|
| Missing Authorization header | **401** `UNAUTHORIZED` |
| Malformed header (no `Bearer` prefix) | **401** |
| Malformed token (`"this.is.not.a.valid.jwt"`) | **401** `invalid or expired token` |
| Empty bearer token | **401** |
| Expired token (real HS256, correct secret, `exp` in the past) | **401** |
| Token signed with the wrong secret | **401** |
| **Validly-signed, no `exp`, `role: "admin"`** | **401** — confirms the live deployed binary rejects this, not just the unit test in isolation |
| **HS384-signed, valid signature, wrong algorithm** | **401** — same live-deployment confirmation for the algorithm-pinning fix |
| Normal login still issues a valid usable token | Fresh account registered → logged in (**200**, real token) → that exact token used on `GET /api/v1/me` → **200**, correct user data returned |
| Configurable TTL genuinely drives token generation | Decoded a real issued token's `exp`/`iat`: **1440.0** minutes with the default config. Rebuilt with `JWT_ACCESS_TOKEN_TTL_MINUTES=5` override, registered+logged in again, decoded the new token: **5.0** minutes exactly — not a coincidental match, a real, live-verified configuration path |

All required scenarios passed. Test data cleaned up after each phase (`DELETE FROM users WHERE email LIKE 's26a2.%@test.local'`); the scratch test file deleted immediately after collecting its output; `gofmt`/`go build`/`go vet` re-run clean after its removal; backend rebuilt one final time with production defaults (`JWT_ACCESS_TOKEN_TTL_MINUTES=1440`) and re-confirmed via `docker compose exec backend printenv` and one last normal login.

### Not done this session (explicitly out of scope for 26A2)

- **No refresh tokens introduced** — none existed before, none added now, per instruction 4.
- **No auth-system redesign** — `RequireAuth`'s header parsing, password hashing, and the overall single-JWT-no-refresh architecture are all unchanged, per instruction 5.
- **No `SetTrustedProxies` fix** — already identified as a real gap in Stage 26A1 (affecting `clientIP()`'s rate-limit key, not JWT validation), explicitly deferred there and not revisited here since it's a Gin-level proxy-trust setting, not a JWT/session behavior.
- **No frontend changes**, per instruction 6.
- **No OAuth/social login**, per instruction 7.
- **No full regression pass** — out of scope for this focused session.

## Stage 26B2

### Setup

- Read this file (`STAGE26_PROGRESS.md`) and inspected `git status` (clean) before starting, per instruction.
- Scope: **JWT/session security only**, verified live against the running Docker Compose stack. Explicitly excluded: rate-limit re-testing (Stage 26B1's scope, not repeated here), Stage 27, and any full-platform regression.
- Brought the full Docker Compose stack up (it was down at session start); all services reached healthy/running state; `GET /api/v1/health` → 200; no panics in backend logs.
- Confirmed the running backend's live config via `docker compose exec backend printenv`: `JWT_ACCESS_TOKEN_TTL_MINUTES=1440`, `JWT_SECRET=replace-with-a-long-random-secret-per-environment` (production defaults, unchanged since Stage 26A2).

### Design decisions

- No code changes anticipated going in — Stage 26A2 already implemented and unit-verified this exact hardening (`WithValidMethods`, `WithExpirationRequired`, secret-based signature verification). This session's job was to prove those guarantees hold in the **live deployed binary**, not to re-derive them.
- Reused the same technique as Stage 25A1/26A2: a temporary `backend/internal/auth/verify_scratch_test.go` forged the precise malicious/malformed tokens that are impractical to produce through the live HTTP surface alone (wrong algorithm, no-`exp` claim, tampered signature, etc.), printed them to stdout via `go test -v`, then those exact token strings were fed to the live running backend over `curl`. The file was deleted immediately after capturing its output — nothing left behind.

### Change made

**None.** Every scenario in scope passed against the already-deployed Stage 26A2 code; no bugs were found, so no source files were modified this session.

### Files changed

- None (verification-only session). `backend/internal/auth/verify_scratch_test.go` was created and deleted within this session; it does not appear in `git status`.

### Verification performed

**Unit-level** (temporary `verify_scratch_test.go`, `go test ./internal/auth/... -run TestScratchVerifyStage26B2 -v`, deleted immediately after): forged a valid token, an expired token, a wrong-secret token, a tampered-signature token, an HS384 (wrong-algorithm) token, an `alg=none` token, and a validly-signed-but-no-`exp` token (the latter two both carrying a manipulated `role: "admin"` claim) — asserted via `ParseToken` that every forged/invalid one is rejected and the valid one succeeds. **PASS** on all.

**Live, against the real running backend** (`GET /api/v1/me`, the exact forged token strings captured from the unit-level run above):

| Case | Result |
|---|---|
| Missing Authorization header | **401** `UNAUTHORIZED` — "missing or invalid authorization header" |
| Malformed token (`"not.a.jwt"`) | **401** `UNAUTHORIZED` — "invalid or expired token" |
| Expired token (valid secret/algorithm, `exp` in the past) | **401** |
| Invalid signature (correct claims, wrong secret) | **401** |
| Tampered signature (last 4 chars of a valid token corrupted) | **401** |
| Wrong signing algorithm (HS384 instead of HS256) | **401** |
| `alg=none`, `role: "admin"` claimed | **401** |
| Validly-signed, no `exp` claim at all, `role: "admin"` claimed | **401** |
| Valid token (forged, correct secret/algorithm/claims) | Accepted by auth middleware — reached the handler and returned **404** `USER_NOT_FOUND` (expected: the forged token uses a random UUID not present in the DB; this proves the token itself passed JWT validation, distinct from the "normal login" case below) |
| Normal login issues a valid usable token | Fresh account registered (`POST /auth/register` → **201**) → logged in (`POST /auth/login` → **200**, real token issued) → that exact token used on `GET /api/v1/me` → **200**, correct user data returned |

All 9 required live JWT/session scenarios passed. No bugs found; no fixes needed.

**Cleanup:** test account (`s26b2.<timestamp>@example.com`) deleted after the round-trip check; confirmed **zero** residual rows via `SELECT count(*) FROM users WHERE email LIKE 's26b2%'` → `0`. Scratch test file deleted; `gofmt -l .`, `go build ./...`, `go vet ./...` all re-run clean afterward.

### Not done this session (explicitly out of scope for 26B2)

- **No rate-limit verification** — Stage 26B1's scope, not repeated here, per instruction.
- **No Stage 27 work.**
- **No full-platform regression.**
- **No frontend changes.**

## Stage 26B3 — final closeout

### Setup

- Read this file and inspected `git status` before starting: only the uncommitted 26B2 edit to this file was pending (`M STAGE26_PROGRESS.md`); working tree otherwise clean against `e819151 feat: complete stage 26 auth hardening`.
- Scope: final checks only, plus this closeout writeup. Per instruction, did **not** re-run the JWT or rate-limit test matrices — 26A2/26B2's JWT results and 26A1's design/unit results stand as recorded; none of them are unclear.

### Final checks performed

| Check | Result |
|---|---|
| `gofmt -l .` (backend) | Clean — no output |
| `go build ./...` | **OK** |
| `go vet ./...` | **OK** |
| `docker compose ps` | All 10 services `Up`/`healthy` (postgres, minio, mailpit, backend, code-runner, notification-worker, video-worker, frontend, plus one-shot migrate/minio-init already exited successfully) |
| `GET /api/v1/health` | **200** `{"status":"ok","database":"ok"}` |
| `docker compose logs backend --tail=50` | No `panic`/`fatal`/`error` lines |

### Final security results (summary, not re-tested this session)

- **JWT/session hardening (26A2 design + 26B2 live verification):** algorithm pinned to HS256 (`WithValidMethods`), `exp` claim mandatory (`WithExpirationRequired`), signature verified against the configured secret. Live-confirmed in 26B2: missing/malformed/expired/wrong-signature/tampered/wrong-algorithm/no-exp+forged-admin-role tokens all rejected with 401 by the deployed binary; normal register → login → authenticated request round-trip returns a working token. **Verified live, standing result.**
- **Rate limiting (26A1 design + unit-level only):** in-process, fixed-window, `(endpoint, RemoteAddr)`-keyed limiter on `/auth/login` and `/auth/register`, `429` + `Retry-After` on threshold breach, background eviction of idle buckets. Implementation and design were reviewed in 26A1, but **no live HTTP verification of this mechanism has been performed in any session** — Stage 26B1 (requested to cover exactly this) was started but never completed, and no `## Stage 26B1` section exists in this file. This is a genuine, open gap, not a formality.

### Files changed (cumulative, Stage 26 overall)

- `backend/internal/auth/ratelimit.go` — new (26A1).
- `backend/internal/auth/handler.go` — `RegisterRoutes` gained `rateLimiter` param (26A1).
- `backend/internal/auth/jwt.go` — algorithm pinning, required expiration, configurable TTL param (26A2).
- `backend/internal/auth/service.go` — `accessTokenTTL` field/param (26A2).
- `backend/internal/config/config.go` — `AuthRateLimitMaxAttempts`, `AuthRateLimitWindowSec` (26A1), `JWTAccessTokenTTLMinutes` (26A2).
- `backend/cmd/api/main.go` — wired `RateLimiter` and TTL-aware `auth.NewService` (26A1/26A2).
- `docker-compose.yml` — `AUTH_RATE_LIMIT_MAX_ATTEMPTS`, `AUTH_RATE_LIMIT_WINDOW_SEC`, `JWT_ACCESS_TOKEN_TTL_MINUTES` env entries (26A1/26A2).
- `STAGE26_PROGRESS.md` — this file (26A1/26A2/26B2/26B3; verification-only sessions, no source changes).

No files changed in 26B2 or 26B3 themselves — both were verification/closeout sessions.

### Known limitations

- **Rate limiting was never live-verified.** Threshold-breach → 429, `Retry-After` correctness, post-window recovery, cross-client bucket isolation, and cross-endpoint isolation are all unverified against the running server — only reviewed at the design/code level in 26A1. This is the single biggest open item from this stage.
- **No `SetTrustedProxies` configuration.** `clientIP()` uses `RemoteAddr` specifically to avoid trusting spoofable headers under Gin's default (trust-all) proxy setting — correct today since there is no reverse proxy in front of this backend, but it will need revisiting together with a real `SetTrustedProxies` call if one is ever introduced (anticipated around Stage 28).
- **No frontend messaging for `429 RATE_LIMITED`.** The backend returns a structured error and `Retry-After`, but no UI surfaces it yet.
- **No refresh-token flow** — unchanged from before Stage 26; this backend has always issued a single fixed-TTL access token with no refresh endpoint, and that architecture was explicitly out of scope to change.
- **No full-platform regression** was run in any Stage 26 session (26A1, 26A2, 26B2, or this closeout) — each stayed scoped to auth-adjacent code and live checks of the auth surface specifically.

## Stage 26B1 — final live verification of authentication rate limiting

### Setup

- Read this file and inspected `git status` before starting: only the uncommitted 26B3 edit to this file was pending; working tree otherwise clean against `e819151 feat: complete stage 26 auth hardening`.
- Inspected only `backend/internal/auth/ratelimit.go` (the existing limiter) and its config wiring (`internal/config/config.go`, `docker-compose.yml`) — did not touch or reimplement 26A1, 26A2, or 26B2.
- Design confirmed by inspection: in-process, fixed-window counter keyed by `endpoint + "|" + RemoteAddr`, `MaxAttempts`/`Window` from `AUTH_RATE_LIMIT_MAX_ATTEMPTS`/`AUTH_RATE_LIMIT_WINDOW_SEC` (defaults 10/300s), `429 RATE_LIMITED` + `Retry-After` header on breach, background sweep of idle buckets.
- Used a temporary small threshold/window for practical testing: recreated the `backend` container with `AUTH_RATE_LIMIT_MAX_ATTEMPTS=3 AUTH_RATE_LIMIT_WINDOW_SEC=8` (one-off `docker compose up -d --force-recreate backend` env override, not written to `.env` or `docker-compose.yml`, so restoring defaults required no file edit — just recreating the container again with no override).

### Change made

**None.** Every scenario in scope passed against the already-deployed Stage 26A1 code; no bugs were found, so no source files were modified this session.

### Verification performed (live, against the running Docker Compose backend)

| Case | Result |
|---|---|
| Normal login works (single request, well under threshold) | **200** |
| Attempts below threshold behave normally | Requests 1–3 to `/auth/login` (max=3) → **401** bad-credentials each time, never blocked |
| Exceeding threshold returns 429 | Requests 4–5 → **429** `RATE_LIMITED` |
| `Retry-After` header present | **Yes** — `Retry-After: 7` on both 429s, consistent with the 8s test window |
| Limiter recovers after configured window | After sleeping past the window (9s > 8s), the same client's next login attempt → **401** again (not 429) |
| Unrelated endpoints remain unaffected | While `/auth/login` was blocked for a client, `GET /courses` → **200**, `GET /health` → **200** |
| Register/login buckets behave independently | While the login bucket was exhausted for a client, that same client's `POST /auth/register` → **400** (validation error, reached the handler — not blocked by the login bucket) |
| Separate clients/IP keys do not share one bucket | Host client fully exhausted its login bucket (**429** confirmed) while, concurrently, a genuinely separate client — a different container IP on the same Docker network, hitting the identical `http://backend:8080/api/v1/auth/login` — got **401** (normal bad-credentials handling, not blocked at all) |

All 8 required scenarios passed. No bugs found; no fixes needed.

**Restore + cleanup:** recreated `backend` with no env override, confirmed live via `docker compose exec backend printenv` → `AUTH_RATE_LIMIT_MAX_ATTEMPTS=10`, `AUTH_RATE_LIMIT_WINDOW_SEC=300` (production defaults); `GET /health` → 200; no panics/errors in logs. Test account (`s26b1.*@example.com`) deleted; confirmed **zero** residual rows via `SELECT count(*) FROM users WHERE email LIKE 's26b1%'` → `0`. Re-ran `gofmt -l .`, `go build ./...`, `go vet ./...` — all clean.

### Not done this session (explicitly out of scope for 26B1)

- **No JWT/session re-verification** — 26A2/26B2's results stand, not repeated, per instruction.
- **No Stage 27 work.**
- **No full-platform regression.**

## Final Stage 26 status: **COMPLETE**

- JWT/session hardening: **done and live-verified** (26A2 + 26B2).
- Rate limiting: **done and live-verified** (26A1 + 26B1) — all 8 required live scenarios pass, including the previously-outstanding threshold/429/Retry-After/recovery/cross-client-isolation checks.
- Build/format/vet: clean. Live Docker Compose stack: healthy, no errors.
- Remaining items are explicitly deferred infra/frontend follow-ups, not blockers to closing this stage: `SetTrustedProxies` (pending a real reverse proxy, anticipated Stage 28), frontend `429` messaging, and the standing note that no full-platform regression has been run against this auth-adjacent code.
