package store_test

import (
	"testing"

	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/testdb"
)

// testDB is the suite's own migrated database. It is taken before RunSpecs
// because testdb.New needs the *testing.T to register its cleanup and to skip
// the suite when no Postgres is reachable.
//
//nolint:gochecknoglobals // shared test DB handle for the suite (002-go-conventions.md section 9).
var testDB *sqlx.DB

func TestStore(t *testing.T) {
	testDB = testdb.New(t)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Card Images Store Suite")
}
