# Infrastructure and CI

## Databases in a container

### docker-image-pulls-are-blocked-postgres-comes-from-the-image
`docker compose up postgres` cannot work in this project's agent containers: the
daemon runs, but the registry blob fetch is refused at the egress proxy
(`production.cloudfront.docker.com: Forbidden`). `docker-compose.yml` is still
correct for a developer machine and for CI's service container — just not here.

`tools/test-db-local.sh` is the path that works, and it needs no Docker. It
takes the Debian-packaged cluster when `pg_lsclusters` reports `16 main`, and
otherwise creates a private one with `initdb` + `pg_ctl` under
`/var/lib/postgresql/binder-16-test`. Both run as the `postgres` user (a server
refuses to start as root) and both end by printing `export TEST_DATABASE_URL=…`
to `eval`. `make test`, `make test-fast`, `make test-changed` and
`make migrate-test` call it automatically when that variable is unset.
Evidence: `tools/test-db-local.sh` · since 2026-09-16 · verified 2026-09-16

## CI

### ci-and-the-local-gate-share-one-classifier
`.github/workflows/ci.yml`'s `changes` job and `tools/changed-scopes.sh` hold the
**same two regexes** (`go_re`, `shared_re`), and `tools/check-ci-parity.sh` fails
the gate if they stop being byte-identical, if a scope the classifier emits has
no CI job gating on it, or if CI runs a command with no recorded local
equivalent. Editing one side alone is the failure it exists to catch: fix both,
and add the new CI step to the `known` map in the parity script.
Evidence: `tools/check-ci-parity.sh` · since 2026-09-16 · verified 2026-09-16
