package importer_test

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimport/importer"
	"github.com/simeonkorchev/binder/cmd/cardimport/importer/importerfakes"
	"github.com/simeonkorchev/binder/cmd/cardimport/model"
	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

var (
	errDB = errors.New("db error")
	// setCodeShape is card_printings_set_code_shape, copied from
	// db/migrations/001_cards.sql. A spec below asserts the importer never
	// hands the store a code the database would reject.
	setCodeShape = regexp.MustCompile(`^[^-]+-.+$`)
)

var _ = Describe("Importer", func() {
	var (
		fakeFetcher *importerfakes.FakeFetcher
		fakeStore   *importerfakes.FakeStore
		batchSize   int
		result      importer.Result
		err         error
		batches     []model.Batch
	)

	BeforeEach(func() {
		fakeFetcher = new(importerfakes.FakeFetcher)
		fakeStore = new(importerfakes.FakeStore)
		fakeFetcher.FetchAllCardsReturns(fixtureCards(), nil)
		// Larger than the fixture, so the default case is one batch and a
		// spec that cares about slicing says so itself.
		batchSize = 100
	})

	JustBeforeEach(func() {
		subject, newErr := importer.New(importer.Config{
			Fetcher:   fakeFetcher,
			Store:     fakeStore,
			BatchSize: batchSize,
		})
		Expect(newErr).NotTo(HaveOccurred())

		result, err = subject.Import(context.Background())
		batches = allBatches(fakeStore.UpsertBatchArgsForCall, fakeStore.UpsertBatchCallCount())
	})

	When("the dump is the committed fixture", func() {
		It("fetches the whole dump in one request", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeFetcher.FetchAllCardsCallCount()).To(Equal(1))
		})

		It("writes every card in the dump, including one with no printings", func() {
			Expect(cardsOf(batches)).To(ConsistOf(
				model.CardRow{YgoprodeckID: 89631139, Name: "Blue-Eyes White Dragon"},
				model.CardRow{YgoprodeckID: 46986414, Name: "Dark Magician"},
				model.CardRow{YgoprodeckID: 44095762, Name: "Mirror Force"},
				model.CardRow{YgoprodeckID: 10000000, Name: "Obelisk the Tormentor"},
				model.CardRow{YgoprodeckID: 40640057, Name: "Kuriboh"},
			))
			Expect(result.Cards).To(Equal(5))
		})

		It("keeps both rarities printed under one code", func() {
			Expect(printingsOf(batches)).To(ContainElements(
				model.PrintingRow{YgoprodeckID: 89631139, SetCode: "LOB-EN001", Rarity: "Ultra Rare"},
				model.PrintingRow{YgoprodeckID: 89631139, SetCode: "LOB-EN001", Rarity: "Secret Rare"},
			))
		})

		It("de-duplicates a printing the dump repeats", func() {
			repeated := 0
			for _, printing := range printingsOf(batches) {
				if printing.SetCode == "LOB-EN001" && printing.Rarity == "Ultra Rare" {
					repeated++
				}
			}
			Expect(repeated).To(Equal(1), "the dump lists this printing twice; ON CONFLICT cannot touch one row twice")
		})

		It("upper-cases a code the dump spelled in lower case", func() {
			Expect(printingsOf(batches)).To(ContainElement(
				model.PrintingRow{YgoprodeckID: 46986414, SetCode: "SDY-EN006", Rarity: "Common"},
			))
		})

		It("keeps the remainder of a multi-hyphen code in the printing", func() {
			Expect(printingsOf(batches)).To(ContainElement(
				model.PrintingRow{YgoprodeckID: 89631139, SetCode: "SDK-EN-A01", Rarity: "Common"},
			))
		})

		It("writes one set row per prefix, the first name seen winning", func() {
			Expect(setsOf(batches)).To(ContainElement(
				model.SetRow{Code: "LOB", Name: "Legend of Blue Eyes White Dragon"},
			))
			Expect(setsOf(batches)).NotTo(ContainElement(
				model.SetRow{Code: "LOB", Name: "Legend of Blue Eyes White Dragon: Reprint"},
			))
			// LOB, SDK, SDY and MRD. JMP is not among them: its only printing
			// was skipped for a blank rarity, and a set whose every printing
			// was skipped is not one this run has seen.
			Expect(result.Sets).To(Equal(4))
		})

		It("never hands the store a code the CHECK constraint would reject", func() {
			for _, printing := range printingsOf(batches) {
				Expect(setCodeShape.MatchString(printing.SetCode)).
					To(BeTrue(), "set code %q violates card_printings_set_code_shape", printing.SetCode)
			}
		})

		It("counts every malformed code and keeps the raw spelling for the log", func() {
			Expect(result.Skipped).To(HaveKey(model.SkipMalformedSetCode))
			malformed := result.Skipped[model.SkipMalformedSetCode]
			Expect(malformed.Count).To(Equal(3), "PROMO has no hyphen, -EN139 no prefix, MRD- no number")
			Expect(malformed.Examples).To(ConsistOf("PROMO", "-EN139", "MRD-"))
		})

		It("counts a printing whose rarity is blank", func() {
			Expect(result.Skipped[model.SkipMissingRarity].Count).To(Equal(1))
			Expect(result.Skipped[model.SkipMissingRarity].Examples).To(ConsistOf("JMP-EN001"))
		})

		It("reports the skipped total across every reason", func() {
			Expect(result.SkippedTotal()).To(Equal(4))
		})

		It("gives every printing a set row in the same batch", func() {
			for _, batch := range batches {
				codes := make([]string, 0, len(batch.Sets))
				for _, set := range batch.Sets {
					codes = append(codes, set.Code)
				}
				for _, printing := range batch.Printings {
					prefix, _, _ := strings.Cut(printing.SetCode, "-")
					Expect(codes).To(ContainElement(prefix),
						"card_printings.set_prefix is the foreign key to card_sets.code")
				}
			}
		})
	})

	When("the dump holds more cards than one batch", func() {
		BeforeEach(func() {
			batchSize = 2
		})

		It("writes one transaction per slice of the dump", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStore.UpsertBatchCallCount()).To(Equal(3), "five fixture cards in batches of two")
		})

		It("still accounts for every card and every skip", func() {
			Expect(result.Cards).To(Equal(5))
			Expect(result.SkippedTotal()).To(Equal(4))
		})

		It("repeats a set row in every batch whose printings need it", func() {
			// De-duplication is per batch, because the batch is the unit of
			// transaction: LOB is in the slice holding Blue-Eyes and again in
			// the one holding Kuriboh. The upsert is what makes the repeat
			// harmless — the same property that makes a re-run harmless.
			Expect(result.Sets).To(Equal(5), "four distinct sets, LOB written by two batches")
		})
	})

	When("the dump is empty", func() {
		BeforeEach(func() {
			fakeFetcher.FetchAllCardsReturns(nil, nil)
		})

		It("writes nothing and does not fail", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeStore.UpsertBatchCallCount()).To(BeZero())
			Expect(result).To(Equal(importer.Result{Skipped: map[model.SkipReason]importer.SkipSummary{}}))
		})
	})

	When("more printings are malformed than the log keeps examples of", func() {
		BeforeEach(func() {
			fakeFetcher.FetchAllCardsReturns([]ygoprodeck.Card{malformedCard(7)}, nil)
		})

		It("counts all of them but keeps only the first few codes", func() {
			Expect(err).NotTo(HaveOccurred())
			malformed := result.Skipped[model.SkipMalformedSetCode]
			Expect(malformed.Count).To(Equal(7))
			Expect(malformed.Examples).To(ConsistOf("PROMO0", "PROMO1", "PROMO2", "PROMO3", "PROMO4"))
		})
	})

	When("fetching the dump fails", func() {
		BeforeEach(func() {
			fakeFetcher.FetchAllCardsReturns(nil, ygoprodeck.ErrUnexpectedStatus)
		})

		It("reports the failure and never touches the database", func() {
			Expect(err).To(MatchError(ygoprodeck.ErrUnexpectedStatus))
			Expect(fakeStore.UpsertBatchCallCount()).To(BeZero())
		})
	})

	When("a batch fails to write", func() {
		BeforeEach(func() {
			batchSize = 2
			fakeStore.UpsertBatchReturnsOnCall(1, errDB)
		})

		It("stops the run and reports what was committed before the failure", func() {
			Expect(err).To(MatchError(errDB))
			Expect(fakeStore.UpsertBatchCallCount()).To(Equal(2), "the third batch is never attempted")
			Expect(result.Cards).To(Equal(2), "the first batch committed; a re-run repeats it harmlessly")
		})
	})
})

var _ = Describe("New", func() {
	var (
		fakeFetcher *importerfakes.FakeFetcher
		fakeStore   *importerfakes.FakeStore
		cfg         importer.Config
		err         error
	)

	BeforeEach(func() {
		fakeFetcher = new(importerfakes.FakeFetcher)
		fakeStore = new(importerfakes.FakeStore)
		cfg = importer.Config{Fetcher: fakeFetcher, Store: fakeStore, BatchSize: 1}
	})

	JustBeforeEach(func() {
		_, err = importer.New(cfg)
	})

	When("every dependency is present", func() {
		It("builds the importer", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the fetcher is missing", func() {
		BeforeEach(func() { cfg.Fetcher = nil })

		It("refuses to start", func() {
			Expect(err).To(MatchError(ContainSubstring("fetcher is nil")))
		})
	})

	When("the store is missing", func() {
		BeforeEach(func() { cfg.Store = nil })

		It("refuses to start", func() {
			Expect(err).To(MatchError(ContainSubstring("store is nil")))
		})
	})

	When("the batch size is not positive", func() {
		BeforeEach(func() { cfg.BatchSize = 0 })

		It("refuses to start rather than slicing the dump into nothing", func() {
			Expect(err).To(MatchError(ContainSubstring("batch size must be positive")))
		})
	})
})

// malformedCard builds a card whose every printing carries a code with no
// hyphen, which is the one thing the schema's CHECK rejects outright.
func malformedCard(printings int) ygoprodeck.Card {
	sets := make([]ygoprodeck.CardSet, 0, printings)
	for i := range printings {
		sets = append(sets, ygoprodeck.CardSet{
			Name:   "Tournament Pack",
			Code:   "PROMO" + strconv.Itoa(i),
			Rarity: "Common",
		})
	}

	return ygoprodeck.Card{ID: 1, Name: "Promo Only", Sets: sets}
}
