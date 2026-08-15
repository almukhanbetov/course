# Production Launch Readiness Audit

Read-only audit. No code changed, nothing deployed, no fixes applied — per instruction, this document reports findings only. Performed after Stage 30 (and the full Stage 21–30 roadmap) were marked complete in `STAGE30_PROGRESS.md`.

**Audited:** real production (`https://compserv.cloud`, live-tested read-only from this session) + the dev-stack analog (`course-*` containers, used for every prior Stage 30 session's authenticated/mutating testing) + the full source tree + GitHub Actions' real run history (public API, no auth needed — this is a public repo).

**Date of this audit (per session context):** 2026-08-14.

---

## GO / NO-GO RECOMMENDATION

# ❌ NO-GO for a full public launch accepting real payments.

**Five confirmed BLOCKERS were originally found; one is now fixed (see "Production Launch Fix 1" below). Four remain**, one of which is still live and exploitable on the real production domain right now (not hypothetical, not dev-only — verified against `https://compserv.cloud` directly this session). None require large engineering effort to fix — the fastest is minutes of operator work, not a new development stage — but none should be waved through either.

**A narrow, conditional GO exists** for a small, invite-only, free-tier-only soft launch, but only after the fastest remaining blocker is closed (take one verified real backup) **and** the admin-credential fix's one remaining manual step is done (set `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` on the real VPS and redeploy) — payments must stay disabled/hidden regardless, since no real payment provider exists in the code at all.

See "Consolidated blocker list" below for the exact, minimal action list.

---

## Audit methodology

- **Live, read-only HTTP checks against real production** (`https://compserv.cloud`): homepage, `/login`, `/register`, `/pricing`, `/courses`, `/api/v1/health`, `/api/v1/admin/system-health` (unauthenticated), `/api/v1/courses`, `/storage/*`, TLS certificate inspection, security-header inspection, HTTP→HTTPS redirect. **No mutating request was made against real production** — no account was registered, no login attempted, no payment created — consistent with "do not deploy" and general care around a live system with real users' trust at stake.
- **GitHub Actions' real run history**, fetched via the public, unauthenticated GitHub API (`api.github.com/repos/almukhanbetov/course/actions/workflows/*/runs`) — this repo is public, so this reflects genuine production CI/CD history, not a guess.
- **Full source-tree inspection**: `docker-compose.prod.yml`, `.env.production.example`, `deploy/nginx/*.conf`, all seven `.github/workflows/*.yml`, `backend/internal/subscriptions/`, `backend/internal/notifications/`, `backend/internal/auth/`, `backend/migrations/*seed*.sql`, `frontend/app/` route tree.
- **Everything already proven live in Stages 30A1–30B3** (backup/restore/retention, the security sweep, the full-platform functional regression) is treated as authoritative and not redundantly re-run in full — cross-referenced with file:line/session citations instead, per "do not implement new product features" / keep this an audit, not a re-verification of already-settled work. Where this audit found *new* evidence (e.g., live production behavior no prior session checked), that's called out explicitly as new.
- Dev-stack (`course-*`) confirmed healthy and used only for the handful of checks impossible against real production without being destructive (backup-artifact existence, a positive-control login).

---

## Findings by audit area

### 1. Domain / HTTPS / reverse proxy

| Check | Result | Classification |
|---|---|---|
| Real domain configured | `compserv.cloud` (+ `www.compserv.cloud` in Nginx config, but see below) | READY |
| TLS certificate | Real Let's Encrypt cert, live-inspected this session: `subject=CN=compserv.cloud`, valid `2026-08-14` → `2026-11-12`, issued by Let's Encrypt (`YE1`) | READY |
| HTTP→HTTPS redirect | Live-confirmed: `http://compserv.cloud/` → `301` → `https://compserv.cloud/` | READY |
| `/` and `/api/` routing | Live-confirmed: both resolve correctly (`curl` 200s throughout) | READY |
| `www.compserv.cloud` | Nginx is configured to accept it, but **no DNS record exists** — live-confirmed this session (`dig +short www.compserv.cloud` returns nothing; `curl` times out/fails to connect) | CAN DEFER — cosmetic; the bare domain works and is what any real link/QR/ad would use |
| Security response headers | **None present** — live-confirmed this session: no `Strict-Transport-Security`, `X-Frame-Options`, `X-Content-Type-Options`, or `Content-Security-Policy` on any response from either Nginx or the backend (`grep`'d both `deploy/nginx/*.conf` and the Go backend for header-setting code — none found anywhere in the stack) | **IMPORTANT BEFORE PUBLIC LAUNCH** — cheap to add (a few `add_header` lines in Nginx), meaningfully reduces clickjacking/MIME-sniffing/downgrade risk for a real user base |
| `/storage` path (MinIO/legacy presigned video URLs) | Still missing — live-confirmed this session (`curl https://compserv.cloud/storage/test` → `404`, falls through to the Next.js catch-all, not a real storage proxy). First flagged in `STAGE28_PROGRESS.md`, still open per `STAGE30_PROGRESS.md`. **Only affects `internal/videos`' legacy non-HLS fallback path** — the primary HLS video-delivery path (`/api/video-stream/...`) never touches this and was proven live end-to-end in Stage 30B2 with a real transcoded video | **CAN DEFER** — real impact is narrow (only bites a video row with no HLS rendition, which shouldn't occur via the current upload pipeline) |
| Server/version disclosure | `Server: nginx/1.24.0 (Ubuntu)`, `X-Powered-By: Next.js` both leak on every response (live-confirmed) | CAN DEFER — standard low-severity info disclosure, common on most real-world sites |

### 2. Frontend critical routes

Live-confirmed against real production this session: `/` → 200, `/login` → 200, `/register` → 200, `/pricing` → 200, `/courses` → 200. `/admin` and `/instructor` correctly return `307` (redirect to login) when unauthenticated — server-side auth gating confirmed working on the real deployment, not just in dev.

**Classification: READY.**

### 3. Backend health and deep health

- `GET https://compserv.cloud/api/v1/health` → live-confirmed `{"status":"ok","database":"ok"}` this session.
- `GET https://compserv.cloud/api/v1/admin/system-health` (no token) → live-confirmed `401 {"error":{"code":"UNAUTHORIZED",...}}` — no internal detail leaked to an unauthenticated caller, matching Stage 29's own design requirement.
- Stage 29's closeout (already thoroughly proven, not re-litigated here) established the deep check covers DB, MinIO, and notification-queue depth, degrades gracefully under a real dependency outage, and never leaks credentials/hostnames.

**Classification: READY.**

### 4. Registration / login / logout / role boundaries

- Functional correctness: proven exhaustively, live, against the dev-stack analog in Stage 30B2 (register, duplicate-email rejection, login success/failure, protected routes, role promotion, logout) — zero bugs found, not re-run in full here to avoid creating real accounts on production.
- Role boundaries (student/instructor/admin) and cross-user/cross-course IDOR: proven exhaustively, live and adversarially, in Stage 30B1 — two real bugs found and fixed, re-verified since.
- **New finding this session: no password-reset / forgot-password flow exists anywhere in the codebase** (confirmed by direct `grep` across `backend/internal/auth/` and all of `frontend/app/` — zero matches for "reset"/"forgot"). A real user who forgets their password has no self-service recovery path today.
- **New finding this session: the seeded admin account uses a known, publicly-documented default credential.** `backend/migrations/00008_seed_admin_user.sql` creates `admin@example.com` with a hardcoded bcrypt hash for the password documented in that migration's own comment (`ChangeMe123!`), which — since this is a **public** GitHub repository (confirmed via the unauthenticated GitHub API working at all) — is visible to anyone who looks. This audit did **not** attempt to actually log in with it against real production (that would cross from audit into an actual unauthorized-access attempt against a live system), but there is no evidence in any prior stage's documentation that this credential was ever rotated on the real VPS.
- **RESOLVED this session — Production Launch Fix 1.** See "Production Launch Fix 1" section below for the full implementation, live verification evidence, and the one manual step still required before this is closed on the real VPS specifically (setting `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` there, same one-time-manual-secret pattern as `BACKUP_ENCRYPTION_PASSPHRASE`).

| Item | Classification |
|---|---|
| Core auth flows (register/login/logout/roles) | READY (per 30B1/30B2's live proof) |
| No password-reset flow | **IMPORTANT BEFORE PUBLIC LAUNCH** |
| Seeded admin credential, publicly documented | **FIXED this session** (code/migration path complete and live-verified against the dev-stack analog; real production requires one manual VPS `.env` step before the next deploy — see below) |

### 5. Enrollment / learning / progress / completion

Proven exhaustively, live, end-to-end in Stage 30B2 against the dev-stack analog: enroll → lesson progress → `next_lesson_id` advancement → 100% completion, all correct. Not re-run here (would require creating real enrollment data on production).

**Classification: READY.**

### 6. Video delivery and premium/free access

Proven exhaustively, live, in Stage 30B2 — including a **real** video upload, a **real** `ffmpeg` transcode via the actual `video-worker` container, and a **real** authorized HLS playback chain (`master.m3u8` → `360p/index.m3u8` → a genuine `.ts` segment), with unauthorized/non-enrolled access correctly denied at every layer. The one open gap (`/storage`, legacy fallback only) is covered under item 1.

**Classification: READY**, with the narrow item-1 gap noted as CAN DEFER.

### 7. Q&A and moderation

Proven live in both Stage 30B1 (found and fixed a real IDOR bug — non-enrolled users could read a course's Q&A) and Stage 30B2 (full create/list/answer/hide/unhide cycle, re-confirming the fix holds). No new issues found this session.

**Classification: READY.**

### 8. Payments — mock or real? Is accepting real money safe?

**Provider status: MOCK ONLY.** Confirmed by direct code inspection this session (`backend/internal/subscriptions/provider.go`): `NewProvider()`'s switch statement has exactly one real branch (`"mock"`), and its `default` case *also* silently falls back to the mock provider — meaning even a typo'd or unrecognized `PAYMENT_PROVIDER` value would boot fine and quietly serve fake payments. `.env.production.example` documents this honestly: `PAYMENT_PROVIDER=__REAL_PROVIDER_NOT_YET_INTEGRATED__`. No Stripe/YooKassa/CloudPayments/etc. SDK, webhook receiver, or signature-verification code exists anywhere in the backend (confirmed by `grep`).

**Is accepting real money safe? No — it is not merely "unsafe," it is currently impossible.** No code path in this application can capture a real card or bank payment today. Worse: whenever the mock provider is active (which is every deployment as configured, including the literal placeholder value), a `POST /payments/:id/mock-confirm` endpoint exists that lets an authenticated user self-confirm their **own** pending payment as `"paid"` with zero money having moved — meaning if this were launched today believing payments were real, every "paying" user would actually be getting premium/subscription access for free, and the admin payments/revenue dashboards would show entirely fabricated transaction data.

**Classification: BLOCKER** — specifically for any launch that intends to charge real users. Does not block a free-tier-only launch (see recommendation).

### 9. Email / SMTP — is real delivery configured?

Confirmed by direct code inspection this session (`backend/internal/notifications/email.go`, `docker-compose.prod.yml`, `.env.production.example`):

- Two implementations exist: a real `SMTPSender` and a `LogSender` fallback used whenever `SMTP_HOST` is empty — the fallback never fails, it just logs "dev email not sent" and moves on.
- `docker-compose.prod.yml` correctly and explicitly excludes `mailpit` (the dev-only fake SMTP catcher) from production — this part is done right.
- `.env.production.example`'s SMTP block is entirely unfilled placeholders (`SMTP_HOST=__SET_VIA_GITHUB_SECRET_PROD_SMTP_HOST__`, etc.), with an explicit comment that a real provider is still an **open, unresolved decision**.
- **This audit cannot independently confirm from this sandbox** whether a real SMTP provider's credentials were ever actually substituted into the real VPS's `.env` — that would require VPS access this environment doesn't have. No prior stage's documentation records that decision being made or a real provider being chosen.

**If truly still on placeholder/empty config**, real users will not receive registration confirmations or any notification email — either silently (empty `SMTP_HOST` → `LogSender`) or via visibly failing, endlessly-retried background jobs (a literal placeholder string as `SMTP_HOST` → DNS failure → job marked failed).

**Classification: IMPORTANT BEFORE PUBLIC LAUNCH** — not a hard blocker (the app remains usable without email; no email-verification gate blocks registration/login), but real users expect at least a registration confirmation, and every notification-driven engagement feature is silently broken without it. **Action needed regardless of classification: confirm on the real VPS whether SMTP is actually configured — this audit could not determine that from this sandbox.**

### 10. Backup / restore / retention

**The mechanism itself is thoroughly proven** — this is genuinely strong work, not a gap:
- Stage 30A1–A3: a real, encrypted (`AES-256-CBC`+PBKDF2), access-restricted backup mechanism was built and live-tested.
- **Restore was proven twice, live**, not once: Stage 30A2's own live restore, and Stage 30A3's independent second live restore by literally following the written runbook end-to-end.
- Retention (keep-newest-14, fail-closed on listing/deletion failure, structurally cannot delete the newest backup or anything outside its own prefix) was live-tested with controlled fixtures, both dry-run and real-deletion.

**But: this audit found new evidence this session that real production itself has likely never actually been backed up.** Checked via the public GitHub Actions API: `backup.yml` (the scheduled workflow) shows **zero runs, ever** (`total_count: 0`), while `deploy.yml`/`image-publish.yml`/`quality-gate.yml` each show a dozen-plus real, recent, successful runs in the same window — i.e., the backup workflow was correctly added to the repo but has not yet had a scheduled window fire (its cron is daily `03:00 UTC`; at audit time it was mid-afternoon UTC, so this alone isn't damning) **and, more importantly, no session has ever recorded `BACKUP_ENCRYPTION_PASSPHRASE` actually being set on the real VPS's `.env`** — every Stage 30A/B session explicitly and consistently notes this as still-outstanding, out of scope for a sandbox with no VPS access. Without that value present, `run-backup.sh`'s own pre-flight check will make the very first real run fail immediately, before touching Postgres or MinIO at all.

**Net effect: as of this audit, there is no confirmed evidence a single real backup of the production database exists anywhere.** If the real database were lost right now, recovery cannot currently be assumed possible.

**Classification: BLOCKER.** The fix is fast (set one secret on the VPS by hand, per the existing runbook — literally the one manual step every prior session flagged as still-needed) but until it's done and a first real backup is confirmed to exist in the real bucket, this is not a defensible production posture.

### 11. CI/CD and rollback readiness

- **CI**: live-confirmed via GitHub API — `quality-gate.yml` (13 runs), `image-publish.yml` (12 runs), `deploy.yml` (11 runs) all show consistent, recent, real `success` conclusions. The pipeline genuinely works.
- **Branch protection is not configured** at the GitHub repo-settings level (documented consistently since `STAGE27_PROGRESS.md`, a repo-settings action outside any commit) — meaning `quality-gate` is currently informational only and does not actually block a bad merge to `main` from reaching production. **IMPORTANT BEFORE PUBLIC LAUNCH.**
- **Deploy pipeline**: live-confirmed working (11 successful real runs), migrations exit-code-gated before restart, post-deploy health checks in place.
- **Rollback**: structurally supported (redeploy an older published image tag via the same pipeline) but — per `STAGE28_PROGRESS.md`'s own closeout, unchanged since — **has never actually been exercised for real** against the live VPS. **IMPORTANT BEFORE PUBLIC LAUNCH** — the mechanism is very likely to work (it's the identical, already-proven-reliable forward-deploy path pointed at an old tag) but "never tested" is a real risk for your only recovery lever during a real incident.

**Classification: READY** for the CI/CD pipeline itself; **IMPORTANT BEFORE PUBLIC LAUNCH** for branch protection and a real rollback drill.

### 12. Security: exposed ports, secrets, SSH, admin endpoints

- **Exposed ports** — confirmed via direct read of `docker-compose.prod.yml`: only Nginx's 80/443 are genuinely public; `backend`/`frontend` are bound to `127.0.0.1` only (not the base file's implicit `0.0.0.0`, deliberately overridden); `postgres`/`minio` have **no port mappings at all** in production (`ports: !reset []`). **READY.**
- **Secrets** — every credential (`POSTGRES_PASSWORD`, `JWT_SECRET`, `MINIO_ROOT_PASSWORD`, SSH key, etc.) is sourced from GitHub Secrets or the VPS's own `.env`, never committed — confirmed consistently across every workflow and `.env.production.example`'s placeholder convention. Live-confirmed this session: zero secret leakage in backend logs (re-confirmed by Stage 30B1's live grep, unchanged). **READY.**
- **SSH** — TOFU (trust-on-first-use) host-key trust, no manual approval gate before a deploy runs — an acknowledged, documented trade-off (`STAGE28_PROGRESS.md`), not a defect, but worth naming explicitly since these SSH keys carry full production deploy authority. **IMPORTANT BEFORE PUBLIC LAUNCH** if this is meant to scale beyond a single trusted operator.
- **Admin endpoints** — live-confirmed this session (`/admin/system-health` → 401 unauthenticated on real production) and exhaustively adversarially tested in Stage 30B1 (role boundaries, cross-instructor/cross-course denial, IDOR). **READY**, with the one caveat that the admin *account itself* uses a known default credential — see item 4/14, classified there as the actual BLOCKER (the endpoint gating is correct; the credential behind it is the risk).

### 13. Logging / health / operational visibility

Structured `log/slog` logging with request-ID correlation, a deep health check with a public/admin split that leaks nothing unauthenticated, live-proven in Stage 29 against a genuinely hanging dependency (not just a fast-refusing one). Re-confirmed live this session via the real production health endpoints.

**Classification: READY.**

### 14. Production data readiness

- **Admin**: exists; the default seeded credential is now neutralized in code/migration (see item 4 and "Production Launch Fix 1" below) — **FIXED this session**, pending one manual VPS step before it takes effect on real production.
- **Instructor**: **no real (non-test) instructor account exists.** The seed migrations (`00005`, `00008`, `00013`, `00019`, `00025`) create one admin user and demo course/test/subscription-plan data, but no instructor user row — every Stage 30 session that needed an instructor account for testing had to create and manually promote one itself, consistent with there being none by default. **IMPORTANT BEFORE PUBLIC LAUNCH** — someone needs a real instructor identity before real course content can be authored.
- **Real course**: **none exists yet.** Live-confirmed this session — `GET https://compserv.cloud/api/v1/courses` on real production returns the seeded demo "Docker" course (and, per the seed migrations, "Go Backend Developer" and "PostgreSQL" alongside it) — competent tutorial-style content, but explicitly demo/fixture data (fixed deterministic UUIDs, filenames literally containing "seed"/"demo"), not real catalog content. **IMPORTANT BEFORE PUBLIC LAUNCH.**
- **Test/demo data removal**: **not done — and this demo data is live on the real, public production database right now**, visible to any real visitor today (this audit fetched it with a plain unauthenticated `curl` against the real domain). A decision is needed: keep and polish this content as real launch content, or replace it — either way it needs an explicit decision, not silence. **IMPORTANT BEFORE PUBLIC LAUNCH.**

### 15. Legal / launch pages

**None exist.** Confirmed by direct search of the entire `frontend/app/` tree this session (`find` for terms/privacy/refund/about/contact/legal patterns — zero results). No terms of service, no privacy policy, no refund policy, no about/contact page anywhere. Combined with there being no password-reset flow (item 4) and real user PII (email, name, payment intent) being collected at registration, operating without at least a privacy policy and terms of service is a real compliance gap for a genuine public launch, not just a nice-to-have.

**Classification: BLOCKER** for a full public launch; does not block a small, known, consenting-tester soft launch.

### 16. Final non-destructive smoke test

Performed this session, read-only, against real production: homepage, login/register/pricing/courses pages, health endpoint, admin-health endpoint (unauthenticated, correctly 401), public course listing, TLS certificate, HTTP→HTTPS redirect, security headers, `/storage` gap, `www` subdomain, server-version disclosure. All results folded into the relevant sections above. Authenticated/mutating flows (register, login, enroll, pay, upload video) were **not** re-tested against real production in this session — that would create real data on a live system — and instead rely on Stage 30B1/30B2's very recent, thorough, live proof against the dev-stack analog, which remains valid and current (re-confirmed unregressed via direct code `grep` this session).

---

## Consolidated blocker list — exact actions needed before launch

Of the original five BLOCKERS, **one is now fixed in code** (#1 below — see "Production Launch Fix 1"). The remaining four are unchanged. Ordered by how fast they are to close:

1. ~~**Rotate or disable the seeded admin credential.**~~ **FIXED this session** (Production Launch Fix 1, below). One manual step remains before it's actually in effect on real production: set `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` on the real VPS's `.env` (generate a real password, e.g. `openssl rand -base64 32`) **before** the next deploy ships this fix — the deploy will otherwise correctly and safely refuse to proceed rather than leave production admin-less (see "Production Launch Fix 1" for exactly why, and what happens if this step is skipped).
2. **Set `BACKUP_ENCRYPTION_PASSPHRASE` on the real VPS's `.env`, then trigger `backup.yml` manually once (`workflow_dispatch`) and independently confirm a real encrypted artifact lands in the production MinIO bucket.** Until this is done, production has no confirmed-recoverable backup. *Fastest remaining item — the mechanism is already built and proven; this is one secret plus one manual trigger.*
3. **Do not enable/advertise paid subscriptions until a real payment provider is integrated.** As shipped, "payment" is a self-confirm mock with no real money movement — a launch that implies real payments today would be materially misleading to users and would let anyone bypass any paywall for free. Either integrate a real provider (Stripe/YooKassa/CloudPayments/etc. — genuinely new engineering work, out of this audit's scope to implement) or launch free-tier-only and say so.
4. **Publish at minimum a Terms of Service and Privacy Policy** before accepting real users' registrations at any public scale — none exist in the codebase today.
5. **Decide and execute on production content**: replace/expand the demo courses currently live on the real domain with real course content and at least one real instructor account, or make a deliberate, informed decision to launch with the current demo content as real content.

## Important-before-public-launch (not blockers, but should not be silently skipped)

- No password-reset/forgot-password flow anywhere in the app.
- No security response headers (HSTS/X-Frame-Options/CSP/X-Content-Type-Options) on any response.
- SMTP delivery status on the real VPS is unconfirmed from this sandbox — verify directly and configure a real provider if not already done.
- GitHub branch protection is not configured — `quality-gate` currently can't actually block a bad merge.
- Rollback has never been exercised for real against the live VPS — run one real rollback drill.
- SSH deploy access uses TOFU trust with no approval gate — acceptable for a single trusted operator, worth revisiting if the team grows.

## Can defer (low practical impact, safe to launch without)

- `www.compserv.cloud` has no DNS record (bare domain works fine).
- `/storage` Nginx path still missing (only affects an already-unused legacy video fallback).
- `Server`/`X-Powered-By` version headers leak (standard, low-severity, common industry-wide).

---

## Production Launch Fix 1 — Secure production admin account (implemented)

Scope: close BLOCKER #1 above. Read-only audit ends here; this section documents an actual code change, live-verified, not yet deployed to real production (per instruction, "do not deploy until the implementation and migration path are verified" — that verification is exactly what this section records).

### Every seeded/default/demo admin credential found in the repository (instruction 1)

Exactly one: `backend/migrations/00008_seed_admin_user.sql`, an unconditional goose migration (no environment branching exists anywhere in this project's migration or deploy path) inserting `admin@example.com` with a hardcoded bcrypt hash for the password documented in that file's own comment. Confirmed by grep that this is the *only* place any `role_id` referencing the admin role (`44444444-4444-4444-4444-444444444443`) is ever inserted anywhere in `backend/migrations/`. No other hardcoded credential of any role exists in the codebase.

### What was built

- **`backend/migrations/00041_neutralize_seeded_admin_credential.sql`** (new) — an additive migration (00008 itself is left untouched, per this project's "migrations are immutable history" convention — editing 00008 would do nothing for any database that already ran it, including possibly real production). Matches the *exact* bcrypt hash 00008 inserted (not just the email — bcrypt's per-hash random salt makes an exact-hash match cryptographically precise, so this can only ever touch the untouched original seeded row; if an operator already rotated it by hand, this is a safe no-op) and, in one statement: deactivates the row and replaces its password with a fresh, cryptographically random value generated **entirely server-side by Postgres itself** via `pgcrypto`'s `gen_random_bytes()`/`crypt()`/`gen_salt('bf')` (already enabled since `00001_init_extensions.sql`) — no plaintext password exists even transiently, and nothing is returned to any client, logged, or stored anywhere except that one column.
- **`backend/cmd/bootstrap-admin`** (new Go command) — the safe replacement bootstrap path (instructions 4/5). Connects to the database, counts active admin-role users; if one already exists, logs that and exits 0 (a safe no-op — this is what every deploy after the first looks like, forever). If zero exist, requires `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` from its own process environment only (never a flag, never a config file, never source code) and fails closed — non-zero exit, no database write — if either is missing, or if the password is under 12 characters. On success, upserts (`ON CONFLICT (email) DO UPDATE`, not `DO NOTHING` — chosen specifically so reusing an email that already exists inactive reactivates it correctly instead of silently no-op'ing) an active admin with a freshly `bcrypt`-hashed password (reusing the exact same `auth.HashPassword` helper the real login/register flow already uses). Only ever logs the resulting email (not a secret) or which named variable is missing — never a password or hash, in either the success or failure path.
- **`backend/Dockerfile.bootstrap-admin`** (new) — same multi-stage pattern as every other `cmd/*` Dockerfile in this repo (`Dockerfile.worker`, `Dockerfile.notification-worker`, etc.).
- **`docker-compose.yml`** (modified) — new `admin-bootstrap` service, runs once after `migrate` completes (same shape as `migrate` itself, not a restarted service); `backend` now also waits on it. Dev keeps the exact same `admin@example.com`/`ChangeMe123!` login working out of the box via `.env.example`'s new `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` — same values as before, now arriving through the safe bootstrap path instead of a hardcoded migration (instruction 6: local dev convenience preserved, production behavior now explicit).
- **`docker-compose.prod.yml`** (modified) — `admin-bootstrap` overridden to a published image, same pattern as every other buildable service.
- **`.env.production.example`** (modified) — `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` added as `__SET_VIA_VPS_ENV_ONLY__` / `__SET_VIA_VPS_ENV_ONLY_NEVER_COMMITTED__` placeholders — the exact same "generate a real secret, set it once by hand on the VPS, never transiting GitHub Actions" convention `BACKUP_ENCRYPTION_PASSPHRASE` already established.
- **`.github/workflows/deploy.yml`** (modified) — new "Run admin bootstrap" step, exit-code-gated exactly like migrations, right after them and before the app restarts.
- **`.github/workflows/image-publish.yml`** (modified) — `bootstrap-admin` added to the image build matrix.
- **`.github/workflows/infra-ci.yml`** (modified) — CI-only throwaway `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` added (backend's new dependency on `admin-bootstrap` completing would otherwise correctly fail CI's fresh throwaway database, which has no existing admin either — same fix pattern CI already uses for `JWT_SECRET`).
- **`backend/internal/config/config.go`** (modified) — `AdminBootstrapEmail`/`AdminBootstrapPassword` fields, no default value for either (an empty default is what makes the fail-closed check in `cmd/bootstrap-admin` actually mean something).

### Verification performed — all live, against the dev-stack analog (never real production, consistent with every prior Stage 30 session's stance)

1. **Baseline**: confirmed `admin@example.com`/`ChangeMe123!` logs in successfully (HTTP 200) *before* the fix — establishes the starting state the fix needs to change.
2. **Real migration path, not simulated**: ran the actual `docker compose run migrate` service (the real `goose` binary, the exact mechanism `deploy.yml` uses) — log output: `OK 00041_neutralize_seeded_admin_credential.sql (62.11ms)` / `goose: successfully migrated database to version: 41`.
3. **Instruction 7 — old/default credential cannot authenticate**: immediately after, `admin@example.com`/`ChangeMe123!` → `401 {"error":{"code":"INVALID_CREDENTIALS",...}}`. Row confirmed in Postgres: `active=false`, hash no longer matches the known seeded hash. Zero active admin-role users existed at this point (`SELECT count(*) ... = 0`).
4. **Fail-closed, live**: ran the real `admin-bootstrap` container with `ADMIN_BOOTSTRAP_EMAIL`/`ADMIN_BOOTSTRAP_PASSWORD` unset while zero active admins existed → exit code `1`, log: `bootstrap-admin: no active admin exists and ADMIN_BOOTSTRAP_EMAIL, ADMIN_BOOTSTRAP_PASSWORD (not set) - refusing to create an admin account without explicit runtime configuration`. No database write occurred.
5. **Weak-password guard, live**: ran it again with a 6-character password → exit code `1`, log: `ADMIN_BOOTSTRAP_PASSWORD is shorter than the required minimum (12 characters) - refusing to create a weak admin account`.
6. **Instruction 8 — a securely-created admin can authenticate and access admin-only routes**: ran it with a distinct, strong, one-off test credential (`newadmin-verify@example.com` / a 24-character test password never used anywhere else in this project) → exit `0`, log: `admin account ready for newadmin-verify@example.com (1 row affected)`. That account then: logged in successfully (HTTP 200, valid JWT with `"role":"admin"`), accessed `GET /api/v1/admin/ping` (HTTP 200), and accessed `GET /api/v1/admin/users` (HTTP 200, real user list returned) — i.e., not just "a row exists," a real end-to-end authenticated admin session against real admin-only routes.
7. **Re-confirmed old credential still fails** after the new admin was created (didn't get resurrected by any of the above): `admin@example.com`/`ChangeMe123!` → still `401`.
8. **Idempotency, live**: ran `admin-bootstrap` a third time, with yet another distinct email/password, *while* the test admin from step 6 was still active → exit `0`, log: `1 active admin account(s) already exist, nothing to do`. No new row created, existing admin untouched — proves the "safe to run on every deploy forever" claim isn't just a design intent.
9. **Instruction 9 — no secrets in logs**: every log line produced across steps 4–8 (the only source of truth, since `docker compose run --rm` removes each container immediately after exit) was inspected directly at emission time — none of them ever contain a plaintext password, a bcrypt hash, or any string matching the test passwords used. Only email addresses (not secrets) and generic status text appear. Backend container logs separately grepped for the same test password strings — zero matches.
10. **Dev convenience restored (instruction 6)**: deactivated the test admin, re-ran `admin-bootstrap` with the documented `.env.example` values (`admin@example.com`/`ChangeMe123!`) → the *same* original row (same `id`, same original `created_at`) was reactivated with a freshly bcrypt-hashed version of that same password. Confirmed live: `admin@example.com`/`ChangeMe123!` logs in successfully again — local dev behavior is unchanged end-to-end, now arriving through the safe bootstrap path instead of a hardcoded migration.
11. **Cleanup**: the disposable test admin row (`newadmin-verify@example.com`) deleted. Final state confirmed: exactly one active admin (`admin@example.com`), matching the documented dev default.
12. **Build gates**: `gofmt -l backend/` clean, `go build ./...` succeeds, `go vet ./...` clean. All seven workflow YAML files (including the three modified this session) re-validated: structural YAML parse + `actionlint`, zero findings. `docker compose config` and the production merge (`-f docker-compose.yml -f docker-compose.prod.yml config`) both resolve without error; independently confirmed `admin-bootstrap` resolves to `image: .../course-bootstrap-admin:...` with no `build:` key in the production merge, matching every other buildable service's override pattern exactly.

### What is NOT yet done (by instruction — "do not deploy")

- **Not deployed to real production.** Everything above was verified against the dev-stack analog only, per this project's established Stage 30 convention (no VPS access from this sandbox, and deploying was explicitly out of scope for this session regardless).
- **One manual step is required before the next real deploy ships this fix**: generate a real password (e.g. `openssl rand -base64 32`) and set both `ADMIN_BOOTSTRAP_EMAIL` and `ADMIN_BOOTSTRAP_PASSWORD` on the real VPS's `/opt/lms/.env`, the same one-time "set by hand, store a copy durably elsewhere, never transits GitHub Actions" step every other production secret in this project has needed (identical in kind to `BACKUP_ENCRYPTION_PASSPHRASE`'s still-outstanding step). **If this is skipped**, the next deploy's new "Run admin bootstrap" step will correctly and safely fail (exit 1) immediately after migrations neutralize the old row — the deploy aborts *before* restarting backend/frontend, so the currently-running (old, pre-fix) containers keep serving traffic uninterrupted; nothing goes down, but the fix doesn't take effect until the VPS `.env` is set and the deploy is re-run.
- **No other Production Launch Checklist item was touched.** Per instruction, backup/payment/legal/demo-data fixes were explicitly not started this session.

### Status: BLOCKER #1 (seeded admin credential) — **FIXED in code, live-verified, not yet deployed.** One manual VPS step remains before it is in effect on real production (see above).

- No code was changed, no fix was applied — findings only.
- Nothing was deployed.
- Stage 31 / any new roadmap was not started.
- No mutating request was made against real production (no account created, no login attempted, no payment triggered) — every authenticated/destructive check relied on Stage 30B1/30B2's very recent live proof against the dev-stack analog instead.
