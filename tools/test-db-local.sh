#!/usr/bin/env bash
# Boots a throwaway Postgres for the Go store suites WITHOUT Docker, and prints
# the export lines to eval:
#
#   eval "$(tools/test-db-local.sh)" && [ -n "$TEST_DATABASE_URL" ]
#   make test
#
# docker-compose.yml is the right thing on a developer machine. It is not usable
# in this project's agent containers: the daemon runs, but the registry blob
# fetch is refused at the egress proxy (production.cloudfront.docker.com: 403),
# so `docker pull postgres:16` cannot succeed. Everything below therefore uses
# the postgresql-16 packages that ship in the image.
#
# Three ways to get a cluster, tried in that order:
#   1. a server already listening on the port — a developer's own Postgres,
#      which on macOS/Homebrew is the normal case (`brew services start
#      postgresql@16`) and runs as the logged-in user, not as `postgres`;
#   2. the Debian-packaged cluster (pg_lsclusters shows `16 main`), started with
#      pg_ctlcluster — this is what the project's Linux containers have;
#   3. a private cluster created with initdb and started with pg_ctl.
#
# Idempotent: safe to run repeatedly. It does NOT require root: it drops to the
# `postgres` user only when it is root and that user exists, because a server
# refuses to run as root. On macOS it runs as you throughout.
set -euo pipefail

PG_VERSION="${PG_VERSION:-16}"
# Debian keeps the binaries off PATH under /usr/lib; Homebrew keeps them in a
# versioned opt prefix. pg_config knows either when it is on PATH.
find_pg_bin() {
  if [ -n "${PG_BIN:-}" ]; then echo "${PG_BIN}"; return; fi
  if command -v pg_config >/dev/null 2>&1; then pg_config --bindir; return; fi
  for candidate in \
    "/usr/lib/postgresql/${PG_VERSION}/bin" \
    "/opt/homebrew/opt/postgresql@${PG_VERSION}/bin" \
    "/usr/local/opt/postgresql@${PG_VERSION}/bin"; do
    [ -x "${candidate}/initdb" ] && { echo "${candidate}"; return; }
  done
  echo ""
}
PG_BIN="$(find_pg_bin)"
DB_USER="${TEST_DB_USER:-binder}"
DB_PASSWORD="${TEST_DB_PASSWORD:-binder}"
DB_NAME="${TEST_DB_NAME:-binder_test}"
DB_PORT="${TEST_DB_PORT:-5432}"
# A private cluster's data directory must be writable by whoever runs the
# server: postgres's home as root, our own otherwise.
if [ "$(id -u)" = "0" ] && id postgres >/dev/null 2>&1; then
  STANDALONE_DATA="${TEST_DB_DATA:-/var/lib/postgresql/binder-${PG_VERSION}-test}"
else
  STANDALONE_DATA="${TEST_DB_DATA:-${HOME}/.local/share/binder/pg-${PG_VERSION}-test}"
fi

log() { echo "[test-db-local] $*" >&2; }
die() { log "$*"; exit 1; }

# Drop to `postgres` only when we are root and it exists: a server refuses to
# run as root, and on macOS there is no postgres user or group at all — which
# is what makes `install -g postgres` fail with "unknown group postgres".
if [ "$(id -u)" = "0" ] && id postgres >/dev/null 2>&1; then
  as_postgres() { su postgres -c "$1"; }
  install_owned() { install -d -o postgres -g postgres "$1"; }
  install_log()   { install -m 0644 -o postgres -g postgres /dev/null "$1"; }
else
  as_postgres() { bash -c "$1"; }
  install_owned() { mkdir -p "$1"; }
  install_log()   { : > "$1"; }
fi

# Can we reach a Postgres on DB_PORT without being asked for anything? -w
# forbids the password prompt, which would hang this check for ever, and
# PGCONNECT_TIMEOUT bounds a port that accepts but never answers.
can_reach_port() {
  command -v psql >/dev/null 2>&1 || return 1
  PGCONNECT_TIMEOUT=3 psql -w -h localhost -p "${DB_PORT}" -d postgres \
    -qtAc 'SELECT 1' >/dev/null 2>&1
}

use_debian_cluster() {
  command -v pg_lsclusters >/dev/null 2>&1 \
    && pg_lsclusters -h | awk '{print $1, $2}' | grep -q "^${PG_VERSION} main$"
}

if can_reach_port; then
  # Somebody else's server, already up: preferred over booting a second one
  # beside it, and the normal case on a developer machine.
  MODE=existing
  PG_LOG=""
elif use_debian_cluster; then
  MODE=cluster
  PG_LOG="/var/log/postgresql/postgresql-${PG_VERSION}-main.log"
  DB_PORT="$(pg_lsclusters -h | awk -v v="${PG_VERSION}" '$1 == v && $2 == "main" { print $3 }')"
else
  MODE=standalone
  PG_LOG="${STANDALONE_DATA}.log"
fi

start_db() {
  case "$MODE" in
    existing) : ;; # ours to use, not to manage
    cluster)
      pg_lsclusters -h | awk '{print $1, $2, $4}' | grep -q "^${PG_VERSION} main online" \
        || { log "starting postgresql ${PG_VERSION}/main"; pg_ctlcluster "${PG_VERSION}" main start >&2; } ;;
    standalone)
      [ -n "${PG_BIN}" ] || die "no Postgres ${PG_VERSION} binaries found. Install them (macOS: brew install postgresql@${PG_VERSION}) or set PG_BIN."
      if [ ! -s "${STANDALONE_DATA}/PG_VERSION" ]; then
        log "creating a private cluster at ${STANDALONE_DATA}"
        install_owned "${STANDALONE_DATA}"
        as_postgres "${PG_BIN}/initdb -D '${STANDALONE_DATA}' -U postgres --auth-local=trust --auth-host=scram-sha-256" >&2
      fi
      as_postgres "${PG_BIN}/pg_ctl -D '${STANDALONE_DATA}' status" >/dev/null 2>&1 \
        || { log "starting the private cluster on port ${DB_PORT}"
             install_log "${PG_LOG}"
             as_postgres "${PG_BIN}/pg_ctl -D '${STANDALONE_DATA}' -l '${PG_LOG}' -o '-p ${DB_PORT} -k ${STANDALONE_DATA}' -w start" >&2; } ;;
  esac
}

restart_db() {
  case "$MODE" in
    existing)
      die "this wants a restart, which is not ours to do on a server we did not start.
  Restart it yourself (macOS: brew services restart postgresql@${PG_VERSION}), or use a
  private cluster on another port: TEST_DB_PORT=5433" ;;
    cluster)    pg_ctlcluster "${PG_VERSION}" main restart >&2 ;;
    standalone) as_postgres "${PG_BIN}/pg_ctl -D '${STANDALONE_DATA}' -l '${PG_LOG}' -o '-p ${DB_PORT} -k ${STANDALONE_DATA}' -w restart" >&2 ;;
  esac
}

# On a server we did not start we are already whoever owns it, so connect over
# TCP rather than becoming a user that may not exist.
if [ "$MODE" = "existing" ]; then
  psql_super() { bash -c "psql -w -h localhost -p ${DB_PORT} -d postgres -v ON_ERROR_STOP=1 $1"; }
elif [ "$MODE" = "standalone" ]; then
  # -h is the socket directory the cluster was started with; -U is the role
  # initdb created, which is not the OS user running this.
  STANDALONE_CONN="-h '${STANDALONE_DATA}' -U postgres"
  psql_super() { as_postgres "psql ${STANDALONE_CONN} -p ${DB_PORT} -v ON_ERROR_STOP=1 $1"; }
else
  psql_super() { as_postgres "psql -p ${DB_PORT} -v ON_ERROR_STOP=1 $1"; }
fi

start_db

# Plan visibility for the /sql-optimize skill: auto_explain logs an
# EXPLAIN (ANALYZE, BUFFERS) plan for every statement the suites run into the
# cluster log, and pg_stat_statements counts calls per distinct statement (an
# N+1 shows up as one statement with calls >> specs). Preloading needs one
# restart; later runs find the setting in place and skip it.
want_preload="pg_stat_statements,auto_explain"
have_preload="$(psql_super "-qtAc \"SHOW shared_preload_libraries\"" | tr -d ' ')"
if [ "${have_preload}" != "${want_preload}" ] && [ "$MODE" = "existing" ]; then
  # Plan visibility is a convenience for /sql-optimize, not something the store
  # suites need. Do not reconfigure a developer's own Postgres behind their back.
  log "note: ${want_preload} not preloaded here, so auto_explain plans are unavailable."
  log "      Everything else works. To enable, add to your postgresql.conf:"
  log "        shared_preload_libraries = 'pg_stat_statements,auto_explain'"
elif [ "${have_preload}" != "${want_preload}" ]; then
  log "enabling ${want_preload} (restarting the cluster once)"
  # LOAD registers auto_explain's GUCs in this session so ALTER SYSTEM accepts
  # them before the restart. pg_stat_statements only registers its GUCs when
  # preloaded, so its tuning happens below, after the restart.
  as_postgres "psql ${STANDALONE_CONN:-} -p ${DB_PORT} -v ON_ERROR_STOP=1 -q" >&2 <<SQL
LOAD 'auto_explain';
ALTER SYSTEM SET shared_preload_libraries = pg_stat_statements, auto_explain;
ALTER SYSTEM SET auto_explain.log_min_duration = 0;
ALTER SYSTEM SET auto_explain.log_analyze = on;
ALTER SYSTEM SET auto_explain.log_buffers = on;
ALTER SYSTEM SET auto_explain.log_nested_statements = on;
ALTER SYSTEM SET auto_explain.log_format = 'text';
SQL
  restart_db
fi
# Reloadable, so it applies without another restart; idempotent on later runs.
if [ "$MODE" != "existing" ]; then
  psql_super "-q -o /dev/null -c \"ALTER SYSTEM SET pg_stat_statements.track = 'all';\" -c 'SELECT pg_reload_conf();'" >&2
fi

# Superuser so the migrations' CREATE EXTENSION statements succeed — the card
# name rung of the match ladder needs pg_trgm (db/migrations/001_cards.sql).
psql_super "-qtAc \"SELECT 1 FROM pg_roles WHERE rolname = '${DB_USER}'\"" | grep -q 1 \
  || psql_super "-qc \"CREATE ROLE ${DB_USER} WITH LOGIN SUPERUSER PASSWORD '${DB_PASSWORD}';\""

psql_super "-qtAc \"SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'\"" | grep -q 1 \
  || psql_super "-qc \"CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};\""

if [ "$MODE" != "existing" ]; then
  psql_super "-qd ${DB_NAME} -c 'SET client_min_messages = warning;' -c 'CREATE EXTENSION IF NOT EXISTS pg_stat_statements;'" >&2
fi

URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
PGPASSWORD="${DB_PASSWORD}" PGCONNECT_TIMEOUT=5 \
  psql -w -h localhost -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -qtAc 'SELECT 1' >/dev/null
log "ready (${MODE}): ${URL}"
[ -n "${PG_LOG}" ] && log "plans: ${PG_LOG} (auto_explain); call counts: SELECT calls, rows, query FROM pg_stat_statements"
echo "export TEST_DATABASE_URL='${URL}'"
[ -n "${PG_LOG}" ] && echo "export TEST_DATABASE_LOG='${PG_LOG}'"
exit 0
