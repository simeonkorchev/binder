package store_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/testdb"
)

// testDB is the suite's own migrated database. testdb.New hands each suite a
// database of its own, so the truncation in BeforeEach cannot reach another
// suite's fixtures.
//
//nolint:gochecknoglobals // the suite's shared database handle, opened once in TestStore.
var testDB *sqlx.DB

func TestStore(t *testing.T) {
	// Opened before RunSpecs rather than in a BeforeSuite: testdb.New skips when
	// no database is reachable, and a skip has to happen on the test's own
	// goroutine to end the test rather than crash Ginkgo's.
	testDB = testdb.New(t)

	RegisterFailHandler(Fail)
	RunSpecs(t, "User Store Suite")
}

// truncateUsers empties the one table these specs write. Everything that
// references it cascades.
func truncateUsers() {
	_, err := testDB.Exec(`TRUNCATE users CASCADE`)
	Expect(err).NotTo(HaveOccurred())
}

// storedContact reads a row's contact columns straight out of the database,
// which is how a spec asserts what was actually written rather than what the
// store said it wrote.
func storedContact(id uuid.UUID) (email, phone *string) {
	var row struct {
		Email *string `db:"contact_email"`
		Phone *string `db:"contact_phone"`
	}
	dbErr := testDB.Get(&row, `SELECT contact_email, contact_phone FROM users WHERE id = $1`, id)
	Expect(dbErr).NotTo(HaveOccurred())
	return row.Email, row.Phone
}

// ptr is a pointer to a copy of v, for the optional columns.
func ptr[T any](v T) *T {
	return &v
}
