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

var _ = Describe("Binder slots", func() {
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
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		passThroughTx(fakeStore)
		fakeStore.GetBinderByIDReturns(owned, nil)
		fakeStore.InsertSlotStub = func(_ context.Context, slot model.Slot) (model.Slot, error) {
			return slot, nil
		}
		svc = service.NewService(fakeStore)
	})

	Describe("AddSlot", func() {
		var (
			input model.AddSlotInput
			added model.Slot
			err   error
		)

		BeforeEach(func() {
			input = model.AddSlotInput{
				SlotCard: model.SlotCard{
					CardID:         cardID,
					CardPrintingID: &printingID,
					SetResolution:  cardmodel.SetResolutionExact,
				},
				Position: nil,
			}
			fakeStore.CountSlotsReturns(4, nil)
		})

		JustBeforeEach(func() {
			added, err = svc.AddSlot(ctx, ownerID, binderID, input)
		})

		When("no position is given", func() {
			It("appends the card after the last one", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(added.Position).To(Equal(4))
				Expect(added.BinderID).To(Equal(binderID))
				Expect(added.CardID).To(Equal(cardID))
				Expect(added.CardPrintingID).To(HaveValue(Equal(printingID)))
				Expect(added.SetResolution).To(Equal(cardmodel.SetResolutionExact))
				Expect(added.ID).NotTo(Equal(uuid.Nil))
			})

			It("opens no gap: the position after the last card is already free", func() {
				Expect(fakeStore.OpenPositionGapCallCount()).To(BeZero())
			})
		})

		When("a position inside the binder is given", func() {
			BeforeEach(func() {
				position := 1
				input.Position = &position
			})

			It("opens a gap there first, then inserts into it", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeStore.OpenPositionGapCallCount()).To(Equal(1))
				_, passedBinderID, at := fakeStore.OpenPositionGapArgsForCall(0)
				Expect(passedBinderID).To(Equal(binderID))
				Expect(at).To(Equal(1))

				_, slot := fakeStore.InsertSlotArgsForCall(0)
				Expect(slot.Position).To(Equal(1))
			})
		})

		When("the position given is one past the last card", func() {
			BeforeEach(func() {
				position := 4
				input.Position = &position
			})

			It("appends without opening a gap", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(added.Position).To(Equal(4))
				Expect(fakeStore.OpenPositionGapCallCount()).To(BeZero())
			})
		})

		When("the position given would leave a hole", func() {
			BeforeEach(func() {
				position := 5
				input.Position = &position
			})

			It("is refused, and nothing is written", func() {
				Expect(err).To(MatchError(service.ErrPositionOutOfRange))
				Expect(fakeStore.InsertSlotCallCount()).To(BeZero())
				Expect(fakeStore.OpenPositionGapCallCount()).To(BeZero())
			})
		})

		When("the position given is negative", func() {
			BeforeEach(func() {
				position := -1
				input.Position = &position
			})

			It("is refused", func() {
				Expect(err).To(MatchError(service.ErrPositionOutOfRange))
				Expect(fakeStore.InsertSlotCallCount()).To(BeZero())
			})
		})

		// The Go half of binder_slots_printing_matches_resolution. Refusing
		// here rather than letting the CHECK fire is what keeps a wrong request
		// a 4xx instead of a 500.
		When("a resolution that determines a set arrives without its printing", func() {
			BeforeEach(func() {
				input.CardPrintingID = nil
			})

			It("is refused before the transaction opens", func() {
				Expect(err).To(MatchError(service.ErrPrintingRequired))
				Expect(fakeStore.InTxCallCount()).To(BeZero())
			})
		})

		When("a resolution that determines no set arrives carrying a printing", func() {
			BeforeEach(func() {
				input.SetResolution = cardmodel.SetResolutionByName
			})

			It("is refused before the transaction opens", func() {
				Expect(err).To(MatchError(service.ErrPrintingNotAllowed))
				Expect(fakeStore.InTxCallCount()).To(BeZero())
			})
		})

		When("the resolution is not a rung of the ladder", func() {
			BeforeEach(func() {
				input.SetResolution = cardmodel.SetResolution("probably")
			})

			It("is refused", func() {
				Expect(err).To(MatchError(service.ErrResolutionUnknown))
				Expect(fakeStore.InTxCallCount()).To(BeZero())
			})
		})

		When("the resolution is the zero value", func() {
			BeforeEach(func() {
				input.SetResolution = cardmodel.SetResolutionUnset
				input.CardPrintingID = nil
			})

			It("is refused: nobody decided is not a decision", func() {
				Expect(err).To(MatchError(service.ErrResolutionUnknown))
			})
		})

		DescribeTable("a resolution that determines no set is accepted without a printing",
			func(resolution cardmodel.SetResolution) {
				accepted, addErr := svc.AddSlot(ctx, ownerID, binderID, model.AddSlotInput{
					SlotCard: model.SlotCard{CardID: cardID, SetResolution: resolution},
				})

				Expect(addErr).NotTo(HaveOccurred())
				Expect(accepted.CardPrintingID).To(BeNil())
				Expect(accepted.SetResolution).To(Equal(resolution))
			},
			Entry("by name", cardmodel.SetResolutionByName),
			Entry("unresolved", cardmodel.SetResolutionUnresolved),
		)

		When("the card does not exist", func() {
			BeforeEach(func() {
				fakeStore.InsertSlotReturns(model.Slot{},
					dataerror.WrapInvalidReferenceError(model.ReferenceCard, errDB))
			})

			It("says so, rather than leaking the persistence error", func() {
				Expect(err).To(MatchError(service.ErrCardUnknown))
			})
		})

		When("the printing is not a printing of that card", func() {
			BeforeEach(func() {
				fakeStore.InsertSlotReturns(model.Slot{},
					dataerror.WrapInvalidReferenceError(model.ReferenceCardPrinting, errDB))
			})

			It("says so", func() {
				Expect(err).To(MatchError(service.ErrPrintingUnknown))
			})
		})

		When("a concurrent write took the position", func() {
			BeforeEach(func() {
				fakeStore.InsertSlotReturns(model.Slot{},
					dataerror.WrapConflictError("binder slot position", errDB))
			})

			It("is a conflict the caller can retry", func() {
				Expect(err).To(MatchError(service.ErrPositionTaken))
			})
		})

		When("the binder is not the owner's", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(model.Binder{ID: binderID, OwnerID: uuid.New()}, nil)
			})

			It("is refused without writing", func() {
				Expect(err).To(MatchError(service.ErrBinderNotFound))
				Expect(fakeStore.InsertSlotCallCount()).To(BeZero())
				Expect(fakeStore.OpenPositionGapCallCount()).To(BeZero())
			})
		})
	})

	Describe("MoveSlot", func() {
		var (
			slotID uuid.UUID
			input  model.MoveSlotInput
			err    error
		)

		BeforeEach(func() {
			slotID = uuid.New()
			input = model.MoveSlotInput{SlotID: slotID, ToPosition: 3}
			fakeStore.CountSlotsReturns(5, nil)
			fakeStore.GetSlotByIDReturns(model.Slot{
				ID: slotID, BinderID: binderID, Position: 1, CardID: cardID,
			}, nil)
		})

		JustBeforeEach(func() {
			err = svc.MoveSlot(ctx, ownerID, binderID, input)
		})

		When("the destination is inside the binder", func() {
			It("moves the slot from where it is to where it is asked for", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeStore.MoveSlotPositionCallCount()).To(Equal(1))
				_, passedBinderID, passedSlotID, from, to := fakeStore.MoveSlotPositionArgsForCall(0)
				Expect(passedBinderID).To(Equal(binderID))
				Expect(passedSlotID).To(Equal(slotID))
				Expect(from).To(Equal(1))
				Expect(to).To(Equal(3))
			})
		})

		When("the destination is the position the slot already holds", func() {
			BeforeEach(func() {
				input.ToPosition = 1
			})

			It("does nothing rather than rewriting a range for no reason", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeStore.MoveSlotPositionCallCount()).To(BeZero())
			})
		})

		When("the destination is past the last card", func() {
			BeforeEach(func() {
				input.ToPosition = 5
			})

			It("is refused: a move cannot extend the binder", func() {
				Expect(err).To(MatchError(service.ErrPositionOutOfRange))
				Expect(fakeStore.MoveSlotPositionCallCount()).To(BeZero())
			})
		})

		When("the destination is negative", func() {
			BeforeEach(func() {
				input.ToPosition = -1
			})

			It("is refused", func() {
				Expect(err).To(MatchError(service.ErrPositionOutOfRange))
				Expect(fakeStore.MoveSlotPositionCallCount()).To(BeZero())
			})
		})

		When("the slot is not in that binder", func() {
			BeforeEach(func() {
				fakeStore.GetSlotByIDReturns(model.Slot{},
					dataerror.WrapMissingEntityError("binder slot", errDB))
			})

			It("is not found, and nothing is moved", func() {
				Expect(err).To(MatchError(service.ErrSlotNotFound))
				Expect(fakeStore.MoveSlotPositionCallCount()).To(BeZero())
			})
		})

		When("the binder is not the owner's", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(model.Binder{ID: binderID, OwnerID: uuid.New()}, nil)
			})

			It("is refused before the slot is even read", func() {
				Expect(err).To(MatchError(service.ErrBinderNotFound))
				Expect(fakeStore.GetSlotByIDCallCount()).To(BeZero())
				Expect(fakeStore.MoveSlotPositionCallCount()).To(BeZero())
			})
		})

		When("the move itself fails", func() {
			BeforeEach(func() {
				fakeStore.MoveSlotPositionReturns(errDB)
			})

			It("reports the failure", func() {
				Expect(err).To(MatchError(errDB))
				Expect(err.Error()).To(ContainSubstring("moving the slot"))
			})
		})
	})

	Describe("RemoveSlot", func() {
		var (
			slotID uuid.UUID
			err    error
		)

		BeforeEach(func() {
			slotID = uuid.New()
			fakeStore.GetSlotByIDReturns(model.Slot{
				ID: slotID, BinderID: binderID, Position: 2, CardID: cardID,
			}, nil)
		})

		JustBeforeEach(func() {
			err = svc.RemoveSlot(ctx, ownerID, binderID, slotID)
		})

		When("the slot is in the binder", func() {
			It("removes it", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeStore.DeleteSlotByIDCallCount()).To(Equal(1))
				_, passedBinderID, passedSlotID := fakeStore.DeleteSlotByIDArgsForCall(0)
				Expect(passedBinderID).To(Equal(binderID))
				Expect(passedSlotID).To(Equal(slotID))
			})

			// The database cannot enforce that positions are dense, so a
			// removal that does not close its own hole leaves a binder nothing
			// downstream will notice is broken.
			It("closes the gap it left, at the position the slot held", func() {
				Expect(fakeStore.ClosePositionGapCallCount()).To(Equal(1))
				_, passedBinderID, at := fakeStore.ClosePositionGapArgsForCall(0)
				Expect(passedBinderID).To(Equal(binderID))
				Expect(at).To(Equal(2))
			})

			It("does both in one transaction, so no reader sees the hole", func() {
				Expect(fakeStore.InTxCallCount()).To(Equal(1))
			})
		})

		When("the slot is not in that binder", func() {
			BeforeEach(func() {
				fakeStore.GetSlotByIDReturns(model.Slot{},
					dataerror.WrapMissingEntityError("binder slot", errDB))
			})

			It("is not found, and nothing is deleted or shifted", func() {
				Expect(err).To(MatchError(service.ErrSlotNotFound))
				Expect(fakeStore.DeleteSlotByIDCallCount()).To(BeZero())
				Expect(fakeStore.ClosePositionGapCallCount()).To(BeZero())
			})
		})

		When("the binder is not the owner's", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(model.Binder{ID: binderID, OwnerID: uuid.New()}, nil)
			})

			It("is refused without deleting anything", func() {
				Expect(err).To(MatchError(service.ErrBinderNotFound))
				Expect(fakeStore.DeleteSlotByIDCallCount()).To(BeZero())
			})
		})

		When("the delete fails", func() {
			BeforeEach(func() {
				fakeStore.DeleteSlotByIDReturns(errDB)
			})

			It("reports the failure and does not shift anything", func() {
				Expect(err).To(MatchError(errDB))
				Expect(fakeStore.ClosePositionGapCallCount()).To(BeZero())
			})
		})
	})
})
