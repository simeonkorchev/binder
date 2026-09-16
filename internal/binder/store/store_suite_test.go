package store_test

import (
	"context"
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
	RunSpecs(t, "Binder Store Suite")
}

// truncateAll empties every table these specs write. binders and binder_slots
// cascade from users; cards and card_printings are referenced by slots.
func truncateAll() {
	_, err := testDB.Exec(`TRUNCATE users, cards, card_sets CASCADE`)
	Expect(err).NotTo(HaveOccurred())
}

// seedUser inserts the owner a binder needs and returns its id. The provider
// subject is the id itself, which is unique and is all these specs need of it.
func seedUser() uuid.UUID {
	id := uuid.New()
	_, err := testDB.Exec(
		`INSERT INTO users (id, auth_provider, auth_subject) VALUES ($1, 'google', $2)`, id, id.String())
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedCard inserts one card and returns its id. ygoprodeck_id is UNIQUE, so it
// is derived from the row's own uuid — shifted to stay positive.
func seedCard(name string) uuid.UUID {
	id := uuid.New()
	ygoprodeckID := int64(binary.BigEndian.Uint64(id[:8]) >> 1)

	_, err := testDB.Exec(
		`INSERT INTO cards (id, ygoprodeck_id, name) VALUES ($1, $2, $3)`, id, ygoprodeckID, name)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedPrinting inserts one printing of a card, creating its set if need be.
func seedPrinting(cardID uuid.UUID, setCode, rarity string) uuid.UUID {
	prefix := setCode[:len(setCode)-len("-EN000")]
	_, err := testDB.Exec(
		`INSERT INTO card_sets (code, name) VALUES ($1, $1) ON CONFLICT (code) DO NOTHING`, prefix)
	Expect(err).NotTo(HaveOccurred())

	id := uuid.New()
	_, err = testDB.Exec(
		`INSERT INTO card_printings (id, card_id, set_code, rarity) VALUES ($1, $2, $3, $4)`,
		id, cardID, setCode, rarity)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedBinder inserts one binder and returns its id.
func seedBinder(ownerID uuid.UUID, name string) uuid.UUID {
	id := uuid.New()
	_, err := testDB.Exec(`INSERT INTO binders (id, owner_id, name) VALUES ($1, $2, $3)`, id, ownerID, name)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedSlot inserts one slot at a position, resolved by name so it carries no
// printing. Specs that care about the printing seed it themselves.
func seedSlot(binderID, cardID uuid.UUID, position int) uuid.UUID {
	id := uuid.New()
	_, err := testDB.Exec(
		`INSERT INTO binder_slots (id, binder_id, "position", card_id, set_resolution)
		 VALUES ($1, $2, $3, $4, 'by_name')`,
		id, binderID, position, cardID)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// positionsOf reads the binder's slot ids in position order, and asserts on the
// way that the positions are dense from zero — the invariant the database
// cannot enforce and every rearrangement here has to keep.
func positionsOf(binderID uuid.UUID) []uuid.UUID {
	var rows []struct {
		ID       uuid.UUID `db:"id"`
		Position int       `db:"position"`
	}
	err := testDB.SelectContext(context.Background(), &rows,
		`SELECT id, "position" FROM binder_slots WHERE binder_id = $1 ORDER BY "position"`, binderID)
	Expect(err).NotTo(HaveOccurred())

	ids := make([]uuid.UUID, 0, len(rows))
	for i, row := range rows {
		Expect(row.Position).To(Equal(i), "positions must be dense from zero, with no gap and no duplicate")
		ids = append(ids, row.ID)
	}
	return ids
}

// truncateSlots empties one binder, for a spec that wants to reseed it with a
// different number of slots than its outer BeforeEach did.
func truncateSlots(binderID uuid.UUID) {
	_, err := testDB.Exec(`DELETE FROM binder_slots WHERE binder_id = $1`, binderID)
	Expect(err).NotTo(HaveOccurred())
}
