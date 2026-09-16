# Go backend memory

## Gate and toolchain

### go-list-dot-dot-dot-picks-up-node-modules
`go list ./...` at the repo root matches Go packages **inside `node_modules/`** —
`node_modules/flatted/golang/pkg/flatted` is one, and it is enough to make
`golangci-lint run ./...` and `ginkgo run ./...` lint and test third-party code.
Go only skips directories named `testdata` or starting with `.`/`_`, so
`node_modules` is not excluded for us.

Address the first-party tree explicitly instead: the Makefile's `GO_ROOTS` is
`./internal/... ./pkg/... ./cmd/...` and every Go lane uses it. Add a new
top-level Go directory to `GO_ROOTS` and to `tools/changed-go-pkgs.sh`, or it is
silently outside the gate.
Evidence: `Makefile` (GO_ROOTS) · since 2026-09-16 · verified 2026-09-16

### empty-go-tree-fails-the-linter-and-ginkgo
With no Go package in the tree at all, `golangci-lint` exits 5
(`no go files to analyze`) and `ginkgo run` exits 1 (`Found no test suites`) —
both treat "nothing to do" as an error, so the gate would be red on a tree with
nothing wrong with it. `make lint` / `make test` guard on `go list $(GO_ROOTS)`
being empty and say so out loud. The guard stops mattering by itself the moment
the first package lands; do not remove it by hand while `internal/`, `pkg/` and
`cmd/` are still skeletons.
Evidence: `Makefile` (GO_SKIP_IF_EMPTY) · since 2026-09-16 · verified 2026-09-16

### golangci-lint-install-script-is-blocked-use-go-install
golangci-lint's own installer (`curl … raw.githubusercontent.com/golangci/…/install.sh`)
is answered with **403 by the egress proxy** in this project's containers.
`proxy.golang.org` is reachable, so `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(cat .golangci-version)`
is the one path that works both on a laptop and in an agent container. That is
what `make install-go-tools` does, and CI calls the same target so the pinned
version has one source.

The image also ships an **older golangci-lint in `/usr/local/bin`** (2.5.0). It
shadows the pinned one unless the Go bin dir comes **first**: run
`export PATH="$(go env GOPATH)/bin:$PATH"` in the same shell as the gate.
`make check-golangci-version` fails loudly rather than linting with the wrong one.
Evidence: `Makefile` (install-go-tools, check-golangci-version) · since 2026-09-16 · verified 2026-09-16

### migrations-are-applied-by-tools-migrate-sh
`db/migrations/*.sql` are forward-only, carry their own `BEGIN`/`COMMIT` and use
no `IF NOT EXISTS` on tables, so re-running one is an error on purpose.
`tools/migrate.sh <url>` applies them in filename order and records each in a
`schema_migrations` table, so "already applied" is recorded rather than guessed.
`make migrate` (dev, `DB_URL`) and `make migrate-test` (boots a database if
`TEST_DATABASE_URL` is unset) are the entry points; `migrate-test` is part of
`gate-go` because CI runs it before the suites.
Evidence: `tools/migrate.sh` · since 2026-09-16 · verified 2026-09-16
