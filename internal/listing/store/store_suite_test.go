package store_test

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

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
	// Opened before RunSpecs rather than in a BeforeSuite: testdb.New skips
	// when no database is reachable, and a skip has to happen on the test's own
	// goroutine to end the test rather than crash Ginkgo's.
	testDB = testdb.New(t)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Listing Store Suite")
}

// truncateAll empties every table these specs write. binders, binder_slots and
// listings all cascade from users.
func truncateAll() {
	_, err := testDB.Exec(`TRUNCATE users, cards, card_sets CASCADE`)
	Expect(err).NotTo(HaveOccurred())
}

// seedUser inserts one account and returns its id. The provider subject is the
// id itself, which is unique and is all these specs need of it.
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

// seedCardWithImage inserts one card that cmd/cardimages has already fetched an
// image for, so a spec can prove the object key reaches the model.
func seedCardWithImage(name, objectKey string) uuid.UUID {
	id := seedCard(name)
	_, err := testDB.Exec(`UPDATE cards SET image_object_key = $2 WHERE id = $1`, id, objectKey)
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

// seedSlot inserts one slot resolved by name, so it carries no printing — the
// case where the seller's scan identified the card but not its set.
func seedSlot(binderID, cardID uuid.UUID, position int) uuid.UUID {
	id := uuid.New()
	_, err := testDB.Exec(
		`INSERT INTO binder_slots (id, binder_id, "position", card_id, set_resolution)
		 VALUES ($1, $2, $3, $4, 'by_name')`,
		id, binderID, position, cardID)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedSlotWithPrinting inserts one slot whose scan resolved the set exactly, so
// it names a printing and therefore a set code.
func seedSlotWithPrinting(binderID, cardID, printingID uuid.UUID, position int) uuid.UUID {
	id := uuid.New()
	_, err := testDB.Exec(
		`INSERT INTO binder_slots (id, binder_id, "position", card_id, card_printing_id, set_resolution)
		 VALUES ($1, $2, $3, $4, $5, 'exact')`,
		id, binderID, position, cardID, printingID)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedListing puts a slot up for sale without going through the store, for the
// specs that read or delete a listing rather than create one.
func seedListing(binderSlotID uuid.UUID) uuid.UUID {
	id := uuid.New()
	_, err := testDB.Exec(`INSERT INTO listings (id, binder_slot_id) VALUES ($1, $2)`, id, binderSlotID)
	Expect(err).NotTo(HaveOccurred())
	return id
}

// seedListingAt puts a slot up for sale at a given moment. A spec about the
// order of the feed says when each listing happened rather than relying on the
// clock to separate two inserts a millisecond apart. It returns nothing: the
// specs that use it identify the listings by their card, not by id.
func seedListingAt(binderSlotID uuid.UUID, listedAt time.Time) {
	_, err := testDB.Exec(
		`INSERT INTO listings (id, binder_slot_id, created_at) VALUES ($1, $2, $3)`,
		uuid.New(), binderSlotID, listedAt)
	Expect(err).NotTo(HaveOccurred())
}

// countListings is how many listings exist at all, for the specs that prove one
// went away.
func countListings() int {
	var count int
	err := testDB.GetContext(context.Background(), &count, `SELECT count(*) FROM listings`)
	Expect(err).NotTo(HaveOccurred())
	return count
}
