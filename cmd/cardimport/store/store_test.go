package store_test

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimport/model"
	"github.com/simeonkorchev/binder/cmd/cardimport/store"
)

// printingRecord is what the specs read back; set_prefix and set_number are
// GENERATED columns, so reading them is how the split is asserted rather than
// assumed.
type printingRecord struct {
	CardID    uuid.UUID `db:"card_id"`
	SetCode   string    `db:"set_code"`
	SetPrefix string    `db:"set_prefix"`
	SetNumber string    `db:"set_number"`
	Rarity    string    `db:"rarity"`
}

type cardRecord struct {
	ID           uuid.UUID `db:"id"`
	YgoprodeckID int64     `db:"ygoprodeck_id"`
	Name         string    `db:"name"`
}

type setRecord struct {
	Code string `db:"code"`
	Name string `db:"name"`
}

var _ = Describe("Store", func() {
	// The batch these specs write is the fixture dump's shape, hand-built here
	// rather than run through the importer's builder: this suite pins what the
	// *store* promises, so a change to the builder must not be able to make it
	// pass. pkg/ygoprodeck/testdata/README.md explains where the shapes come
	// from.
	blueEyes := model.CardRow{YgoprodeckID: 89631139, Name: "Blue-Eyes White Dragon"}
	darkMagician := model.CardRow{YgoprodeckID: 46986414, Name: "Dark Magician"}
	fixtureBatch := model.Batch{
		Sets: []model.SetRow{
			{Code: "LOB", Name: "Legend of Blue Eyes White Dragon"},
			{Code: "SDK", Name: "Starter Deck: Kaiba"},
		},
		Cards: []model.CardRow{blueEyes, darkMagician},
		Printings: []model.PrintingRow{
			{YgoprodeckID: 89631139, SetCode: "LOB-EN001", Rarity: "Ultra Rare"},
			{YgoprodeckID: 89631139, SetCode: "LOB-EN001", Rarity: "Secret Rare"},
			{YgoprodeckID: 89631139, SetCode: "SDK-EN-A01", Rarity: "Common"},
			{YgoprodeckID: 46986414, SetCode: "LOB-EN005", Rarity: "Ultra Rare"},
		},
	}

	var (
		ctx     context.Context
		subject *store.Store
		batch   model.Batch
		err     error
	)

	BeforeEach(func() {
		ctx = context.Background()
		subject = store.NewStore(testDB)
		batch = fixtureBatch

		_, truncErr := testDB.ExecContext(ctx, `TRUNCATE cards, card_sets, card_printings CASCADE`)
		Expect(truncErr).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		err = subject.UpsertBatch(ctx, batch)
	})

	When("the batch is new", func() {
		It("writes the set rows its printings reference", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(allSets(ctx)).To(ConsistOf(
				setRecord{Code: "LOB", Name: "Legend of Blue Eyes White Dragon"},
				setRecord{Code: "SDK", Name: "Starter Deck: Kaiba"},
			))
		})

		It("writes every card with the name the dump gave it", func() {
			cards := allCards(ctx)
			Expect(cards).To(HaveLen(2))
			Expect(namesByUpstreamID(cards)).To(Equal(map[int64]string{
				89631139: "Blue-Eyes White Dragon",
				46986414: "Dark Magician",
			}))
		})

		It("stores both rarities printed under one code as two printings", func() {
			printings := printingsFor(ctx, "LOB-EN001")
			Expect(printings).To(HaveLen(2))
			Expect([]string{printings[0].Rarity, printings[1].Rarity}).
				To(ConsistOf("Ultra Rare", "Secret Rare"))
			Expect(printings[0].CardID).To(Equal(printings[1].CardID))
		})

		It("splits a multi-hyphen code at the first hyphen only", func() {
			printings := printingsFor(ctx, "SDK-EN-A01")
			Expect(printings).To(HaveLen(1))
			Expect(printings[0].SetPrefix).To(Equal("SDK"))
			Expect(printings[0].SetNumber).To(Equal("EN-A01"), "the remainder stays in the number")
		})

		It("hangs each printing off the card the dump listed it under", func() {
			cards := allCards(ctx)
			idsByUpstream := make(map[int64]uuid.UUID, len(cards))
			for _, card := range cards {
				idsByUpstream[card.YgoprodeckID] = card.ID
			}

			Expect(printingsFor(ctx, "LOB-EN005")[0].CardID).To(Equal(idsByUpstream[46986414]))
		})
	})

	When("the same dump is imported a second time", func() {
		var firstPass []cardRecord

		BeforeEach(func() {
			Expect(subject.UpsertBatch(ctx, batch)).To(Succeed())
			firstPass = allCards(ctx)
		})

		It("leaves exactly the same rows, so a killed run is simply re-run", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(allCards(ctx)).To(HaveLen(len(firstPass)))
			Expect(allPrintings(ctx)).To(HaveLen(len(fixtureBatch.Printings)))
			Expect(allSets(ctx)).To(HaveLen(len(fixtureBatch.Sets)))
		})

		It("keeps every card id, so a binder slot pointing at one survives", func() {
			Expect(allCards(ctx)).To(ConsistOf(firstPass))
		})
	})

	When("a card was renamed upstream", func() {
		BeforeEach(func() {
			Expect(subject.UpsertBatch(ctx, batch)).To(Succeed())

			renamed := batch
			renamed.Cards = []model.CardRow{
				{YgoprodeckID: blueEyes.YgoprodeckID, Name: "Blue-Eyes W. Dragon"},
				darkMagician,
			}
			batch = renamed
		})

		It("updates the existing row instead of inserting a second one", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(namesByUpstreamID(allCards(ctx))).To(HaveKeyWithValue(
				blueEyes.YgoprodeckID, "Blue-Eyes W. Dragon",
			))
			Expect(allCards(ctx)).To(HaveLen(2))
		})
	})

	When("the batch carries printings but not the set row they name", func() {
		BeforeEach(func() {
			withoutSets := batch
			withoutSets.Sets = nil
			batch = withoutSets
		})

		It("writes nothing at all, because the set is the printing's foreign key", func() {
			// card_printings.set_prefix is GENERATED from set_code and is the
			// FK to card_sets.code. The rollback is the point: the cards of a
			// batch must not survive without the printings that name them.
			Expect(err).To(HaveOccurred())
			Expect(allCards(ctx)).To(BeEmpty())
			Expect(allPrintings(ctx)).To(BeEmpty())
		})
	})

	When("a printing names a card the batch does not carry", func() {
		BeforeEach(func() {
			orphaned := batch
			orphaned.Printings = append([]model.PrintingRow{
				{YgoprodeckID: 999999, SetCode: "LOB-EN999", Rarity: "Common"},
			}, batch.Printings...)
			batch = orphaned
		})

		It("refuses the batch rather than writing a printing with no card", func() {
			Expect(err).To(MatchError(ContainSubstring("card that is not in the batch")))
			Expect(allPrintings(ctx)).To(BeEmpty())
		})
	})

	When("the batch is empty", func() {
		BeforeEach(func() {
			batch = model.Batch{}
		})

		It("is a no-op and not an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(allCards(ctx)).To(BeEmpty())
		})
	})

	When("the batch holds more rows than one statement may bind", func() {
		// 65535 bind parameters over three columns is 21845 cards per
		// statement, so one more than that is the smallest batch that proves
		// the insert is chunked instead of being refused by Postgres.
		const cards = 21846

		BeforeEach(func() {
			batch = model.Batch{Cards: syntheticCards(cards)}
		})

		It("writes every row", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(allCards(ctx)).To(HaveLen(cards))
		})
	})
})

func syntheticCards(count int) []model.CardRow {
	cards := make([]model.CardRow, 0, count)
	for i := range count {
		cards = append(cards, model.CardRow{YgoprodeckID: int64(i) + 1, Name: "Card " + strconv.Itoa(i)})
	}

	return cards
}

func namesByUpstreamID(cards []cardRecord) map[int64]string {
	names := make(map[int64]string, len(cards))
	for _, card := range cards {
		names[card.YgoprodeckID] = card.Name
	}

	return names
}

func allCards(ctx context.Context) []cardRecord {
	var cards []cardRecord
	Expect(testDB.SelectContext(ctx, &cards, `SELECT id, ygoprodeck_id, name FROM cards`)).To(Succeed())

	return cards
}

func allSets(ctx context.Context) []setRecord {
	var sets []setRecord
	Expect(testDB.SelectContext(ctx, &sets, `SELECT code, name FROM card_sets`)).To(Succeed())

	return sets
}

const printingColumns = `SELECT card_id, set_code, set_prefix, set_number, rarity FROM card_printings`

func allPrintings(ctx context.Context) []printingRecord {
	var printings []printingRecord
	Expect(testDB.SelectContext(ctx, &printings, printingColumns)).To(Succeed())

	return printings
}

func printingsFor(ctx context.Context, setCode string) []printingRecord {
	var printings []printingRecord
	Expect(testDB.SelectContext(ctx, &printings, printingColumns+` WHERE set_code = $1`, setCode)).To(Succeed())

	return printings
}
