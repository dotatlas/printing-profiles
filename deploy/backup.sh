#!/bin/sh
# Writes a pg_dump of the database to BACKUP_DIR, removes dumps older than 14 days, and copies the dump to OFFSITE.
# printing-profiles-backup.timer runs it every night; update.sh runs it with the label "pre-update" before a new image.
# shortcut: the off-site copy is not encrypted and old copies stay there, so use a machine you own over SSH.
set -eu
cd "$(dirname "$0")"
umask 077

dir=${BACKUP_DIR:-/var/backups/printing-profiles}
mkdir -p "$dir"
file="$dir/printing-$(date -u +%Y%m%dT%H%M%SZ)${1:+-$1}.dump"

# The dump goes to a temporary name first, so a failed dump never looks like a good one.
docker compose exec -T db sh -c 'pg_dump --format=custom -U "$POSTGRES_USER" "$POSTGRES_DB"' >"$file.part"
mv "$file.part" "$file"
find "$dir" -name 'printing-*.dump' -mtime +14 -delete

if [ -n "${OFFSITE:-}" ]; then
	rsync -a "$file" "$OFFSITE/"
fi
echo "backup: $file"
