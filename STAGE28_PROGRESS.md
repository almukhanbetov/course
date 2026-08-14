# Stage 28 — CD & production deployment automation

Tracking doc — status only, not a spec restatement.

## Stage 28A1 — production deployment design and CI/CD preparation (this session)

Scope: design only. No image builds, no image publishing, no VPS access, no production infrastructure changes, no real secrets, no `deploy.yml`. This session's deliverable is the plan the next Stage 28 sub-sessions will implement against.

### Inspection performed

Read `ROADMAP_STAGE_21_30.md`'s Stage 28 section — its own complexity note explicitly recommends splitting Stage 28 into three sub-sessions ("a Dockerfile/image-hardening session, a deploy-workflow session, and an Nginx/HTTPS session"), which this design adopts directly as the shape of 28A2/28A3/28A4 (see "Remaining for Stage 28" below). Read `STAGE27_PROGRESS.md` in full — Stage 27's `quality-gate` workflow is the dependency this design builds on ("CI gate must pass before anything deploys," per the roadmap). Inspected `git status` (clean, working tree matches `81d9e5b ci: complete stage 27 quality pipeline`).

Inspected `backend/Dockerfile` (two-stage: `golang:1.25-alpine` build → `alpine:3.20` runtime, `CGO_ENABLED=0`, copies just the compiled binary, `EXPOSE 8080`) and `frontend/Dockerfile` (three-stage: `node:24-alpine` deps → build → run, copies `public`/`.next`/`node_modules`/`package.json`, `npm run start`, `EXPOSE 3000`). Both already multi-stage; neither currently drops to a non-root `USER`, and the frontend's final stage carries the full `node_modules` rather than Next.js's `output: "standalone"` trace (which would be meaningfully slimmer). Also found `backend/Dockerfile.worker`, `backend/Dockerfile.notification-worker`, `backend/Dockerfile.runner` — three more buildable images this repo already defines, for `video-worker`, `notification-worker`, and `code-runner` respectively — all following the same golang-alpine two-stage pattern as the main backend Dockerfile.

Inspected `docker-compose.yml` in full (again, having read it closely in Stage 26/27 sessions too) for exactly which services have a `build:` key (`backend`, `video-worker`, `notification-worker`, `code-runner`, `frontend` — five total) versus which use a public/pulled image (`postgres`, `minio`, `minio-init`, `mailpit`, and `migrate`, which runs `golang:1.25-alpine` directly with a `command:`, no `Dockerfile` of its own). Confirmed `migrate`'s service definition bind-mounts `./backend/migrations:/migrations:ro` from the **host filesystem**, not from inside a built image — a concrete, non-obvious design fact that shapes the migration-ordering section below. Confirmed `backend`/`frontend` have no `healthcheck:` block (unlike `postgres`/`minio`/`mailpit`/the three worker services, which all do).

Inspected all four `.github/workflows/*.yml` files (`backend-ci.yml`, `frontend-ci.yml`, `infra-ci.yml`, `quality-gate.yml`) — confirmed none of them build with production tags, none push to any registry, and `quality-gate` is the one existing check this design assumes `deploy.yml` will require passing before it ever runs. Checked `git remote -v`: `origin` is `https://github.com/almukhanbetov/course.git` — used below to derive the GHCR image-naming convention. Found no existing Nginx config, no existing deploy scripts, no `.env.production*` file anywhere in the repo before this session. Did not inspect any application domain code, per instruction.

## Production deployment design

### Images that need to be built

| Image | Source | Dockerfile | Registry path (proposed) |
|---|---|---|---|
| Backend API | `backend/` | `backend/Dockerfile` | `ghcr.io/almukhanbetov/course-backend` |
| Frontend | `frontend/` | `frontend/Dockerfile` | `ghcr.io/almukhanbetov/course-frontend` |
| Video worker | `backend/` | `backend/Dockerfile.worker` | `ghcr.io/almukhanbetov/course-video-worker` |
| Notification worker | `backend/` | `backend/Dockerfile.notification-worker` | `ghcr.io/almukhanbetov/course-notification-worker` |
| Code runner | `backend/` | `backend/Dockerfile.runner` | `ghcr.io/almukhanbetov/course-code-runner` |

Not built/published: `postgres` (official `postgres:17-alpine`), `minio`/`minio-init` (official `minio/minio`, `minio/mc`), `mailpit` (official `axllent/mailpit`) — all pulled directly from their public upstreams in production too, unchanged from dev. `migrate` is a special case, addressed under "Migration ordering" below — it is not an image this repo builds and publishes.

### Image registry strategy

**Recommendation: GitHub Container Registry (`ghcr.io`), not Docker Hub.**

- Authenticates in the build workflow with the automatically-provided `GITHUB_TOKEN` (job needs `permissions: packages: write`) — **zero additional registry secret**, directly serving instruction 5's "do not add real production secrets" concern by simply needing one fewer secret to exist at all.
- Docker Hub was considered and rejected for this specific reason: it would need a `DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN` secret pair for no offsetting benefit here.
- **Tagging:** every image gets an immutable `sha-<7-char-commit-sha>` tag on every build from `main`; a floating `latest` tag is also pushed for convenience (e.g. local pulls, debugging) but **is never what the deploy workflow or a rollback references** — the running `.env` on the VPS always pins an explicit `sha-*` tag (see `.env.production.example`'s `IMAGE_TAG`), so "what's currently deployed" is always a specific, addressable, reproducible build, and rollback is always "point at a different specific SHA tag," never "hope `latest` still means what it used to."
- **Visibility:** GHCR packages default to private, inheriting the repo's own visibility unless changed — the VPS then needs one authenticated `docker login ghcr.io` (a read-only PAT, or a fine-grained token scoped to `packages:read`) to pull. Making the packages public would remove that one login step at the cost of exposing image contents (backend binary code, dependency versions) publicly; recommendation is to **keep them private** and accept the one extra login step, since there's no compelling reason to publish image internals — this is a decision for the deploy-workflow sub-session to implement, not this session.

### Production compose strategy

**Recommendation: an override file, not a parallel standalone file.** `docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d` — the base `docker-compose.yml` keeps defining the full service graph, `depends_on` conditions, and healthchecks exactly once (avoiding dev/prod drift on the parts that shouldn't differ), while `docker-compose.prod.yml` overrides only what genuinely needs to differ:

- Replace every `build:` key (backend, frontend, video-worker, notification-worker, code-runner) with `image: ${IMAGE_REGISTRY}/course-<service>:${IMAGE_TAG}` — the VPS never builds anything, only pulls.
- Drop or localhost-bind the `ports:` mappings for `postgres`, `minio` (both API and console), and `mailpit` — none of these need to be reachable from outside the VPS in production; only a reverse proxy (Nginx, Stage 28's third planned sub-session) should be bound to `0.0.0.0:80`/`:443`. Postgres/MinIO/Mailpit are reached by other containers via their Compose service name on the internal network, exactly as in dev.
- Add the Nginx reverse-proxy service (not designed in detail here — explicitly deferred to the "Nginx/HTTPS session" the roadmap already calls for).

This file does not exist yet — per instruction 8, creating it now would be implementation, not design, and the roadmap explicitly scopes it to the deploy-workflow sub-session. Documented here as the target shape for that session to build against.

### Required environment/secrets

GitHub **Environment** secrets (a `production` Environment, not repo-wide secrets — Environments support required reviewers/approval gates, which is appropriate for anything that touches the real VPS):

| Secret | Purpose | Existing dev equivalent |
|---|---|---|
| `DEPLOY_SSH_HOST` | VPS address | — |
| `DEPLOY_SSH_USER` | Non-root deploy user | — |
| `DEPLOY_SSH_PRIVATE_KEY` | Key for that user | — |
| `DEPLOY_SSH_KNOWN_HOSTS` | Pinned host key, avoids interactive/MITM-able host-key prompts | — |
| `PROD_POSTGRES_USER` / `PROD_POSTGRES_PASSWORD` | Real DB credentials | `.env`'s `POSTGRES_USER`/`POSTGRES_PASSWORD` |
| `PROD_JWT_SECRET` | Real signing secret — **must not equal** `.env.example`'s placeholder string, matching the roadmap's explicit "no dev-only default can silently reach production" requirement | `.env`'s `JWT_SECRET` |
| `PROD_MINIO_ROOT_USER` / `PROD_MINIO_ROOT_PASSWORD` | Real object-storage credentials (also used as `S3_ACCESS_KEY`/`S3_SECRET_KEY`, per the existing "must match" convention already documented in `.env.example`) | `.env`'s `MINIO_ROOT_USER`/`MINIO_ROOT_PASSWORD` |
| `PROD_SMTP_HOST` / `_PORT` / `_USER` / `_PASSWORD` / `_FROM_EMAIL` | A **real** SMTP provider — Mailpit is explicitly dev-only (its own compose comment: "nothing is ever actually delivered"). **Which provider is an open decision, not resolved by this session** — flagged as a blocker for real production readiness, independent of the CI/CD mechanics this design covers | `.env`'s `SMTP_*` (all point at Mailpit) |
| `PROD_PAYMENT_PROVIDER` + provider keys | A real payment integration — **`internal/config/config.go`'s own comment states "there is no real provider wired up yet"; only `PAYMENT_PROVIDER=mock` exists in code today.** This is a product/backend gap, not a deployment-pipeline gap — flagged prominently, not solved here | `.env`'s `PAYMENT_PROVIDER=mock` |

No registry secret needed (GHCR uses `GITHUB_TOKEN`, see above). Non-sensitive values (`NEXT_PUBLIC_API_URL`, the production domain) belong in GitHub Actions **variables** (`vars:`), not `secrets:` — they're not confidential, just environment-specific.

**VPS-side:** the values above ultimately need to exist in a `.env` file on the VPS itself (compose reads from the working directory's `.env`, same as dev) — the deploy workflow's job is to get them there without ever printing them to a log or committing them. Two viable approaches, to be decided in the deploy-workflow sub-session: (a) template the file over SSH from the GitHub secrets each deploy (`scp`/heredoc, values never touch disk in the runner beyond the job's own memory), or (b) set it once, manually, directly on the VPS, and have the deploy workflow only ever update `IMAGE_TAG` there. **(b) is the safer default** — it minimizes how often real secrets transit the deploy pipeline at all, at the cost of manual secret rotation being a manual VPS-side step rather than a re-run of the workflow. Recorded here as a decision the deploy-workflow session needs to make explicitly, not decided by this session.

### Health checks

- `backend` already exposes `GET /api/v1/health` (`internal/health/service.go`, unchanged since Stage 27A3) — pings the DB pool, returns `{"status":"ok","database":"ok"}` or a degraded status. This is what a post-deploy smoke check should poll, through Nginx once it exists (`https://<domain>/api/v1/health`), using the same retry-loop pattern already proven working in `infra-ci.yml` (Stage 27A3): repeated `curl -sf`, bounded attempts, fail loudly if it never returns 200.
- Neither `backend` nor `frontend` has a Compose-level `healthcheck:` block today (unlike the three worker services and `postgres`/`minio`/`mailpit`). Recommend adding one to `backend` (`curl -f http://localhost:8080/api/v1/health`, mirroring the pattern the worker services already use) as part of the Dockerfile-hardening sub-session — this would let `docker compose up -d --wait` on the VPS give a real signal too, not just the deploy workflow's own external curl loop. Not added this session (instruction 7: no production infrastructure changes; this is a `docker-compose.yml` change with no urgency until the deploy workflow exists to benefit from it).
- Frontend has no equivalent internal check; a plain `curl -f http://localhost:3000/` is enough for a basic liveness signal if a `healthcheck:` is added there too — same deferral as above.

### Deployment ordering

```
merge to main
      │
      ▼
 quality-gate (Stage 27) must already be green — branch protection enforces this
      │
      ▼
 deploy.yml (does not exist yet — Stage 28's deploy-workflow sub-session)
      │
      ├─ 1. Verify required secrets are present — fail closed immediately if not
      │     (roadmap's explicit requirement; see Failure conditions)
      │
      ├─ 2. Build all 5 images, tag :sha-<short-sha> (and :latest)
      │
      ├─ 3. Push all 5 images to ghcr.io
      │
      ├─ 4. SSH to the VPS (key-based, known_hosts pinned)
      │
      ├─ 5. On the VPS: refresh docker-compose.yml + docker-compose.prod.yml
      │     + backend/migrations/ (see Migration ordering — these are not
      │     baked into any image today) and update IMAGE_TAG in the VPS's .env
      │
      ├─ 6. docker compose -f docker-compose.yml -f docker-compose.prod.yml pull
      │     — pull BEFORE touching any running container; if this fails,
      │     nothing currently running is disturbed
      │
      ├─ 7. Run migrations to completion, exit-code gated
      │     (docker compose up --exit-code-from migrate migrate — same
      │     pattern already proven in Stage 27A3's infra-ci.yml)
      │
      ├─ 8. Only if migrations succeeded: docker compose up -d
      │     (brings up backend → frontend → workers, ordering already
      │     encoded in docker-compose.yml's depends_on graph, unchanged)
      │
      ├─ 9. Health-check loop against the real public endpoint
      │
      └─ 10. Fail loudly (and see Rollback approach) if the health check
            never passes; otherwise the deploy is done
```

### Migration ordering

- Migrations must reach `exit 0` **before** `backend` (the new image) is started — this is already the exact dependency `docker-compose.yml` encodes today (`backend: depends_on: migrate: condition: service_completed_successfully`) and this design carries it into production unchanged, gated explicitly in the deploy workflow with the same `--exit-code-from migrate` technique already proven in Stage 27A3, rather than trusting `depends_on` alone to be caught in time.
- **Concrete, non-obvious gap this design surfaces:** `migrate`'s Compose service bind-mounts `./backend/migrations` from the **host filesystem** — it is not baked into any built image. That means the VPS needs a way to have the current migration `.sql` files present locally before every deploy. Two options, to be decided in the deploy-workflow sub-session (not decided here):
  1. **Keep a minimal git checkout on the VPS**, updated via `git pull` (or `rsync` of just `docker-compose*.yml` + `backend/migrations/`) as an early deploy step. Simplest, changes nothing about the existing `migrate` service design.
  2. **Bake migrations into the backend image** (`COPY migrations /app/migrations` in `backend/Dockerfile`) and change the `migrate` service to run against that image instead of the generic `golang:1.25-alpine` + bind-mount pattern. More self-contained (the VPS needs zero source checkout at all), but is a real change to an existing, already-verified (Stage 27A3) service definition.
  - **Recommendation: option 1.** It's strictly additive — the existing `migrate` service, proven working end-to-end in Stage 27A3, is not touched at all.
- Migrations are forward-only (`goose up`). No automated `goose down` is ever run as part of a deploy or a rollback — see Rollback approach.

### Rollback approach

- **Redeploy the previous image tag — never `goose down` the database automatically.** Rolling back application code is safe and fast (images are already built and sitting in GHCR under their old `sha-*` tag); rolling back a schema is a fundamentally riskier operation against live user data, and the roadmap's own E2E requirement calls for "a documented, tested rollback procedure (redeploy the previous image tag)" specifically — not a schema rollback.
- Mechanism: set the VPS `.env`'s `IMAGE_TAG` back to the previous known-good `sha-*` value, then repeat deployment-ordering steps 6–9 (pull → skip migrate, since the schema is already at or ahead of what the old code expects → restart → health-check) — no rebuild needed, since the target images already exist in GHCR.
- Recommended UX (for the deploy-workflow sub-session to implement, not built this session): a `workflow_dispatch` input (`rollback_to_sha`) on `deploy.yml` that runs exactly this shortened pull-restart-health-check path against an operator-specified prior SHA — keeps rollback auditable in the Actions log and secret-free from anyone's local machine, rather than requiring a manual SSH session to trigger.
- **Forward-compatibility implication for migration authors** (a practice to carry forward, not a mechanism to build): a migration should stay compatible with the *previous* release's binary for at least one deploy cycle, so that rolling back the code after a bad deploy never leaves the old binary talking to a schema it can't understand. Noted as guidance for future migration-writing sessions.

### Failure conditions

| Failure point | Behavior |
|---|---|
| A required secret is missing | Workflow fails immediately, before building or touching the VPS — matches the roadmap's explicit "fail closed if a required secret is missing" |
| Image build fails | Pipeline stops; nothing pushed, nothing deployed |
| Image push to GHCR fails | Pipeline stops before the SSH step |
| SSH connection to the VPS fails | Pipeline stops; VPS untouched |
| `docker compose pull` fails on the VPS | Pipeline stops **before** restarting anything — pull always happens before restart, never the other way around, so a registry/network hiccup never leaves the stack half-updated |
| Migration step exits non-zero | Pipeline stops before `backend`/`frontend` are touched at all — the currently-running (old) containers are left serving traffic untouched, exactly mirroring the exit-code-gated pattern already proven in `infra-ci.yml` |
| Post-restart health check never returns healthy | Pipeline fails loudly (not a silent success) and the rollback procedure above is the documented recovery path — whether rollback itself becomes automatic-on-failure or stays a manual `workflow_dispatch` trigger is a decision for the deploy-workflow sub-session |
| Partial restart (e.g. backend up, frontend fails) | `docker compose up -d` itself returns non-zero if any service fails to start, which the workflow must check explicitly rather than assuming success from `pull` alone |

### Required VPS prerequisites

- Docker Engine + the Docker Compose v2 plugin installed.
- A dedicated **non-root** deploy user, member of the `docker` group (not deploying as `root`) — its public key installed in that user's `~/.ssh/authorized_keys`; the private half lives only in `DEPLOY_SSH_PRIVATE_KEY`.
- A persistent working directory holding `docker-compose.yml`, `docker-compose.prod.yml` (once it exists), `backend/migrations/`, and a real `.env` (populated once, manually or via the deploy workflow — see "Required environment/secrets" above; never committed).
- `docker login ghcr.io` already done once for that deploy user (assuming GHCR packages stay private, per the registry-strategy recommendation above), or packages made public if that trade-off is later preferred.
- Firewall: only 22 (SSH — key-only auth, consider a non-default port), 80, and 443 open. Postgres/MinIO/Mailpit ports must **not** be exposed to the public internet — see "Production compose strategy" above.
- A domain pointed at the VPS's IP — required for the Nginx + Let's Encrypt/certbot sub-session (Stage 28's third planned split), not needed until then.
- Sufficient disk for Docker images plus the `postgres-data` and `minio-data` volumes; a backup strategy for those volumes is a real, currently-unsolved gap — noted here, not addressed by this session (CI/CD automation and data backup are separate concerns).

### Change made

`.env.production.example` (new, repo root) — a template mirroring `.env.example`'s structure, every value a placeholder, explicitly cross-referencing which GitHub secret (from the table above) each real value should come from. Judged "genuinely required for later Stage 28 tasks" (instruction 8) because the deploy-workflow sub-session will need exactly this reference to know the production `.env` shape, and it carries zero risk (no real values, purely documentation, same pattern the repo already uses for `.env.example`).

No `deploy.yml`, no `docker-compose.prod.yml`, no Dockerfile changes, no Nginx config — all explicitly deferred to their own later sub-sessions per the roadmap's own recommended split (see "Remaining for Stage 28" below). Nothing was deployed, no image was built or pushed, no VPS was contacted, no secret (real or placeholder-as-real) was added anywhere.

### Files changed

- `.env.production.example` — new.
- `STAGE28_PROGRESS.md` — new (this file).

### Not done this session (explicitly out of scope for 28A1)

- **No image builds or pushes** — design only, per instruction 4.
- **No VPS contact of any kind** — no SSH, per instruction 6.
- **No production infrastructure changes** — per instruction 7.
- **No real secrets added anywhere** — per instruction 5; every value in `.env.production.example` is an explicit placeholder.
- **No `deploy.yml`, no `docker-compose.prod.yml`, no Nginx config, no Dockerfile hardening** — all deferred to their own sub-sessions, per the roadmap's own recommended three-way split.
- **28A2 not started.**

### Known limitations / open decisions carried forward

- **Real payment provider integration does not exist yet** (`PAYMENT_PROVIDER=mock` is the only implemented option) — a genuine product/backend gap, independent of this CI/CD design, that blocks true production readiness regardless of how good the deploy pipeline is.
- **Real SMTP provider not yet chosen** — Mailpit is explicitly dev-only; production needs a real provider (SES, Postmark, etc.), decided in a future session.
- **Migration-file delivery to the VPS is an open decision** (git-checkout-on-VPS vs. baking migrations into the backend image) — recommendation given above (git checkout, least disruptive to the already-proven `migrate` service), not implemented.
- **VPS `.env` secret-provisioning mechanism is an open decision** (templated-per-deploy vs. set-once-manually) — recommendation given above (set-once-manually), not implemented.
- **No backup strategy for `postgres-data`/`minio-data` volumes** — a real production-readiness gap, outside this session's CI/CD-design scope.
- **`backend`/`frontend` still have no Compose-level `healthcheck:`** — recommended for the Dockerfile-hardening sub-session, not added now.
- **This entire design is unvalidated against a real VPS**, by explicit instruction — it is a plan, not yet proven in an actual environment.

## Stage 28A2 — production image build and GHCR publishing (this session)

Scope: prepare (not run) build/publish automation for all 5 production images, harden their Dockerfiles only where genuinely necessary, validate locally. No VPS/SSH, no remote migrations, no production infrastructure changes, no real secrets.

### Inspection performed

Read this file's 28A1 section and inspected `git status` (only 28A1's two files — `.env.production.example`, `STAGE28_PROGRESS.md` — pending; nothing else). Re-read `backend/Dockerfile` and `frontend/Dockerfile` (already read in 28A1) and, newly this session, `backend/Dockerfile.worker`, `backend/Dockerfile.notification-worker`, `backend/Dockerfile.runner`. Found a critical, already-documented constraint in `Dockerfile.runner`'s own comments: `code-runner` **must** run as container-root, not because it needs host privileges, but because the sandbox's `unshare --user --map-root-user --net --pid --fork` call needs `CAP_SYS_ADMIN`, and granting that capability to a non-root OS user "did not work in testing" per Stage 16's own trust-boundary writeup. This directly shaped this session's hardening scope — see below. Grepped `internal/videos/worker.go` for filesystem assumptions (`os.MkdirTemp("", ...)` — uses the default `/tmp`, world-writable in Alpine by default, so a non-root user needs no extra directory setup there). Checked `frontend/next.config.ts` (no `output: "standalone"`) and `frontend/package.json`'s scripts, and confirmed the existing `frontend/Dockerfile`'s `deps` stage ran plain `npm install` (not `npm ci`) and the final `run` stage copied that same `node_modules` — which includes devDependencies (`typescript`, `eslint`, `@types/*`) needed only by the build, not the runtime — straight into the production image. Re-read all four existing `.github/workflows/*.yml` files for the established GHCR/Actions conventions this session's new workflow follows (least-privilege `permissions:`, `actionlint`-validated, matching a prior stage's style).

### Design decisions

**Dockerfile hardening — deliberately scoped to what the inspection actually found unnecessary, not a blanket pass:**

- **`backend`, `video-worker`, `notification-worker`: added a dedicated non-root `appuser` (uid 10001).** None of the three needs root for anything — `video-worker`'s only filesystem write is `os.MkdirTemp("", ...)`, which resolves to the already-world-writable `/tmp`, so no extra `RUN mkdir`/`chown` was needed beyond the binary's own ownership (`COPY --chown=appuser:appuser`).
- **`code-runner`: left completely untouched.** Its own Dockerfile comment already documents, from Stage 16's own investigation, exactly why it must stay root — adding a non-root user here would not just be unnecessary, it would break the sandboxing mechanism this service exists to provide. Instruction 11 ("do not redesign application behavior") and instruction 10's "only where genuinely necessary" both point the same direction: leave it alone. Verified the file is still byte-for-byte unchanged (`git diff --stat` confirms no line changed).
- **`frontend`: switched `npm install` → `npm ci` (determinism, matching 27A2's own CI convention) and split dependency installation into two stages** — `deps` (full install, including devDependencies, for `next build`'s own type-checking) and a new `prod-deps` (`npm ci --omit=dev`). The final `run` stage now copies `node_modules` from `prod-deps`, not `deps` — this is exactly instruction 10's "no dev-only dependencies" concern, and it wasn't hypothetical: the old image shipped `typescript`/`eslint`/`@types/*` into the running container for no reason. Measured, not assumed — see Verification below.
- **`frontend`: switched to the base image's built-in non-root `node` user** (uid 1000, already present in every official `node:*-alpine` image) rather than creating a new one — simpler than Next.js's own commonly-cited example (which creates a fresh `nextjs:nodejs` user), and just as effective here since there's no multi-tenant-host UID-collision concern for this project. `.next` is explicitly `--chown`'d to `node:node` since Next.js can write into `.next/cache` at runtime (ISR/image-optimization caching); `node_modules`/`package.json`/`public` stay root-owned but world-readable, which is all a read-only runtime process needs.
- **Considered and explicitly not done: Next.js `output: "standalone"`.** Would shrink the frontend image further (traces only the exact runtime dependency closure instead of a `--omit=dev` `node_modules`), but requires changing `next.config.ts` and the Dockerfile's final-stage server entrypoint (`node server.js` instead of `npm run start`) — closer to "redesigning" than "hardening" per instruction 11. Noted as a future option in Known limitations, not implemented.
- **`EXPOSE`/healthcheck additions: not made.** `backend`/`video-worker`/`notification-worker`/`code-runner` already `EXPOSE` (or don't need to — `EXPOSE` is documentation only) what they need; adding Dockerfile-level `HEALTHCHECK` directives wasn't in this session's explicit instruction list and 28A1 already flagged Compose-level healthchecks for `backend`/`frontend` as a 28A2 candidate — deferred again here since it's not "genuinely necessary for production image build" specifically (it's an orchestration concern, not a build one), and instruction 12's actual health verification was done via a live `/health` HTTP check instead (see Verification).

**Publish workflow design:**

- **Trigger: `workflow_run` on `CI Quality Gate` completion, filtered to `branches: [main]`, with an explicit `if: conclusion == 'success'` guard on the first job** — not `push`/`pull_request` directly. This is the one clean way to express "only after the gate has already verified this exact commit, and only for `main`" without re-running (and therefore duplicating) the gate's own checks, extending the same non-duplication principle Stage 27B established for the gate itself. `workflow_run` here is used for its textbook purpose (react to another workflow's completion) rather than 27B's rejected use of it (aggregating parallel results across files) — a materially simpler application of the same primitive.
- **`ref: ${{ github.event.workflow_run.head_sha }}` on checkout** — builds the exact commit the gate verified, not whatever `HEAD` happens to be when the publish job starts (these could differ if another push landed in between).
- **`permissions: contents: read, packages: write`** at the workflow level, exactly instruction 4's list — no other scope requested or needed.
- **Matrix build across all 5 images**, not 5 separate copy-pasted jobs — same context/dockerfile/tag pattern repeated with different inputs, avoiding the kind of duplication Stage 27B's own design explicitly cautioned against. `fail-fast: false` so one image's build failure (e.g. a transient `apk` mirror issue for `code-runner`'s heavier toolchain install) doesn't cancel the other four mid-flight.
- **Tags: `sha-<short-sha>` and `latest`, both pushed, but `latest` is never treated as authoritative** — directly implementing 28A1's own design decision and instruction 3's explicit requirement. `latest` exists purely for convenience (manual pulls, debugging); nothing in this design, and nothing yet built in a later Stage 28 session, will ever have a rollback or deploy step reference it.
- **`verify-inputs` job, run before any build starts:** the frontend build needs `NEXT_PUBLIC_API_URL` baked in as a build-arg (Next.js inlines `NEXT_PUBLIC_*` values into the client bundle at *build* time, not runtime) — sourced from a GitHub Actions **variable** (`vars.PROD_NEXT_PUBLIC_API_URL`, not a secret, matching 28A1's own "non-sensitive values belong in `vars`" call), which doesn't exist yet since nothing has configured it. Rather than silently baking in an empty string if it's unset (which would ship a broken frontend image with no build failure to show for it), this job fails the whole workflow closed immediately — the same "fail closed if a required value is missing" philosophy 28A1's design already committed to for secrets, extended here to a required non-secret build input.
- **`docker/setup-buildx-action` + `docker/login-action` + `docker/build-push-action`** (official, Docker-maintained actions), rather than hand-rolled `docker build`/`docker push` CLI steps — standard, well-tested, and gives clean multi-tag/build-arg support without extra scripting.

### Change made

- `backend/Dockerfile`, `backend/Dockerfile.worker`, `backend/Dockerfile.notification-worker` — added a non-root `appuser` (uid 10001), `COPY --chown`, `USER appuser`. No behavior change beyond privilege level.
- `frontend/Dockerfile` — `npm install` → `npm ci`; new `prod-deps` stage (`npm ci --omit=dev`); final stage now copies `node_modules` from `prod-deps` instead of `deps`; runs as the base image's built-in `node` user; `.next` ownership set to `node:node`.
- `backend/Dockerfile.runner` — **unchanged**, deliberately, per the design decision above.
- `.github/workflows/image-publish.yml` (new) — `verify-inputs` + matrix `publish` jobs, as designed above.

### Files changed

- `backend/Dockerfile`
- `backend/Dockerfile.worker`
- `backend/Dockerfile.notification-worker`
- `frontend/Dockerfile`
- `.github/workflows/image-publish.yml` — new.
- `STAGE28_PROGRESS.md` — this section added.

No application code (Go or TypeScript source) was touched — every change is Dockerfile/CI configuration, matching instruction 11.

### Image names, tag scheme, registry paths

| Service | Registry path | Dockerfile |
|---|---|---|
| Backend API | `ghcr.io/almukhanbetov/course-backend` | `backend/Dockerfile` |
| Frontend | `ghcr.io/almukhanbetov/course-frontend` | `frontend/Dockerfile` |
| Video worker | `ghcr.io/almukhanbetov/course-video-worker` | `backend/Dockerfile.worker` |
| Notification worker | `ghcr.io/almukhanbetov/course-notification-worker` | `backend/Dockerfile.notification-worker` |
| Code runner | `ghcr.io/almukhanbetov/course-code-runner` | `backend/Dockerfile.runner` |

Every image gets two tags per publish: `sha-<7-char-commit-sha>` (immutable, the only one anything should ever reference for a real deploy or rollback) and `latest` (floating convenience tag, never authoritative — instruction 3).

### Verification performed

**Workflow syntax, validated locally:**
- `python3 -c "yaml.safe_load(...)"` on `image-publish.yml` — parses cleanly; confirmed `on.workflow_run.workflows == ["CI Quality Gate"]`, `branches == ["main"]`, both jobs present.
- `actionlint` across all five workflow files together (re-installed fresh this session into `/tmp/gobin`, since the prior binary from 27C's ephemeral `/tmp` was gone) — **zero findings**.
- `act -l -W .github/workflows/image-publish.yml` — resolved the real dependency graph: **Stage 0** — `verify-inputs`; **Stage 1** — `publish` (needs `verify-inputs`), both on the `workflow_run` event — confirms the job graph is structurally correct, not just YAML-valid.

**All 5 images actually built locally** (not just linted), using the hardened Dockerfiles exactly as `image-publish.yml` will:

| Image | Build result |
|---|---|
| `backend` | **OK** |
| `video-worker` | **OK** (ffmpeg/ffprobe install unaffected) |
| `notification-worker` | **OK** |
| `code-runner` | **OK** (unchanged Dockerfile, rebuilt to confirm no drift) |
| `frontend` | **OK** |

**Non-root hardening, verified by actually running each image**, not just reading the Dockerfile:

| Image | Check | Result |
|---|---|---|
| `backend` | `id` inside the container | `uid=10001(appuser)` — binary owned by `appuser`, executable |
| `video-worker` | `id`; `ffmpeg -version` | `uid=10001(appuser)`; ffmpeg still runs correctly as non-root |
| `notification-worker` | `id` | `uid=10001(appuser)` |
| `code-runner` | `id` | `uid=0(root)` — **confirmed still root, as intentionally designed**, not an oversight |
| `frontend` | `id`; `.next` ownership | `uid=1000(node)`; `.next` owned by `node:node`, other files root-owned but world-readable |

**Real startup behavior, not just static inspection** (instruction 12's "containers start where practical"):
- **Frontend**: started standalone (no dependencies needed for `next start` to listen) — `GET /` → **200**, clean logs (`✓ Ready in 208ms`), no permission errors.
- **Backend**: the one case genuinely worth a full live test, since it has an actual `/health` endpoint. Spun up an isolated, throwaway Postgres (separate Docker network/container, no interference with this repo's own live dev stack — confirmed unaffected before and after), ran all 40 goose migrations against it (same command shape as Stage 27A3), then started the hardened `backend` image pointed at that database: **`GET /api/v1/health` → 200, `{"status":"ok","database":"ok"}`** — the non-root user change does not break real startup or DB connectivity.
- **video-worker / notification-worker**: started against the same working test database (no real S3/SMTP endpoint given, deliberately, since that's outside this session's scope) — both started cleanly and stayed running; `video-worker` logged an S3-unreachable warning (expected, given a fake endpoint) but did **not** fail with any permission-denied error, confirming the `appuser` change didn't break their startup path.
- All test containers, the throwaway Postgres, the isolated network, and the locally-built test images were removed afterward; the repo's own live dev stack (`course-*` containers) was confirmed running, unaffected, and still healthy throughout and after.

**Frontend image size, measured before/after** (not just asserted): the previously-running dev stack's `course-frontend:latest` (built from the old Dockerfile) is **1.1GB**; the newly-built `test-frontend:local` (prod-only `node_modules`) is **960MB** — a real, measured ~140MB reduction from excluding devDependencies, not a theoretical one.

**Not done this session:** an actual push through GitHub Actions itself (no commit has been pushed; `verify-inputs` would currently fail closed anyway, since `PROD_NEXT_PUBLIC_API_URL` has not been configured as a repository/environment variable — see Known limitations).

### Not done this session (explicitly out of scope for 28A2)

- **No VPS/SSH** — per instruction 6.
- **No remote migrations** — per instruction 7; the only migrations run this session were against a throwaway, local, isolated test database, purely to prove the backend image's own startup path works.
- **No production infrastructure changes** — per instruction 8.
- **No real secrets added** — per instruction 9; `GITHUB_TOKEN` (used for GHCR auth) is the automatically-provided ephemeral token instruction 4 itself calls for, not a secret this session introduced.
- **No application behavior redesign** — per instruction 11; every change is Dockerfile/CI configuration only.
- **28A3 not started** — no `deploy.yml`, no `docker-compose.prod.yml`, no actual image push to GHCR has happened.

### Post-session update — live GitHub Actions run confirmed (2026-08-13)

After this session ended, the design's own recommendation was followed exactly: `PROD_NEXT_PUBLIC_API_URL` was added under **Variables** (an earlier attempt added it under **Secrets** first — corrected once the `vars.*`/`secrets.*` distinction was pointed out, matching this doc's own "non-sensitive values belong in `vars`" design decision above). `image-publish.yml` was then re-run for real on GitHub's infrastructure (run `#1`, commit `42aa7cf`, triggered via `workflow_run` off `CI Quality Gate`):

- `verify-inputs` — **passed** (2s).
- `publish` matrix — **all 5 jobs completed successfully** (`backend`, `frontend`, `video-worker`, `notification-worker`, `code-runner`), 5 artifacts produced, total run duration 1m37s.

This closes the "not yet proven against a real GitHub Actions run" limitation below — the workflow now has one real, successful, observed run, not just local validation.

### Known limitations

- ~~`PROD_NEXT_PUBLIC_API_URL` is not yet configured~~ — **resolved**, see the post-session update above.
- ~~Not yet proven against a real GitHub Actions run or a real GHCR push~~ — **resolved**, see the post-session update above. (Local validation — YAML parsing, `actionlint`, `act -l`, full local builds/runs of all 5 images including a live DB-backed backend health check — is still what this session itself performed; the live run came after, confirming it.)
- **`code-runner` remains the one image that runs as root** — by design, not an oversight; a future session could revisit whether an alternative sandboxing mechanism (e.g., gVisor, a different namespace-creation approach) could avoid needing `CAP_SYS_ADMIN` as root, but that's a Stage-16-level sandboxing redesign, well outside this session's Dockerfile-hardening scope.
- **Next.js `output: "standalone"` not adopted** — would shrink the frontend image further than the `prod-deps` split achieved, but touches `next.config.ts` and the Dockerfile's runtime entrypoint; left as a future option, not implemented, per instruction 11's "do not redesign application behavior."
- **No Dockerfile-level `HEALTHCHECK` or Compose-level `healthcheck:` added for `backend`/`frontend`** — still an open item from 28A1, deliberately not addressed here either (it's an orchestration concern for the deploy-workflow sub-session, not an image-build one).
- **GHCR package visibility (private vs. public) not yet reviewed** — 5 packages now exist under the account's Packages tab from the run above; 28A1's recommendation (keep private, VPS does one `docker login`) still stands as the plan for 28A3, but the actual visibility setting on the now-real packages hasn't been explicitly checked.
- **The registry-owner path (`ghcr.io/almukhanbetov/...`) assumes `github.repository_owner`** resolves to the expected account — confirmed correct by the live run above.

## Stage 28A3 — production VPS deployment workflow (this session)

Scope: prepare (design + build, not run) the actual VPS deploy workflow using the already-published GHCR images from 28A2. No Nginx/HTTPS, no full rollback stage, no real VPS contact — this session's deploy.yml has never been executed against a real server.

### Inspection performed

Read this file in full (28A1/28A2 sections plus the post-session update recording `image-publish.yml`'s first successful live run). Inspected `git status` (clean besides the earlier uncommitted `STAGE28_PROGRESS.md` post-session-update edit). Re-read `.github/workflows/image-publish.yml` fresh — confirmed the exact tag format it produces (`sha-<7char>` + `latest`), its `permissions: contents: read, packages: write`, and that it triggers via `workflow_run` off `CI Quality Gate` — this session's `deploy.yml` chains off *that* workflow's completion instead (not off `quality-gate` directly), since deploy needs the images to actually exist in GHCR first, not just the gate to have passed. Re-read `docker-compose.yml` in full again to get the exact current service graph, `depends_on` conditions, and healthcheck definitions right before writing an override against it. Did not inspect any application domain code, per instruction.

### Design decisions

**Production deploy directory: `/opt/lms`** — hardcoded as a job-level `env:` in `deploy.yml` rather than a configurable GitHub variable. It's a fixed, non-sensitive, rarely-changing value; adding a variable for it would be one more prerequisite to configure before anything works, for no real flexibility benefit. Holds only `docker-compose.yml`, `docker-compose.prod.yml`, `backend/migrations/`, and a real `.env` (populated once, manually, per 28A1's already-decided provisioning approach) — never application source.

**Production docker compose file: `docker-compose.prod.yml`** (new, repo root) — an override, always invoked together with the base file (`docker compose -f docker-compose.yml -f docker-compose.prod.yml <cmd>`), exactly as 28A1 designed. Concretely:
- Every buildable service (`backend`, `frontend`, `video-worker`, `notification-worker`, `code-runner`) gets `build: !reset null` + `image: ${IMAGE_REGISTRY}/course-<service>:${IMAGE_TAG}`. **`!reset` is load-bearing here, not decorative** — leaving both `build:` and `image:` present after a plain merge is ambiguous about whether `docker compose up` should ever attempt a local build; `!reset null` removes the base file's `build:` key entirely, verified below.
- `postgres`/`minio` get `ports: !reset []` — dropped from the host entirely, not just rebound to localhost, since nothing on the VPS host itself needs to reach them directly either; only other containers do, via service name, unchanged from dev.
- **`mailpit` is deliberately never named anywhere in this override or in `deploy.yml`'s `up -d` service list.** It stays defined in the base file (dev is unaffected) but is simply never one of the services `docker compose up -d` is told to start in production — the simplest, most explicit way to exclude a service without touching the base file or relying on Compose profiles.
- **`notification-worker`'s `depends_on` is overridden with `!override`, not left to merge.** Compose deep-merges maps by default — a plain (unmarked) `depends_on: {migrate: ...}` override does **not** replace the base file's `depends_on: {mailpit: ..., migrate: ...}`, it merges into it, leaving `mailpit: condition: service_healthy` still present and still blocking startup on a service that's never started. Caught this by actually resolving the merged config (see Verification) rather than assuming the override worked — `!override` was needed specifically to replace the whole map instead of merging it.
- `NEXT_PUBLIC_API_URL` needs no override here — it's already baked into the frontend image at build time by `image-publish.yml`'s own build-arg (Next.js inlines `NEXT_PUBLIC_*` into the client bundle at build time). The base file's runtime `environment: NEXT_PUBLIC_API_URL` passthrough is inherited unchanged and still matters for any server-rendered/server-component code that reads `process.env` live, distinct from what's already baked into the client bundle — both should carry the same real production URL, so this isn't a conflict, just worth naming.

**Image names and exact SHA tag usage** — unchanged from 28A2's design, reused directly: `ghcr.io/almukhanbetov/course-{backend,frontend,video-worker,notification-worker,code-runner}:sha-<7char>`. `deploy.yml` resolves which tag to deploy in one step (`Resolve image tag to deploy`): either an explicit `workflow_dispatch` input (`image_tag`), or — for the automatic path — the first 7 characters of the commit SHA that `image-publish.yml` just built, taken from `github.event.workflow_run.head_sha`. **`latest` is never referenced anywhere in `deploy.yml`** — satisfies instruction 5 directly; every pull is pinned to an explicit, immutable `sha-*` tag written into the VPS's own `.env`.

**Trigger design:** `workflow_run` off `Publish Production Images` completing on `main` (not off `CI Quality Gate` directly — chaining `quality-gate → image-publish → deploy` in sequence, each stage only starting once the previous one's actual artifact exists, not just its check passing) **plus** `workflow_dispatch` with an optional `image_tag` input. The manual path is deliberately also the rollback path: re-dispatching with an older, already-published `sha-*` tag runs the exact same pull → migrate → restart → health-check sequence against that older image — nothing rollback-specific needs to exist separately, per instruction 10's "rollback-safe structure, but do not implement the full rollback stage yet." (Migrations are forward-only and idempotent via goose's own tracking table, so re-running the migrate step during a "rollback" dispatch is always safe — it just finds nothing new to apply.)

**No `environment: production` job key.** 28A1's original design recommended a dedicated GitHub Environment (for required-reviewer approval gates). This session didn't add it: every secret/variable observed being configured so far (`PROD_NEXT_PUBLIC_API_URL`, both the corrected Variable and the earlier mistaken Secret) has been added at the plain **repository** level, not environment-scoped — matching that actual, observed practice rather than silently introducing an approval-gate requirement the user hasn't set up. Noted below as a future hardening option, not applied now.

**GHCR authentication on the VPS (instruction 8):** re-authenticates fresh on every single deploy using that run's own ephemeral `GITHUB_TOKEN` (declared `packages: read` at the workflow level), piped over SSH via stdin straight into `docker login --password-stdin` — never as a literal command-line argument on either end (avoids it ever appearing in a `ps aux` listing). This is a refinement of 28A1's original assumption of "the VPS does one persistent `docker login`" — no separate long-lived PAT needs to be created or stored on the VPS at all; whatever credential lands in the VPS's Docker config expires with that run's token shortly after, rather than sitting there indefinitely.

**Host-key trust: TOFU (trust-on-first-use), not pinned.** Instruction 7's required-secrets list is exactly `PROD_VPS_HOST`/`PROD_VPS_USER`/`PROD_VPS_SSH_PORT`/`PROD_VPS_SSH_PRIVATE_KEY` — no pinned-known-hosts secret. `deploy.yml` runs `ssh-keyscan` against the VPS at the start of every deploy and accepts whatever key is presented. This is weaker than 28A1's original design (which included a `DEPLOY_SSH_KNOWN_HOSTS` secret) but matches the exact secret list this session was given — flagged explicitly below as a real, not cosmetic, security trade-off, with the concrete fix (`PROD_VPS_KNOWN_HOSTS`, pinned once and never scanned again) recorded for later.

**Failure conditions (instruction 9), mapped to concrete steps, not left implicit:**

| Required failure condition | `deploy.yml` step that enforces it |
|---|---|
| SSH failure | `Verify SSH connectivity` — dedicated step, fails fast with a clear name before any file transfer or remote command is attempted |
| GHCR login/pull failure | `Log in to GHCR on the VPS` and `Set IMAGE_TAG and pull images` are separate steps — a failed login never gets to attempt a pull, and a failed pull never gets to touch running containers |
| Migration failure | `Run migrations` — `--exit-code-from migrate migrate`, the same exit-code-gated pattern already proven in Stage 27A3; a non-zero exit stops the job before `Restart services` ever runs |
| Backend health failure | `Backend health check` — bounded `curl -sf` retry loop (30 attempts × 2s) against `http://localhost:$BACKEND_PORT/api/v1/health` **run on the VPS itself** (not from the GitHub runner over the public internet — no reverse proxy exists yet, pre-28A4, so there's no assumption about public reachability baked into this check) |
| Frontend startup failure | `Frontend availability check` — identical retry-loop shape against `http://localhost:$FRONTEND_PORT/`, also VPS-local |

Every one of these is its own named workflow step specifically so a failure surfaces as a distinct, identifiable red step in the Actions UI, not a single opaque "deploy" blob — directly serving instruction 9's requirement that each of these is a real, distinguishable failure mode, not just a mentioned concept.

### Change made

- `docker-compose.prod.yml` (new) — production override, as designed above.
- `.github/workflows/deploy.yml` (new) — the deploy workflow, as designed above.

No changes to `docker-compose.yml`, no Dockerfile changes, no Nginx config, no application code. `.env.production.example` already anticipated everything this session needed (`IMAGE_TAG`, `IMAGE_REGISTRY`, `BACKEND_PORT`, `FRONTEND_PORT` were all already present from 28A1) — nothing to add there.

### Files changed

- `docker-compose.prod.yml` — new.
- `.github/workflows/deploy.yml` — new.
- `STAGE28_PROGRESS.md` — this section added.

### Required GitHub Secrets (this session's exact scope, instruction 7)

| Secret | Purpose |
|---|---|
| `PROD_VPS_HOST` | VPS address |
| `PROD_VPS_USER` | Deploy user on the VPS |
| `PROD_VPS_SSH_PORT` | SSH port |
| `PROD_VPS_SSH_PRIVATE_KEY` | Private key for that user |

No new secret for GHCR access (reuses the job's own `GITHUB_TOKEN`, see Design decisions). The broader secret list from 28A1 (`PROD_POSTGRES_*`, `PROD_JWT_SECRET`, `PROD_MINIO_*`, `PROD_SMTP_*`, `PROD_PAYMENT_PROVIDER`) is **not** read by `deploy.yml` at all — per 28A1's own "set once, manually, directly on the VPS" decision, those live only in the VPS's own `.env`, populated outside of any GitHub Actions run; `deploy.yml` only ever updates that file's `IMAGE_TAG` line.

### Migration command/order

`docker compose -f docker-compose.yml -f docker-compose.prod.yml up --exit-code-from migrate migrate`, run **after** images are pulled and **before** `Restart services`. Reuses `migrate`'s existing base-file definition completely unchanged (bind-mounts `./backend/migrations:/migrations:ro`, `depends_on: postgres: condition: service_healthy`) — the deploy workflow's `Sync compose files and migrations to the VPS` step is what makes that relative bind-mount path resolve correctly on the VPS (see Verification: the exact directory structure was tested, not assumed).

### Service restart order

`docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d postgres minio minio-init backend frontend video-worker notification-worker code-runner` — one command, explicit service list (mailpit excluded). Actual startup ordering among these is unchanged from `docker-compose.yml`'s own existing `depends_on` graph: `migrate` (already run and gated in the previous step) → `backend`/`video-worker`/`code-runner` (all depend only on `migrate` + `minio-init`) → `frontend` (depends on `backend`). Nothing about this ordering was redesigned; the deploy workflow just triggers the same graph the dev stack has always used.

### Backend health check / frontend availability check

Both run **on the VPS itself** via SSH, not from the GitHub Actions runner over the public internet — deliberate, since no reverse proxy exists yet (28A4) and this session makes no assumption about what's publicly reachable. `curl -sf http://localhost:$BACKEND_PORT/api/v1/health` and `curl -sf http://localhost:$FRONTEND_PORT/`, each in a 30-attempt/2-second-interval retry loop, reading `$BACKEND_PORT`/`$FRONTEND_PORT` from the VPS's own `.env` (sourced fresh in every remote command) rather than hardcoding a port number.

### Rollback-safe structure (not the full rollback stage — instruction 10)

`deploy.yml`'s `workflow_dispatch` input (`image_tag`) is the rollback mechanism: dispatching manually with a prior `sha-*` tag runs the identical pull → migrate → restart → health-check sequence against that older, already-published image — no separate rollback job, workflow, or script exists or is needed for this to work, because nothing in the deploy sequence is forward-only except the migrations themselves, which are safe to re-run (goose only applies what's new; re-running against an unchanged or already-forward schema is a no-op). What's genuinely **not** built yet: a dedicated, friendlier rollback UX (e.g., a workflow input that says "roll back to the previous deploy" without the operator needing to know/paste the exact prior SHA themselves), and this has never been exercised against a real VPS.

### Validation performed

**YAML/actionlint (instruction 11):**
- `python3 -c "yaml.safe_load(...)"` on `deploy.yml` — parses cleanly; confirmed both `workflow_run` (off `Publish Production Images`) and `workflow_dispatch` triggers present, `deploy` job present.
- `actionlint` across all six workflow files together (re-installed fresh into `/tmp/gobin`, same as every prior session — the binary doesn't survive between sessions since it lives in ephemeral `/tmp`) — **zero findings**.
- `act -l -W .github/workflows/deploy.yml` — resolved correctly: one job (`deploy`), triggered by either `workflow_run` or `workflow_dispatch`, exactly as designed.

**`docker compose config` (instruction 11) — actually resolved, not assumed:**
- First pass caught a real bug: `notification-worker`'s `depends_on` override, written as a plain (unmarked) map, did **not** drop the base file's `mailpit: condition: service_healthy` entry as intended — Compose deep-merges maps by default. Confirmed by inspecting the actual resolved YAML (`docker compose -f docker-compose.yml -f docker-compose.prod.yml config`, parsed with `yaml.safe_load` and checked field-by-field), not by reading the source file and assuming it worked.
- Fixed with `depends_on: !override { migrate: ... }`, which replaces the whole map instead of merging into it. Re-resolved and reconfirmed: `notification-worker`'s merged `depends_on` now contains only `migrate`, `mailpit` is gone.
- Also confirmed, on the same resolved output: `build:` is absent (not just overridden-alongside-`image:`) for all five buildable services, each carrying the correct `ghcr.io/almukhanbetov/course-<service>:sha-<tag>` image reference; `postgres`/`minio` have no `ports:` entry at all in the merged config.
- Test env values used throughout were placeholders in the same style as every prior session's isolated testing (`prodtest`/`prod-test-placeholder-not-real`/etc.) — never real credentials.

**Deploy-directory path structure — tested, not assumed:** simulated the exact `rsync` invocations `deploy.yml` uses (`rsync backend/migrations` into a `backend/` subdirectory at the target, rather than flattening it) against a local throwaway directory standing in for the VPS. Confirmed the result is `$DEPLOY_DIR/backend/migrations/*.sql` (all 40 files present) — the exact relative path `docker-compose.yml`'s existing `./backend/migrations:/migrations:ro` bind mount expects, requiring zero changes to that file.

**Embedded bash logic — simulated directly**, since there is no real VPS to exercise these steps against end-to-end this session:
- Tag-resolution logic: no manual input → correctly derives `sha-<7char>` from the triggering commit; manual `workflow_dispatch` input provided → correctly uses that value instead (the rollback path).
- Missing-secrets check: all four present → passes; any one missing → fails with the specific missing name(s) listed.
- Health-check retry loop: mocked a `curl` that fails twice then succeeds → loop correctly detects success on the 3rd attempt; mocked a `curl` that always fails → loop correctly exhausts its attempts and exits 1 (not a silent success).

**Not done this session, and explicitly cannot be done without a real VPS:** an actual SSH connection, an actual GHCR pull on a remote host, an actual migration run against a production-shaped database, an actual backend/frontend container start under this workflow, or a real end-to-end deploy. Every check above validates the workflow's *logic* and the compose file's *resolved configuration* — none of it is the same as watching `deploy.yml` actually deploy something.

### Known limitations

- **Host-key trust is TOFU, not pinned** — `deploy.yml` accepts whatever host key the VPS presents on each run (`ssh-keyscan`), rather than verifying against a pre-recorded fingerprint. This is a real weakening relative to 28A1's original design (which included a pinned `DEPLOY_SSH_KNOWN_HOSTS` secret) — a consequence of this session's exact required-secrets list (instruction 7) not including one. Fix, if wanted later: add a `PROD_VPS_KNOWN_HOSTS` secret (the output of `ssh-keyscan` run once, by hand, and saved) and replace the `ssh-keyscan` step with writing that value directly to `~/.ssh/known_hosts`.
- **No dedicated GitHub Environment / required-reviewer approval gate** — `deploy.yml` runs unattended the moment its triggers fire; nothing pauses for a human "approve this deploy" click. 28A1 recommended this; not added, per the "matches observed repo-level secret practice" reasoning above. A future hardening step, not a current bug.
- **Never run against a real VPS** — by explicit instruction, and because none has been provisioned yet. Every validation this session performed is local: YAML/actionlint, `docker compose config` resolution (with a real, caught, fixed bug), local rsync path simulation, and direct bash-logic simulation of each script block. This is meaningfully more verification than "read the file and assume it's right," but it is still not the same as a real deploy.
- **VPS prerequisites (Docker installed, deploy user in the `docker` group, `/opt/lms` created, an initial `.env` populated by hand from `.env.production.example`, firewall rules) are still exactly what 28A1 documented** — none of them have been set up, since no VPS exists yet. `deploy.yml` will fail immediately and clearly at `Verify SSH connectivity` if pointed at a host that isn't ready, rather than partially succeeding.
- **The SMTP-provider gap from 28A1 is unchanged** — `docker-compose.prod.yml` correctly stops trying to depend on Mailpit in production, but no real provider has been chosen, so `notification-worker` will start successfully yet be unable to actually send email until that decision is made and `SMTP_*` in the VPS's `.env` points somewhere real.
- **The payment-provider gap from 28A1 is unchanged** — still `mock`-only in code.
- **Rollback is structurally supported (manual re-dispatch with an older tag) but has no dedicated UX and has never been exercised for real.**

## Stage 28A3 safety fix — fail-closed production config validation (this session)

Scope: one pre-deploy safety fix only, prompted by two read-only pre-deploy checks in prior sessions — the general safety review (which surfaced that `deploy.yml` never validated the VPS's own `.env` before touching anything) and the SMTP-specific check (which confirmed empty `SMTP_HOST` is a safe, intentional state, not a misconfiguration to reject). No deploy, no push, no new files besides this section.

### Inspection performed

Read `.github/workflows/deploy.yml`, `.env.production.example`, `docker-compose.yml`, `docker-compose.prod.yml`, and this file fresh, per instruction. Confirmed the exact placeholder string this fix needs to reject verbatim: `.env.production.example:26` — `JWT_SECRET=__SET_VIA_GITHUB_SECRET_PROD_JWT_SECRET__`. Confirmed `cmd/notification-worker/main.go:31-33` and `internal/notifications/email.go`'s `LogSender` (already read in the prior SMTP-specific check this session continues from) — empty `SMTP_HOST` deterministically falls back to a no-op sender that never fails, so it must **not** be part of this validation, per instruction 3.

### Design decisions

- **Runs on the VPS itself, over the existing SSH connection — not in the GitHub Actions runner.** None of the 8 values this check validates ever exist anywhere this workflow's runner can see them (per 28A1's "set once, manually, on the VPS" decision, reaffirmed by the pre-deploy safety report: `deploy.yml` never uploads or creates `.env`). The only place capable of checking them is the VPS's own shell, after sourcing its own `.env` — so this had to be a remote step, following the exact same `ssh ... "cd '$DEPLOY_DIR' && set -a && . ./.env && set +a && ..."` pattern already established by the health-check steps, for consistency.
- **Placed immediately after `Sync compose files and migrations to the VPS`, before `Log in to GHCR on the VPS`.** As early as it can possibly run (needs `$DEPLOY_DIR` to exist, which the sync step just ensured) and strictly before anything that pulls an image, authenticates to a registry, runs a migration, or restarts a container — a bad config now fails before any of those, not after.
- **Checks presence for `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB`/`DATABASE_URL`/`MINIO_ROOT_USER`/`MINIO_ROOT_PASSWORD`, and two distinct conditions for `JWT_SECRET`** (empty, or exactly equal to `.env.production.example`'s literal placeholder string) — each failure reason collected into one array and reported together in a single message, rather than failing on the first problem found, so a first-time VPS setup with multiple unfilled placeholders gets one complete list instead of a fix-one-fail-again loop.
- **`SMTP_HOST`/`SMTP_USER`/`SMTP_PASSWORD` deliberately excluded**, per instruction 3 and the prior SMTP-specific check's own finding: empty `SMTP_HOST` is a real, working, intentionally-designed production state (`LogSender`), not something to reject.
- **An explicit `[ ! -f .env ]` check with a clear message, before attempting to source it** — `sed`/`source` against a missing file already fail (verified directly in the earlier safety-report session: `sed -i` on a nonexistent file exits 2), but a plain "No such file or directory" is a worse first-deploy debugging experience than a message that names the missing file and points at `.env.production.example`.
- **The placeholder string is duplicated between `.env.production.example` and `deploy.yml`** — a real, acknowledged coupling. If that placeholder text is ever edited in one file, it must be updated in the other or this specific check silently stops catching it (every other check in this step is unaffected). Recorded here rather than solved with e.g. a shared constants file, since that would be new infrastructure for one string.

### Change made

`.github/workflows/deploy.yml` — one new step, `Validate critical production config on the VPS`, inserted between `Sync compose files and migrations to the VPS` and `Log in to GHCR on the VPS`. No other step changed.

### Files changed

- `.github/workflows/deploy.yml` — one new step added.
- `STAGE28_PROGRESS.md` — this section added.

No real production values were hardcoded anywhere (instruction 4) — the new step only ever reads values that already live in the VPS's own `.env`, never a literal secret in this repo. No `.env` is created or uploaded (instruction 5, unchanged from before this fix). `DEPLOY_DIR` remains `/opt/lms` (instruction 6, untouched).

### Verification performed

- `python3 -c "yaml.safe_load(...)"` — `deploy.yml` still parses cleanly; confirmed step order programmatically: `Sync compose files and migrations to the VPS` → `Validate critical production config on the VPS` → `Log in to GHCR on the VPS`, exactly as designed.
- `actionlint` across all six workflow files — **zero findings**.
- **The actual validation logic, extracted and run locally against real mock `.env` files** (not just read and trusted), covering four scenarios:

  | Scenario | Result |
  |---|---|
  | Fully valid config, `SMTP_HOST` empty | **PASS** (exit 0) — confirms instruction 3's requirement holds |
  | `POSTGRES_PASSWORD` empty, everything else valid | **FAIL** (exit 1), correctly names only `POSTGRES_PASSWORD` |
  | `JWT_SECRET` set to the exact `.env.production.example` placeholder string | **FAIL** (exit 1), correctly flagged as `JWT_SECRET_still_the_example_placeholder` (distinct from the empty-string case) |
  | Every required variable missing | **FAIL** (exit 1), all 7 problems listed together in one message (`JWT_SECRET_empty` fires, not the placeholder check too — no double-counting since an empty string isn't equal to the placeholder text) |

- Separately confirmed the `.env`-missing-entirely case produces the intended clear message (`".env not found in /opt/lms - see .env.production.example"`) rather than a bare filesystem error.

### Not done this session

- **No deploy, no push** — per instructions 7/8; nothing has been run against a real VPS.
- **No new GitHub secrets or variables** — this check only reads what's already on the VPS.
- **No change to which values are considered optional beyond SMTP** — `PAYMENT_PROVIDER`, `S3_*`, and the various numeric-default config values are still unvalidated by this step, since instruction 2's list is the explicit, minimum scope for this fix, not a general config-linter.
- **The placeholder-string duplication between the two files is not resolved**, only documented (see Design decisions).

## Trigger-chain investigation — why the chain stalled after `c9b70b6` (this session)

Scope: investigate why `CI Quality Gate` succeeding on `main` did not cascade into `Publish Production Images` or `Deploy to Production`, per a live observation (quality gate run #3 green, neither downstream workflow appeared). Fix the trigger issue only if a real one exists; no deploy, no application code touched.

### Investigation performed

Read `.github/workflows/quality-gate.yml`, `image-publish.yml`, and `deploy.yml` fresh. Checked exact `name:` fields against every `workflows: [...]` reference used by a `workflow_run` trigger — `image-publish.yml` listens for `"CI Quality Gate"`, matching `quality-gate.yml`'s `name:` exactly; `deploy.yml` listens for `"Publish Production Images"`, matching `image-publish.yml`'s `name:` exactly. No typo, no case mismatch. Checked `branches:`/`types:` filters on both (`branches: [main]`, `types: [completed]`) against the actual triggering event (a direct push to `main`) — correct. Checked both jobs' `if:` conditions (`workflow_run.conclusion == 'success'`) — correct for the `workflow_run` path.

Since nothing in the trigger *matching logic* itself was wrong, ran `git show --stat c9b70b6` to see exactly what that push changed: **only `.github/workflows/deploy.yml` (new), `docker-compose.prod.yml` (new), and `STAGE28_PROGRESS.md`.** `image-publish.yml` and `quality-gate.yml` were untouched by this push — they're byte-identical to the versions that already ran successfully once before (image-publish.yml's own first live run, recorded earlier in this file's post-session update).

### Root cause identified

**`deploy.yml` did not exist on `main` before this push introduced it.** GitHub's documented behavior for `workflow_run`: the *listening* workflow must already exist on the default branch to be eligible to receive a completion event from its upstream workflow — a workflow introduced in the very same push that would trigger its first potential upstream event cannot catch that first cycle; there is no retroactive matching. This is a one-time bootstrapping gap inherent to how GitHub registers new `workflow_run` listeners, not a defect in `deploy.yml`'s trigger definition, which is syntactically and semantically correct as written.

This fully explains why `Deploy to Production` never appeared. It does not, on its own, explain why `Publish Production Images` (not new, previously proven working) also produced no new run — no defect was found in its trigger config either; the most likely explanation is simply that no fresh `workflow_run` completion event for it had occurred since its own last run, and this investigation found no code-level reason to expect a new one from this particular push chain alone.

### Fix applied

No bug existed in any trigger-matching logic (names, branches, types, or `if:` conditions were all already correct), so nothing needed correcting there. Instead, added a **manual escape hatch** — `workflow_dispatch` — to `image-publish.yml`, matching the trigger `deploy.yml` already had:

- `.github/workflows/image-publish.yml`: added `workflow_dispatch:` alongside the existing `workflow_run:` trigger.
- **Two follow-on fixes this immediately required, caught by tracing the run through under the new trigger rather than assuming it would just work:**
  1. `verify-inputs`'s `if:` condition was `github.event.workflow_run.conclusion == 'success'` only — under `workflow_dispatch`, `github.event.workflow_run` doesn't exist, so that expression evaluates falsy and the job would have silently skipped itself, defeating the entire point of adding a manual trigger. Fixed to `github.event_name == 'workflow_dispatch' || github.event.workflow_run.conclusion == 'success'`, the exact pattern `deploy.yml`'s own `deploy` job already uses correctly.
  2. The `publish` job's checkout `ref:` and SHA-computation step both read `github.event.workflow_run.head_sha` directly, with no fallback — under `workflow_dispatch` this is empty, which would have produced a broken `sha-` tag (the prefix with nothing after it) rather than a real commit reference. Fixed both to `github.event.workflow_run.head_sha || github.sha`, again mirroring `deploy.yml`'s already-correct fallback pattern exactly.

`deploy.yml` needed no changes — it already has `workflow_dispatch` and the correct `||`-fallback pattern in both places; it was used as the reference implementation for fixing `image-publish.yml`.

### Files changed

- `.github/workflows/image-publish.yml` — added `workflow_dispatch` trigger; fixed `verify-inputs`'s `if:` condition; added `|| github.sha` fallback to the checkout ref and SHA-computation step.
- `STAGE28_PROGRESS.md` — this section added.

No application code touched. No deploy attempted. No push made by this session.

### Verification performed

- `python3 -c "yaml.safe_load(...)"` — `image-publish.yml` still parses cleanly; confirmed both `workflow_run` and `workflow_dispatch` present under `on:`.
- `actionlint` across all six workflow files — **zero findings**.
- Simulated the fixed `if:` logic directly: `workflow_dispatch` (no `workflow_run` context) → job runs; `workflow_run` + `conclusion=success` → job runs; `workflow_run` + `conclusion=failure` → job correctly skipped.
- Simulated the SHA-fallback logic directly: empty `workflow_run.head_sha` (the `workflow_dispatch` case) → falls back to `github.sha`; a real `head_sha` present → used as-is, fallback never triggered.

### What exact push/re-run is needed next

**No push is required.** Everything needed is already on `main` as of `c9b70b6`, plus this session's fix once committed. The concrete next steps, in order:

1. Commit and push this fix (not done by this session, per instruction).
2. From the Actions tab, open **Publish Production Images** and use **Run workflow** (the new `workflow_dispatch` trigger) on `main`. This exercises the exact same build-and-push logic that already worked once before, now runnable on demand.
3. If that run succeeds, **Deploy to Production** should fire automatically via its own `workflow_run` listener — and this time it genuinely can, since `deploy.yml` now already exists on `main` (the one-time bootstrapping gap identified above no longer applies).
4. What happens next depends entirely on whether the 4 `PROD_VPS_*` secrets are configured: if not, `deploy.yml` fails immediately and harmlessly at `Verify required secrets are present`, exactly as designed; if they are configured and point at a real host, this is a real deployment attempt.

## Stage 28A4 — production Nginx + HTTPS preparation (this session)

Scope: design and prepare host-level Nginx configuration for `compserv.cloud` in front of the already-deployed, healthy production stack, plus a Certbot/Let's Encrypt path. No VPS contact, no deploy, no application code changes.

### Inspection performed

Read this file (28A1–28A3 plus the trigger-chain investigation), `docker-compose.yml`, `docker-compose.prod.yml`, and `.github/workflows/deploy.yml` fresh. Inspected the frontend's actual API-client code (`frontend/lib/api.ts`) rather than assuming: `apiBaseUrl()` returns `SERVER_API_URL` (`API_INTERNAL_URL`) when running server-side and `PUBLIC_API_URL` (`NEXT_PUBLIC_API_URL`) when running in the browser (`typeof window === "undefined"` check) — confirming the browser genuinely does make direct client-side fetches to the backend, not everything routed through a Next.js server proxy. Inspected the backend's router setup (`backend/cmd/api/main.go`): every route is registered under `router.Group("/api/v1")` — the literal `/api` prefix is part of every real route, not something a proxy should strip. Grepped the entire backend for `cors`/`CORS`/`Access-Control`/`gin-contrib` and for `FRONTEND_URL` across the whole repo — **found none of either**: this backend has no CORS middleware at all, and `FRONTEND_URL` isn't a config value that exists anywhere in this codebase. Checked the frontend for WebSocket usage (`grep -rln "WebSocket\|socket.io\|ws://"`) — found one hit, a comment in `lib/actions.ts` explicitly stating the opposite: `"no WebSocket, same interval-poll shape as VideoProcessingStatusPanel"` — this app deliberately doesn't use WebSockets, so `Upgrade`/`Connection` proxy headers aren't needed for anything currently real.

### Findings that shaped the design

- **No backend CORS exists, and none needs to be added.** Instruction 6 asked whether backend CORS needs changing for `https://compserv.cloud` — the honest answer is there's nothing to change because there's nothing there. The reason this is safe: with Nginx proxying `compserv.cloud/` to the frontend and `compserv.cloud/api/` to the backend, every browser request the frontend's client code makes lands on the **same origin** as the page itself (same scheme, same host, same port — only the path differs). Same-origin requests are never subject to CORS restrictions at all, by browser design. This only holds if `NEXT_PUBLIC_API_URL` is set to `https://compserv.cloud/api` (same-origin, path-based) — not to a different host or port, which would reintroduce a real cross-origin requirement with no CORS middleware to satisfy it.
- **`FRONTEND_URL` doesn't exist in this codebase** — confirmed by grepping the whole repo, not assumed from the instruction's phrasing.
- **`NEXT_PUBLIC_API_URL` matters in two genuinely different places, and both need to end up as `https://compserv.cloud/api`:**
  1. **Baked into the frontend image's client bundle at build time**, by `image-publish.yml`'s build-arg, sourced from the GitHub Actions repository variable `PROD_NEXT_PUBLIC_API_URL` — **not** from anything in this repo's files. Editing `.env.production.example` or the VPS's `.env` does not change an already-built image. This variable's current value is unknown to this session (no `gh` CLI access) — it must be checked/updated, and the frontend image rebuilt and redeployed, for the *client-side* API URL to actually become `https://compserv.cloud/api`.
  2. **Read live by any server-side code** that touches `process.env.NEXT_PUBLIC_API_URL` — this path *does* read the VPS's own `.env` at runtime. Updated `.env.production.example`'s `NEXT_PUBLIC_API_URL` to `https://compserv.cloud/api` accordingly (was a `__YOUR_PROD_DOMAIN__` placeholder).
- **A real gap surfaced, not solved:** `.env.production.example`'s `S3_PUBLIC_ENDPOINT` was already templated as `https://__YOUR_PROD_DOMAIN__/storage` — implying a `/storage` proxy path that this session's Nginx design, per its exact explicit scope (`/` and `/api/` only), does not provide. MinIO has no public port at all (`docker-compose.prod.yml`'s `ports: !reset []`, from 28A3). Presigned video URLs built from this value will not resolve anywhere. Flagged explicitly in `.env.production.example` and here — not fixed, since it's outside this session's exact two-route scope and would need its own design decision (proxy `/storage` to MinIO, or move to a real S3 provider with its own endpoint).
- **`backend`/`frontend` ports restricted to `127.0.0.1` in `docker-compose.prod.yml`.** `docker-compose.prod.yml` already stated the intent ("Nginx is the only service that should ever bind a public port") since 28A3 — this session follows through: now that Nginx is actually being designed, the raw application ports have no reason to stay reachable from outside the VPS. **Caught a real bug applying this**, the same class already hit once this session with `depends_on`: Compose's default merge behavior for list-type fields like `ports:` is to *concatenate*, not replace — a plain (unmarked) override would have left the base file's public `0.0.0.0` binding sitting alongside the new `127.0.0.1` one, achieving nothing. Fixed with `!override`; verified by resolving the actual merged config (not assumed) — confirmed exactly one `ports:` entry each, `host_ip: '127.0.0.1'`, no leftover public binding.
- **No WebSocket support added to the Nginx config.** Checked, not assumed: this app doesn't use WebSockets anywhere today. Adding `Upgrade`/`Connection` proxy headers preemptively would be speculative complexity for a feature that doesn't exist — left out, noted here as trivial to add later if one ever is.
- **`deploy.yml` needs no changes.** It only manages Docker Compose services; Nginx runs on the host, entirely outside its scope, per instruction 2. Its own backend/frontend health checks already run against `localhost:$BACKEND_PORT`/`localhost:$FRONTEND_PORT` directly, bypassing any proxy by design — unaffected by adding one.
- **`proxy_pass` target form was deliberately chosen to avoid a real path-stripping bug**, not just written and assumed correct. `proxy_pass http://127.0.0.1:8080/;` (trailing slash) would have stripped the matched `/api/` location prefix, turning a request for `/api/v1/health` into `/v1/health` at the backend — which registers nothing there (everything lives under `/api/v1/...`, confirmed above). Used the bare form instead — `proxy_pass http://127.0.0.1:8080;` (no trailing path at all) — which Nginx passes the original request URI through completely unmodified, regardless of which location matched. Verified live (see Verification below), not just reasoned about.

### Nginx design

Two files, `deploy/nginx/compserv.cloud.pre-cert.conf` and `deploy/nginx/compserv.cloud.conf` — a deliberate two-stage design, not one file with commented-out sections, because Certbot's HTTP-01 challenge needs port 80 already serving `/.well-known/acme-challenge/` for the domain *before* a certificate can exist, so the HTTPS server block genuinely cannot be installed first.

**Stage 1 (`compserv.cloud.pre-cert.conf`)** — HTTP only, installed first:
- One `server { listen 80; }` block for `compserv.cloud`/`www.compserv.cloud`.
- `location /.well-known/acme-challenge/ { root /var/www/certbot; }` — the path Certbot's webroot plugin writes challenge files into.
- `location /api/` → `proxy_pass http://127.0.0.1:8080;` and `location /` → `proxy_pass http://127.0.0.1:3001;`, both with `Host`/`X-Real-IP`/`X-Forwarded-For`/`X-Forwarded-Proto` set, per instruction 4.

**Stage 2 (`compserv.cloud.conf`)** — installed after a certificate exists:
- Port 80 server block reduced to the ACME-challenge location (kept alive indefinitely, so unattended renewal keeps working) plus a `return 301 https://$host$request_uri;` redirect for everything else.
- Port 443 server block with `ssl_certificate`/`ssl_certificate_key` pointing at the standard Certbot-managed paths (`/etc/letsencrypt/live/compserv.cloud/{fullchain,privkey}.pem`), explicit modern TLS settings (`TLSv1.2`/`TLSv1.3`, Mozilla "intermediate" baseline) rather than `include /etc/letsencrypt/options-ssl-nginx.conf` — that file is generated specifically by Certbot's *nginx plugin*, which this design deliberately doesn't use (see below), so depending on it would be depending on a file that might not exist.
- Same `/api/` and `/` proxy blocks as Stage 1, carried over unchanged.

**Why `certbot certonly --webroot`, not `certbot --nginx`:** the nginx plugin auto-edits whatever config it finds, in ways that are hard to review ahead of time or reproduce exactly — the opposite of what a "preparation" deliverable should be. `certonly --webroot` only obtains/renews the certificate file and touches nothing in `/etc/nginx` — every line of both config files here is something a human (or this session) actually wrote and can review, not something a plugin generated.

### Verification performed

**`nginx -t`, genuinely run** (via the official `nginx:alpine` image, no host install needed) **against the actual repo files**, not drafts assumed to be equivalent:
- `deploy/nginx/compserv.cloud.pre-cert.conf` — **syntax OK, test successful**.
- `deploy/nginx/compserv.cloud.conf` — validated with a real (throwaway, self-signed, generated purely for this test and discarded after) certificate mounted at the exact paths the config references, so Nginx's own certificate-loading logic was genuinely exercised, not just brace-matched — **syntax OK, test successful**.

**End-to-end request routing, proven live, not reasoned about** — stood up two minimal Python echo servers (stand-ins for backend/frontend, each reporting exactly which path and headers they received) plus a real Nginx container running the same location/proxy_pass logic as the design, all on an isolated Docker network (removed afterward):

| Request | Reached backend/frontend as | Correct? |
|---|---|---|
| `GET /api/v1/health` | `/api/v1/health` | Yes — full path preserved, not stripped to `/v1/health` |
| `GET /api/v1/auth/login` | `/api/v1/auth/login` | Yes |
| `GET /` | `/` | Yes |
| `GET /dashboard/courses` | `/dashboard/courses` | Yes — frontend routing paths pass through untouched |

**Proxy headers, confirmed actually received by the backend**, not just declared in the config: `Host`, `X-Real-IP`, `X-Forwarded-For` all arrived correctly. `X-Forwarded-Proto` was deliberately tested by sending a spoofed `X-Forwarded-Proto: https` header as the *client* — the backend received `X-Forwarded-Proto: http` instead, confirming `proxy_set_header X-Forwarded-Proto $scheme;` uses Nginx's own knowledge of the actual connection scheme and overrides whatever a client tries to inject, not a client-controllable passthrough. A real, useful security property, confirmed rather than assumed.

**`docker compose config`**, re-verified after the `docker-compose.prod.yml` port changes: exactly one `ports:` entry each for `backend`/`frontend`, `host_ip: '127.0.0.1'`, no leftover public `0.0.0.0` binding from the base file.

### Change made

- `deploy/nginx/compserv.cloud.pre-cert.conf` — new.
- `deploy/nginx/compserv.cloud.conf` — new.
- `docker-compose.prod.yml` — `backend`/`frontend` now bind `127.0.0.1:<port>` instead of the base file's implicit `0.0.0.0`.
- `.env.production.example` — `NEXT_PUBLIC_API_URL`/`S3_PUBLIC_ENDPOINT` placeholders replaced with real `compserv.cloud` values; both annotated with the build-time-vs-runtime distinction and the storage-proxy gap respectively.

No application code changed. No VPS contacted. Nothing deployed.

### Files changed

- `deploy/nginx/compserv.cloud.pre-cert.conf` — new.
- `deploy/nginx/compserv.cloud.conf` — new.
- `docker-compose.prod.yml` — port bindings restricted to localhost.
- `.env.production.example` — domain placeholders filled in, two gaps annotated.
- `STAGE28_PROGRESS.md` — this section added.

### Known limitations

- **`PROD_NEXT_PUBLIC_API_URL`'s current GitHub Actions variable value is unknown to this session** (no `gh` CLI access) — must be checked and, if not already `https://compserv.cloud/api`, updated and the frontend image rebuilt/republished before the browser-side API URL is actually correct. Until that happens, the *already-deployed, currently-healthy* frontend keeps working exactly as it does now (this is a forward-looking change, not something broken today).
- **`S3_PUBLIC_ENDPOINT`/presigned video URLs are not covered by this Nginx design** — flagged prominently in `.env.production.example` and above; video playback via presigned MinIO URLs will not work through `compserv.cloud` until a future session resolves it (proxy `/storage`, or a real S3 provider).
- **TOFU host-key trust and the missing GitHub Environment approval gate**, both noted in 28A3, are unchanged — unrelated to Nginx but still open.
- **This entire design is unvalidated against the real VPS**, by explicit instruction — `nginx -t` and the routing/header behavior were proven genuinely, but against stand-ins (Docker containers), not `194.31.55.106` itself.
- **Certificate renewal automation isn't set up yet** — Certbot's installed package typically registers a systemd timer/cron job automatically, but this session didn't verify that on the real VPS (couldn't — no VPS access). Worth an explicit check as part of the manual steps below (`certbot renew --dry-run`).

## Exact manual VPS commands (Stage 28A4 — nothing above has been run against `194.31.55.106`)

Run as a user with `sudo` (the `devops` deploy user, or root). None of this has been executed by this session, per instruction.

**1. Install nginx and certbot, if not already present:**
```bash
sudo apt update
sudo apt install -y nginx certbot
sudo mkdir -p /var/www/certbot
```

**2. Copy the bootstrap (HTTP-only) config to the VPS** (from your local checkout, adjust the source path as needed):
```bash
scp -P 22122 deploy/nginx/compserv.cloud.pre-cert.conf devops@194.31.55.106:/tmp/compserv.cloud.conf
ssh -p 22122 devops@194.31.55.106 "sudo mv /tmp/compserv.cloud.conf /etc/nginx/sites-available/compserv.cloud.conf"
```

**3. Enable the site and remove the default one (if present and still listening on 80):**
```bash
ssh -p 22122 devops@194.31.55.106 "sudo ln -sf /etc/nginx/sites-available/compserv.cloud.conf /etc/nginx/sites-enabled/compserv.cloud.conf && sudo rm -f /etc/nginx/sites-enabled/default"
```

**4. Test and reload:**
```bash
ssh -p 22122 devops@194.31.55.106 "sudo nginx -t && sudo systemctl reload nginx"
```
(If nginx isn't running yet: `sudo systemctl enable --now nginx` instead of `reload`.)

**5. Confirm DNS actually points `compserv.cloud` (and `www.compserv.cloud`, if used) at `194.31.55.106` before requesting a certificate** — Certbot's HTTP-01 challenge will fail otherwise:
```bash
dig +short compserv.cloud
dig +short www.compserv.cloud
```

**6. Obtain the certificate** (webroot method — does not touch the Nginx config itself):
```bash
ssh -p 22122 devops@194.31.55.106 \
  "sudo certbot certonly --webroot -w /var/www/certbot -d compserv.cloud -d www.compserv.cloud"
```
(Drop `-d www.compserv.cloud` if that subdomain isn't actually in use/pointed at this server.)

**7. Install the final config (HTTP→HTTPS redirect + HTTPS serving):**
```bash
scp -P 22122 deploy/nginx/compserv.cloud.conf devops@194.31.55.106:/tmp/compserv.cloud.conf
ssh -p 22122 devops@194.31.55.106 "sudo mv /tmp/compserv.cloud.conf /etc/nginx/sites-available/compserv.cloud.conf && sudo nginx -t && sudo systemctl reload nginx"
```

**8. Restrict the application ports to localhost** (requires the `docker-compose.prod.yml` change from this session to already be on the VPS, i.e. after the next `deploy.yml` run — or apply manually now):
```bash
ssh -p 22122 devops@194.31.55.106 \
  "cd /opt/lms && docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d backend frontend"
```

**9. Final verification:**
```bash
curl -I https://compserv.cloud/
curl -s https://compserv.cloud/api/v1/health
curl -I http://compserv.cloud/          # expect a 301 to https://
ssh -p 22122 devops@194.31.55.106 "sudo certbot renew --dry-run"   # confirms auto-renewal is wired up
```

**10. Update the two `NEXT_PUBLIC_API_URL` locations identified above**, separately from everything else here:
- GitHub → repo Settings → Secrets and variables → Actions → Variables → `PROD_NEXT_PUBLIC_API_URL` → confirm/set to `https://compserv.cloud/api`, then manually dispatch **Publish Production Images** (or push a commit) to rebuild the frontend image with the correct baked-in client URL, then let **Deploy to Production** run to actually roll it out.
- The VPS's `/opt/lms/.env` — confirm `NEXT_PUBLIC_API_URL=https://compserv.cloud/api` (already reflected in this repo's `.env.production.example`, but the *live* file needs the same edit made by hand, since `deploy.yml` never touches this file directly per 28A3's design).

## Remaining for Stage 28 (not started)

28A2 covered the Dockerfile-hardening work 28A1 anticipated as its own session together with the GHCR build/publish workflow (confirmed live, see the post-session update above). 28A3 (this session) built the actual VPS deploy workflow and production compose override — designed, built, and validated as thoroughly as possible without a real server, but never executed against one. What's left:

- **A VPS.** The single real blocker for everything below — nothing in `deploy.yml` can be proven end-to-end until one exists, is provisioned per 28A1/28A3's documented prerequisites (Docker installed, deploy user, `/opt/lms` created, an initial `.env` from `.env.production.example`, firewall rules), and its SSH details are added as the 4 secrets 28A3 requires.
- **28A4 — Nginx/HTTPS session:** reverse proxy in front of frontend/backend, Let's Encrypt/certbot, the firewall/port changes this design assumes. Also the natural point to revisit whether the backend/frontend health checks should additionally be checked from outside the VPS (through the proxy), not just locally as 28A3 does today.
- **A real GitHub Actions run of `deploy.yml`** — not yet fired; needs a VPS and its 4 secrets configured first.
- **Post-deploy smoke test**, per the roadmap's own E2E requirement: health check, login, enroll, a payment-provider sandbox transaction, certificate verification — depends on a real deploy existing first, and on the payment-provider gap being resolved for the payment-sandbox portion specifically.
- **Rollback procedure, tested for real** — 28A1 designed it, 28A3 built the structural mechanism (manual re-dispatch with an older tag), a later session needs to actually exercise it once a VPS and a real prior deploy both exist.
- **Optional hardening, not blockers:** a pinned `PROD_VPS_KNOWN_HOSTS` secret (currently TOFU), a dedicated GitHub Environment with required-reviewer approval gates, a friendlier rollback UX that doesn't require pasting an exact SHA by hand.
- **The SMTP-provider and payment-provider gaps** — unresolved since 28A1, independent of the deploy pipeline's own correctness.
