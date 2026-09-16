package service_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/binder/service"
	"github.com/simeonkorchev/binder/internal/binder/service/servicefakes"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

var _ = Describe("Binders", func() {
	var (
		ctx      = context.Background()
		ownerID  = uuid.New()
		binderID = uuid.New()
		owned    = model.Binder{ID: binderID, OwnerID: ownerID, Name: "Main binder"}
	)

	var (
		fakeStore *servicefakes.FakeStore
		svc       *service.Service
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		passThroughTx(fakeStore)
		fakeStore.GetBinderByIDReturns(owned, nil)
		svc = service.NewService(fakeStore)
	})

	Describe("CreateBinder", func() {
		var (
			created model.Binder
			err     error
		)

		JustBeforeEach(func() {
			created, err = svc.CreateBinder(ctx, ownerID, "Trade binder")
		})

		When("the store accepts the binder", func() {
			BeforeEach(func() {
				fakeStore.CreateBinderStub = func(_ context.Context, b model.Binder) (model.Binder, error) {
					return b, nil
				}
			})

			It("returns the binder the store stored", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(created.OwnerID).To(Equal(ownerID))
				Expect(created.Name).To(Equal("Trade binder"))
			})

			It("mints the id server-side rather than taking one from the caller", func() {
				Expect(fakeStore.CreateBinderCallCount()).To(Equal(1))
				_, passed := fakeStore.CreateBinderArgsForCall(0)
				Expect(passed.ID).NotTo(Equal(uuid.Nil))
				Expect(passed.OwnerID).To(Equal(ownerID))
			})
		})

		When("the store fails", func() {
			BeforeEach(func() {
				fakeStore.CreateBinderReturns(model.Binder{}, errDB)
			})

			It("reports the failure", func() {
				Expect(err).To(MatchError(errDB))
				Expect(err.Error()).To(ContainSubstring("creating binder"))
			})
		})
	})

	Describe("ListBinders", func() {
		var (
			binders []model.Binder
			err     error
		)

		JustBeforeEach(func() {
			binders, err = svc.ListBinders(ctx, ownerID)
		})

		When("the owner has binders", func() {
			BeforeEach(func() {
				fakeStore.ListBindersByOwnerReturns([]model.Binder{owned}, nil)
			})

			It("returns them, asking the store for this owner's", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(binders).To(ConsistOf(owned))

				_, passed := fakeStore.ListBindersByOwnerArgsForCall(0)
				Expect(passed).To(Equal(ownerID))
			})
		})

		When("the owner has none", func() {
			BeforeEach(func() {
				fakeStore.ListBindersByOwnerReturns([]model.Binder{}, nil)
			})

			It("is an empty list, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(binders).To(BeEmpty())
			})
		})
	})

	Describe("RenameBinder", func() {
		var (
			renamed model.Binder
			err     error
		)

		JustBeforeEach(func() {
			renamed, err = svc.RenameBinder(ctx, ownerID, binderID, "Trade binder")
		})

		When("the binder is the owner's", func() {
			BeforeEach(func() {
				fakeStore.UpdateBinderNameReturns(model.Binder{
					ID: binderID, OwnerID: ownerID, Name: "Trade binder",
				}, nil)
			})

			It("renames it", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(renamed.Name).To(Equal("Trade binder"))

				_, passedID, passedName := fakeStore.UpdateBinderNameArgsForCall(0)
				Expect(passedID).To(Equal(binderID))
				Expect(passedName).To(Equal("Trade binder"))
			})
		})

		When("the binder belongs to somebody else", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(model.Binder{
					ID: binderID, OwnerID: uuid.New(), Name: "Someone else's",
				}, nil)
			})

			It("is refused as not found, so nothing is confirmed to a stranger", func() {
				Expect(err).To(MatchError(service.ErrBinderNotFound))
			})

			It("never writes", func() {
				Expect(fakeStore.UpdateBinderNameCallCount()).To(BeZero())
			})
		})

		When("the binder does not exist", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(model.Binder{},
					dataerror.WrapMissingEntityError("binder", errDB))
			})

			It("is not found, and nothing is written", func() {
				Expect(err).To(MatchError(service.ErrBinderNotFound))
				Expect(fakeStore.UpdateBinderNameCallCount()).To(BeZero())
			})
		})

		When("the ownership read itself fails", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(model.Binder{}, errDB)
			})

			It("reports the failure rather than calling it a 404", func() {
				Expect(err).To(MatchError(errDB))
				Expect(err).NotTo(MatchError(service.ErrBinderNotFound))
			})
		})
	})

	Describe("GetPage", func() {
		var (
			page    model.Page
			err     error
			request int
		)

		BeforeEach(func() {
			request = 0
		})

		JustBeforeEach(func() {
			page, err = svc.GetPage(ctx, ownerID, binderID, request)
		})

		When("the binder holds more than one page", func() {
			var firstSlot model.Slot

			BeforeEach(func() {
				firstSlot = model.Slot{ID: uuid.New(), BinderID: binderID, Position: 9, CardID: uuid.New()}
				fakeStore.CountSlotsReturns(12, nil)
				fakeStore.ListSlotsInRangeReturns([]model.Slot{firstSlot}, nil)
				request = 1
			})

			It("returns the page with its slots and the binder's page count", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(page.Number).To(Equal(1))
				Expect(page.PageCount).To(Equal(2))
				Expect(page.Slots).To(ConsistOf(firstSlot))
			})

			It("asks the store for exactly that page's range", func() {
				_, passedBinderID, from, to := fakeStore.ListSlotsInRangeArgsForCall(0)
				Expect(passedBinderID).To(Equal(binderID))
				Expect(from).To(Equal(9))
				Expect(to).To(Equal(17))
			})
		})

		When("the binder is empty", func() {
			BeforeEach(func() {
				fakeStore.CountSlotsReturns(0, nil)
				fakeStore.ListSlotsInRangeReturns([]model.Slot{}, nil)
			})

			It("is a page with no slots and no pages, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(page.Slots).To(BeEmpty())
				Expect(page.PageCount).To(BeZero())
			})
		})

		When("the page is past the end of the binder", func() {
			BeforeEach(func() {
				fakeStore.CountSlotsReturns(3, nil)
				fakeStore.ListSlotsInRangeReturns([]model.Slot{}, nil)
				request = 7
			})

			It("is an empty page, not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(page.Slots).To(BeEmpty())
				Expect(page.PageCount).To(Equal(1))
			})
		})

		When("the page number is negative", func() {
			BeforeEach(func() {
				request = -1
			})

			It("is refused without reading anything", func() {
				Expect(err).To(MatchError(service.ErrPositionOutOfRange))
				Expect(fakeStore.GetBinderByIDCallCount()).To(BeZero())
				Expect(fakeStore.ListSlotsInRangeCallCount()).To(BeZero())
			})
		})

		When("the binder is not the owner's", func() {
			BeforeEach(func() {
				fakeStore.GetBinderByIDReturns(model.Binder{ID: binderID, OwnerID: uuid.New()}, nil)
			})

			It("is refused before any slot is read", func() {
				Expect(err).To(MatchError(service.ErrBinderNotFound))
				Expect(fakeStore.ListSlotsInRangeCallCount()).To(BeZero())
				Expect(fakeStore.CountSlotsCallCount()).To(BeZero())
			})
		})
	})
})
