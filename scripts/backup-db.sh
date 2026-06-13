#!/usr/bin/env bash
#
# backup-db.sh — dump the platform database, upload it to MinIO, and verify the
# backup restores cleanly into a throwaway database.
#
# Usage: scripts/backup-db.sh
# Requires the local stack up (`make up`). Schedule via cron for regular backups.
# Production upgrade path: continuous WAL archiving / PITR.
set -euo pipefail
export MSYS_NO_PATHCONV=1

cd "$(dirname "$0")/.."
MOUNT="$(pwd -W 2>/dev/null || pwd)"
COMPOSE="docker compose -f deploy/docker-compose.yml"
NET="vpn-platform_default"
MUSER="${MINIO_ROOT_USER:-minioadmin}"
MPASS="${MINIO_ROOT_PASSWORD:-minioadmin}"

TS="$(date +%Y%m%d-%H%M%S)"
FILE="vpn-${TS}.sql.gz"
mkdir -p backups

echo ">> dumping database…"
$COMPOSE exec -T postgres pg_dump -U vpn -d vpn | gzip > "backups/${FILE}"
echo "   dump: backups/${FILE} ($(wc -c < "backups/${FILE}") bytes)"

echo ">> uploading to MinIO (bucket: backups)…"
docker run --rm --network "$NET" -v "${MOUNT}/backups:/backups:ro" --entrypoint sh minio/mc:latest -c "
  mc alias set m http://minio:9000 '${MUSER}' '${MPASS}' >/dev/null
  mc mb -p m/backups >/dev/null 2>&1 || true
  mc cp /backups/${FILE} m/backups/${FILE}
"

echo ">> verifying restore into a throwaway database…"
$COMPOSE exec -T postgres sh -c "dropdb -U vpn --if-exists vpn_restore_check >/dev/null 2>&1; createdb -U vpn vpn_restore_check"
gunzip -c "backups/${FILE}" | $COMPOSE exec -T postgres psql -q -U vpn -d vpn_restore_check >/dev/null
ROWS="$($COMPOSE exec -T postgres psql -U vpn -d vpn_restore_check -tAc "SELECT count(*) FROM users")"
$COMPOSE exec -T postgres dropdb -U vpn vpn_restore_check
echo "   restore OK — users rows in restored copy: ${ROWS}"

echo "✅ backup uploaded to MinIO (backups/${FILE}) and restore verified."
