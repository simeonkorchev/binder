package service_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/internal/user/service"
	"github.com/simeonkorchev/binder/internal/user/service/servicefakes"
)

var _ = Describe("Contact details", func() {
	ctx := context.Background()
	userID := uuid.New()

	account := model.User{
		ID:        userID,
		Identity:  model.Identity{Provider: model.ProviderApple, Subject: "001234.apple.subject.5678"},
		Contact:   model.Contact{Email: nil, Phone: nil},
		CreatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
	}
	// missingUser is what the store returns for an id that names nothing.
	missingUser := dataerror.WrapMissingEntityError("user", errDB)

	var (
		fakeStore *servicefakes.FakeStore
		subject   *service.Service
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		fakeStore.GetUserByIDReturns(account, nil)
		fakeStore.SetUserContactStub = func(
			_ context.Context, id uuid.UUID, contact model.Contact,
		) (model.User, error) {
			stored := account
			stored.ID = id
			stored.Contact = contact
			return stored, nil
		}
		subject = service.NewService(fakeStore, new(servicefakes.FakeIdentityVerifier),
			new(servicefakes.FakeSessionIssuer), uuid.New)
	})

	Describe("GetUser", func() {
		var (
			found  model.User
			getErr error
		)

		JustBeforeEach(func() {
			found, getErr = subject.GetUser(ctx, userID)
		})

		When("the account is there", func() {
			It("returns it", func() {
				Expect(getErr).NotTo(HaveOccurred())
				Expect(found).To(Equal(account))
			})
		})

		When("no account has that id", func() {
			BeforeEach(func() {
				fakeStore.GetUserByIDReturns(model.User{}, missingUser)
			})

			It("reports the account not found rather than leaking the store's error type", func() {
				Expect(getErr).To(MatchError(service.ErrUserNotFound))
			})
		})

		When("the store fails", func() {
			BeforeEach(func() {
				fakeStore.GetUserByIDReturns(model.User{}, errDB)
			})

			It("reports the failure", func() {
				Expect(getErr).To(MatchError(errDB))
				Expect(getErr).NotTo(MatchError(service.ErrUserNotFound))
			})
		})
	})

	Describe("SetContact", func() {
		var (
			contact model.Contact
			updated model.User
			setErr  error
		)

		JustBeforeEach(func() {
			updated, setErr = subject.SetContact(ctx, userID, contact)
		})

		When("the user opts in to both fields", func() {
			BeforeEach(func() {
				contact = model.Contact{Email: ptr("seller@example.com"), Phone: ptr("+359888123456")}
			})

			It("stores both and returns the account as stored", func() {
				Expect(setErr).NotTo(HaveOccurred())
				Expect(updated.Contact.Email).To(HaveValue(Equal("seller@example.com")))
				Expect(updated.Contact.Phone).To(HaveValue(Equal("+359888123456")))
			})
		})

		When("a shared field has whitespace around it", func() {
			BeforeEach(func() {
				contact = model.Contact{Email: ptr("  seller@example.com \n"), Phone: nil}
			})

			It("trims it before it reaches the store, so one address is one value", func() {
				Expect(setErr).NotTo(HaveOccurred())

				_, _, passed := fakeStore.SetUserContactArgsForCall(0)
				Expect(passed.Email).To(HaveValue(Equal("seller@example.com")))
			})
		})

		When("the user opts out of both fields", func() {
			BeforeEach(func() {
				contact = model.Contact{Email: nil, Phone: nil}
			})

			It("stores nothing shared, which is a legal state and not an error", func() {
				Expect(setErr).NotTo(HaveOccurred())
				Expect(updated.Contact).To(Equal(model.Contact{Email: nil, Phone: nil}))

				_, _, passed := fakeStore.SetUserContactArgsForCall(0)
				Expect(passed).To(Equal(model.Contact{Email: nil, Phone: nil}))
			})
		})

		When("no account has that id", func() {
			BeforeEach(func() {
				contact = model.Contact{Email: ptr("seller@example.com"), Phone: nil}
				fakeStore.SetUserContactStub = nil
				fakeStore.SetUserContactReturns(model.User{}, missingUser)
			})

			It("reports the account not found", func() {
				Expect(setErr).To(MatchError(service.ErrUserNotFound))
			})
		})

		When("the store fails", func() {
			BeforeEach(func() {
				contact = model.Contact{Email: ptr("seller@example.com"), Phone: nil}
				fakeStore.SetUserContactStub = nil
				fakeStore.SetUserContactReturns(model.User{}, errDB)
			})

			It("reports the failure", func() {
				Expect(setErr).To(MatchError(errDB))
			})
		})
	})

	// The blank-field cases live in their own container: the Describe above
	// invokes SetContact from a JustBeforeEach, which would fire with the
	// enclosing zero-valued contact before a table entry could set one.
	Describe("SetContact, with a blank field", func() {
		DescribeTable("a shared field that is nothing but whitespace",
			func(blank model.Contact) {
				_, err := subject.SetContact(ctx, userID, blank)
				Expect(err).To(MatchError(service.ErrContactBlank))
				Expect(fakeStore.SetUserContactCallCount()).To(BeZero(),
					"a blank string must not reach the column, where it would read as opted-in")
			},
			Entry("an empty email", model.Contact{Email: ptr(""), Phone: nil}),
			Entry("a space-only email", model.Contact{Email: ptr("   "), Phone: nil}),
			// btrim() in the column's CHECK trims spaces only, so these two are
			// caught here and nowhere else — see
			// .ai/findings/open/2026-09-17-contact-blank-check-trims-spaces-only.md.
			Entry("a tab-only email", model.Contact{Email: ptr("\t"), Phone: nil}),
			Entry("a newline-only email", model.Contact{Email: ptr("\n"), Phone: nil}),
			Entry("an empty phone number", model.Contact{Email: nil, Phone: ptr("")}),
			Entry("a tab-only phone number", model.Contact{Email: nil, Phone: ptr("\t")}),
			Entry("a blank phone number beside a good email",
				model.Contact{Email: ptr("seller@example.com"), Phone: ptr(" ")}),
		)

		It("names the fields and not their values: an error message gets logged", func() {
			_, err := subject.SetContact(ctx, userID, model.Contact{Email: ptr("   "), Phone: ptr("\t")})
			Expect(err).To(MatchError(service.ErrContactBlank))
			Expect(err.Error()).To(ContainSubstring(string(model.ContactFieldEmail)))
			Expect(err.Error()).To(ContainSubstring(string(model.ContactFieldPhone)))
			Expect(err.Error()).NotTo(ContainSubstring("seller@example.com"))
		})
	})

	Describe("SellerContact", func() {
		var (
			contact model.Contact
			err     error
		)

		JustBeforeEach(func() {
			contact, err = subject.SellerContact(ctx, userID)
		})

		When("the seller shared both fields", func() {
			BeforeEach(func() {
				shared := account
				shared.Contact = model.Contact{Email: ptr("seller@example.com"), Phone: ptr("+359888123456")}
				fakeStore.GetUserByIDReturns(shared, nil)
			})

			It("returns them", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(contact.Email).To(HaveValue(Equal("seller@example.com")))
				Expect(contact.Phone).To(HaveValue(Equal("+359888123456")))
			})
		})

		When("the seller shared nothing", func() {
			It("returns both fields absent and no error: not opting in is normal", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(contact).To(Equal(model.Contact{Email: nil, Phone: nil}))
			})
		})

		When("no account has that id", func() {
			BeforeEach(func() {
				fakeStore.GetUserByIDReturns(model.User{}, missingUser)
			})

			It("leaves the missing-entity error intact, which is what SellerContacts documents", func() {
				Expect(dataerror.IsMissingEntityError(err)).To(BeTrue())
			})
		})

		When("the store fails", func() {
			BeforeEach(func() {
				fakeStore.GetUserByIDReturns(model.User{}, errDB)
			})

			It("reports the failure, and it is not a missing account", func() {
				Expect(err).To(MatchError(errDB))
				Expect(dataerror.IsMissingEntityError(err)).To(BeFalse())
			})
		})
	})
})
