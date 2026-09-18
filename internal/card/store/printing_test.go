package store_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/card/store"
)

// These specs run against the real schema, so they also pin what
// db/migrations/001_cards.sql generates: set_prefix and set_number are derived
// from set_code by the database, and every lookup below keys on that derivation.
var _ = Describe("Printing lookups", func() {
	var (
		ctx = context.Background()

		darkMagicianImage = "cards/dark-magician.jpg"
	)

	var (
		subject *store.Store

		darkMagicianID     uuid.UUID
		darkMagicianGirlID uuid.UUID
		lob005ID           uuid.UUID
		sdy005ID           uuid.UUID
		mfc005ID           uuid.UUID
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)

		seedSet("LOB", "Legend of Blue Eyes White Dragon")
		seedSet("SDY", "Starter Deck: Yugi")
		seedSet("MFC", "Magician's Force")

		darkMagicianID = seedCard("Dark Magician", &darkMagicianImage)
		darkMagicianGirlID = seedCard("Dark Magician Girl", nil)

		// Two printings of one card share the number EN005; a third printing
		// of a *different* card shares it too. That is the whole ladder's
		// fixture: which rung can tell them apart is what each spec asserts.
		lob005ID = seedPrinting(darkMagicianID, "LOB-EN005", "Ultra Rare")
		sdy005ID = seedPrinting(darkMagicianID, "SDY-EN005", "Common")
		mfc005ID = seedPrinting(darkMagicianGirlID, "MFC-EN005", "Secret Rare")
	})

	AfterEach(truncateCards)

	Describe("FindPrintingsBySetCode", func() {
		var (
			printings []model.PrintedCard
			err       error
			setCode   string
		)

		JustBeforeEach(func() {
			printings, err = subject.FindPrintingsBySetCode(ctx, setCode)
		})

		When("the whole printed code names a printing", func() {
			BeforeEach(func() { setCode = "LOB-EN005" })

			It("returns that one printing, with the card it prints", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(HaveLen(1))
				Expect(printings[0]).To(Equal(model.PrintedCard{
					Card: model.Card{
						ID:             darkMagicianID,
						Name:           "Dark Magician",
						ImageObjectKey: &darkMagicianImage,
					},
					Printing: model.CardPrinting{
						ID:      lob005ID,
						CardID:  darkMagicianID,
						SetCode: "LOB-EN005",
						Rarity:  "Ultra Rare",
					},
				}))
			})
		})

		When("one code was printed in two rarities", func() {
			BeforeEach(func() {
				seedPrinting(darkMagicianID, "LOB-EN005", "Secret Rare")
				setCode = "LOB-EN005"
			})

			It("returns both, deterministically ordered by rarity", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(HaveLen(2))
				Expect(printings[0].Printing.Rarity).To(Equal("Secret Rare"))
				Expect(printings[1].Printing.Rarity).To(Equal("Ultra Rare"))
			})
		})

		When("no printing carries that code", func() {
			BeforeEach(func() { setCode = "LOB-EN999" })

			It("is an empty result, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(BeEmpty())
			})
		})
	})

	Describe("FindPrintingsBySetPrefixAndNumber", func() {
		var (
			printings []model.PrintedCard
			err       error
			prefix    string
			number    string
		)

		BeforeEach(func() {
			prefix = "LO"
			number = "EN005"
		})

		JustBeforeEach(func() {
			printings, err = subject.FindPrintingsBySetPrefixAndNumber(ctx, prefix, number)
		})

		When("the set's prefix starts with the prefix that was read", func() {
			It("matches only that set's printing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(HaveLen(1))
				Expect(printings[0].Printing.ID).To(Equal(lob005ID))
				Expect(printings[0].Printing.SetCode).To(Equal("LOB-EN005"))
			})
		})

		When("the prefix that was read is the whole set prefix", func() {
			BeforeEach(func() { prefix = "SDY" })

			It("matches that set's printing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(HaveLen(1))
				Expect(printings[0].Printing.ID).To(Equal(sdy005ID))
			})
		})

		When("no set prefix starts with what was read", func() {
			BeforeEach(func() { prefix = "ZZ" })

			It("is an empty result, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(BeEmpty())
			})
		})

		When("the prefix matches but the number does not", func() {
			BeforeEach(func() { number = "EN999" })

			It("matches nothing: the number is not a prefix match", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(BeEmpty())
			})
		})
	})

	Describe("FindPrintingsBySetNumber", func() {
		var (
			printings []model.PrintedCard
			err       error
			number    string
		)

		JustBeforeEach(func() {
			printings, err = subject.FindPrintingsBySetNumber(ctx, number)
		})

		When("several sets printed that number", func() {
			BeforeEach(func() { number = "EN005" })

			It("returns every printing carrying it, across cards and sets", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(HaveLen(3))

				ids := make([]uuid.UUID, 0, len(printings))
				for _, printing := range printings {
					ids = append(ids, printing.Printing.ID)
				}
				Expect(ids).To(ConsistOf(lob005ID, sdy005ID, mfc005ID))
			})

			It("carries each printing's own card, which is what makes the ambiguity visible", func() {
				Expect(printings[0].Printing.SetCode).To(Equal("LOB-EN005"))
				Expect(printings[0].Card.ID).To(Equal(darkMagicianID))
				Expect(printings[1].Printing.SetCode).To(Equal("MFC-EN005"))
				Expect(printings[1].Card.ID).To(Equal(darkMagicianGirlID))
				Expect(printings[1].Card.Name).To(Equal("Dark Magician Girl"))
				Expect(printings[1].Card.ImageObjectKey).To(BeNil())
			})
		})

		When("nothing carries that number", func() {
			BeforeEach(func() { number = "EN999" })

			It("is an empty result, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(BeEmpty())
			})
		})

		When("the number is the tail of a code with a second hyphen", func() {
			BeforeEach(func() {
				seedPrinting(darkMagicianID, "LOB-EN-005", "Rare")
				number = "EN-005"
			})

			It("matches it, because the database keeps the remainder in the number", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(printings).To(HaveLen(1))
				Expect(printings[0].Printing.SetCode).To(Equal("LOB-EN-005"))
			})
		})
	})
})
