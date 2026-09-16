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
# Two ways to get a cluster, tried in that order:
#   1. the Debian-packaged cluster (pg_lsclusters shows `16 main`), started with
#      pg_ctlcluster — this is what the sandbox has;
#   2. a private cluster created with initdb and started with pg_ctl, for an
#      image that ships the binaries but no cluster.
#
# Idempotent: safe to run repeatedly. Requires root, because a Postgres server
# refuses to run as root and everything here is done as the postgres user.
set -euo pipefail

PG_VERSION="${PG_VERSION:-16}"
PG_BIN="${PG_BIN:-/usr/lib/postgresql/${PG_VERSION}/bin}"
DB_USER="${TEST_DB_USER:-binder}"
DB_PASSWORD="${TEST_DB_PASSWORD:-binder}"
DB_NAME="${TEST_DB_NAME:-binder_test}"
DB_PORT="${TEST_DB_PORT:-5432}"
STANDALONE_DATA="${TEST_DB_DATA:-/var/lib/postgresql/binder-${PG_VERSION}-test}"

log() { echo "[test-db-local] $*" >&2; }
as_postgres() { su postgres -c "$1"; }

use_debian_cluster() {
  command -v pg_lsclusters >/dev/null 2>&1 \
    && pg_lsclusters -h | awk '{print $1, $2}' | grep -q "^${PG_VERSION} main$"
}

if use_debian_cluster; then
  MODE=cluster
  PG_LOG="/var/log/postgresql/postgresql-${PG_VERSION}-main.log"
  DB_PORT="$(pg_lsclusters -h | awk -v v="${PG_VERSION}" '$1 == v && $2 == "main" { print $3 }')"
else
  MODE=standalone
  PG_LOG="${STANDALONE_DATA}.log"
fi

start_db() {
  case "$MODE" in
    cluster)
      pg_lsclusters -h | awk '{print $1, $2, $4}' | grep -q "^${PG_VERSION} main online" \
        || { log "starting postgresql ${PG_VERSION}/main"; pg_ctlcluster "${PG_VERSION}" main start >&2; } ;;
    standalone)
      if [ ! -s "${STANDALONE_DATA}/PG_VERSION" ]; then
        log "creating a private cluster at ${STANDALONE_DATA}"
        install -d -o postgres -g postgres "${STANDALONE_DATA}"
        as_postgres "${PG_BIN}/initdb -D '${STANDALONE_DATA}' -U postgres --auth-local=trust --auth-host=scram-sha-256" >&2
      fi
      as_postgres "${PG_BIN}/pg_ctl -D '${STANDALONE_DATA}' status" >/dev/null 2>&1 \
        || { log "starting the private cluster on port ${DB_PORT}"
             install -m 0644 -o postgres -g postgres /dev/null "${PG_LOG}"
             as_postgres "${PG_BIN}/pg_ctl -D '${STANDALONE_DATA}' -l '${PG_LOG}' -o '-p ${DB_PORT}' -w start" >&2; } ;;
  esac
}

restart_db() {
  case "$MODE" in
    cluster)    pg_ctlcluster "${PG_VERSION}" main restart >&2 ;;
    standalone) as_postgres "${PG_BIN}/pg_ctl -D '${STANDALONE_DATA}' -l '${PG_LOG}' -o '-p ${DB_PORT}' -w restart" >&2 ;;
  esac
}

psql_super() { as_postgres "psql -p ${DB_PORT} -v ON_ERROR_STOP=1 $1"; }

start_db

# Plan visibility for the /sql-optimize skill: auto_explain logs an
# EXPLAIN (ANALYZE, BUFFERS) plan for every statement the suites run into the
# cluster log, and pg_stat_statements counts calls per distinct statement (an
# N+1 shows up as one statement with calls >> specs). Preloading needs one
# restart; later runs find the setting in place and skip it.
want_preload="pg_stat_statements,auto_explain"
have_preload="$(psql_super "-qtAc \"SHOW shared_preload_libraries\"" | tr -d ' ')"
if [ "${have_preload}" != "${want_preload}" ]; then
  log "enabling ${want_preload} (restarting the cluster once)"
  # LOAD registers auto_explain's GUCs in this session so ALTER SYSTEM accepts
  # them before the restart. pg_stat_statements only registers its GUCs when
  # preloaded, so its tuning happens below, after the restart.
  as_postgres "psql -p ${DB_PORT} -v ON_ERROR_STOP=1 -q" >&2 <<SQL
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
psql_super "-q -o /dev/null -c \"ALTER SYSTEM SET pg_stat_statements.track = 'all';\" -c 'SELECT pg_reload_conf();'" >&2

# Superuser so the migrations' CREATE EXTENSION statements succeed — the card
# name rung of the match ladder needs pg_trgm (db/migrations/001_cards.sql).
psql_super "-qtAc \"SELECT 1 FROM pg_roles WHERE rolname = '${DB_USER}'\"" | grep -q 1 \
  || psql_super "-qc \"CREATE ROLE ${DB_USER} WITH LOGIN SUPERUSER PASSWORD '${DB_PASSWORD}';\""

psql_super "-qtAc \"SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'\"" | grep -q 1 \
  || psql_super "-qc \"CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};\""

psql_super "-qd ${DB_NAME} -c 'SET client_min_messages = warning;' -c 'CREATE EXTENSION IF NOT EXISTS pg_stat_statements;'" >&2

URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
PGPASSWORD="${DB_PASSWORD}" psql -h localhost -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -qtAc 'SELECT 1' >/dev/null
log "ready (${MODE}): ${URL}"
log "plans: ${PG_LOG} (auto_explain); call counts: SELECT calls, rows, query FROM pg_stat_statements"
echo "export TEST_DATABASE_URL='${URL}'"
echo "export TEST_DATABASE_LOG='${PG_LOG}'"
