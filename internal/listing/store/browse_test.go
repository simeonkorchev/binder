package store_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/internal/listing/store"
)

// browseLimit is a limit no spec here is trying to reach; the one spec about the
// limit sets its own.
const browseLimit = 50

var _ = Describe("Browsing listings", func() {
	var ctx = context.Background()

	var (
		subject  *store.Store
		search   model.ListingSearch
		listed   []model.ListedCard
		err      error
		sellerID uuid.UUID
		binderID uuid.UUID
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)
		sellerID = seedUser()
		binderID = seedBinder(sellerID, "Trade binder")
		search = model.ListingSearch{Query: "", SetPrefix: "", Limit: browseLimit}
	})

	AfterEach(truncateAll)

	JustBeforeEach(func() {
		listed, err = subject.BrowseListings(ctx, search)
	})

	When("nothing is for sale", func() {
		It("is an empty feed and not an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(listed).NotTo(BeNil(), "an empty feed is [], never null")
			Expect(listed).To(BeEmpty())
		})
	})

	When("a card with a resolved set is for sale", func() {
		var (
			listingID uuid.UUID
			cardID    uuid.UUID
		)

		BeforeEach(func() {
			cardID = seedCardWithImage("Dark Magician", "cards/dark-magician.jpg")
			printingID := seedPrinting(cardID, "LOB-EN005", "Ultra Rare")
			listingID = seedListing(seedSlotWithPrinting(binderID, cardID, printingID, 0))
		})

		It("carries the card, its set code and who to ask, and nothing else about the seller", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(listed).To(HaveLen(1))

			one := listed[0]
			Expect(one.ListingID).To(Equal(listingID))
			Expect(one.SellerID).To(Equal(sellerID))
			Expect(one.Card.ID).To(Equal(cardID))
			Expect(one.Card.Name).To(Equal("Dark Magician"))
			Expect(one.Card.ImageObjectKey).To(HaveValue(Equal("cards/dark-magician.jpg")))
			Expect(one.SetCode).To(HaveValue(Equal("LOB-EN005")))
			Expect(one.ListedAt).NotTo(BeZero())
		})
	})

	When("a card whose set was never resolved is for sale", func() {
		BeforeEach(func() {
			listedCard := seedCard("Kuriboh")
			seedListing(seedSlot(binderID, listedCard, 0))
		})

		It("is in the feed with no set code, because the name rung named no set", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(listed).To(HaveLen(1))
			Expect(listed[0].Card.Name).To(Equal("Kuriboh"))
			Expect(listed[0].SetCode).To(BeNil())
			Expect(listed[0].Card.ImageObjectKey).To(BeNil())
		})
	})

	When("several cards are for sale", func() {
		// Listed at stated moments, oldest first, so "newest first" is a fact of
		// the fixture rather than of how fast three inserts ran.
		BeforeEach(func() {
			listedFirst := time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)

			blueEyes := seedCard("Blue-Eyes White Dragon")
			blueEyesPrinting := seedPrinting(blueEyes, "LOB-EN001", "Ultra Rare")
			seedListingAt(seedSlotWithPrinting(binderID, blueEyes, blueEyesPrinting, 0), listedFirst)

			redEyes := seedCard("Red-Eyes B. Dragon")
			redEyesPrinting := seedPrinting(redEyes, "MRD-EN000", "Ultra Rare")
			seedListingAt(seedSlotWithPrinting(binderID, redEyes, redEyesPrinting, 1), listedFirst.Add(time.Hour))

			seedListingAt(seedSlot(binderID, seedCard("Kuriboh"), 2), listedFirst.Add(2*time.Hour))
		})

		It("returns all of them, newest first", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(names(listed)).To(Equal([]string{"Kuriboh", "Red-Eyes B. Dragon", "Blue-Eyes White Dragon"}))
		})

		When("the buyer searches part of a name in the wrong case", func() {
			BeforeEach(func() {
				search.Query = "eYeS"
			})

			It("keeps the cards whose name contains it and drops the rest", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(names(listed)).To(ConsistOf("Blue-Eyes White Dragon", "Red-Eyes B. Dragon"))
			})
		})

		When("the buyer filters by a set", func() {
			BeforeEach(func() {
				search.SetPrefix = "LOB"
			})

			It("keeps only the cards listed as printed in that set", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(names(listed)).To(ConsistOf("Blue-Eyes White Dragon"),
					"a card whose set was never resolved names no set to filter on")
			})
		})

		When("the feed is limited", func() {
			BeforeEach(func() {
				search.Limit = 2
			})

			It("returns the newest that fit", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(names(listed)).To(Equal([]string{"Kuriboh", "Red-Eyes B. Dragon"}))
			})
		})
	})

	// A name is user-typed and goes into a LIKE pattern, so its metacharacters
	// have to mean themselves: a buyer searching "100%" wants the three
	// characters, not "anything starting with 100".
	When("a searched name contains a LIKE metacharacter", func() {
		BeforeEach(func() {
			seedListing(seedSlot(binderID, seedCard("Ojama Trio"), 0))
			seedListing(seedSlot(binderID, seedCard("100% Ojama"), 1))
			search.Query = "100%"
		})

		It("matches the name that really contains it", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(names(listed)).To(ConsistOf("100% Ojama"))
		})
	})

	When("two sellers each have a card for sale", func() {
		var otherSellerID uuid.UUID

		BeforeEach(func() {
			seedListing(seedSlot(binderID, seedCard("Kuriboh"), 0))

			otherSellerID = seedUser()
			otherBinderID := seedBinder(otherSellerID, "Spare binder")
			seedListing(seedSlot(otherBinderID, seedCard("Mystical Space Typhoon"), 0))
		})

		It("attributes each listing to the owner of the binder it is in", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(listed).To(HaveLen(2))

			sellers := map[string]uuid.UUID{}
			for _, one := range listed {
				sellers[one.Card.Name] = one.SellerID
			}
			Expect(sellers).To(HaveKeyWithValue("Kuriboh", sellerID))
			Expect(sellers).To(HaveKeyWithValue("Mystical Space Typhoon", otherSellerID))
		})
	})
})

// names is the card names of a feed, in the order the feed returned them.
func names(listed []model.ListedCard) []string {
	got := make([]string, 0, len(listed))
	for _, one := range listed {
		got = append(got, one.Card.Name)
	}
	return got
}
