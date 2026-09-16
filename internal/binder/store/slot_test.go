package store_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/binder/store"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

var _ = Describe("Binder slots", func() {
	var ctx = context.Background()

	var (
		subject    *store.Store
		binderID   uuid.UUID
		cardID     uuid.UUID
		printingID uuid.UUID
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)
		binderID = seedBinder(seedUser(), "Main binder")
		cardID = seedCard("Dark Magician")
		printingID = seedPrinting(cardID, "LOB-EN005", "Ultra Rare")
	})

	AfterEach(truncateAll)

	Describe("InsertSlot", func() {
		var (
			slot   model.Slot
			stored model.Slot
			err    error
		)

		BeforeEach(func() {
			slot = model.Slot{
				ID:             uuid.New(),
				BinderID:       binderID,
				Position:       0,
				CardID:         cardID,
				CardPrintingID: &printingID,
				SetResolution:  cardmodel.SetResolutionExact,
			}
		})

		JustBeforeEach(func() {
			stored, err = subject.InsertSlot(ctx, slot)
		})

		When("the slot resolves to a printing", func() {
			It("returns the slot as stored, with the timestamps the database assigned", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stored.ID).To(Equal(slot.ID))
				Expect(stored.BinderID).To(Equal(binderID))
				Expect(stored.Position).To(BeZero())
				Expect(stored.CardID).To(Equal(cardID))
				Expect(stored.CardPrintingID).To(HaveValue(Equal(printingID)))
				Expect(stored.SetResolution).To(Equal(cardmodel.SetResolutionExact))
				Expect(stored.CreatedAt).NotTo(BeZero())
				Expect(stored.UpdatedAt).NotTo(BeZero())
			})
		})

		// db/migrations/003_binders.sql states the pairing as an equivalence:
		// (set_resolution IN ('exact','by_prefix_and_number','by_number'))
		//   = (card_printing_id IS NOT NULL).
		// model.RequiresPrinting is the Go side of it. These specs insert
		// through the real schema so the two cannot drift apart.
		DescribeTable("the printing a resolution is allowed to carry",
			func(resolution cardmodel.SetResolution, withPrinting bool) {
				candidate := model.Slot{
					ID:            uuid.New(),
					BinderID:      binderID,
					Position:      1,
					CardID:        cardID,
					SetResolution: resolution,
				}
				if withPrinting {
					candidate.CardPrintingID = &printingID
				}

				_, insertErr := subject.InsertSlot(ctx, candidate)

				if cardmodel.RequiresPrinting(resolution) == withPrinting {
					Expect(insertErr).NotTo(HaveOccurred(),
						"%s with printing=%t is the pairing the CHECK allows", resolution, withPrinting)
					return
				}
				Expect(insertErr).To(HaveOccurred(),
					"%s with printing=%t is the pairing the CHECK forbids", resolution, withPrinting)
			},
			Entry("exact, with a printing", cardmodel.SetResolutionExact, true),
			Entry("exact, without one", cardmodel.SetResolutionExact, false),
			Entry("by prefix and number, with a printing", cardmodel.SetResolutionByPrefixAndNumber, true),
			Entry("by prefix and number, without one", cardmodel.SetResolutionByPrefixAndNumber, false),
			Entry("by number, with a printing", cardmodel.SetResolutionByNumber, true),
			Entry("by number, without one", cardmodel.SetResolutionByNumber, false),
			Entry("by name, with a printing", cardmodel.SetResolutionByName, true),
			Entry("by name, without one", cardmodel.SetResolutionByName, false),
			Entry("unresolved, with a printing", cardmodel.SetResolutionUnresolved, true),
			Entry("unresolved, without one", cardmodel.SetResolutionUnresolved, false),
		)

		When("the position is already taken", func() {
			BeforeEach(func() {
				seedSlot(binderID, cardID, 0)
			})

			It("is a conflict, not an opaque driver error", func() {
				Expect(dataerror.IsConflictError(err)).To(BeTrue(), "got %v", err)
			})
		})

		When("the card does not exist", func() {
			BeforeEach(func() {
				slot.CardID = uuid.New()
				slot.CardPrintingID = nil
				slot.SetResolution = cardmodel.SetResolutionByName
			})

			It("names the card as the unusable reference", func() {
				Expect(dataerror.IsInvalidReferenceError(err, "card")).To(BeTrue(), "got %v", err)
			})
		})

		When("the printing belongs to a different card", func() {
			BeforeEach(func() {
				otherCardID := seedCard("Blue-Eyes White Dragon")
				otherPrintingID := seedPrinting(otherCardID, "LOB-EN001", "Ultra Rare")
				slot.CardPrintingID = &otherPrintingID
			})

			It("names the printing as the unusable reference", func() {
				Expect(dataerror.IsInvalidReferenceError(err, "card printing")).To(BeTrue(), "got %v", err)
			})
		})
	})

	Describe("GetSlotByID", func() {
		var (
			slotID uuid.UUID
			slot   model.Slot
			err    error
		)

		BeforeEach(func() {
			slotID = seedSlot(binderID, cardID, 0)
		})

		JustBeforeEach(func() {
			slot, err = subject.GetSlotByID(ctx, binderID, slotID)
		})

		When("the slot is in the binder", func() {
			It("returns it, with no printing for a name-rung slot", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(slot.ID).To(Equal(slotID))
				Expect(slot.BinderID).To(Equal(binderID))
				Expect(slot.CardID).To(Equal(cardID))
				Expect(slot.CardPrintingID).To(BeNil())
				Expect(slot.SetResolution).To(Equal(cardmodel.SetResolutionByName))
			})
		})

		When("the slot belongs to another binder", func() {
			BeforeEach(func() {
				otherBinderID := seedBinder(seedUser(), "Someone else's binder")
				slotID = seedSlot(otherBinderID, cardID, 0)
			})

			It("is simply not found, rather than found and refused", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(), "got %v", err)
			})
		})
	})

	Describe("ListSlotsInRange", func() {
		var (
			slotIDs []uuid.UUID
			slots   []model.Slot
			err     error
			from    int
			to      int
		)

		BeforeEach(func() {
			slotIDs = make([]uuid.UUID, 0, 12)
			for position := range 12 {
				slotIDs = append(slotIDs, seedSlot(binderID, seedCard("Card"), position))
			}
			from, to = 0, model.SlotsPerPage-1
		})

		JustBeforeEach(func() {
			slots, err = subject.ListSlotsInRange(ctx, binderID, from, to)
		})

		When("the range is the first page", func() {
			It("returns that page's slots in position order", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(slots).To(HaveLen(model.SlotsPerPage))
				Expect(slots[0].ID).To(Equal(slotIDs[0]))
				Expect(slots[8].ID).To(Equal(slotIDs[8]))
				Expect(slots[8].Position).To(Equal(8))
			})
		})

		When("the range is a partly filled last page", func() {
			BeforeEach(func() {
				from, to = model.SlotsPerPage, 2*model.SlotsPerPage-1
			})

			It("returns only the occupied positions", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(slots).To(HaveLen(3))
				Expect(slots[0].ID).To(Equal(slotIDs[9]))
				Expect(slots[0].Page()).To(Equal(1))
				Expect(slots[0].SlotOnPage()).To(BeZero())
			})
		})

		When("the range is past the end of the binder", func() {
			BeforeEach(func() {
				from, to = 90, 98
			})

			It("is an empty page, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(slots).To(BeEmpty())
			})
		})
	})

	Describe("CountSlots", func() {
		var (
			count int
			err   error
		)

		JustBeforeEach(func() {
			count, err = subject.CountSlots(ctx, binderID)
		})

		When("the binder holds cards", func() {
			BeforeEach(func() {
				seedSlot(binderID, cardID, 0)
				seedSlot(binderID, seedCard("Another card"), 1)
			})

			It("counts them", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(2))
			})
		})

		When("the binder is empty", func() {
			It("is zero, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(BeZero())
			})
		})
	})

	Describe("DeleteSlotByID", func() {
		var (
			slotID uuid.UUID
			err    error
		)

		BeforeEach(func() {
			slotID = seedSlot(binderID, cardID, 0)
		})

		JustBeforeEach(func() {
			err = subject.DeleteSlotByID(ctx, binderID, slotID)
		})

		When("the slot is there", func() {
			It("removes it", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(BeEmpty())
			})
		})

		When("the slot is not there", func() {
			BeforeEach(func() {
				slotID = uuid.New()
			})

			It("reports it missing rather than succeeding silently", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(), "got %v", err)
			})
		})
	})
})
