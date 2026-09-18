package store_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/card/store"
)

var _ = Describe("Card lookups", func() {
	var (
		ctx = context.Background()

		// The threshold the name rung uses; restated here so these specs pin
		// the same behaviour the ladder gets.
		threshold = 0.3

		darkMagicianImage = "cards/dark-magician.jpg"
	)

	var (
		subject *store.Store

		darkMagicianID     uuid.UUID
		darkMagicianGirlID uuid.UUID
		blueEyesID         uuid.UUID
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)

		seedSet("LOB", "Legend of Blue Eyes White Dragon")
		seedSet("MFC", "Magician's Force")

		darkMagicianID = seedCard("Dark Magician", &darkMagicianImage)
		darkMagicianGirlID = seedCard("Dark Magician Girl", nil)
		blueEyesID = seedCard("Blue-Eyes White Dragon", nil)

		seedPrinting(darkMagicianID, "LOB-EN005", "Ultra Rare")
		seedPrinting(blueEyesID, "LOB-EN001", "Ultra Rare")
		seedPrinting(darkMagicianGirlID, "MFC-EN005", "Secret Rare")
	})

	AfterEach(truncateCards)

	Describe("FindCardsByName", func() {
		var (
			matches []model.NameMatch
			err     error
			name    string
			limit   int
		)

		BeforeEach(func() {
			name = "Dark Magician"
			limit = 5
		})

		JustBeforeEach(func() {
			matches, err = subject.FindCardsByName(ctx, name, threshold, limit)
		})

		When("a name is similar to several cards", func() {
			It("returns them best first, each with its similarity", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(HaveLen(2))

				Expect(matches[0].Card.ID).To(Equal(darkMagicianID))
				Expect(matches[0].Card.Name).To(Equal("Dark Magician"))
				Expect(matches[0].Card.ImageObjectKey).To(HaveValue(Equal(darkMagicianImage)))
				Expect(matches[0].Similarity).To(BeNumerically("==", 1))

				Expect(matches[1].Card.ID).To(Equal(darkMagicianGirlID))
				Expect(matches[1].Similarity).To(BeNumerically("<", matches[0].Similarity))
				Expect(matches[1].Similarity).To(BeNumerically(">=", threshold))
			})
		})

		When("OCR dropped a couple of characters", func() {
			BeforeEach(func() { name = "Drk Magican" })

			It("still finds the card: that is what the rung is for", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).NotTo(BeEmpty())
				Expect(matches[0].Card.ID).To(Equal(darkMagicianID))
			})
		})

		When("the limit is below the number of similar cards", func() {
			BeforeEach(func() { limit = 1 })

			It("returns only the best", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(HaveLen(1))
				Expect(matches[0].Card.ID).To(Equal(darkMagicianID))
			})
		})

		When("nothing is similar enough", func() {
			BeforeEach(func() { name = "Pot of Greed" })

			It("is an empty result, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(BeEmpty())
			})
		})
	})

	Describe("SearchCards", func() {
		var (
			cards  []model.Card
			err    error
			search model.CardSearch
		)

		BeforeEach(func() {
			search = model.CardSearch{Query: "", SetPrefix: "", Page: 1, PageSize: 10}
		})

		JustBeforeEach(func() {
			cards, err = subject.SearchCards(ctx, search)
		})

		When("nothing is filtered", func() {
			It("returns every card by name", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(HaveLen(3))
				Expect(cards[0].Name).To(Equal("Blue-Eyes White Dragon"))
				Expect(cards[0].ID).To(Equal(blueEyesID))
				Expect(cards[1].Name).To(Equal("Dark Magician"))
				Expect(cards[2].Name).To(Equal("Dark Magician Girl"))
			})
		})

		When("a query is given", func() {
			BeforeEach(func() { search.Query = "magician" })

			It("matches anywhere in the name, case-insensitively", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(HaveLen(2))
				Expect(cards[0].ID).To(Equal(darkMagicianID))
				Expect(cards[1].ID).To(Equal(darkMagicianGirlID))
			})
		})

		When("a set is given", func() {
			BeforeEach(func() { search.SetPrefix = "MFC" })

			It("restricts to cards with a printing in that set", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(HaveLen(1))
				Expect(cards[0].ID).To(Equal(darkMagicianGirlID))
			})
		})

		When("a query and a set are given together", func() {
			BeforeEach(func() {
				search.Query = "magician"
				search.SetPrefix = "LOB"
			})

			It("applies both", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(HaveLen(1))
				Expect(cards[0].ID).To(Equal(darkMagicianID))
			})
		})

		When("the query contains a LIKE metacharacter", func() {
			BeforeEach(func() {
				seedCard("100% Fake Card", nil)
				search.Query = "100%"
			})

			It("reads it as three characters, not as a wildcard", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(HaveLen(1))
				Expect(cards[0].Name).To(Equal("100% Fake Card"))
			})
		})

		When("a later page is asked for", func() {
			BeforeEach(func() {
				search.PageSize = 2
				search.Page = 2
			})

			It("offsets by the pages before it", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(HaveLen(1))
				Expect(cards[0].Name).To(Equal("Dark Magician Girl"))
			})
		})

		When("the page is past the end of the results", func() {
			BeforeEach(func() { search.Page = 9 })

			It("is an empty page, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(BeEmpty())
			})
		})
	})
})
