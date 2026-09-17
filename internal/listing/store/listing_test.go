package store_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/internal/listing/store"
)

var _ = Describe("Listings", func() {
	var ctx = context.Background()

	var (
		subject *store.Store
		ownerID uuid.UUID
		slotID  uuid.UUID
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)
		ownerID = seedUser()
		binderID := seedBinder(ownerID, "Main binder")
		slotID = seedSlot(binderID, seedCard("Dark Magician"), 0)
	})

	AfterEach(truncateAll)

	Describe("InsertListing", func() {
		var (
			listing model.Listing
			stored  model.Listing
			err     error
		)

		BeforeEach(func() {
			listing = model.Listing{
				ID:           uuid.New(),
				BinderSlotID: slotID,
				// Zero on purpose: the database assigns the timestamps.
				CreatedAt: time.Time{},
				UpdatedAt: time.Time{},
			}
		})

		JustBeforeEach(func() {
			stored, err = subject.InsertListing(ctx, listing)
		})

		When("the slot is not for sale yet", func() {
			It("returns the listing as stored, with the timestamps the database assigned", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stored.ID).To(Equal(listing.ID))
				Expect(stored.BinderSlotID).To(Equal(slotID))
				Expect(stored.CreatedAt).NotTo(BeZero())
				Expect(stored.UpdatedAt).NotTo(BeZero())
			})
		})

		// The UNIQUE constraint is what makes one listing per slot true, so this
		// spec drives the real one: the translation below it is only reachable
		// through a driver error, and a spec that faked that would prove the
		// mapping and not the rule.
		When("the slot is already for sale", func() {
			BeforeEach(func() {
				seedListing(slotID)
			})

			It("reports a conflict, which is what the api layer answers 409 to", func() {
				Expect(err).To(HaveOccurred())
				Expect(dataerror.IsConflictError(err)).To(BeTrue(),
					"expected a dataerror.ConflictError from listings_binder_slot_id_key, got %v", err)
				Expect(countListings()).To(Equal(1), "the second listing must not have been written")
			})
		})

		When("there is no such slot", func() {
			BeforeEach(func() {
				listing.BinderSlotID = uuid.New()
			})

			It("reports the slot reference as unusable", func() {
				Expect(err).To(HaveOccurred())
				Expect(dataerror.IsInvalidReferenceError(err, model.ReferenceBinderSlot)).To(BeTrue(),
					"expected an InvalidReferenceError over %q, got %v", model.ReferenceBinderSlot, err)
			})
		})
	})

	Describe("DeleteListingByID", func() {
		var (
			listingID uuid.UUID
			err       error
		)

		BeforeEach(func() {
			listingID = seedListing(slotID)
		})

		JustBeforeEach(func() {
			err = subject.DeleteListingByID(ctx, listingID)
		})

		When("the listing is there", func() {
			It("takes it off sale and leaves the card in the binder", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(countListings()).To(BeZero())

				var slots int
				dbErr := testDB.GetContext(ctx, &slots, `SELECT count(*) FROM binder_slots WHERE id = $1`, slotID)
				Expect(dbErr).NotTo(HaveOccurred())
				Expect(slots).To(Equal(1), "unlisting must not remove the card from the binder")
			})
		})

		When("there is no such listing", func() {
			BeforeEach(func() {
				listingID = uuid.New()
			})

			It("reports it missing rather than reporting an unlisting that did not happen", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(),
					"expected a MissingEntityError, got %v", err)
			})
		})
	})

	// ON DELETE CASCADE in db/migrations/004_listings.sql: taking the card out of
	// the binder takes its listing with it, so nothing can be for sale that its
	// seller no longer holds. Proven against the real schema, because the schema
	// is the only place this rule is written.
	Describe("a card taken out of the binder", func() {
		It("takes its listing with it", func() {
			listingID := seedListing(slotID)
			Expect(countListings()).To(Equal(1))

			_, dbErr := testDB.ExecContext(ctx, `DELETE FROM binder_slots WHERE id = $1`, slotID)
			Expect(dbErr).NotTo(HaveOccurred())

			Expect(countListings()).To(BeZero(), "the listing should have cascaded away with the slot")

			_, err := subject.ListingSeller(ctx, listingID)
			Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(),
				"a cascaded-away listing must read as missing, got %v", err)
		})
	})

	Describe("SlotSeller", func() {
		var (
			seller uuid.UUID
			lookup uuid.UUID
			err    error
		)

		BeforeEach(func() {
			lookup = slotID
		})

		JustBeforeEach(func() {
			seller, err = subject.SlotSeller(ctx, lookup)
		})

		When("the slot is in a binder", func() {
			It("is the owner of that binder", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(seller).To(Equal(ownerID))
			})
		})

		When("there is no such slot", func() {
			BeforeEach(func() {
				lookup = uuid.New()
			})

			It("reports the slot missing", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(),
					"expected a MissingEntityError, got %v", err)
				Expect(seller).To(Equal(uuid.Nil))
			})
		})
	})

	Describe("ListingSeller", func() {
		var (
			seller uuid.UUID
			lookup uuid.UUID
			err    error
		)

		BeforeEach(func() {
			lookup = seedListing(slotID)
		})

		JustBeforeEach(func() {
			seller, err = subject.ListingSeller(ctx, lookup)
		})

		When("the listing is there", func() {
			It("is the owner of the binder the listed card is in", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(seller).To(Equal(ownerID))
			})
		})

		When("there is no such listing", func() {
			BeforeEach(func() {
				lookup = uuid.New()
			})

			It("reports the listing missing", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(),
					"expected a MissingEntityError, got %v", err)
				Expect(seller).To(Equal(uuid.Nil))
			})
		})
	})
})
