package store_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/internal/user/store"
)

// Timestamps the database assigned are compared with BeTemporally rather than
// Equal throughout: Postgres keeps microseconds and Go carries nanoseconds, so a
// round trip is equal in time without being equal in representation.
var _ = Describe("Store", func() {
	ctx := context.Background()
	appleIdentity := model.Identity{Provider: model.ProviderApple, Subject: "001234.apple.subject.5678"}

	var subject *store.Store

	BeforeEach(func() {
		truncateUsers()
		subject = store.NewStore(testDB)
	})

	Describe("EnsureUserByIdentity", func() {
		When("the identity has never signed in", func() {
			var (
				created model.User
				err     error
			)

			JustBeforeEach(func() {
				created, err = subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
			})

			It("creates the account", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(created.ID).NotTo(Equal(uuid.Nil))
				Expect(created.Identity).To(Equal(appleIdentity))
			})

			It("shares no contact details: not having opted in is the starting state", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(created.Contact).To(Equal(model.Contact{Email: nil, Phone: nil}))
			})

			It("returns the timestamps the database assigned", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(created.CreatedAt).NotTo(BeZero())
				Expect(created.UpdatedAt).NotTo(BeZero())
			})
		})

		When("the identity signs in again", func() {
			var (
				first  model.User
				second model.User
			)

			BeforeEach(func() {
				var err error
				first, err = subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				var err error
				second, err = subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
				Expect(err).NotTo(HaveOccurred())
			})

			It("reuses the account rather than minting a second one", func() {
				Expect(second.ID).To(Equal(first.ID))
				Expect(second.CreatedAt).To(BeTemporally("==", first.CreatedAt))
			})

			It("leaves updated_at alone: a sign-in changes nothing about the account", func() {
				Expect(second.UpdatedAt).To(BeTemporally("==", first.UpdatedAt))
			})

			It("keeps the contact details the account had", func() {
				updated, err := subject.SetUserContact(ctx, first.ID,
					model.Contact{Email: ptr("seller@example.com"), Phone: nil})
				Expect(err).NotTo(HaveOccurred())

				again, err := subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
				Expect(err).NotTo(HaveOccurred())
				Expect(again.Contact.Email).To(HaveValue(Equal("seller@example.com")))
				Expect(again.UpdatedAt).To(BeTemporally("==", updated.UpdatedAt))
			})
		})

		When("the same subject string arrives under the other provider", func() {
			It("is a different account: a subject is unique only within its provider", func() {
				apple, err := subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
				Expect(err).NotTo(HaveOccurred())

				google, err := subject.EnsureUserByIdentity(ctx, uuid.New(), model.Identity{
					Provider: model.ProviderGoogle,
					Subject:  appleIdentity.Subject,
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(google.ID).NotTo(Equal(apple.ID))
				Expect(google.Identity.Provider).To(Equal(model.ProviderGoogle))
			})
		})

		When("the provider is not one the enum knows", func() {
			It("fails rather than storing an account nothing can sign in to", func() {
				_, err := subject.EnsureUserByIdentity(ctx, uuid.New(), model.Identity{
					Provider: model.AuthProvider("facebook"),
					Subject:  "whoever",
				})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetUserByID", func() {
		var stored model.User

		BeforeEach(func() {
			var err error
			stored, err = subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns the account", func() {
			found, err := subject.GetUserByID(ctx, stored.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(found.ID).To(Equal(stored.ID))
			Expect(found.Identity).To(Equal(appleIdentity))
		})

		It("returns a missing-entity error for an id that is not there", func() {
			_, err := subject.GetUserByID(ctx, uuid.New())
			Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(),
				"a by-id lookup that finds nothing must not let sql.ErrNoRows escape the store")
		})
	})

	Describe("SetUserContact", func() {
		var stored model.User

		BeforeEach(func() {
			var err error
			stored, err = subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the user opts in to both an email and a phone number", func() {
			It("stores both", func() {
				updated, err := subject.SetUserContact(ctx, stored.ID, model.Contact{
					Email: ptr("seller@example.com"),
					Phone: ptr("+359888123456"),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(updated.Contact.Email).To(HaveValue(Equal("seller@example.com")))
				Expect(updated.Contact.Phone).To(HaveValue(Equal("+359888123456")))

				email, phone := storedContact(stored.ID)
				Expect(email).To(HaveValue(Equal("seller@example.com")))
				Expect(phone).To(HaveValue(Equal("+359888123456")))
			})

			It("records that the account changed", func() {
				updated, err := subject.SetUserContact(ctx, stored.ID, model.Contact{
					Email: ptr("seller@example.com"),
					Phone: nil,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(updated.UpdatedAt).To(BeTemporally(">", stored.UpdatedAt))
			})
		})

		When("the user opts in to an email only", func() {
			It("leaves the phone number unshared", func() {
				updated, err := subject.SetUserContact(ctx, stored.ID, model.Contact{
					Email: ptr("seller@example.com"),
					Phone: nil,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(updated.Contact.Phone).To(BeNil())

				_, phone := storedContact(stored.ID)
				Expect(phone).To(BeNil(), "an unshared field is NULL, never an empty string")
			})
		})

		When("the user opts out of a field they had shared", func() {
			BeforeEach(func() {
				_, err := subject.SetUserContact(ctx, stored.ID, model.Contact{
					Email: ptr("seller@example.com"),
					Phone: ptr("+359888123456"),
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("clears it, because both columns are written every time", func() {
				updated, err := subject.SetUserContact(ctx, stored.ID, model.Contact{
					Email: nil,
					Phone: ptr("+359888123456"),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(updated.Contact.Email).To(BeNil())
				Expect(updated.Contact.Phone).To(HaveValue(Equal("+359888123456")))
			})
		})

		When("the user opts out of both", func() {
			BeforeEach(func() {
				_, err := subject.SetUserContact(ctx, stored.ID, model.Contact{
					Email: ptr("seller@example.com"),
					Phone: ptr("+359888123456"),
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("shares nothing, which is a legal row and not an error", func() {
				updated, err := subject.SetUserContact(ctx, stored.ID,
					model.Contact{Email: nil, Phone: nil})
				Expect(err).NotTo(HaveOccurred())
				Expect(updated.Contact).To(Equal(model.Contact{Email: nil, Phone: nil}))
			})
		})

		// The column's own CHECK is the last line of defence, not the first:
		// the service refuses a blank field before this, and the API boundary
		// refuses it before that. What is pinned here is that the database
		// would still stop one, so no future caller can write a value that
		// reads as opted-in to everything that only tests for NULL.
		//
		// btrim() with no second argument trims spaces only, so a tab-only or
		// newline-only value does pass this constraint — see
		// .ai/findings/open/2026-09-17-contact-blank-check-trims-spaces-only.md.
		// model.Contact.Normalise uses strings.TrimSpace and catches those, and
		// a service spec pins it.
		DescribeTable("a blank string reaching the column",
			func(contact model.Contact) {
				_, err := subject.SetUserContact(ctx, stored.ID, contact)
				Expect(err).To(HaveOccurred(),
					`an empty string would read as opted-in to every caller that only checks for NULL`)
			},
			Entry("an empty email", model.Contact{Email: ptr(""), Phone: nil}),
			Entry("a space-only email", model.Contact{Email: ptr("   "), Phone: nil}),
			Entry("an empty phone number", model.Contact{Email: nil, Phone: ptr("")}),
			Entry("a space-only phone number", model.Contact{Email: nil, Phone: ptr(" ")}),
		)

		It("returns a missing-entity error for an id that is not there", func() {
			_, err := subject.SetUserContact(ctx, uuid.New(),
				model.Contact{Email: ptr("nobody@example.com"), Phone: nil})
			Expect(dataerror.IsMissingEntityError(err)).To(BeTrue())
		})
	})

	Describe("InTx", func() {
		It("composes several writes into one unit that rolls back together", func() {
			var createdID uuid.UUID

			err := subject.InTx(ctx, func(ctx context.Context) error {
				created, err := subject.EnsureUserByIdentity(ctx, uuid.New(), appleIdentity)
				if err != nil {
					return err
				}
				createdID = created.ID

				// A blank contact fails the column's CHECK, which aborts the
				// transaction the account was created in.
				_, err = subject.SetUserContact(ctx, created.ID, model.Contact{Email: ptr(" "), Phone: nil})
				return err
			})
			Expect(err).To(HaveOccurred())

			_, err = subject.GetUserByID(context.Background(), createdID)
			Expect(dataerror.IsMissingEntityError(err)).To(BeTrue(),
				"the account must not survive the transaction it was created in")
		})
	})
})
