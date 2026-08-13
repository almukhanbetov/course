# Stage 27 — CI pipeline (build, lint, gate)

Tracking doc — status only, not a spec restatement.

## Stage 27A1 — backend CI workflow (this session)

Scope: `.github/workflows/backend-ci.yml` only — `gofmt`/`go build`/`go vet` on push and pull_request. No frontend CI, no deployment, no Docker image publishing, no secrets, no application code changes unless CI exposed a real issue (it didn't).

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 27 section. It describes a combined `.github/workflows/ci.yml` with both a backend job (`gofmt`/`go vet`/`go build`, plus `go test ./...` "if/when any tests exist" against an ephemeral Postgres service container) and a frontend job (`tsc`/`eslint`/`next build`). This session's explicit instructions narrow that to backend-only, and to exactly three checks — no `go test` (per this project's established convention: no automated test suite exists anywhere, confirmed again by `find backend -name '*_test.go'` finding nothing checked in), so no Postgres service container is needed either, since nothing in this workflow touches a database.

Inspected `backend/go.mod` (`go 1.25.7`), `backend/Makefile` (existing `build`/`migrate-*` targets — `build` already runs plain `go build -o bin/api ./cmd/api`, confirming `go build ./...` is a safe superset), and confirmed no `.github/` directory existed yet. Did not inspect `frontend/` at all, per instruction.

### Design decisions

- **`go-version-file: backend/go.mod`, not a hardcoded version string.** `actions/setup-go@v5` reads the `go 1.25.7` directive directly from the module file, so the workflow can never silently drift from what the module actually declares — bumping `go.mod` is the only thing ever needed to change CI's Go version too.
- **`cache: true` + `cache-dependency-path: backend/go.sum`.** `setup-go`'s built-in module/build cache, keyed off the lockfile in the `backend/` subdirectory (the module root is not the repo root, so the default cache-dependency-path guess would miss it).
- **`defaults.run.working-directory: backend`**, once, at the job level — every step's `run:` (`gofmt`, `go build`, `go vet`) executes from `backend/` without repeating `cd backend &&` in each step.
- **`gofmt -l .` wrapped in a shell check that exits 1 on any output**, rather than relying on `gofmt`'s own exit code — `gofmt -l` always exits 0 even when it lists unformatted files (it only prints filenames); the exit code alone would never fail the job.
- **Trigger: bare `push:`/`pull_request:` with no branch filter**, matching the literal instruction ("Trigger on: push, pull_request") — every push to every branch and every PR gets checked, not just `main`. Quoted as `"on":` in the YAML (not bare `on:`) purely to sidestep the well-known YAML 1.1 quirk where an unquoted `on` key is parsed as the boolean `true` by strict YAML 1.1 parsers (e.g. PyYAML) — GitHub's own workflow parser handles the bare form correctly either way, but the quoted form parses identically everywhere and avoids the ambiguity.
- **No Postgres service container, no `go test` step, no secrets** — none of the three required checks touch a database or need any credential, so adding either would be unused surface area, not defense in depth.

### Change made

`.github/workflows/backend-ci.yml` (new):
- Job `backend`, `runs-on: ubuntu-latest`, `working-directory: backend` default.
- Steps: `actions/checkout@v4` → `actions/setup-go@v5` (version from `go.mod`, module cache enabled) → `gofmt -l .` check (fails the job if any file is unformatted) → `go build ./...` → `go vet ./...`.

No application code changed — all three checks already pass cleanly against the current `backend/` tree (see Verification below), so instruction 10's "only if CI exposes a real backend build issue" condition was never triggered.

### Files changed

- `.github/workflows/backend-ci.yml` — new.
- `STAGE27_PROGRESS.md` — new (this file).

### Verification performed

**Workflow syntax, validated locally as far as practical without a live GitHub Actions run:**
- `python3 -c "yaml.safe_load(...)"` — parses without error; structural checks confirmed `on.push`, `on.pull_request`, and `jobs.backend` are all present and shaped as expected.
- `go install github.com/rhysd/actionlint/cmd/actionlint@latest` (the Go module proxy was reachable even though generic internet access is not) then ran `actionlint` — the standard purpose-built GitHub Actions workflow linter, which checks job/step schema validity, expression syntax, and known action input shapes against `actions/checkout`/`actions/setup-go`'s actual metadata. **Zero findings.**
- Not done: an actual push/PR through GitHub Actions itself (no CI run was triggered this session — that would require pushing, which is a separate, explicit action from writing the workflow file).

**The three checks themselves, run locally exactly as the workflow invokes them, against the current `backend/` tree:**

| Check | Result |
|---|---|
| `gofmt -l .` | Clean — no unformatted files |
| `go build ./...` | **OK** |
| `go vet ./...` | **OK** |

All pass today, so this workflow will go green on the very next push without needing any code change.

### Not done this session (explicitly out of scope for 27A1)

- **No frontend CI** — `tsc`/`eslint`/`next build` job intentionally not added; that's a separate, explicitly deferred sub-stage.
- **No CD/deployment** — no image build/push, no SSH/deploy step; that's Stage 28's scope entirely.
- **No secrets** — nothing in this workflow needs one (no DB, no external service), so none were added, per instruction.
- **No branch-protection configuration** — enabling "require this check to pass" is a GitHub repo-settings action, not a file in this repo; noted here as a recommendation, not performed (matches the roadmap's own "document, not enforce" framing).
- **No live GitHub Actions run** — validated locally (YAML parse + `actionlint` + running the exact three commands) but not yet proven against a real push/PR through GitHub's own runners.
- **No application code changes** — none needed; all three checks already pass.

## Stage 27A2 — frontend CI workflow (this session)

Scope: `.github/workflows/frontend-ci.yml` only — `npm ci`/typecheck/`eslint`/production build on push and pull_request. No backend changes, no deployment, no Docker image publishing, no secrets, no application behavior changes.

### Inspection performed

Read this file's 27A1 section and inspected `git status` (only `.github/` and `STAGE27_PROGRESS.md` untracked from 27A1, nothing else pending). Inspected `frontend/package.json` — `scripts.build` (`next build`), `scripts.lint` (`eslint .`), `scripts.typecheck` (`tsc --noEmit`) already exist, so the workflow only needed to call them, not invent new commands. Confirmed `frontend/package-lock.json` exists (required for `npm ci` and for `actions/setup-node`'s cache-dependency-path). Checked for a Node version pin: no `.nvmrc`, no `engines` field in `package.json`; found the actual production Node version instead, in `frontend/Dockerfile` (`FROM node:24-alpine`, all three build stages) — used that as the CI version so CI matches what's actually deployed. Read the existing `.github/workflows/backend-ci.yml` from 27A1 to mirror its structure/style. Did not inspect any backend domain code, per instruction.

### Design decisions

- **Separate `frontend-ci.yml` file, not a second job appended to `backend-ci.yml`.** Instruction 6 allowed either; a separate file was simplest and clearest here because it mirrors the existing `backend-ci.yml` naming/structure exactly (one workflow file per stack, `"on": push/pull_request` at the top, one job, `defaults.run.working-directory` scoped to that stack's directory), keeps each workflow independently readable in the Actions UI, and means this session touches zero bytes of the backend workflow file.
- **`node-version: "24"`, matching `frontend/Dockerfile`'s `node:24-alpine`**, not a different "latest LTS" guess — CI should build with the same major version that actually ships, and this is the one place in the repo that version is already pinned.
- **`actions/setup-node@v4` with `cache: npm` + `cache-dependency-path: frontend/package-lock.json`** — the built-in npm cache, keyed off the lockfile in the `frontend/` subdirectory (same reasoning as 27A1's `cache-dependency-path: backend/go.sum`: the package root isn't the repo root, so the default path guess would miss it).
- **`npm ci`, not `npm install`** — deterministic, lockfile-exact install, the standard CI choice; also matches what `frontend/Dockerfile`'s own `deps` stage already does.
- **Step order: install → typecheck → lint → build.** Fails fast on the cheapest checks first; `next build` (the slowest step, and the one that would also re-surface type errors) runs last.
- **ESLint's existing pre-existing warnings are not a CI failure.** `npm run lint` exits 0 with 4 `@next/next/no-img-element` warnings (plain `<img>` tags in `wishlist/page.tsx`, `ContinueLearningCard.tsx`, `CourseCard.tsx`, `RecommendationCard.tsx`) — ESLint's default behavior is to fail only on errors, not warnings, and ratcheting that to `--max-warnings 0` would fail the very first CI run on pre-existing code, which is an application-code change this session's scope excludes (instruction 7: "do not change application behavior"). Left as-is; noted below as a known limitation, not fixed.
- **Same `"on": push:` / `pull_request:` quoting as `backend-ci.yml`**, for the same reason noted in that file's 27A1 entry (sidesteps the YAML 1.1 bare-`on`-parses-as-boolean quirk in strict parsers).

### Change made

`.github/workflows/frontend-ci.yml` (new):
- Job `frontend`, `runs-on: ubuntu-latest`, `working-directory: frontend` default.
- Steps: `actions/checkout@v4` → `actions/setup-node@v4` (Node 24, npm cache) → `npm ci` → `npm run typecheck` → `npm run lint` → `npm run build`.

No application code changed — all four commands already pass cleanly against the current `frontend/` tree (see Verification below).

### Files changed

- `.github/workflows/frontend-ci.yml` — new.
- `STAGE27_PROGRESS.md` — this section added.

### Verification performed

**Workflow syntax, validated locally:**
- `python3 -c "yaml.safe_load(...)"` — parses without error; confirmed `on.push`, `on.pull_request`, and `jobs.frontend` present and shaped as expected.
- `actionlint` (same binary installed in 27A1 via `go install github.com/rhysd/actionlint/cmd/actionlint@latest`) — **zero findings**.
- Not done: an actual push/PR through GitHub Actions itself — no CI run was triggered this session.

**The four commands, run locally exactly as the workflow invokes them, against the current `frontend/` tree (local Node confirmed `v24.19.0`, matching the workflow's pinned major version):**

| Check | Result |
|---|---|
| `npm ci` | **OK** — 354 packages installed (pre-existing `npm audit` notices: 1 low + 1 moderate advisory, and an `allow-scripts` notice for `unrs-resolver`'s postinstall; both pre-date this session and are out of scope to address here) |
| `npm run typecheck` (`tsc --noEmit`) | **OK** — no errors |
| `npm run lint` (`eslint .`) | **OK** — exits 0; 4 pre-existing `@next/next/no-img-element` warnings, 0 errors |
| `npm run build` (`next build`) | **OK** — production build completed, all routes compiled |

All pass today, so this workflow will go green on the very next push without needing any code change.

### Not done this session (explicitly out of scope for 27A2)

- **No backend changes** — `backend-ci.yml` untouched, no backend application code inspected or modified.
- **No CD/deployment**, **no Docker image publishing**, **no secrets** — none needed, none added.
- **No application behavior changes** — the 4 pre-existing lint warnings were left as-is rather than "fixed" by swapping `<img>` for `next/image`, since that would be an application code change outside this session's CI-only scope.
- **No live GitHub Actions run** — validated locally (YAML parse + `actionlint` + running the exact four commands) but not yet proven against a real push/PR through GitHub's own runners.
- **27A3 not started.**

## Stage 27A3 — Docker Compose and migration CI checks (this session)

Scope: `.github/workflows/infra-ci.yml` only — `docker compose config` validity, backend image build, Postgres health, and a fresh-database migration run, plus (as far as practical) backend startup and a live health-endpoint check. No frontend/video-worker/notification-worker/code-runner builds, no full app E2E, no image publishing, no deployment, no real secrets.

### Inspection performed

Read this file's 27A1/27A2 sections and inspected `git status` (only `.github/` and `STAGE27_PROGRESS.md` untracked, nothing else pending). Inspected `docker-compose.yml` in full to enumerate every environment variable without a `${VAR:-default}` fallback across *every* service — not just the ones this job actually starts — since `docker compose config` resolves the whole file, so all of them need a value or the validation step would emit "variable is not set" warnings even for services never brought up (frontend, video-worker, notification-worker, code-runner, mailpit). Inspected `backend/migrations/` (40 numbered goose migration files, most recent `00040_create_audit_logs.sql`, consistent with Stage 25's audit log work). Inspected `internal/health/service.go` — the health check only pings the DB pool, nothing else — and the start of `cmd/api/main.go` — confirmed `videos.NewS3Storage` only builds an AWS SDK client with static credentials (`config.LoadDefaultConfig` + explicit `credentials.NewStaticCredentialsProvider`), it does not contact MinIO at startup, so the backend's actual runtime dependency for a successful boot is the database, not object storage — MinIO only ends up in this job's dependency graph because `backend`'s own `depends_on: minio-init: condition: service_completed_successfully` pulls it in, not because the Go code needs it reachable to start. Did not inspect any unrelated application domain (courses, tests, payments, etc.), per instruction.

### Design decisions

- **A third, separate workflow file (`infra-ci.yml`), not a job bolted onto `backend-ci.yml`.** Same reasoning as 27A2's separate-file choice: one workflow per concern, independently readable in the Actions UI, zero risk of touching the other two files.
- **CI-only values via the job's `env:` block, not a written `.env` file and not GitHub Actions secrets.** Every value (`ci`/`ci_password`/`ci-only-test-secret-not-for-production`/etc.) is a literal, obviously-fake placeholder committed directly in the workflow — visible, auditable, and satisfies instruction 7/8 exactly ("no production secrets," "safe CI-only environment values") without pulling in the GitHub Secrets mechanism for values that aren't sensitive in the first place. Deliberately distinct from both `.env.example`'s placeholder strings and the real running dev stack's actual `.env` values, so nothing here could be confused with (or accidentally match) a real credential.
- **Only `backend` is built.** `postgres`/`minio`/`mailpit` are pulled images; `migrate` uses the public `golang:1.25-alpine` image with a `command:`, no `Dockerfile`. `backend` is the only service in this job's dependency graph with a `build:` key, so `docker compose build backend` is already the complete "required services can build" check for what this job touches — `video-worker`/`notification-worker`/`code-runner`/`frontend` are independent services with no `depends_on` edge into `backend`/`migrate`/`postgres`, so they're correctly out of scope here (matches instruction 3, "prefer a focused CI setup," and instruction 4, "do not run the full application E2E suite").
- **`docker compose up -d --wait postgres`** as its own explicit step — `--wait` blocks until Postgres's existing healthcheck (`pg_isready`, already defined in `docker-compose.yml`) passes or the command fails, giving a clean, direct answer to instruction 2's "PostgreSQL starts healthy" rather than inferring it indirectly from a later step succeeding.
- **`docker compose up --exit-code-from migrate migrate`**, not `docker compose run migrate` — `migrate`'s `depends_on: postgres: condition: service_healthy` is already declared in the compose file, so this one command both waits on that dependency and surfaces the one-shot goose container's real exit code as the command's own exit code — a standard, direct way to gate CI on a one-shot service's success/failure.
- **Freshness of the database is structural, not an extra step.** Every job on a GitHub-hosted runner gets a brand-new VM and therefore a brand-new named volume (`postgres-data`) on every run — there is no way for a previous run's data to leak in. A defensive `docker compose down -v --remove-orphans` still runs first anyway (cheap, and it's the same command used for final teardown), covering the hypothetical case of a future self-hosted/reused runner, per instruction 9.
- **Health check via a plain `curl` retry loop (30 attempts × 2s = up to 60s), not `docker compose up --wait` for backend.** `backend` has no `healthcheck:` block defined in `docker-compose.yml` (unlike `postgres`/`minio`/`mailpit`/the three worker services) — `--wait` without a healthcheck only confirms the container reached the "running" state, not that the Go HTTP server has actually finished binding and is answering requests. A real HTTP GET against `/api/v1/health` is the only way to satisfy instruction 10's "health endpoint responds" literally, and doubles as the "backend can start after migrations" proof in the same step.
- **`docker compose ps` unconditionally (`if: always()`), backend logs only `if: failure()`.** Status is cheap and always useful; full logs are noisy and only worth the space when something needs debugging.
- **Teardown (`docker compose down -v --remove-orphans`) with `if: always()`** — runs even if an earlier step failed, so a failed CI run never leaves orphaned containers/volumes behind (matters more for a future self-hosted runner than for GitHub-hosted ones, but costs nothing either way).

### Change made

`.github/workflows/infra-ci.yml` (new):
- Job `infra`, `runs-on: ubuntu-latest`, CI-only env values for every non-defaulted variable the full `docker-compose.yml` references.
- Steps: checkout → `docker compose config` → `docker compose down -v --remove-orphans` (defensive reset) → `docker compose build backend` → `docker compose up -d --wait postgres` → `docker compose up --exit-code-from migrate migrate` → `docker compose up -d backend` → curl retry loop against `/api/v1/health` → `docker compose ps` (always) → `docker compose logs backend` (on failure) → `docker compose down -v --remove-orphans` (always, teardown).

No application code changed — every check passed cleanly against the current tree; CI never surfaced a real infrastructure bug (instruction 11's condition for a code change was never triggered).

### Files changed

- `.github/workflows/infra-ci.yml` — new.
- `STAGE27_PROGRESS.md` — this section added.

### Verification performed

**Workflow syntax, validated locally:**
- `python3 -c "yaml.safe_load(...)"` — parses without error; confirmed `on.push`, `on.pull_request`, and `jobs.infra` present and shaped as expected.
- `actionlint` (same binary from 27A1/27A2) — **zero findings**.

**Full pipeline, actually run locally end-to-end** (not just linted) — the exact sequence of `docker compose` commands the workflow uses, run against the real `docker-compose.yml`, with the same CI-only env values:

A safety concern surfaced during setup, addressed before running anything: this repository's own live dev stack (`course-*` containers) was running throughout this session, in this same directory. Since Compose's default project name is derived from the directory name, running the workflow's commands naively — especially the teardown step's `docker compose down -v` — risked stopping and deleting the user's live dev containers and volumes. Fixed by running the local validation under an isolated `COMPOSE_PROJECT_NAME=lmscitest` with remapped host ports (15432/18080/19000/19001/etc., vs. the live stack's 5434/8082/9000/9001/etc.), so the two stacks never shared a container, network, volume, or port. **This isolation applies only to how the validation was invoked locally — the workflow file itself is unchanged and uses ordinary ports/project-naming, which is correct and safe on an always-fresh GitHub-hosted runner where no such collision can exist.**

| Case | Result |
|---|---|
| `docker compose config -q` | **Valid** — no errors |
| `docker compose build backend` | **OK** — image built (mostly cache-hit layers) |
| `docker compose up -d --wait postgres` | **OK** — `lmscitest-postgres-1` reached `healthy` |
| `docker compose up --exit-code-from migrate migrate` against the fresh database | **OK** — all 40 migrations applied (`00001_init_extensions.sql` → `00040_create_audit_logs.sql`), goose reported "successfully migrated database to version: 40", container exit code 0 |
| `docker compose up -d backend` (after migrations) | **OK** — backend started; migrate re-ran automatically as part of satisfying its `depends_on: service_completed_successfully` condition and again exited 0 (goose migrations are idempotent/tracked, so this is expected, harmless, and actually a second live confirmation of migration re-run safety) |
| `curl -sf http://localhost:$BACKEND_PORT/api/v1/health` retry loop | **200**, `{"status":"ok","database":"ok"}`, succeeded on the 2nd attempt (~4s after `up -d backend`) |
| `docker compose down -v --remove-orphans` (teardown) | **OK** — all `lmscitest-*` containers, volumes (including `go-mod-cache`), and the network removed cleanly |
| Live dev stack (`course-*`) unaffected throughout | Confirmed via `docker ps --filter name=course-` immediately after teardown — all 8 services still `Up`, uptimes unbroken (e.g. `course-postgres-1: Up 43 minutes`), never touched |

Every scenario in scope — compose config validity, required service builds, Postgres health, fresh-database migrations, backend startup after migrations, and a live health-endpoint response — passed. No infrastructure bug was found; no application code changes were needed.

### Not done this session (explicitly out of scope for 27A3)

- **No full application E2E suite** — only compose validity + build + Postgres health + migrations + backend boot + `/health`, per instruction 4.
- **No image publishing, no deployment** — that's Stage 28's scope entirely.
- **No production secrets** — every value is an obviously-fake CI-only literal in the workflow file itself.
- **No frontend/video-worker/notification-worker/code-runner build or start** — outside this job's dependency graph and this session's focused scope.
- **No live GitHub Actions run** — validated locally (YAML parse + `actionlint` + the actual `docker compose` command sequence run end-to-end against a real, isolated stack) but not yet proven against a real push/PR through GitHub's own runners.
- **27B not started.**

## Stage 27B — combined CI quality gate (this session)

Scope: one new orchestrating workflow that depends on backend/frontend/infra CI and fails if any of them fails. No deployment, no image publishing, no secrets, no application code changes, no duplication of the existing checks' logic.

### Inspection performed

Read this file's 27A1/27A2/27A3 sections and inspected `git status` (only `.github/` and `STAGE27_PROGRESS.md` untracked, nothing else pending — none of the three prior workflow files had been committed yet). Inspected only the three existing workflow files (`backend-ci.yml`, `frontend-ci.yml`, `infra-ci.yml`) — their job IDs (`backend`, `frontend`, `infra`), triggers (`push`/`pull_request` on all three, independently), and step contents. Did not inspect any application domain, per instruction.

### Design decisions

- **The core constraint that shaped everything else: `needs:` only works between jobs *within the same workflow file*.** Since backend/frontend/infra CI are three separate files, a job cannot natively "depend on" them where they stand. GitHub Actions has exactly one first-class mechanism for a job in one workflow to depend on the *actual execution* of jobs defined in another workflow file without copying their steps: **reusable workflows** (`on: workflow_call` + `jobs.<id>.uses: ./path/to/workflow.yml` from a caller). That is the mechanism used here.
- **Chosen: one orchestrating workflow (`quality-gate.yml`) that calls all three as reusable workflows, plus a `needs`-gated `quality-gate` job.** This is "Option A" from the instruction's own framing, and was clearer here than "Option B" (a single dependent job) because Option B would require *merging* the three existing job bodies into one file to give `needs:` something to attach to in the first place — which is a bigger structural change than adding one new file, and reads as exactly the "duplicate/rewrite the checks" instruction 4 warns against (moving three independently-authored, independently-verified files into one is still touching all three, whereas calling them leaves each one exactly as it was authored in 27A1–27A3).
- **The three existing workflows' triggers changed from `push`/`pull_request` to `workflow_call`-only — their job bodies are untouched.** This was the one substantive trade-off decision this session made, and is worth stating plainly: without this change, every push would run each check *twice* — once from its own standalone trigger, once again as the orchestrator calls it — which is real, measurable duplication of CI compute, not just duplicated YAML. Instruction 4 ("do not duplicate the backend/frontend/infra checks unnecessarily") reads as being about exactly this kind of waste, not only about copy-pasted step definitions. Making them `workflow_call`-only means each check now runs exactly once per push/PR, invoked solely through the gate — the checks themselves are byte-for-byte identical to 27A1/27A2/27A3, only their entry point changed.
- **Instruction 5's "keep push/pull_request triggers" is satisfied at the system level, not per-file.** The new `quality-gate.yml` is what now carries `on: push` / `on: pull_request` for the whole Stage 27 CI system; the three called workflows still execute on every push/PR exactly as before, just triggered indirectly. Documented here explicitly since it's a legitimate alternate reading of that instruction, and the trade-off (the three checks can no longer be triggered completely standalone — e.g. via a manual re-run of just `Backend CI` in the Actions UI — only as part of the combined gate, or via `workflow_dispatch` if that were added later) is real and worth being visible rather than silently baked in.
- **`quality-gate` job runs unconditionally (`if: always()`) and explicitly fails via `exit 1`, rather than relying on default skip propagation.** GitHub's default behavior is: if a needed job fails, jobs that `need` it are *skipped*, not failed — and a skipped required check can be ambiguous or even permissive in some branch-protection configurations. Making the gate job always run, inspect `needs.*.result` for all three, and explicitly `exit 1` on anything other than `success` gives one unambiguous, always-present status check to point branch protection at, per instruction 2 ("must fail if any required Stage 27 CI job fails") — not "might be silently skipped instead."
- **Alternative considered and rejected: a `workflow_run`-triggered polling/aggregator workflow** that leaves all three existing files completely untouched (including their own `push`/`pull_request` triggers) and instead queries the GitHub API for each workflow's latest run conclusion on the same commit SHA once all three have completed. This would avoid re-triggering the checks entirely — arguably even less "duplication" than the chosen approach. Rejected because it's materially more complex (a scripted API-polling step instead of native `needs:`), triggers the gate workflow up to three times per commit (once per completing source workflow, with only the last being authoritative), and carries known sharp edges around `workflow_run` timing and fork-PR permissions — none of which apply to the reusable-workflow approach. Noted here as the considered-but-not-chosen alternative, per instruction 3's "whichever is clearest."

### Change made

`.github/workflows/quality-gate.yml` (new):
- Jobs `backend`, `frontend`, `infra` — each `uses:` the corresponding existing workflow file as a reusable workflow (no step logic duplicated).
- Job `quality-gate` — `needs: [backend, frontend, infra]`, `if: always()`, one step that checks all three `needs.*.result` values and fails with a clear message if any is not `success`.

`.github/workflows/backend-ci.yml`, `frontend-ci.yml`, `infra-ci.yml`:
- `"on":` changed from `push:`/`pull_request:` to `workflow_call:`. No other line changed in any of the three files — job IDs, step names, commands, and env values are byte-for-byte identical to 27A1/27A2/27A3.

### Files changed

- `.github/workflows/quality-gate.yml` — new.
- `.github/workflows/backend-ci.yml` — trigger only.
- `.github/workflows/frontend-ci.yml` — trigger only.
- `.github/workflows/infra-ci.yml` — trigger only.
- `STAGE27_PROGRESS.md` — this section added.

No application code changed.

### Quality gate design (summary)

```
push / pull_request
        │
        ▼
  quality-gate.yml
   ├── backend  → uses backend-ci.yml   (gofmt / go build / go vet)
   ├── frontend → uses frontend-ci.yml  (npm ci / typecheck / eslint / next build)
   ├── infra    → uses infra-ci.yml     (compose config / build / postgres / migrate / health)
   └── quality-gate (needs all three, if: always())
        → exits 1 unless backend, frontend, and infra all report "success"
```

**Required checks:** `backend`, `frontend`, `infra` (Stage 27A1/27A2/27A3's own logic, unchanged) and the new `quality-gate` job, which is the single check meant to be set as required in GitHub branch protection — its own pass/fail already encodes all three underlying results.

### Verification performed

**Workflow syntax, validated locally as far as practical:**
- `python3 -c "yaml.safe_load(...)"` on all four workflow files — all parse cleanly.
- `actionlint` (same tool used in 27A1–27A3; reinstalled this session since the prior binary lived in an ephemeral `/tmp` path from a previous session) run against all four files together — **zero findings**, including validation of the `uses: ./.github/workflows/*.yml` local reusable-workflow references and the `needs:`/`if:` job graph.
- Installed `act` (a local GitHub Actions runner) via `go install github.com/nektos/act@latest` and ran `act -l` (list/plan mode, no containers executed) against `quality-gate.yml` — it correctly resolved the reusable-workflow calls and produced the expected two-stage dependency graph: **Stage 0** — `backend`, `frontend`, `infra` (parallel); **Stage 1** — `quality-gate` (depends on all three). This independently confirms the `needs:` graph is structurally correct, not just YAML-valid.
- Simulated the `quality-gate` step's bash logic directly (not through Actions, since a full `act` run would rebuild backend/frontend/infra images redundantly with no new information — those step bodies were already proven working, unchanged, in 27A1–27A3): all-`success` → exits 0 with the "all passed" message; any one of the three as `failure` → exits 1; any one as `cancelled` (representing a skip/cancellation, not just an explicit failure) → also exits 1. All three cases behaved exactly as designed.
- Not done: an actual push/PR through GitHub Actions itself, and a full `act` execution of the called workflows' real steps (Docker-in-Docker for `infra-ci.yml` in particular is heavy and was already proven to work end-to-end locally in 27A3, against the unmodified job body).

### Not done this session (explicitly out of scope for 27B)

- **No deployment, no image publishing, no secrets** — none needed, none added.
- **No application code changes.**
- **No live GitHub Actions run** proving the gate actually fails on a broken commit and passes on a clean one through GitHub's own runners — validated locally (syntax + dependency graph + gate logic) but not fired through a real push.
- **27C not started.**

### Known limitations

- **The three underlying checks can no longer trigger standalone.** Since `backend-ci.yml`/`frontend-ci.yml`/`infra-ci.yml` are now `workflow_call`-only, they no longer appear as independent checks in a PR unless invoked via `quality-gate.yml` (or manually via `workflow_dispatch`, which hasn't been added). This is intentional (see Design decisions) but is a real behavior change from 27A1–27A3, worth calling out explicitly since those sessions' own docs describe them as independently push/PR-triggered.
- **Not yet proven against a real GitHub Actions run.** Everything above was validated locally (YAML parsing, `actionlint`, `act -l`'s dependency-graph resolution, and a direct simulation of the gate's bash logic) — this is "as far as practical" without pushing to GitHub, but is not the same as watching the gate actually go red on a broken commit and green on a clean one through GitHub's own infrastructure.
- **No `workflow_dispatch` trigger anywhere.** None of the four workflows can currently be run manually from the Actions UI — not required by this session's scope, but worth noting as a small follow-up if manual re-runs of just one check are ever wanted.
- **Branch protection is still just a recommendation, not configured.** Per 27A1's own note, requiring `quality-gate` (or the three individual jobs) to pass before merging `main` is a GitHub repo-settings action outside this repo's files.

## Stage 27C — final CI pipeline verification + Stage 27 report (this session)

Scope: re-verify all four Stage 27 workflows together (trigger design, dependency graph, syntax, security posture), run only the local checks not already exhaustively proven in 27A1–27A3, fix anything found (nothing was), and close out Stage 27 with a final report. No Stage 28, no deployment, no unrelated regression.

### Inspection performed

Read this file's 27A1/27A2/27A3/27B sections and inspected `git status` (only `.github/` and `STAGE27_PROGRESS.md` untracked, unchanged since 27B — no drift, nothing committed yet). Re-read all four workflow files (`backend-ci.yml`, `frontend-ci.yml`, `infra-ci.yml`, `quality-gate.yml`) directly rather than trusting this doc's own description of them — content matched exactly what 27B recorded, byte-for-byte. Did not inspect any application domain, per instruction.

### Verification performed

**Trigger design:**

| Check | Result |
|---|---|
| `push` triggers `quality-gate.yml` | Confirmed — `on: push:` present |
| `pull_request` triggers `quality-gate.yml` | Confirmed — `on: pull_request:` present |
| `backend-ci.yml`/`frontend-ci.yml`/`infra-ci.yml` are not independently triggered | Confirmed — all three declare `on: workflow_call:` only, no `push`/`pull_request` of their own |
| Reusable workflows are callable correctly | Confirmed — all three files declare `workflow_call`; `quality-gate.yml` references them via `uses: ./.github/workflows/{backend,frontend,infra}-ci.yml`, and all three paths exist on disk exactly as referenced |

**Dependency graph** — verified two independent ways:
1. `actionlint` (schema/reference validation) — **zero findings** across all four files together, including the `uses:`/`needs:`/`if:` graph.
2. `act -l -W .github/workflows/quality-gate.yml` (an actual local GitHub Actions planner, not just a linter) — resolved the real execution plan:

   | Stage | Jobs | Events |
   |---|---|---|
   | 0 | `backend`, `frontend`, `infra` | `push, pull_request` |
   | 1 | `quality-gate` | `push, pull_request` |

   This confirms backend/frontend/infra run in parallel (same stage) and `quality-gate` genuinely waits for all three (next stage), independent of what the YAML merely *says* — `act` computed this from the actual `needs:` graph.
3. `quality-gate`'s pass/fail bash logic re-simulated directly against 6 scenarios: all-`success` → **pass**; `backend`=`failure` → **fail**; `frontend`=`failure` → **fail**; `infra`=`failure` → **fail**; `infra`=`cancelled` → **fail**; `infra`=`skipped` → **fail**. All six behaved exactly as designed — the gate fails on any non-success result from any of the three, not just on an explicit "failure".

**Other validation (instruction 4):**

| Check | Result |
|---|---|
| YAML syntax, all 4 files | Parses cleanly (`yaml.safe_load`) |
| `actionlint`, all 4 files | Zero findings |
| Referenced workflow paths | All three `uses: ./.github/workflows/*.yml` targets exist and declare `workflow_call` |
| Permissions | No `permissions:` block declared in any of the four files — default `GITHUB_TOKEN` permissions apply. Nothing in any job needs write access (checkout is read-only, no PR comments/releases/packages/deployments are touched), so this is safe as-is; noted below as an optional least-privilege hardening opportunity, not a bug |
| No required secrets | `grep -rn "secrets\."` across `.github/workflows/` — zero matches in any of the four files |
| No deployment/image publishing | `grep` for `docker push`/`docker/build-push-action`/`docker login`/`deploy`/`release`/`ssh`/`scp`/`rsync` across all four files — zero matches |

**Local checks — run only where not already expensively proven in 27A1–27A3** (per instruction 5, since the only change to `backend-ci.yml`/`frontend-ci.yml`/`infra-ci.yml` since those sessions was the `on:` trigger block — every step body is byte-for-byte unchanged):

| Check | This session | Why |
|---|---|---|
| `gofmt -l .` / `go build ./...` / `go vet ./...` | **Re-run, all clean/OK** | Cheap (seconds); worth a fresh confirmation |
| `docker compose config -q` | **Re-run, valid** | Cheap sanity check that the compose file itself hasn't drifted |
| `frontend/package.json` scripts (`build`/`lint`/`typecheck`) | **Presence-checked, not re-executed** | The scripts `frontend-ci.yml` calls still exist unchanged; a full `npm ci && npm run build` was already exhaustively proven in 27A2 and would add ~1–2 minutes for zero new information |
| Full `docker compose build/up/migrate/health` pipeline | **Not re-run** | Already proven end-to-end in 27A3 (all 40 migrations, backend boot, live `/health` 200) against this exact, unchanged job body; re-running would be the "expensive work" instruction 5 explicitly says to skip when already clearly verified |

No bugs were found anywhere in this verification pass, so instruction 6 ("fix only verified Stage 27 CI bugs") had nothing to act on — no files were changed this session.

### Final workflow architecture

```
push / pull_request
        │
        ▼
  quality-gate.yml  (the only workflow with its own push/pull_request trigger)
   ├── backend  → uses backend-ci.yml   (gofmt / go build / go vet)              ┐
   ├── frontend → uses frontend-ci.yml  (npm ci / typecheck / eslint / next build) ├─ Stage 0, parallel
   ├── infra    → uses infra-ci.yml     (compose config / build / postgres / migrate / health) ┘
   └── quality-gate (needs: [backend, frontend, infra], if: always())            ── Stage 1
        → exits 1 unless backend, frontend, and infra all report "success"
```

Four files total: three reusable, `workflow_call`-only workflows holding the actual check logic (unchanged since 27A1/27A2/27A3), and one orchestrating workflow (`quality-gate.yml`, added in 27B) that is the sole push/PR entry point and the one recommended required status check.

### Files changed

**None this session.** 27C was verification-only — every check passed against the state left by 27B, so no workflow file or application code was touched.

### Known limitations

- **The three underlying checks cannot trigger standalone** — by design since 27B (`workflow_call`-only), they only run via `quality-gate.yml` or a manual `workflow_dispatch` (not currently added).
- **No `permissions:` block declared on any workflow** — safe today since no job needs write access, but not explicit least-privilege; a future hardening pass could add `permissions: contents: read` to each file.
- **Not yet proven against a real GitHub Actions run.** All verification across 27A1–27C has been local: YAML parsing, `actionlint`, `act -l`'s real dependency-graph resolution, direct execution of the underlying checks (gofmt/build/vet, and in 27A3 the full Docker Compose/migration pipeline), and direct simulation of the gate's bash logic. None of this is the same as watching `quality-gate` actually go red on a broken commit and green on a clean one through GitHub's own infrastructure — that remains the one meaningful gap.
- **No `workflow_dispatch` trigger anywhere** — none of the four workflows can be run manually from the Actions UI.
- **Branch protection is still unconfigured** — requiring `quality-gate` to pass before merging `main` is a GitHub repo-settings action, outside this repo's files, not yet done.
- **The 4 pre-existing ESLint `no-img-element` warnings and `npm audit` advisories** (noted in 27A2) remain — cosmetic/dependency items, not CI blockers, out of scope for Stage 27.

### Final Stage 27 status: **COMPLETE (locally verified; live GitHub Actions run still pending)**

- **27A1** (backend CI), **27A2** (frontend CI), **27A3** (infra/migration CI), **27B** (combined quality gate) — all implemented, and every check each one performs has been directly executed and proven to pass at least once (27A1–27A3 via full local runs; the gate's own logic via `act -l` graph resolution plus direct bash-logic simulation in 27B and re-confirmed in 27C).
- **27C** (this session) re-verified the whole system together — trigger design, dependency graph, syntax, security posture (no secrets, no deployment, permissions default-safe) — found zero bugs, changed zero files.
- The one honest gap, called out consistently since 27A1: none of this has been fired through a real push/PR on GitHub's own runners yet. Locally, everything that can be checked without that step has been checked, more than once, two different ways where practical (linter + planner + direct simulation). Recommended next action, whenever wanted: push this branch, open a PR, and watch `quality-gate` actually run — plus the roadmap's own suggested check of deliberately breaking one job to confirm the gate goes red, then fixing it to confirm the gate goes green.

## Remaining for Stage 27 (not started)

- **Live-fire proof the gate is real**, per the roadmap's own E2E requirement — push a deliberately broken commit to a throwaway branch, confirm `quality-gate` fails and correctly reports which of backend/frontend/infra broke, then push a clean commit and confirm it passes. Requires an actual push to GitHub; not done in any Stage 27 session.
- **Branch protection** — configure `main` to require the `quality-gate` check before merge (GitHub repo settings, not a code change).
- **Optional hardening** — explicit `permissions: contents: read` on each workflow (least-privilege, not currently a problem).
- **Optional cleanup** (not a Stage 27 blocker) — the 4 pre-existing ESLint warnings and `npm audit` advisories from 27A2.
