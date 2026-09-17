package service_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/internal/listing/service"
	"github.com/simeonkorchev/binder/internal/listing/service/servicefakes"
)

var _ = Describe("Listings", func() {
	var ctx = context.Background()

	var (
		subject   *service.Service
		fakeStore *servicefakes.FakeStore
		sellerID  uuid.UUID
		slotID    uuid.UUID
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		passThroughTx(fakeStore)
		subject = service.NewService(fakeStore, new(servicefakes.FakeSellerContacts))
		sellerID = uuid.New()
		slotID = uuid.New()
	})

	Describe("CreateListing", func() {
		var (
			created model.Listing
			err     error
		)

		BeforeEach(func() {
			fakeStore.SlotSellerReturns(sellerID, nil)
			fakeStore.InsertListingStub = func(_ context.Context, listing model.Listing) (model.Listing, error) {
				stored := listing
				stored.CreatedAt = time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)
				stored.UpdatedAt = stored.CreatedAt
				return stored, nil
			}
		})

		JustBeforeEach(func() {
			created, err = subject.CreateListing(ctx, sellerID, slotID)
		})

		When("the card is the actor's own", func() {
			It("lists it, minting the id server-side", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(created.BinderSlotID).To(Equal(slotID))
				Expect(created.ID).NotTo(Equal(uuid.Nil))
				Expect(created.CreatedAt).To(Equal(time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)))

				Expect(fakeStore.InsertListingCallCount()).To(Equal(1))
				_, written := fakeStore.InsertListingArgsForCall(0)
				Expect(written.BinderSlotID).To(Equal(slotID))
				Expect(written.ID).To(Equal(created.ID))
				Expect(written.CreatedAt).To(BeZero(), "the database assigns the timestamps")
			})

			It("reads the owner and writes inside one transaction", func() {
				Expect(fakeStore.InTxCallCount()).To(Equal(1))
				Expect(fakeStore.SlotSellerCallCount()).To(Equal(1))
				_, asked := fakeStore.SlotSellerArgsForCall(0)
				Expect(asked).To(Equal(slotID))
			})
		})

		// 404 and not 403: a distinct "forbidden" would confirm to a stranger
		// that the slot exists.
		When("the card is in somebody else's binder", func() {
			BeforeEach(func() {
				fakeStore.SlotSellerReturns(uuid.New(), nil)
			})

			It("is not found, and nothing is written", func() {
				Expect(err).To(MatchError(service.ErrBinderSlotNotFound))
				Expect(fakeStore.InsertListingCallCount()).To(BeZero())
			})
		})

		When("there is no such card in any binder", func() {
			BeforeEach(func() {
				fakeStore.SlotSellerReturns(uuid.Nil, dataerror.WrapMissingEntityError("binder slot", errDB))
			})

			It("is not found, and nothing is written", func() {
				Expect(err).To(MatchError(service.ErrBinderSlotNotFound))
				Expect(fakeStore.InsertListingCallCount()).To(BeZero())
			})
		})

		When("the card is already for sale", func() {
			BeforeEach(func() {
				fakeStore.InsertListingReturns(model.Listing{},
					dataerror.WrapConflictError("binder slot listing", errDB))
			})

			It("says so, which is the one answer a retry cannot fix", func() {
				Expect(err).To(MatchError(service.ErrAlreadyListed))
			})
		})

		// The slot was there when its owner was read, so it has just been taken
		// out of the binder: "no longer there" is the honest answer.
		When("the card leaves the binder between the check and the write", func() {
			BeforeEach(func() {
				fakeStore.InsertListingReturns(model.Listing{},
					dataerror.WrapInvalidReferenceError(model.ReferenceBinderSlot, errDB))
			})

			It("is not found", func() {
				Expect(err).To(MatchError(service.ErrBinderSlotNotFound))
			})
		})

		When("the owner cannot be read", func() {
			BeforeEach(func() {
				fakeStore.SlotSellerReturns(uuid.Nil, errDB)
			})

			It("fails with the cause attached and nothing written", func() {
				Expect(err).To(MatchError(errDB))
				Expect(err).NotTo(MatchError(service.ErrBinderSlotNotFound))
				Expect(fakeStore.InsertListingCallCount()).To(BeZero())
			})
		})

		When("the write fails for an unforeseen reason", func() {
			BeforeEach(func() {
				fakeStore.InsertListingReturns(model.Listing{}, errDB)
			})

			It("fails with the cause attached rather than as a sentinel", func() {
				Expect(err).To(MatchError(errDB))
				Expect(err).NotTo(MatchError(service.ErrAlreadyListed))
			})
		})
	})

	Describe("DeleteListing", func() {
		var (
			listingID uuid.UUID
			err       error
		)

		BeforeEach(func() {
			listingID = uuid.New()
			fakeStore.ListingSellerReturns(sellerID, nil)
		})

		JustBeforeEach(func() {
			err = subject.DeleteListing(ctx, sellerID, listingID)
		})

		When("the listing is the actor's own", func() {
			It("takes it down", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeStore.DeleteListingByIDCallCount()).To(Equal(1))
				_, deleted := fakeStore.DeleteListingByIDArgsForCall(0)
				Expect(deleted).To(Equal(listingID))
				Expect(fakeStore.InTxCallCount()).To(Equal(1))
			})
		})

		When("the listing is somebody else's", func() {
			BeforeEach(func() {
				fakeStore.ListingSellerReturns(uuid.New(), nil)
			})

			It("is not found, and nothing is deleted", func() {
				Expect(err).To(MatchError(service.ErrListingNotFound))
				Expect(fakeStore.DeleteListingByIDCallCount()).To(BeZero())
			})
		})

		When("there is no such listing", func() {
			BeforeEach(func() {
				fakeStore.ListingSellerReturns(uuid.Nil, dataerror.WrapMissingEntityError("listing", errDB))
			})

			It("is not found, and nothing is deleted", func() {
				Expect(err).To(MatchError(service.ErrListingNotFound))
				Expect(fakeStore.DeleteListingByIDCallCount()).To(BeZero())
			})
		})

		// The card was removed from the binder meanwhile, so ON DELETE CASCADE
		// already took the listing away.
		When("the listing cascades away before the delete runs", func() {
			BeforeEach(func() {
				fakeStore.DeleteListingByIDReturns(dataerror.WrapMissingEntityError("listing", errDB))
			})

			It("is not found", func() {
				Expect(err).To(MatchError(service.ErrListingNotFound))
			})
		})

		When("the delete fails for an unforeseen reason", func() {
			BeforeEach(func() {
				fakeStore.DeleteListingByIDReturns(errDB)
			})

			It("fails with the cause attached", func() {
				Expect(err).To(MatchError(errDB))
				Expect(err).NotTo(MatchError(service.ErrListingNotFound))
			})
		})
	})

	Describe("BrowseListings", func() {
		var (
			search model.ListingSearch
			listed []model.ListedCard
			err    error
		)

		BeforeEach(func() {
			search = model.ListingSearch{Query: "dragon", SetPrefix: "LOB", Limit: 50}
		})

		JustBeforeEach(func() {
			listed, err = subject.BrowseListings(ctx, search)
		})

		When("cards are for sale", func() {
			var feed []model.ListedCard

			BeforeEach(func() {
				objectKey := "cards/blue-eyes.jpg"
				setCode := "LOB-EN001"
				feed = []model.ListedCard{{
					ListingID: uuid.New(),
					SellerID:  uuid.New(),
					Card: cardmodel.Card{
						ID:             uuid.New(),
						Name:           "Blue-Eyes White Dragon",
						ImageObjectKey: &objectKey,
					},
					SetCode:  &setCode,
					ListedAt: time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC),
				}}
				fakeStore.BrowseListingsReturns(feed, nil)
			})

			It("returns the feed and passes the filter through untouched", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(listed).To(Equal(feed))

				Expect(fakeStore.BrowseListingsCallCount()).To(Equal(1))
				_, asked := fakeStore.BrowseListingsArgsForCall(0)
				Expect(asked).To(Equal(search))
			})
		})

		When("nothing matches", func() {
			BeforeEach(func() {
				fakeStore.BrowseListingsReturns([]model.ListedCard{}, nil)
			})

			It("is an empty feed and not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(listed).To(BeEmpty())
			})
		})

		When("the feed cannot be read", func() {
			BeforeEach(func() {
				fakeStore.BrowseListingsReturns(nil, errDB)
			})

			It("fails with the cause attached", func() {
				Expect(err).To(MatchError(errDB))
				Expect(listed).To(BeNil())
			})
		})
	})
})
