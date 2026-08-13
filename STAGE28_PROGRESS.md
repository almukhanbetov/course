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

### Known limitations

- **`PROD_NEXT_PUBLIC_API_URL` is not yet configured** as a GitHub Actions repository/environment variable — until it is, `image-publish.yml`'s `verify-inputs` job will fail closed by design, and no image (not even the other four, since `publish` needs `verify-inputs`) will be built. This is the intended behavior (fail closed on a missing required input), not a bug, but it does mean the workflow cannot succeed yet as configured — flagged here so it isn't mistaken for something broken.
- **Not yet proven against a real GitHub Actions run or a real GHCR push** — everything above was validated locally: YAML parsing, `actionlint`, `act -l`'s dependency-graph resolution, and full local builds/runs of all 5 images (including a live, DB-backed backend health check). None of this is the same as watching the workflow actually authenticate to `ghcr.io` and push a real manifest.
- **`code-runner` remains the one image that runs as root** — by design, not an oversight; a future session could revisit whether an alternative sandboxing mechanism (e.g., gVisor, a different namespace-creation approach) could avoid needing `CAP_SYS_ADMIN` as root, but that's a Stage-16-level sandboxing redesign, well outside this session's Dockerfile-hardening scope.
- **Next.js `output: "standalone"` not adopted** — would shrink the frontend image further than the `prod-deps` split achieved, but touches `next.config.ts` and the Dockerfile's runtime entrypoint; left as a future option, not implemented, per instruction 11's "do not redesign application behavior."
- **No Dockerfile-level `HEALTHCHECK` or Compose-level `healthcheck:` added for `backend`/`frontend`** — still an open item from 28A1, deliberately not addressed here either (it's an orchestration concern for the deploy-workflow sub-session, not an image-build one).
- **GHCR package visibility (private vs. public) is not yet set** — nothing has been pushed yet, so there's nothing to configure; 28A1's recommendation (keep private, VPS does one `docker login`) still stands as the plan for 28A3.
- **The registry-owner path (`ghcr.io/almukhanbetov/...`) assumes `github.repository_owner`** resolves to the expected account — correct for this repo's current `origin` remote, not re-verified against any GitHub Environment/org configuration since none exists yet.

## Remaining for Stage 28 (not started)

28A2 (this session) covered both the Dockerfile-hardening work 28A1 anticipated as its own session and the GHCR build/publish workflow — done together since they turned out to be one coherent unit of work. What's left, per the roadmap's own recommended split:

- **Prerequisite for `image-publish.yml` to actually succeed:** configure `PROD_NEXT_PUBLIC_API_URL` as a GitHub Actions repository/environment variable — the workflow fails closed without it, by design (see 28A2's Known limitations).
- **28A3 — deploy-workflow session:** `docker-compose.prod.yml`, `.github/workflows/deploy.yml` implementing the deployment-ordering sequence above, the SSH step, the exit-code-gated migration step, the health-check loop, and the `workflow_dispatch` rollback path — now able to reference `image-publish.yml`'s `sha-*` tags directly, since those are built and ready.
- **28A4 — Nginx/HTTPS session:** reverse proxy in front of frontend/backend, Let's Encrypt/certbot, the firewall/port changes this design assumes.
- **Post-deploy smoke test**, per the roadmap's own E2E requirement: health check, login, enroll, a payment-provider sandbox transaction, certificate verification — depends on 28A3 existing first, and on the payment-provider gap above being resolved for the payment-sandbox portion specifically.
- **Rollback procedure, tested for real** — 28A1 designed it; 28A3 needs to build it; a later session needs to actually exercise it once a VPS exists.
- **A real GitHub Actions run of `image-publish.yml`** — not yet fired, since it needs `PROD_NEXT_PUBLIC_API_URL` configured and a push to `main` after `quality-gate` passes.
