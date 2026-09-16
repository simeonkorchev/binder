package store_test

import (
	"encoding/binary"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/testdb"
)

// testDB is the suite's own migrated database. testdb.New hands each suite a
// database of its own, so the truncation in AfterEach cannot reach another
// suite's fixtures.
//
//nolint:gochecknoglobals // the suite's shared database handle, opened once in TestStore.
var testDB *sqlx.DB

func TestStore(t *testing.T) {
	// Opened before RunSpecs rather than in a BeforeSuite: testdb.New skips
	// when no database is reachable, and a skip has to happen on the test's own
	// goroutine to end the test rather than crash Ginkgo's.
	testDB = testdb.New(t)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Card Store Suite")
}

// truncateCards empties every table the card suite writes. card_printings goes
// with cards (ON DELETE CASCADE) and holds the only reference to card_sets.
func truncateCards() {
	_, err := testDB.Exec(`TRUNCATE cards, card_sets CASCADE`)
	Expect(err).NotTo(HaveOccurred())
}

// seedSet inserts the reference row a printing's prefix has to point at.
func seedSet(code, name string) {
	_, err := testDB.Exec(`INSERT INTO card_sets (code, name) VALUES ($1, $2)`, code, name)
	Expect(err).NotTo(HaveOccurred())
}

// seedCard inserts one card and returns its id. ygoprodeck_id is UNIQUE, so it
// is derived from the row's own uuid — shifted to stay positive — rather than
// leaving every fixture to invent a distinct number.
func seedCard(name string, imageObjectKey *string) uuid.UUID {
	id := uuid.New()
	ygoprodeckID := int64(binary.BigEndian.Uint64(id[:8]) >> 1)

	_, err := testDB.Exec(
		`INSERT INTO cards (id, ygoprodeck_id, name, image_object_key) VALUES ($1, $2, $3, $4)`,
		id, ygoprodeckID, name, imageObjectKey)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedPrinting inserts one printing of a card and returns its id.
func seedPrinting(cardID uuid.UUID, setCode, rarity string) uuid.UUID {
	id := uuid.New()
	_, err := testDB.Exec(
		`INSERT INTO card_printings (id, card_id, set_code, rarity) VALUES ($1, $2, $3, $4)`,
		id, cardID, setCode, rarity)
	Expect(err).NotTo(HaveOccurred())
	return id
}
