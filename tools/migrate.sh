#!/usr/bin/env bash
# Applies db/migrations/*.sql in filename order, each exactly once.
#
# The migrations are forward-only and carry their own BEGIN/COMMIT. None of them
# uses IF NOT EXISTS on a table, so re-applying one is an error on purpose —
# which means "applied already" has to be recorded somewhere rather than
# inferred. That record is the schema_migrations table: a file is applied only
# if its name is not in there, and the name goes in inside the same psql session
# that ran it.
#
# There is no down direction and no runner binary to install: four files and a
# version table do not need golang-migrate, and psql is already a dependency of
# every other database step here.
#
# Usage: tools/migrate.sh <database-url> [migrations-dir]
#   tools/migrate.sh "$TEST_DATABASE_URL"
set -euo pipefail
cd "$(dirname "$0")/.."

url="${1:?usage: migrate.sh <database-url> [migrations-dir]}"
dir="${2:-db/migrations}"

log() { echo "[migrate] $*" >&2; }

psql -v ON_ERROR_STOP=1 -q -d "$url" -c "
  SET client_min_messages = warning;
  CREATE TABLE IF NOT EXISTS schema_migrations (
      version    text        PRIMARY KEY,
      applied_at timestamptz NOT NULL DEFAULT now()
  );"

applied=0
for file in "$dir"/*.sql; do
  [ -f "$file" ] || continue
  version="$(basename "$file")"
  if [ "$(psql -tAX -v ON_ERROR_STOP=1 -d "$url" \
            -c "SELECT count(*) FROM schema_migrations WHERE version = '${version}'")" != "0" ]; then
    continue
  fi
  log "applying ${version}"
  # -f then -c in one session: the file commits its own transaction, and the
  # row that marks it applied is the very next statement on the same connection.
  psql -v ON_ERROR_STOP=1 -q -d "$url" \
    -f "$file" \
    -c "INSERT INTO schema_migrations (version) VALUES ('${version}');"
  applied=$((applied + 1))
done

if [ "$applied" -eq 0 ]; then
  log "already up to date ($(basename "$dir"): $(ls -1 "$dir"/*.sql 2>/dev/null | wc -l | tr -d ' ') file(s))"
else
  log "applied ${applied} migration(s)"
fi
