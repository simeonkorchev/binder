package service_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/binder/service"
	"github.com/simeonkorchev/binder/internal/binder/service/servicefakes"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

var _ = Describe("AddSlots", func() {
	var (
		ctx        = context.Background()
		ownerID    = uuid.New()
		binderID   = uuid.New()
		cardID     = uuid.New()
		printingID = uuid.New()
		owned      = model.Binder{ID: binderID, OwnerID: ownerID, Name: "Main binder"}
	)

	var (
		fakeStore *servicefakes.FakeStore
		svc       *service.Service
		input     model.AddSlotsInput
		added     []model.Slot
		err       error
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		passThroughTx(fakeStore)
		fakeStore.GetBinderByIDReturns(owned, nil)
		fakeStore.CountSlotsReturns(4, nil)
		fakeStore.InsertSlotsStub = func(_ context.Context, slots []model.Slot) ([]model.Slot, error) {
			return slots, nil
		}
		svc = service.NewService(fakeStore)

		input = model.AddSlotsInput{Cards: []model.SlotCard{
			{CardID: cardID, CardPrintingID: &printingID, SetResolution: cardmodel.SetResolutionExact},
			{CardID: cardID, CardPrintingID: nil, SetResolution: cardmodel.SetResolutionByName},
			{CardID: cardID, CardPrintingID: &printingID, SetResolution: cardmodel.SetResolutionManual},
		}}
	})

	JustBeforeEach(func() {
		added, err = svc.AddSlots(ctx, ownerID, binderID, input)
	})

	When("a reviewed session is committed", func() {
		It("appends the cards after the last one, in the order they arrived", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(added).To(HaveLen(3))

			for i, slot := range added {
				Expect(slot.Position).To(Equal(4 + i))
				Expect(slot.BinderID).To(Equal(binderID))
				Expect(slot.CardID).To(Equal(cardID))
				Expect(slot.ID).NotTo(Equal(uuid.Nil))
				Expect(slot.SetResolution).To(Equal(input.Cards[i].SetResolution))
			}
			Expect(added[1].CardPrintingID).To(BeNil())
			Expect(added[2].CardPrintingID).To(HaveValue(Equal(printingID)))
		})

		// The whole point of the endpoint: the ownership check, the count and
		// the insert are one transaction, so a failure anywhere in the commit
		// leaves the binder untouched rather than holding part of a sweep.
		It("does all of it inside one transaction", func() {
			Expect(fakeStore.InTxCallCount()).To(Equal(1))
			Expect(fakeStore.InsertSlotsCallCount()).To(Equal(1))
		})

		It("writes the whole batch in one call, not one call per card", func() {
			_, slots := fakeStore.InsertSlotsArgsForCall(0)
			Expect(slots).To(HaveLen(3))
			Expect(fakeStore.InsertSlotCallCount()).To(BeZero())
		})

		It("shifts nothing: the positions it takes are already free", func() {
			Expect(fakeStore.OpenPositionGapCallCount()).To(BeZero())
			Expect(fakeStore.MoveSlotPositionCallCount()).To(BeZero())
		})
	})

	When("the binder is empty", func() {
		BeforeEach(func() {
			fakeStore.CountSlotsReturns(0, nil)
		})

		It("starts the sweep at position zero", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(added[0].Position).To(BeZero())
		})
	})

	// 000-principles.md section 8b. It still opens the transaction and checks
	// the binder, so an empty commit is not a way to find out whether somebody
	// else's binder exists.
	When("the batch is empty", func() {
		BeforeEach(func() {
			input = model.AddSlotsInput{Cards: nil}
		})

		It("writes nothing and does not call it an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(added).To(BeEmpty())
		})

		It("still checks who owns the binder", func() {
			Expect(fakeStore.GetBinderByIDCallCount()).To(Equal(1))
		})

		Context("and the binder belongs to somebody else", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(
					model.Binder{ID: binderID, OwnerID: uuid.New(), Name: "Somebody else's"}, nil)
			})

			It("is not found, the same answer a full batch gets", func() {
				Expect(err).To(MatchError(service.ErrBinderNotFound))
			})
		})
	})

	When("the binder belongs to somebody else", func() {
		BeforeEach(func() {
			fakeStore.GetBinderByIDReturns(
				model.Binder{ID: binderID, OwnerID: uuid.New(), Name: "Somebody else's"}, nil)
		})

		It("is not found, and nothing is written", func() {
			Expect(err).To(MatchError(service.ErrBinderNotFound))
			Expect(fakeStore.InsertSlotsCallCount()).To(BeZero())
		})
	})

	When("the binder does not exist", func() {
		BeforeEach(func() {
			fakeStore.GetBinderByIDReturns(model.Binder{},
				dataerror.WrapMissingEntityError("binder", errDB))
		})

		It("is not found, and nothing is written", func() {
			Expect(err).To(MatchError(service.ErrBinderNotFound))
			Expect(fakeStore.InsertSlotsCallCount()).To(BeZero())
		})
	})

	// The bound exists so one request cannot insert unboundedly. The boundary
	// refuses an over-long batch first; this is the second half of the same
	// guard (000-principles.md section 10).
	Describe("the batch bound", func() {
		When("the batch is exactly as long as the bound allows", func() {
			BeforeEach(func() {
				input = model.AddSlotsInput{Cards: namedCards(model.MaxSlotsPerBatch, cardID)}
			})

			It("is accepted", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(added).To(HaveLen(model.MaxSlotsPerBatch))
			})
		})

		When("the batch is one card longer than the bound allows", func() {
			BeforeEach(func() {
				input = model.AddSlotsInput{Cards: namedCards(model.MaxSlotsPerBatch+1, cardID)}
			})

			It("is refused before the transaction opens, so nothing is written", func() {
				Expect(err).To(MatchError(service.ErrBatchTooLarge))
				Expect(fakeStore.InTxCallCount()).To(BeZero())
			})
		})
	})

	// The Go half of binder_slots_printing_matches_resolution, applied to every
	// entry. Refusing here rather than letting the CHECK fire is what keeps a
	// wrong request a 4xx instead of a 500 — and refusing before the
	// transaction opens is what keeps one bad card from costing a write.
	Describe("an entry whose resolution and printing disagree", func() {
		AfterEach(func() {
			Expect(fakeStore.InTxCallCount()).To(BeZero())
		})

		When("a resolution that determines a set arrives without its printing", func() {
			BeforeEach(func() {
				input.Cards[2].CardPrintingID = nil
			})

			It("refuses the whole batch", func() {
				Expect(err).To(MatchError(service.ErrPrintingRequired))
			})
		})

		When("a resolution that determines no set arrives carrying a printing", func() {
			BeforeEach(func() {
				input.Cards[1].CardPrintingID = &printingID
			})

			It("refuses the whole batch", func() {
				Expect(err).To(MatchError(service.ErrPrintingNotAllowed))
			})
		})

		When("a resolution is not one a slot can record", func() {
			BeforeEach(func() {
				input.Cards[0].SetResolution = cardmodel.SetResolution("probably")
			})

			It("refuses the whole batch", func() {
				Expect(err).To(MatchError(service.ErrResolutionUnknown))
			})
		})
	})

	// The store's typed errors become this domain's sentinels, exactly as they
	// do for the single-card write: the api layer has one table to map.
	DescribeTable("what the store refuses",
		func(storeErr error, want error) {
			fakeStore.InsertSlotsReturns(nil, storeErr)

			_, batchErr := svc.AddSlots(ctx, ownerID, binderID, input)

			Expect(batchErr).To(MatchError(want))
		},
		Entry("a card that is not in the card database",
			dataerror.WrapInvalidReferenceError(model.ReferenceCard, errDB),
			service.ErrCardUnknown),
		Entry("a printing that is not a printing of that card",
			dataerror.WrapInvalidReferenceError(model.ReferenceCardPrinting, errDB),
			service.ErrPrintingUnknown),
		Entry("a position a concurrent change took",
			dataerror.WrapConflictError("binder slot position", errDB),
			service.ErrPositionTaken),
	)
})

// namedCards is a batch of count cards resolved by name, which is the shape
// that needs no printing. Used to test the bound, where what the cards are does
// not matter and how many there are is the whole point.
func namedCards(count int, cardID uuid.UUID) []model.SlotCard {
	cards := make([]model.SlotCard, 0, count)
	for range count {
		cards = append(cards, model.SlotCard{
			CardID:         cardID,
			CardPrintingID: nil,
			SetResolution:  cardmodel.SetResolutionByName,
		})
	}
	return cards
}
