#!/usr/bin/env bash
# Stage 30A1 — production PostgreSQL backup.
#
# Runs ON THE VPS (synced there and executed over SSH by
# .github/workflows/backup.yml), from the same directory deploy.yml itself
# uses ($DEPLOY_DIR, default /opt/lms) — reads that directory's existing
# .env for every value it needs, the same "VPS secrets stay on the VPS"
# convention every other production secret has followed since Stage 28A3.
# Adds exactly one new required value to that .env: BACKUP_ENCRYPTION_PASSPHRASE
# (see .env.production.example) — never stored in git, never transmitted
# through GitHub Actions, set once by hand on the VPS like every other
# production secret.
#
# set -e: any failing command aborts the script immediately (instruction 9:
# fail closed, non-zero exit). set -u: an unset variable is an error, not a
# silent empty string. set -o pipefail: a failure in the middle of a pipe
# (e.g. pg_dump failing before it reaches the output redirect) isn't masked
# by a later command in the same pipeline succeeding.
set -euo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/opt/lms}"
cd "$DEPLOY_DIR"

if [ ! -f .env ]; then
  echo "backup: .env not found in $DEPLOY_DIR" >&2
  exit 1
fi
set -a
# shellcheck disable=SC1091
. ./.env
set +a

# Fail closed before touching Postgres/MinIO at all — same philosophy as
# deploy.yml's own "Validate critical production config" step (Stage 28A3
# safety fix), extended here to this script's own required values.
missing=""
for var in POSTGRES_DB S3_BUCKET S3_ENDPOINT S3_ACCESS_KEY S3_SECRET_KEY BACKUP_ENCRYPTION_PASSPHRASE; do
  if [ -z "${!var:-}" ]; then
    missing="$missing $var"
  fi
done
if [ -n "$missing" ]; then
  echo "backup: missing required config in $DEPLOY_DIR/.env:$missing" >&2
  exit 1
fi

TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_NAME="lms-backup-${TIMESTAMP}"

# A dedicated, restrictively-permissioned scratch directory — the raw
# (unencrypted) dump only ever exists here, briefly, and is removed by the
# EXIT trap below regardless of how the script ends (success, a later
# failure, or being killed) — instruction 10's "prevent a partially-created
# backup from being treated as successful" starts with never leaving a
# half-finished artifact lying around at all, encrypted or not.
SCRATCH_DIR="$(mktemp -d)"
chmod 700 "$SCRATCH_DIR"
RAW_DUMP="$SCRATCH_DIR/${BACKUP_NAME}.dump"
ENCRYPTED_DUMP="${RAW_DUMP}.enc"

cleanup() {
  rm -rf "$SCRATCH_DIR"
}
trap cleanup EXIT

echo "backup: starting pg_dump (${BACKUP_NAME})"

# -Fc (custom format): compressed already (no separate gzip step needed),
# and restorable selectively/in parallel via pg_restore — a materially
# better artifact for "suitable for reliable restore" (instruction 3) than
# plain SQL text output. Runs *inside* the already-running postgres
# container via `compose exec`, reusing the POSTGRES_USER/POSTGRES_PASSWORD
# already present in that container's own environment (set by
# docker-compose.yml) — no credential is freshly constructed or passed by
# this script at all. PGPASSWORD is read from that container's own env by
# pg_dump itself, and — because it's an environment variable, not a
# command-line argument — never appears in any process listing (`ps aux`)
# on the VPS or inside the container, satisfying instruction 12 directly.
if ! docker compose exec -T postgres sh -c \
    'PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB"' \
    > "$RAW_DUMP"; then
  echo "backup: pg_dump failed" >&2
  exit 1
fi

if [ ! -s "$RAW_DUMP" ]; then
  echo "backup: pg_dump produced an empty or missing artifact" >&2
  exit 1
fi
echo "backup: pg_dump OK ($(wc -c < "$RAW_DUMP") bytes)"

echo "backup: encrypting"

# AES-256-CBC with PBKDF2 key derivation (not the legacy, weak
# EVP_BytesToKey default) — the standard, portable way to encrypt a file
# with openssl's CLI without pulling in a new dependency (openssl is
# already present on essentially any Linux VPS). The passphrase is read
# from an environment variable (`-pass env:...`), never a CLI argument —
# same ps-aux-safety reasoning as PGPASSWORD above.
if ! BACKUP_ENCRYPTION_PASSPHRASE="$BACKUP_ENCRYPTION_PASSPHRASE" openssl enc -aes-256-cbc -pbkdf2 -iter 100000 -salt \
    -pass env:BACKUP_ENCRYPTION_PASSPHRASE \
    -in "$RAW_DUMP" -out "$ENCRYPTED_DUMP"; then
  echo "backup: encryption failed" >&2
  exit 1
fi
rm -f "$RAW_DUMP"

if [ ! -s "$ENCRYPTED_DUMP" ]; then
  echo "backup: encrypted artifact is missing or empty" >&2
  exit 1
fi
echo "backup: encryption OK ($(wc -c < "$ENCRYPTED_DUMP") bytes)"

echo "backup: uploading to object storage"

# MinIO has no public port in production (Stage 28A4's docker-compose.prod.yml
# `ports: !reset []`) — only reachable from other containers on the compose
# network. Rather than guessing Compose's auto-generated network name,
# `--network container:<minio's own container id>` makes this ad-hoc `mc`
# container share MinIO's own network namespace, so MinIO is always
# reachable at plain `localhost:9000` regardless of the compose project
# name. Credentials flow in via MC_HOST_backup (an environment variable,
# mc's own documented non-interactive-credential convention) rather than
# `mc alias set ACCESSKEY SECRETKEY` as literal CLI arguments — same
# ps-aux-safety reasoning as PGPASSWORD/BACKUP_ENCRYPTION_PASSPHRASE above,
# and it never writes a persistent ~/.mc/config.json credential file either.
MINIO_CID="$(docker compose ps -q minio)"
if [ -z "$MINIO_CID" ]; then
  echo "backup: minio container not found/running" >&2
  exit 1
fi

MC_HOST_backup="http://${S3_ACCESS_KEY}:${S3_SECRET_KEY}@localhost:9000"
BACKUP_OBJECT_KEY="backups/${BACKUP_NAME}.dump.enc"

if ! docker run --rm \
    --network "container:${MINIO_CID}" \
    -e MC_HOST_backup="$MC_HOST_backup" \
    -v "${ENCRYPTED_DUMP}:/backup.enc:ro" \
    minio/mc:latest cp /backup.enc "backup/${S3_BUCKET}/${BACKUP_OBJECT_KEY}"; then
  echo "backup: upload failed" >&2
  exit 1
fi

echo "backup: verifying uploaded artifact"

# Don't just trust `mc cp`'s own exit code — independently confirm the
# object actually exists at the destination with a non-zero size
# (instruction 13's explicit "upload/storage succeeds" check), the same
# "verify, don't assume" discipline every prior stage's own verification
# has used. --json is parsed for its "size" field rather than grepping
# mc's human-formatted table output, which is not a stable format to match
# against.
STAT_JSON="$(docker run --rm \
    --network "container:${MINIO_CID}" \
    -e MC_HOST_backup="$MC_HOST_backup" \
    minio/mc:latest stat --json "backup/${S3_BUCKET}/${BACKUP_OBJECT_KEY}")"

UPLOADED_SIZE="$(echo "$STAT_JSON" | grep -o '"size":[0-9]*' | head -1 | cut -d: -f2)"
if [ -z "$UPLOADED_SIZE" ] || [ "$UPLOADED_SIZE" -eq 0 ]; then
  echo "backup: uploaded artifact is missing or reports 0 bytes" >&2
  echo "backup: mc stat output was: $STAT_JSON" >&2
  exit 1
fi

echo "backup: SUCCESS - ${BACKUP_OBJECT_KEY} (${UPLOADED_SIZE} bytes)"
