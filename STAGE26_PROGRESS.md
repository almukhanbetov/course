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

## Remaining for Stage 26 (not started)

- Backend/infra: a real `SetTrustedProxies` configuration once/if a reverse proxy is introduced (Stage 28's future scope) — affects Stage 26A1's rate-limit key, not this session's JWT hardening, but both live in the same package and should be revisited together.
- Frontend: user-facing error/backoff messaging for the `429 RATE_LIMITED` case (Stage 26A1's own deferred item).
- Full-platform smoke regression, per the roadmap's own note that this touches shared auth middleware every request passes through.
- A consolidated Stage 26 close-out session, mirroring every prior stage's A/B/C pattern, once the frontend piece (if any) lands.
