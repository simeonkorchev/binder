// Package testdb hands a store suite its own Postgres database.
//
// Suites run concurrently — several packages here, and several agents against
// one cluster — and a store spec asserts over whole tables, so sharing one
// database would make one suite's fixtures another suite's phantom rows. Each
// call to New therefore gets a database of its own, cloned from a migrated
// template so the migrations run once per cluster rather than once per suite.
package testdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	// databaseURLEnv is what tools/test-db-local.sh prints and what the
	// Makefile exports before any Go test lane runs.
	databaseURLEnv = "TEST_DATABASE_URL"
	// templateDatabase is created once per cluster and never connected to
	// again: CREATE DATABASE ... TEMPLATE is refused while anyone is attached.
	templateDatabase = "binder_test_template"
	// templateLockKey serialises template creation between suites racing to be
	// the first to need one. Any constant does; this one is arbitrary.
	templateLockKey = 8264113
	// migrationsDir is relative to the repository root, which findMigrations
	// walks up to find: a test's working directory is its own package.
	migrationsDir = "db/migrations"
)

// errNoMigrations means the repository's migrations could not be found, which
// is a broken checkout rather than a database problem.
var errNoMigrations = errors.New("migrations not found")

// New returns a connection to an isolated, fully-migrated test database.
// It registers cleanup on t. Skips the test if no database is reachable.
func New(t *testing.T) *sqlx.DB {
	t.Helper()

	adminURL := os.Getenv(databaseURLEnv)
	if adminURL == "" {
		t.Skipf("%s is unset: run `eval \"$(tools/test-db-local.sh)\"`, or `make test` which does it", databaseURLEnv)
	}

	admin, err := sqlx.Connect("postgres", adminURL)
	if err != nil {
		t.Skipf("no database at %s: %v", databaseURLEnv, err)
	}
	defer func() { _ = admin.Close() }()

	if err := ensureTemplate(admin, adminURL); err != nil {
		t.Fatalf("preparing the template database: %v", err)
	}

	name := "binder_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := exec(admin, `CREATE DATABASE %s TEMPLATE %s`, name, templateDatabase); err != nil {
		t.Fatalf("creating the test database: %v", err)
	}
	t.Cleanup(func() { dropDatabase(t, adminURL, name) })

	url, err := withDatabase(adminURL, name)
	if err != nil {
		t.Fatalf("addressing the test database: %v", err)
	}
	db, err := sqlx.Connect("postgres", url)
	if err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// exec runs a DDL statement whose only variable parts are database names.
// Identifiers cannot be bound as parameters, so they are quoted instead — and
// every name reaching here is either a constant above or a uuid this package
// minted, never anything a request or a row supplied (004-security.md).
func exec(db *sqlx.DB, statement string, names ...string) error {
	quoted := make([]any, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, pq.QuoteIdentifier(name))
	}
	//nolint:gosec // DDL identifiers, quoted above; CREATE DATABASE cannot take a bind parameter.
	if _, err := db.ExecContext(context.Background(), fmt.Sprintf(statement, quoted...)); err != nil {
		return fmt.Errorf("running %q: %w", statement, err)
	}
	return nil
}

func dropDatabase(t *testing.T, adminURL string, name string) {
	t.Helper()

	admin, err := sqlx.Connect("postgres", adminURL)
	if err != nil {
		t.Logf("dropping the test database %s: %v", name, err)
		return
	}
	defer func() { _ = admin.Close() }()

	// WITH (FORCE) rather than a plain DROP: a spec that leaked a connection
	// would otherwise leave its database behind for the rest of the cluster's
	// life, and that leak is the spec's bug to fix, not a reason to hoard
	// databases.
	if err := exec(admin, `DROP DATABASE IF EXISTS %s WITH (FORCE)`, name); err != nil {
		t.Logf("dropping the test database %s: %v", name, err)
	}
}

// ensureTemplate migrates the template database once per cluster. The advisory
// lock is what makes two suites starting in the same second safe: the loser
// waits, then finds the template already there.
func ensureTemplate(admin *sqlx.DB, adminURL string) error {
	ctx := context.Background()

	conn, err := admin.Connx(ctx)
	if err != nil {
		return fmt.Errorf("taking a connection for the template lock: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, templateLockKey); err != nil {
		return fmt.Errorf("locking template creation: %w", err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, templateLockKey) }()

	var exists bool
	err = conn.QueryRowxContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, templateDatabase).Scan(&exists)
	if err != nil {
		return fmt.Errorf("looking for the template database: %w", err)
	}
	if exists {
		return nil
	}
	return buildTemplate(admin, adminURL)
}

// buildTemplate migrates a scratch database and only then renames it into
// place, so a migration that fails halfway leaves no half-built template for
// the next suite to clone.
func buildTemplate(admin *sqlx.DB, adminURL string) error {
	scratch := templateDatabase + "_building"
	if err := exec(admin, `DROP DATABASE IF EXISTS %s WITH (FORCE)`, scratch); err != nil {
		return err
	}
	if err := exec(admin, `CREATE DATABASE %s`, scratch); err != nil {
		return err
	}

	url, err := withDatabase(adminURL, scratch)
	if err != nil {
		return err
	}
	if err := migrate(url); err != nil {
		return err
	}
	return exec(admin, `ALTER DATABASE %s RENAME TO %s`, scratch, templateDatabase)
}

// withDatabase points a connection URL at a different database on the same
// cluster, keeping its credentials and options.
func withDatabase(rawURL string, name string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", databaseURLEnv, err)
	}
	parsed.Path = "/" + name
	return parsed.String(), nil
}

// migrate applies db/migrations in filename order. Each file carries its own
// BEGIN/COMMIT and several statements, which lib/pq sends as one simple query
// because there are no parameters to bind. There is no version table here on
// purpose: the database being migrated is brand new every time, so "has this
// already been applied" cannot arise (tools/migrate.sh is the answer for a
// database that outlives a run).
func migrate(databaseURL string) error {
	dir, err := findMigrations()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return fmt.Errorf("listing %s: %w", dir, err)
	}
	if len(files) == 0 {
		return fmt.Errorf("%w: %s holds no .sql file", errNoMigrations, dir)
	}
	sort.Strings(files)

	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return fmt.Errorf("connecting to the template database: %w", err)
	}
	defer func() { _ = db.Close() }()

	for _, file := range files {
		statements, err := os.ReadFile(file) //nolint:gosec // path from Glob over this repository's own db/migrations.
		if err != nil {
			return fmt.Errorf("reading %s: %w", file, err)
		}
		if _, err := db.ExecContext(context.Background(), string(statements)); err != nil {
			return fmt.Errorf("applying %s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

// findMigrations walks up from the working directory — which under `go test` is
// the package's own directory — until it finds the repository's migrations.
func findMigrations() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating the working directory: %w", err)
	}
	for {
		candidate := filepath.Join(dir, migrationsDir)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w: no %s above the working directory", errNoMigrations, migrationsDir)
		}
		dir = parent
	}
}
