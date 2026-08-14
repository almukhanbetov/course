#!/usr/bin/env bash
# Stage 30A3 — backup retention policy.
#
# Runs ON THE VPS, invoked automatically by run-backup.sh right after a
# successful backup (see that script's final step), and can also be run
# standalone (e.g. by hand over SSH) for a dry run or an out-of-band
# cleanup. Same "$DEPLOY_DIR/.env, VPS secrets stay on the VPS" convention
# as run-backup.sh and deploy.yml — no new secret is needed, this script
# reuses the same S3_* values run-backup.sh already requires.
#
# Retention window: keep the newest RETENTION_KEEP_COUNT backups (default
# 14), delete anything older. Backups run daily (backup.yml's cron), so 14
# is a two-week recovery window — enough to notice and recover from a
# slow-burning problem (a bad migration, silent data corruption) that
# isn't caught the same day, while bounding storage growth on a single-VPS
# production setup that has no independent storage-capacity monitoring of
# its own (Stage 29's observability doesn't include a disk-usage alert).
# Override via RETENTION_KEEP_COUNT in .env if that trade-off ever needs
# revisiting; not a required value (defaults to 14 if unset/absent).
#
# set -e/-u/-o pipefail: same fail-closed reasoning as run-backup.sh —
# any unexpected failure (a bad listing, an unset variable) aborts before
# any deletion is attempted, never silently proceeds with partial/wrong
# information about what exists.
set -euo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/opt/lms}"
cd "$DEPLOY_DIR"

if [ ! -f .env ]; then
  echo "retention: .env not found in $DEPLOY_DIR" >&2
  exit 1
fi

# Same line-by-line parser as run-backup.sh (not bash `source`) — tolerates
# this project's real .env format (unquoted values containing spaces).
while IFS='=' read -r key value; do
  case "$key" in
    ''|'#'*) continue ;;
  esac
  export "$key=$value"
done < <(grep -v '^\s*#' .env | grep '=')

missing=""
for var in S3_BUCKET S3_ENDPOINT S3_ACCESS_KEY S3_SECRET_KEY; do
  if [ -z "${!var:-}" ]; then
    missing="$missing $var"
  fi
done
if [ -n "$missing" ]; then
  echo "retention: missing required config in $DEPLOY_DIR/.env:$missing" >&2
  exit 1
fi

RETENTION_KEEP_COUNT="${RETENTION_KEEP_COUNT:-14}"
if ! [[ "$RETENTION_KEEP_COUNT" =~ ^[0-9]+$ ]] || [ "$RETENTION_KEEP_COUNT" -lt 1 ]; then
  echo "retention: RETENTION_KEEP_COUNT must be a positive integer (got '$RETENTION_KEEP_COUNT')" >&2
  exit 1
fi

# DRY_RUN=true: report what would be deleted without deleting anything.
# The default (unset/false) performs real deletions — the same "opt into
# the safe mode explicitly" shape as everywhere else isn't needed here
# since dry-run *is* the safe direction; DRY_RUN defaults to false because
# an automated nightly retention run that never actually deletes anything
# would silently defeat its own purpose.
DRY_RUN="${DRY_RUN:-false}"

MINIO_CID="$(docker compose ps -q minio)"
if [ -z "$MINIO_CID" ]; then
  echo "retention: minio container not found/running" >&2
  exit 1
fi

MC_HOST_backup="http://${S3_ACCESS_KEY}:${S3_SECRET_KEY}@localhost:9000"
# Scoped to exactly the backups/ prefix this bucket's other content (video
# assets, etc.) never lives under — instruction 5's "never delete outside
# the expected backup prefix/path" starts with never even *listing*
# anything else as a deletion candidate.
BACKUP_PREFIX="backup/${S3_BUCKET}/backups/"

echo "retention: listing existing backups under ${BACKUP_PREFIX} (keep newest ${RETENTION_KEEP_COUNT}, dry_run=${DRY_RUN})"

# Fail closed: a listing failure (network blip, bad credentials, bucket
# temporarily unreachable) must abort here, not be treated as "zero
# backups exist" — the latter would make every remaining object look
# eligible for deletion, exactly backwards from safe behavior.
if ! LIST_JSON="$(docker run --rm \
    --network "container:${MINIO_CID}" \
    -e MC_HOST_backup="$MC_HOST_backup" \
    minio/mc:latest ls --json "$BACKUP_PREFIX")"; then
  echo "retention: listing backups failed - aborting, no deletions attempted" >&2
  exit 1
fi

# Only filenames that exactly match this mechanism's own naming
# convention (lms-backup-<UTC timestamp>.dump.enc, as produced by
# run-backup.sh) are ever considered — a second layer of instruction 5's
# safety requirement, independent of the prefix scoping above. Anything
# else under backups/ (there shouldn't be anything, but "shouldn't" is not
# a safety guarantee) is left untouched and reported, not silently
# ignored, so an operator notices unexpected content.
mapfile -t ALL_KEYS < <(echo "$LIST_JSON" | grep -o '"key":"[^"]*"' | cut -d'"' -f4)
mapfile -t BACKUP_NAMES < <(printf '%s\n' "${ALL_KEYS[@]}" | grep -E '^lms-backup-[0-9]{8}T[0-9]{6}Z\.dump\.enc$' | sort)

UNRECOGNIZED_COUNT=0
for k in "${ALL_KEYS[@]}"; do
  if ! [[ "$k" =~ ^lms-backup-[0-9]{8}T[0-9]{6}Z\.dump\.enc$ ]]; then
    echo "retention: NOTE - ignoring unrecognized object under ${BACKUP_PREFIX}: ${k} (not touched)" >&2
    UNRECOGNIZED_COUNT=$((UNRECOGNIZED_COUNT + 1))
  fi
done

TOTAL_COUNT="${#BACKUP_NAMES[@]}"
echo "retention: found ${TOTAL_COUNT} recognized backup(s), ${UNRECOGNIZED_COUNT} unrecognized object(s) left untouched"

if [ "$TOTAL_COUNT" -le "$RETENTION_KEEP_COUNT" ]; then
  echo "retention: ${TOTAL_COUNT} backup(s) <= keep count ${RETENTION_KEEP_COUNT}; nothing to delete"
  exit 0
fi

DELETE_COUNT=$((TOTAL_COUNT - RETENTION_KEEP_COUNT))
# Names sort lexicographically in chronological order (the timestamp
# format is fixed-width and zero-padded), so the first DELETE_COUNT
# entries are the oldest and the last RETENTION_KEEP_COUNT entries
# (including, always, the single newest one) are never candidates —
# instruction 4's "never delete the newest valid backup" is a structural
# property of this split, not a special case bolted on afterward.
NEWEST_NAME="${BACKUP_NAMES[$((TOTAL_COUNT - 1))]}"
echo "retention: newest backup (never a deletion candidate): ${NEWEST_NAME}"
echo "retention: will delete ${DELETE_COUNT} backup(s) older than the newest ${RETENTION_KEEP_COUNT}"

DELETED_COUNT=0
for ((i = 0; i < DELETE_COUNT; i++)); do
  NAME="${BACKUP_NAMES[$i]}"

  # Belt-and-braces: this can never actually trigger given the split
  # above, but an assertion that costs one string comparison is cheap
  # insurance against a future edit to this loop accidentally including
  # the newest entry.
  if [ "$NAME" = "$NEWEST_NAME" ]; then
    echo "retention: refusing to delete the newest backup (${NAME}) - aborting" >&2
    exit 1
  fi

  if [ "$DRY_RUN" = "true" ]; then
    echo "retention: [dry-run] would delete ${NAME}"
    continue
  fi

  echo "retention: deleting ${NAME}"
  if ! docker run --rm \
      --network "container:${MINIO_CID}" \
      -e MC_HOST_backup="$MC_HOST_backup" \
      minio/mc:latest rm "backup/${S3_BUCKET}/backups/${NAME}"; then
    # Fail closed mid-loop too: stop at the first failure rather than
    # pressing on and risking a confusing half-applied cleanup. Backups
    # already deleted in this run stay deleted (that's correct - they
    # were genuinely eligible); backups not yet reached are simply left
    # alone, the safe default.
    echo "retention: deletion failed for ${NAME} - aborting (${DELETED_COUNT} deleted so far this run, ${NAME} and any remaining eligible backups were NOT deleted)" >&2
    exit 1
  fi
  DELETED_COUNT=$((DELETED_COUNT + 1))
done

if [ "$DRY_RUN" = "true" ]; then
  echo "retention: dry-run complete - would have deleted ${DELETE_COUNT} backup(s), 0 actually deleted"
else
  echo "retention: SUCCESS - deleted ${DELETED_COUNT} backup(s), ${RETENTION_KEEP_COUNT} newest retained"
fi
