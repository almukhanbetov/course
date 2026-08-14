# Backup & restore runbook

Operational reference for the Stage 30 backup mechanism (`deploy/backup/run-backup.sh`, `deploy/backup/retention.sh`, `.github/workflows/backup.yml`). Every command below was actually run, in this exact order, during Stage 30A2 (backup + restore) and Stage 30A3 (retention) — this is not a theoretical procedure, it reflects commands proven to work.

Run everything in this document **from `$DEPLOY_DIR`** (`/opt/lms` on the real VPS; the project root when following this runbook locally with `DEPLOY_DIR` overridden — see each stage's own progress doc for how). All commands assume the standard compose stack (`postgres`, `minio`) is up.

**Never run the restore section (steps 3 onward) against the running `postgres` service.** Every restore in this runbook targets a brand-new, separate container. If you ever find yourself pointing `pg_restore` at the compose stack's own `postgres` container, stop — that is not what this runbook does anywhere.

## 1. Manual backup

Trigger a backup on demand (outside the 03:00 UTC schedule) — e.g. before a risky migration, or to test the mechanism itself:

```bash
cd "$DEPLOY_DIR"        # /opt/lms on the real VPS
bash backup/run-backup.sh
```

Requires `.env` in `$DEPLOY_DIR` to already have `POSTGRES_DB`, `S3_BUCKET`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, and `BACKUP_ENCRYPTION_PASSPHRASE` set (see `.env.production.example`). The script fails closed before touching Postgres or MinIO if any are missing.

On success, the last line names the object it created, e.g.:

```
backup: SUCCESS - backups/lms-backup-20260814T134238Z.dump.enc (123952 bytes)
```

immediately followed by the retention step (§6) running automatically. From the GitHub Actions side, the same thing happens via **Actions → Production Database Backup → Run workflow** (`workflow_dispatch`), which SSHes in and runs the exact command above.

## 2. Verify a backup exists

Independent of the script's own success message — confirm the object is really in object storage, from a separate command:

```bash
# Not `. <(...)` (bash source) - this project's real .env has unquoted
# values containing spaces (e.g. SMTP_FROM_NAME=LMS Platform), which is
# valid docker-compose .env syntax but not valid bash syntax to source
# directly (see run-backup.sh/retention.sh, which hit this exact bug in
# Stage 30A2/30A3 and use this same line-by-line parser instead).
while IFS='=' read -r key value; do
  case "$key" in ''|'#'*) continue ;; esac
  export "$key=$value"
done < <(grep -v '^\s*#' .env | grep '=')
MINIO_CID="$(docker compose ps -q minio)"

docker run --rm --network "container:${MINIO_CID}" \
  -e MC_HOST_backup="http://${S3_ACCESS_KEY}:${S3_SECRET_KEY}@localhost:9000" \
  minio/mc:latest stat --json "backup/${S3_BUCKET}/backups/<OBJECT_NAME>"
```

A healthy result reports `"status":"success"` and a non-zero `"size"`. To list every backup currently retained:

```bash
docker run --rm --network "container:${MINIO_CID}" \
  -e MC_HOST_backup="http://${S3_ACCESS_KEY}:${S3_SECRET_KEY}@localhost:9000" \
  minio/mc:latest ls "backup/${S3_BUCKET}/backups/"
```

## 3. Download and decrypt

```bash
docker run --rm --network "container:${MINIO_CID}" \
  -e MC_HOST_backup="http://${S3_ACCESS_KEY}:${S3_SECRET_KEY}@localhost:9000" \
  -v "$(pwd):/out" \
  minio/mc:latest cp "backup/${S3_BUCKET}/backups/<OBJECT_NAME>" /out/downloaded.dump.enc

openssl enc -d -aes-256-cbc -pbkdf2 -iter 100000 \
  -pass env:BACKUP_ENCRYPTION_PASSPHRASE \
  -in downloaded.dump.enc -out restored.dump
```

`file restored.dump` should report `PostgreSQL custom database dump`. If decryption fails outright (`bad decrypt`), the passphrase is wrong — see §7. If it "succeeds" but the file isn't a valid dump, the artifact itself may be corrupted — see §8; either way, **do not proceed to restore an artifact that fails either check.**

## 4. Restore into an isolated PostgreSQL instance (never production)

Always a brand-new container, never the running `postgres` service:

```bash
docker run -d --name restore-verify \
  -e POSTGRES_USER=restoretest \
  -e POSTGRES_PASSWORD=restoretest_pw \
  -e POSTGRES_DB=restoretest \
  postgres:17-alpine

until docker exec restore-verify pg_isready -U restoretest -d restoretest >/dev/null 2>&1; do sleep 1; done

docker cp restored.dump restore-verify:/tmp/restored.dump
docker exec -e PGPASSWORD=restoretest_pw restore-verify \
  pg_restore -U restoretest -d restoretest --no-owner --no-privileges /tmp/restored.dump
```

Confirm it ran cleanly: `echo $?` should be `0`. `restore-verify` is deliberately not attached to the application's own compose network (it's a standalone `docker run`, default bridge network) and uses its own fresh volume — it cannot reach, and is not reachable from, the real `postgres` service.

## 5. Integrity verification

Compare schema and row counts against the live source:

```bash
# Table list, both sides
docker compose exec -T postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A \
  -c "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;" > source_tables.txt
docker exec -e PGPASSWORD=restoretest_pw restore-verify psql -U restoretest -d restoretest -t -A \
  -c "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;" > restored_tables.txt
diff source_tables.txt restored_tables.txt && echo "schema: IDENTICAL"

# Row counts, per table (edit the table list to match the current schema)
# — build one SELECT ... UNION ALL ... per table, run it against both
# databases, diff the two outputs. See STAGE30_PROGRESS.md's 30A2 section
# for the exact query used the last time this was run in full.
```

Also confirm the restored database is actually usable, not just populated: run one real join query, confirm a foreign-key violation is correctly rejected, and confirm a write/rollback round-trip works.

## 6. Retention (automatic, and manual/dry-run)

`run-backup.sh` calls `retention.sh` automatically after every successful backup — no separate step needed for the normal path. To run it by hand (e.g. to check what a policy change would do before it runs for real):

```bash
cd "$DEPLOY_DIR"

# Dry run — reports what would be deleted, deletes nothing:
DRY_RUN=true bash backup/retention.sh

# Real run, with a non-default keep count:
RETENTION_KEEP_COUNT=7 bash backup/retention.sh
```

Default policy: keep the newest 14 daily backups (`RETENTION_KEEP_COUNT`, override in `.env` if needed), delete anything older, under `backups/` only, matching the `lms-backup-<timestamp>.dump.enc` naming pattern only. The newest backup is never a deletion candidate by construction. A listing or deletion failure aborts before deleting anything further and exits non-zero (turns the Actions run red) — it never treats "couldn't list" as "nothing to keep."

## 7. If decryption fails (wrong/missing passphrase)

`openssl enc -d ...` exits non-zero with `bad decrypt` immediately. This is expected, safe behavior — it means either the wrong `BACKUP_ENCRYPTION_PASSPHRASE` was used, or the artifact is not a valid encrypted backup at all. Do not retry with a guessed passphrase; confirm the correct value (durable copy outside the VPS — see `.env.production.example`) before trying again. **Never** proceed to `pg_restore` on output from a decryption that reported an error.

## 8. If the backup artifact is corrupted

A corrupted artifact may or may not fail at the `openssl enc -d` step itself (AES-CBC corruption is local to the affected block, so decryption can "succeed" while producing a broken dump). Always verify the *decrypted* output before restoring:

```bash
file restored.dump   # must say "PostgreSQL custom database dump"
docker exec -e PGPASSWORD=restoretest_pw restore-verify \
  pg_restore -U restoretest -d restoretest -l /tmp/restored.dump   # list-only, no data touched
```

`pg_restore -l` on a genuinely corrupted archive fails cleanly (e.g. `unexpected data offset flag`, non-zero exit) before anything is written. If either check fails, treat that backup as unusable and restore from the next-newest one instead — this is exactly why retention (§6) keeps more than one.

## 9. Cleanup

After any manual restore/verification exercise:

```bash
docker rm -f restore-verify
rm -f downloaded.dump.enc restored.dump source_tables.txt restored_tables.txt
```

Never delete the actual backup object in MinIO as part of a restore exercise — that's retention's (§6) job on its own schedule, not something to do by hand while verifying a restore.
