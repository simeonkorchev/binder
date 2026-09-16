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

## Suites and fakes

### store-suites-take-testdb-in-the-bootstrap-not-beforesuite
`testdb.New(t)` takes a `*testing.T` — it registers cleanup on it and calls
`t.Skipf` when no Postgres is reachable — so a Ginkgo store suite cannot call it
from `BeforeSuite`, where no `*testing.T` exists. Take the handle in the suite
bootstrap, **before** `RunSpecs`, into a package-level var:

```go
//nolint:gochecknoglobals // shared test DB handle for the suite (002 section 9).
var testDB *sqlx.DB

func TestStore(t *testing.T) {
	testDB = testdb.New(t)
	RegisterFailHandler(Fail)
	RunSpecs(t, "…")
}
```

The skip then skips the whole suite (`t.Skipf` calls `runtime.Goexit`, so
`RunSpecs` never runs), which is what you want on a machine with no database.
Each spec gets its own isolation from a `TRUNCATE <tables> CASCADE` in
`BeforeEach` — the database is already private to the suite, cloned from the
migrated template, so truncation is cheaper than a tx-per-spec and survives the
store opening its own transactions.
Evidence: `cmd/cardimport/store/store_suite_test.go` · since 2026-09-16 · verified 2026-09-16

### gochecknoglobals-fires-in-test-files-too
`.golangci.yml` excludes only `forcetypeassert`, `goconst` and `ireturn` from
`_test.go`, so **`gochecknoglobals` applies to specs**. The package-level
"static (inline init)" fixture block that `002` section 9 shows is therefore a
lint error here unless it is an `error` value (the linter exempts those, which
is why a package-level `errDB = errors.New(…)` passes and a
`fixtureBatch = model.Batch{…}` does not).

Declare spec fixtures **inside the `Describe` closure** instead — Ginkgo builds
the tree once, so `blueEyes := model.CardRow{…}` above the `var (…)` block of
dynamic vars behaves identically and needs no suppression. Reserve the
`//nolint:gochecknoglobals` for the suite-shared DB handle, where there is no
closure to hold it.
Evidence: `cmd/cardimport/store/store_test.go` · since 2026-09-16 · verified 2026-09-16

### ginkgo-run-does-fail-on-plain-go-tests
`make test` is `ginkgo run`, which reports specs — but it compiles the package's
whole test binary, so a plain `func TestX(t *testing.T)` in an
`<name>_internal_test.go` **does** run and **does** fail the gate
(`--- FAIL: TestObjectKey…` then `Test Suite Failed`), even while the spec
summary above it says `SUCCESS! -- 37 Passed`. Verified by breaking one on
purpose. The `002` section 9 licence to use plain `testing` for pure helpers and
mapper-completeness tables is therefore real coverage here, not a test that
silently never runs. Read the tail of the output, not the spec line.
Evidence: `cmd/cardimages/pipeline/objectkey_internal_test.go` · since 2026-09-16 · verified 2026-09-16

### no-pkg-eslog-here-demote-with-a-level-returning-method
`002` section 4a tells you to demote an expected error with
`eslog.LeveledErr(slog.LevelWarn, err)` and attach errors with `eslog.Error(err)`.
**`pkg/eslog` does not exist in this repository** — the rules are spotter's and
name a package that was not ported. Do not invent one for a single call site.

The shape in use instead: the closed set of failure reasons is a named type with
a `Level() slog.Level` method, and the one place that logs a per-item failure
calls `slog.LogAttrs(ctx, reason.Level(), …)`. One decision per reason, in one
place, testable as a table. A 404 from the image host is the only WARN; every
other reason is ERROR. If a second layer ever needs the same thing, that is the
moment to port `pkg/eslog`, not before.
Evidence: `cmd/cardimages/pipeline/failure.go` (`FailureReason.Level`) · since 2026-09-16 · verified 2026-09-16

## Store and data

### postgres-unique-is-not-deferrable-park-rows-to-reorder-positions
`UNIQUE (binder_id, position)` is checked **as each row version enters the
index**, not at the end of the statement, so there is no single UPDATE that
shifts a dense run of positions. Even

```sql
UPDATE binder_slots SET "position" = "position" + 1 WHERE "position" >= 3
```

fails with `duplicate key value violates unique constraint
"binder_slots_binder_position_key"`: the row going 3 -> 4 meets the row still at
4. SQL gives no way to order the updates, and for a move there is no order that
works anyway — every position in the window is occupied, so every intermediate
step collides. A constraint declared with `CONSTRAINT ... UNIQUE` is not
deferrable, so `SET CONSTRAINTS ... DEFERRED` is not available either.

Do it in two statements inside one transaction: **park** the affected rows above
`max(position)`, then bring them back at their final positions. The parked range
is disjoint from the occupied one by construction and every landing position was
vacated by the first statement, so no intermediate state holds two rows at one
position. A move is the same shape with a `CASE` that sends the mover to its
destination and closes the rest up behind it. `internal/binder/store/position.go`
is the worked version, and its specs go red if the parking is removed.
Evidence: `internal/binder/store/position.go` · since 2026-09-16 · verified 2026-09-16

### store-suites-must-connect-with-pgx-or-error-translation-is-never-exercised
`cmd/` connects with the **"pgx"** driver (`_ "github.com/jackc/pgx/v5/stdlib"`).
A suite that connects with lib/pq (`sqlx.Connect("postgres", …)`) therefore runs
a different driver than production, and a store that translates driver errors
into `internal/dataerror` types silently translates nothing: the translator
matches `*pgconn.PgError` and lib/pq returns `*pq.Error`, so `errors.As` misses
and the raw error passes straight through. `internal/binder/store` shipped that
way — its conflict and invalid-reference paths were unreachable until three
specs asserted them and failed.

`internal/testdb` connects with "pgx" for this reason. pgx's stdlib driver does
apply the multi-statement migration files (no bind parameters, so the simple
protocol is used); verified from a dropped `binder_test_template`. `pq` is still
imported there for `pq.QuoteIdentifier`, which is a pure string function.
Evidence: `internal/testdb/testdb.go` · since 2026-09-16 · verified 2026-09-16
