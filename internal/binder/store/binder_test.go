package store_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/binder/store"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

var _ = Describe("Binders", func() {
	var ctx = context.Background()

	var (
		subject *store.Store
		ownerID uuid.UUID
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)
		ownerID = seedUser()
	})

	AfterEach(truncateAll)

	Describe("CreateBinder", func() {
		var (
			created model.Binder
			err     error
			input   model.Binder
		)

		BeforeEach(func() {
			input = model.Binder{ID: uuid.New(), OwnerID: ownerID, Name: "Main binder"}
		})

		JustBeforeEach(func() {
			created, err = subject.CreateBinder(ctx, input)
		})

		When("the owner exists", func() {
			It("returns the binder as stored, with the database's timestamps", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(created.ID).To(Equal(input.ID))
				Expect(created.OwnerID).To(Equal(ownerID))
				Expect(created.Name).To(Equal("Main binder"))
				Expect(created.CreatedAt).NotTo(BeZero())
				Expect(created.UpdatedAt).NotTo(BeZero())
			})
		})

		When("the owner does not exist", func() {
			BeforeEach(func() {
				input.OwnerID = uuid.New()
			})

			It("fails rather than orphaning a binder", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetBinderByID", func() {
		var (
			binderID uuid.UUID
			binder   model.Binder
			err      error
		)

		BeforeEach(func() {
			binderID = seedBinder(ownerID, "Main binder")
		})

		JustBeforeEach(func() {
			binder, err = subject.GetBinderByID(ctx, binderID)
		})

		When("the binder is there", func() {
			It("returns every column", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(binder.ID).To(Equal(binderID))
				Expect(binder.OwnerID).To(Equal(ownerID))
				Expect(binder.Name).To(Equal("Main binder"))
				Expect(binder.CreatedAt).NotTo(BeZero())
				Expect(binder.UpdatedAt).NotTo(BeZero())
			})
		})

		When("the binder is not there", func() {
			BeforeEach(func() {
				binderID = uuid.New()
			})

			It("is a missing entity, not a driver error", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(), "got %v", err)
			})
		})
	})

	Describe("ListBindersByOwner", func() {
		var (
			binders []model.Binder
			err     error
		)

		JustBeforeEach(func() {
			binders, err = subject.ListBindersByOwner(ctx, ownerID)
		})

		When("the owner has binders", func() {
			var firstID, secondID uuid.UUID

			BeforeEach(func() {
				firstID = seedBinder(ownerID, "First")
				secondID = seedBinder(ownerID, "Second")
				seedBinder(seedUser(), "Someone else's")
			})

			It("returns only theirs, oldest first", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(binders).To(HaveLen(2))
				Expect(binders[0].ID).To(Equal(firstID))
				Expect(binders[0].Name).To(Equal("First"))
				Expect(binders[1].ID).To(Equal(secondID))
			})
		})

		When("the owner has no binders", func() {
			It("is an empty slice and a nil error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(binders).To(BeEmpty())
			})
		})
	})

	Describe("UpdateBinderName", func() {
		var (
			binderID uuid.UUID
			renamed  model.Binder
			err      error
		)

		BeforeEach(func() {
			binderID = seedBinder(ownerID, "Main binder")
		})

		JustBeforeEach(func() {
			renamed, err = subject.UpdateBinderName(ctx, binderID, "Trade binder")
		})

		When("the binder is there", func() {
			It("returns it renamed", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(renamed.ID).To(Equal(binderID))
				Expect(renamed.Name).To(Equal("Trade binder"))
				Expect(renamed.OwnerID).To(Equal(ownerID))
			})

			It("persists the new name", func() {
				stored, readErr := subject.GetBinderByID(ctx, binderID)
				Expect(readErr).NotTo(HaveOccurred())
				Expect(stored.Name).To(Equal("Trade binder"))
			})
		})

		When("the binder is not there", func() {
			BeforeEach(func() {
				binderID = uuid.New()
			})

			It("is a missing entity", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(), "got %v", err)
			})
		})
	})
})
