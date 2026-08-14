# Stage 30 — Backups, recovery & final production hardening

Tracking doc — status only, not a spec restatement. This is the last stage in `ROADMAP_STAGE_21_30.md`; this file is the implementation plan produced by a preparation-only session. No code has been changed yet.

## Preparation session — Stage 30 scope inspection and sub-stage plan

Scope: read the roadmap and Stage 29's closeout, inspect only what's directly relevant to Stage 30 (existing volumes/CI-CD patterns, the exact Stage 20 deferral text the roadmap itself quotes), and split Stage 30 into small, independently-implementable sessions. No implementation, no VPS contact, no deploy.

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 30 section in full (quoted verbatim below) and confirmed via `git log`/`git status` that Stage 29 is fully committed and pushed (`f62fdb3 feat: complete stage 29 observability and health monitoring`), working tree clean — Stage 30's two stated dependencies (Stage 28: a deployed environment to back up; Stage 29: observability, to detect a failed backup job) are both satisfied.

Read `STAGE20_PROGRESS.md`'s own deferral note, which the roadmap quotes directly: *"Full-platform regression beyond the flows adjacent to Stage 20's own changes (i.e., payments/subscriptions, video pipeline, certificates, achievements, search/recommendations were not re-tested this stage — they share no code path with Stage 20's changes)."* This is the exact, still-outstanding gap Stage 30's regression half exists to close — it has been open since Stage 20 and was never picked up by Stages 21–29 (none of those stages' own scopes touched payments/video/certificates/achievements directly, though Stage 26's auth hardening did touch the shared middleware every one of those domains' endpoints passes through, which is itself a reason a fresh sweep is warranted now, not just a formality).

Inspected `docker-compose.yml` — confirmed the exact two named volumes that exist today (`postgres-data`, `minio-data`) and that no backup mechanism, cron service, or scheduled job of any kind exists anywhere in the repo (`grep -rln "pg_dump|backup"` across every `.yml`/`.go`/`.sh` file returns nothing). Listed `.github/workflows/` — confirmed `deploy.yml`'s established SSH pattern (the 4 `PROD_VPS_*` secrets, TOFU host-key trust, `webfactory/ssh-agent`) is the only existing precedent for anything touching the real VPS, and is the natural thing a backup workflow would reuse rather than invent a second, parallel SSH mechanism.

### 1. Exact Stage 30 scope, from the roadmap

> **Goal:** Close the last production-readiness gap — there is no backup/restore story today — and run the full-platform security and regression sweep Stage 20 explicitly deferred ... now that Stages 21–29 have added real new surface area on top of the existing platform.
> - **Backend/Ops scope:** Scheduled `pg_dump` backup (cron container or a scheduled GitHub Actions workflow hitting the production DB over a secure channel) targeting object storage (reuse the existing MinIO/S3 infra already in `docker-compose.yml`), with a retention policy and a written, **tested** restore runbook.
> - **Frontend scope:** None.
> - **Migration needs:** None.
> - **Security requirements:** Backup artifacts encrypted at rest and access-restricted (not a world-readable bucket); the full-platform security sweep re-confirms auth, IDOR, payment-provider-as-source-of-truth, video delivery, and every admin/instructor ownership boundary added or touched since Stage 20, in one consolidated pass.
> - **E2E/regression requirements:** Actually perform one restore (into a scratch database) and verify row counts/integrity match the source — a backup that has never been restored is not a verified backup. Run the full-platform regression pass every prior stage's own progress doc has been deferring: enrollment, learning, payments, certificates, Q&A, search, recommendations, admin/instructor dashboards, all smoke-tested together.
> - **Dependencies:** Stage 28 (deployed environment to back up), Stage 29 (observability, to detect a failed backup job).
> - **Estimated complexity:** Large — recommend a backup/restore session and a separate full-platform regression session, the same split pattern as Stage 20's 20C1/20C2.

### 2. Roadmap vs. current implementation

| Roadmap item | Current state |
|---|---|
| Scheduled `pg_dump` backup | **Does not exist.** No cron container, no scheduled workflow, confirmed by direct grep. |
| Backup destination: object storage | **Infra exists, unused for this purpose.** MinIO is already running (`docker-compose.yml`), already used for lesson videos/assignments — a new bucket or prefix for backups would reuse the exact same client/credentials pattern, not a new integration. |
| Retention policy | **Does not exist.** No design, no code. |
| Written, tested restore runbook | **Does not exist.** |
| Backup encryption at rest / access-restricted | **Does not exist yet, but the access-restriction half is already true by construction** — every MinIO bucket in this project has always been private (`docker-compose.yml`'s `minio-init` comment: "Never runs `mc anonymous set public`"), so a new backups bucket/prefix inherits that same posture automatically. Encryption of the dump artifact itself is the genuinely new piece. |
| Security sweep (auth, IDOR, payment-source-of-truth, video delivery, ownership) | **Not done since Stage 20.** Stage 26 hardened auth's *mechanics* (rate limiting, JWT validation) but was not itself a cross-domain IDOR/ownership sweep. |
| Full-platform regression (payments/video/certificates/achievements/search/recommendations/admin/instructor) | **Not done since Stage 20** — the exact gap `STAGE20_PROGRESS.md` names explicitly. |
| Migrations | None needed — confirmed by design below (no new tracking table; a failed backup is visible as a red GitHub Actions run, not a new health-check component — see Design note under 30A1). |

**Bottom line:** this is a genuine greenfield build for the backup/restore half, and a genuinely overdue (not hypothetical) regression sweep for the other half — both halves are real, unstarted work, not partially-done gaps.

### 3. Dependencies, migrations, infrastructure implications, security risks, production impact

- **No database migrations** — confirmed matches the roadmap's own explicit "None." A tempting alternative design (a `backup_runs` tracking table, read by Stage 29's deep health check as a new "last successful backup" component) was considered and rejected specifically because it would require a migration, contradicting the roadmap's own stated constraint. Backup-failure visibility comes from the GitHub Actions run's own status (a red run in the Actions tab), not a new health-check component — a deliberate, minimal design choice, not an oversight.
- **No new SSH mechanism** — the backup workflow reuses `deploy.yml`'s exact 4 `PROD_VPS_*` secrets (`PROD_VPS_HOST`/`PROD_VPS_USER`/`PROD_VPS_SSH_PORT`/`PROD_VPS_SSH_PRIVATE_KEY`), not a second parallel credential set.
- **Likely no new GHCR/Postgres/MinIO credentials transiting GitHub Actions at all** — if the dump, encryption, and upload-to-MinIO all happen in one script executed *on the VPS* (triggered remotely over the existing SSH connection), the script can read `POSTGRES_USER`/`POSTGRES_PASSWORD`/`S3_ACCESS_KEY`/`S3_SECRET_KEY` from the VPS's own already-present `.env` (or `docker compose exec postgres pg_dump` inheriting the running container's own env directly) — none of these need to be freshly transmitted from GitHub Actions on every run. This mirrors `deploy.yml`'s own established "VPS secrets stay on the VPS" philosophy from Stage 28A3.
- **One genuinely new secret is needed: a backup-encryption key/passphrase.** Without encrypting the dump before it lands in object storage, a MinIO compromise would expose the full database — including password hashes, PII, and payment records — in plaintext. Recommended to live once in the VPS's own `.env` (same "set once, manually" pattern as every other production secret since Stage 28A1), not re-transmitted through GitHub Actions on every run.
- **Retention/cleanup is a real risk surface, not just a nice-to-have.** A retention policy that's too aggressive (or buggy) silently deletes real recovery capability; one that's absent lets storage (and cost) grow unbounded forever. Needs a dry-run/verification step before any automated deletion ships, not just "delete anything older than N days" trusted on the first try.
- **Production impact of `pg_dump` itself:** a logical dump via `pg_dump` uses a consistent MVCC snapshot and does not take blocking locks against normal reads/writes at this database's current scale — a real but low risk, worth scheduling at a low-traffic time and monitoring the first few real runs' duration/impact rather than assuming it's free.
- **Regression-sweep risk:** none of the domains named (payments, video, certificates, achievements, search/recommendations, admin/instructor ownership) have been touched by *this* preparation session — inspecting them is explicitly 30B1/30B2's own job, not pre-empted here.

### 4. Sub-stage plan

Two independent tracks, matching the roadmap's own explicit recommendation ("a backup/restore session and a separate full-platform regression session") — further split narrowly, consistent with every other "Large" stage in this project (Stage 28's A1–A4+B, Stage 29's A1–A6). The two tracks don't depend on each other and could be done in either order; 30A is recommended first only because it's the narrower, more self-contained technical build.

| Sub-stage | One-line scope |
|---|---|
| 30A1 | Design + build the backup mechanism: `pg_dump`, encrypt, upload to object storage, scheduled trigger |
| 30A2 | Prove it: real restore into a scratch database, verify row counts/integrity |
| 30A3 | Retention policy + written runbook (reflecting what 30A2 actually proved) + backup/restore closeout |
| 30B1 | Security-focused sweep: auth, IDOR, payment-provider-as-source-of-truth, video delivery, ownership boundaries |
| 30B2 | Full-platform functional regression: enrollment, learning, payments, certificates, Q&A, search, recommendations, admin/instructor dashboards |
| 30B3 | Fix any verified findings from 30B1/30B2, final Stage 30 report, and — since this is the roadmap's last stage — a final Stages 21–30 completion statement |

---

#### 30A1 — Backup mechanism: design + build

- **Scope:** Decide and implement the scheduled backup path. Recommended design (to be confirmed/finalized by that session, not pre-decided here): a new `.github/workflows/backup.yml` on a `schedule:` cron trigger, reusing `deploy.yml`'s existing 4 SSH secrets, SSHing to the VPS and running one script that (1) `docker compose exec`s into the `postgres` container to run `pg_dump` (inheriting that container's own DB credentials, no new secret needed for this part), (2) encrypts the resulting dump with a passphrase already present in the VPS's own `.env` (the one genuinely new secret this stage needs, set once, manually, on the VPS — not transiting GitHub Actions), (3) uploads the encrypted artifact to a MinIO bucket/prefix reusing the existing S3 client credentials already on the VPS. The roadmap's alternative ("cron container") should be explicitly weighed against this at the start of that session, not skipped past.
- **Files/domains likely involved:** `.github/workflows/backup.yml` (new); possibly a small shell script committed alongside it (e.g. `deploy/backup/run-backup.sh`, mirroring `deploy/nginx/`'s existing convention of committing real, reviewable scripts rather than inlining everything in YAML); no application code, no migrations.
- **Verification:** `actionlint` + YAML/structural validation (same tooling used for every workflow since Stage 27); a real, live-fired backup run against an isolated throwaway Postgres+MinIO (never the real VPS), confirming a real encrypted dump file actually lands in object storage; confirm the encryption key is genuinely required (a dump encrypted without the right key should not be readable).
- **Stop condition:** A working, locally-proven backup mechanism exists and is validated end-to-end against throwaway infrastructure. Not yet restored (that's 30A2), not yet run against the real VPS.

#### 30A2 — Restore verification (the roadmap's explicit "tested" requirement)

- **Scope:** Actually restore a real backup artifact (produced by 30A1's mechanism) into a scratch/throwaway database and verify row counts and data integrity match the source — the roadmap's own explicit standard: "a backup that has never been restored is not a verified backup."
- **Files/domains likely involved:** Likely a restore counterpart to 30A1's script (e.g. `deploy/backup/restore.sh`); no application code changes expected.
- **Verification:** *Is* the verification — produce a real backup from a real (throwaway) database seeded with known data, decrypt it, restore it into a second, separate throwaway database, and diff row counts/checksums between source and restored copy.
- **Stop condition:** At least one real, successful, verified restore has been performed and its results recorded — not just "the restore command didn't error."

#### 30A3 — Retention policy + tested runbook + backup/restore closeout

- **Scope:** Design and implement the retention policy (e.g. keep N daily backups, decide the exact number based on this project's actual risk tolerance — not pre-decided here), including a dry-run/verification step before any real deletion ships. Write the restore runbook — reflecting the *actual* commands 30A2 proved work, not a theoretical procedure. Close out the backup/restore half of Stage 30.
- **Files/domains likely involved:** The same `.github/workflows/backup.yml`/scripts from 30A1 (extended with retention/cleanup logic); a new runbook document (likely `docs/RESTORE_RUNBOOK.md` or appended to `STAGE30_PROGRESS.md` itself, that session's call).
- **Verification:** Retention logic dry-run before enabling real deletion; the runbook itself re-verified by literally following it once, not just written and assumed correct.
- **Stop condition:** Retention policy implemented and safely verified; runbook written and proven by actually following it; backup/restore half of Stage 30 formally closed.

#### 30B1 — Security-focused sweep

- **Scope:** The roadmap's own four named concerns, checked deliberately, not folded into general smoke-testing: (1) auth — re-verify the Stage 26 hardening (rate limiting, JWT validation) still holds correctly across the endpoints added since; (2) IDOR — systematically check that every domain's ownership checks (a student can't read/modify another student's data, an instructor can't touch a course they don't own, etc.) are actually enforced, not just present in code; (3) payment-provider-as-source-of-truth — confirm subscription/access state is never trusted from client input, only from the payment provider's own confirmed state; (4) video delivery — confirm presigned URL scoping/expiry and access checks still hold.
- **Files/domains likely involved:** `internal/auth`, `internal/subscriptions`, `internal/videos`, `internal/ownership`, plus a systematic pass across every domain's handler-level auth checks (courses, tests, certificates, achievements, assignments, coding, qa, reports, reviews, wishlist, activity, recommendations) — read, not necessarily changed.
- **Verification:** Live, adversarial-style checks (attempt cross-user access, attempt to bypass payment-provider confirmation, attempt to access another user's presigned video URL) against a real running stack — the same live-verification discipline used throughout Stages 26–29, not a code-reading-only pass.
- **Stop condition:** Every one of the four named security concerns explicitly checked with a recorded result (pass, or a found-and-fixed issue); no unchecked item silently skipped.

#### 30B2 — Full-platform functional regression

- **Scope:** The roadmap's own named list, smoke-tested together: enrollment, learning (progress tracking), payments, certificates, Q&A, search, recommendations, admin dashboards, instructor dashboards. Closes the exact gap `STAGE20_PROGRESS.md` has been naming since Stage 20.
- **Files/domains likely involved:** Broad, read-mostly — `internal/learning`, `internal/subscriptions`, `internal/certificates`, `internal/achievements`, `internal/qa`, `internal/courses` (search), `internal/recommendations`, `internal/admin`, `internal/instructor`, and their frontend counterparts under `frontend/app/`.
- **Verification:** A real, live, end-to-end smoke pass through each named flow against a running stack (register → enroll → progress through a lesson → pass a test → earn a certificate → check achievements; a mock payment/subscription flow; Q&A ask/answer/moderate; search/recommendations returning sensible results; admin and instructor dashboards loading and reflecting real data) — not a unit-test-style check, a real walkthrough.
- **Stop condition:** Every named flow smoke-tested with a recorded pass/fail; any found bug fixed and re-verified, or explicitly deferred with a clear reason (matching this project's own "distinguish blockers from deferred" discipline from every prior stage's closeout).

#### 30B3 — Fixes, final Stage 30 report, and Stages 21–30 completion

- **Scope:** Fix anything 30B1/30B2 found and flagged as a real bug (not a deferred item); write the final Stage 30 report (mirroring Stage 28/29's own closeout structure: what's confirmed, what's deferred, formal status); and — because this is the roadmap's final stage — a closing statement on the full Stages 21–30 arc.
- **Files/domains likely involved:** Whatever 30B1/30B2 actually found — genuinely unknown until those sessions run; `STAGE30_PROGRESS.md` for the report itself.
- **Verification:** Any fix re-verified live, the same discipline as every other stage's own closeout.
- **Stop condition:** Stage 30 formally marked complete (or its remaining gaps explicitly listed as deferred, non-blocking); the roadmap's own 21–30 arc formally closed out.

### Not done this session (preparation session)

- **No code changed** — plan only, per instruction 8.
- **No implementation started** for any of the 6 sub-stages above.
- **No VPS contact, no deploy.**

---

## Stage 30A1 — Backup mechanism: design + build (implemented)

Scope: implement, and prove end-to-end against isolated throwaway infrastructure, the production PostgreSQL backup mechanism (`pg_dump` → encrypt → upload to object storage → scheduled trigger). No restore (30A2), no retention/runbook (30A3), no real VPS contact.

### Setup/inspection performed

Read `docker-compose.yml` (confirmed `postgres:17-alpine` includes `pg_dump`; `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB` already set on that container) and `docker-compose.prod.yml` (confirmed MinIO has `ports: !reset []` — no public port at all in production, which shaped the upload design below). Read `.github/workflows/deploy.yml` in full as the structural precedent to reuse (SSH secret names, TOFU host-key trust, `webfactory/ssh-agent@v0.9.0`, the "verify required secrets" step shape, and the established "VPS secrets stay on the VPS" philosophy).

### Design decisions

- **`pg_dump -Fc` (custom/compressed format)**, not plain SQL — already compressed, and supports selective/parallel restore via `pg_restore` in 30A2. Directly serves the roadmap's "suitable for reliable restore" requirement.
- **`pg_dump` runs inside the already-running `postgres` container** via `docker compose exec -T postgres sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" pg_dump ...'`, inheriting that container's own env — no new DB credential is constructed, transmitted, or logged anywhere.
- **One genuinely new secret: `BACKUP_ENCRYPTION_PASSPHRASE`.** AES-256-CBC with PBKDF2 (100k iterations), passphrase read via `-pass env:...` (never a CLI arg, so never visible in `ps aux`). Documented in `.env.production.example` with the same "set once, by hand, on the VPS, never committed, never transmitted through GitHub Actions" convention as every other production secret.
- **Upload reaches MinIO via `docker run --network container:<minio-container-id>`**, sharing MinIO's own network namespace to reach it at plain `localhost:9000` — avoids needing to know Compose's auto-generated network name, and works even though MinIO has no public port in production. Credentials passed via `MC_HOST_backup=http://key:secret@localhost:9000` (mc's documented env-var convention), never as literal CLI args, and never written to a persistent `~/.mc/config.json`.
- **Fail-closed at every stage:** `set -euo pipefail`; a pre-flight check that all required `.env` values are present before touching Postgres/MinIO at all; explicit non-empty-artifact checks after both `pg_dump` and `openssl enc`; a post-upload `mc stat --json` independently re-confirms the uploaded object exists with a non-zero size (parsed from JSON, not grepped from mc's human-readable table format, which isn't a stable target to match against).
- **No partial artifacts survive a failure:** the raw dump only ever exists in a `mktemp -d`, `chmod 700` scratch directory, removed by a `trap cleanup EXIT` regardless of how the script exits.
- **No new migration, no new tracking table** — matches the roadmap's explicit "Migration needs: None." A `backup_runs` table (readable by Stage 29's health check) was considered and rejected for this reason; backup-failure visibility comes from the GitHub Actions run's own red/green status instead.
- **Scheduling:** `.github/workflows/backup.yml`, `schedule: cron: "0 3 * * *"` (daily 03:00 UTC) plus `workflow_dispatch` for manual runs. Reuses `deploy.yml`'s exact 4 SSH secrets and TOFU pattern — no new CI credentials. The workflow only SSHes in, rsyncs the script to `$DEPLOY_DIR/backup/run-backup.sh`, and runs it; it never sees Postgres/MinIO/encryption credentials itself.

### Files changed

- `deploy/backup/run-backup.sh` (new) — the backup script, runs on the VPS.
- `.github/workflows/backup.yml` (new) — scheduled + manual trigger.
- `.env.production.example` (modified) — documents the one new required value, `BACKUP_ENCRYPTION_PASSPHRASE`.

### Configuration required (not yet done — VPS-side, manual, out of scope for this session)

- Set `BACKUP_ENCRYPTION_PASSPHRASE` in the real `/opt/lms/.env` on the VPS (e.g. `openssl rand -base64 48`), and store a copy somewhere durable outside the VPS (a password manager) — losing it makes every backup encrypted with it permanently unrestorable.
- No new GitHub Actions secrets needed — `backup.yml` reuses `deploy.yml`'s existing 4 `PROD_VPS_*` secrets.

### Verification performed

All against a purpose-built, throwaway Docker Compose project (`postgres:17-alpine` + `minio/minio`, mirroring the real service shape) — never the real VPS, never the live dev stack. Live dev stack (`course-*` containers) confirmed unaffected before and after.

- **Happy path, full pipeline, live, once:** seeded a real table (`backup_test_marker`, 3 rows) and ran the unmodified script. Result: `pg_dump OK (2916 bytes)` → `encryption OK (2944 bytes)` → real `mc cp` upload → independent `mc stat --json` re-confirmation → `SUCCESS`. Exit code 0.
- **Independent post-hoc confirmation:** separately listed the bucket (`mc ls`) and downloaded the artifact — `file` identified it as genuine `openssl enc'd data with salted password` (`Salted__` magic bytes), not a disguised plain dump; confirms encryption is real, not cosmetic.
- **Failure — pg_dump (instruction 9/10):** stopped the postgres container, re-ran. Result: `service "postgres" is not running` → `backup: pg_dump failed` → exit 1. No artifact uploaded (bucket listing unchanged).
- **Failure — missing config (instruction 9/10):** removed `BACKUP_ENCRYPTION_PASSPHRASE` from `.env`, re-ran. Result: fails at the pre-flight check before touching Postgres or MinIO at all — `backup: missing required config ... BACKUP_ENCRYPTION_PASSPHRASE` → exit 1.
- **Failure — upload (instruction 9/10):** stopped the minio container, re-ran. Result: `pg_dump OK` → `encryption OK` → `backup: minio container not found/running` → exit 1. No stale/partial object left in the bucket.
- **No leftover scratch directories** after any failure run — the `trap cleanup EXIT` fired correctly in every case.
- **No secret leakage (instruction 12):** grepped every captured log (happy path + all 3 failure runs) for the literal test values of `POSTGRES_PASSWORD`, `S3_SECRET_KEY`, and `BACKUP_ENCRYPTION_PASSPHRASE` — zero occurrences in any run's output.
- **Workflow validation:** `python3 -c "yaml.safe_load(...)"` structural check, and `actionlint` (reinstalled to `/tmp/gobin` this session) — zero findings across all workflow files including the new `backup.yml`.
- **All test Docker resources removed** (`docker compose down -v`, scratch directory deleted); confirmed the real `course-*` containers were running, unaffected, throughout.

### Not done this session

- **No real restore performed** — belongs to 30A2, per instruction 14.
- **No real VPS contact** — everything above ran against throwaway infrastructure only; `BACKUP_ENCRYPTION_PASSPHRASE` has not actually been set on the production VPS yet.
- **No retention policy, no runbook** — belongs to 30A3.
- **Stage 30A2, 30A3, 30B1 not started.**

### Remaining work

- **30A2:** Decrypt + `pg_restore` a real backup artifact into a scratch database; verify row counts/integrity against the source. This session proved the artifact is genuinely encrypted (`openssl enc'd data`) and structurally sound (correct size relationship, `pg_dump -Fc` format) but never decrypted/restored it — that's 30A2's explicit job.
- **30A3:** Retention policy (with a dry-run step before real deletion) + a runbook that reflects the exact commands 30A2 proves work.
- Before the schedule in `backup.yml` starts having real effect: set `BACKUP_ENCRYPTION_PASSPHRASE` on the production VPS's `.env` by hand (see Configuration required above).
