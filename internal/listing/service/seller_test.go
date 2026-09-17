package service_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/internal/listing/service"
	"github.com/simeonkorchev/binder/internal/listing/service/servicefakes"
)

var _ = Describe("Seller contact details", func() {
	var ctx = context.Background()

	var (
		subject     *service.Service
		fakeSellers *servicefakes.FakeSellerContacts
		sellerID    uuid.UUID
		contact     model.SellerContact
		err         error
	)

	BeforeEach(func() {
		fakeSellers = new(servicefakes.FakeSellerContacts)
		subject = service.NewService(new(servicefakes.FakeStore), fakeSellers)
		sellerID = uuid.New()
	})

	JustBeforeEach(func() {
		contact, err = subject.SellerContact(ctx, sellerID)
	})

	When("the seller shared both an email and a phone number", func() {
		BeforeEach(func() {
			email := "seller@example.test"
			phone := "+359888123456"
			fakeSellers.ContactForReturns(model.SellerContact{Email: &email, Phone: &phone}, nil)
		})

		It("returns both, for the seller it was asked about", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(contact.Email).To(HaveValue(Equal("seller@example.test")))
			Expect(contact.Phone).To(HaveValue(Equal("+359888123456")))

			Expect(fakeSellers.ContactForCallCount()).To(Equal(1))
			_, asked := fakeSellers.ContactForArgsForCall(0)
			Expect(asked).To(Equal(sellerID))
		})
	})

	When("the seller shared only an email", func() {
		BeforeEach(func() {
			email := "seller@example.test"
			fakeSellers.ContactForReturns(model.SellerContact{Email: &email, Phone: nil}, nil)
		})

		It("returns the email and leaves the phone number absent", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(contact.Email).To(HaveValue(Equal("seller@example.test")))
			Expect(contact.Phone).To(BeNil())
		})
	})

	When("the seller shared only a phone number", func() {
		BeforeEach(func() {
			phone := "+359888123456"
			fakeSellers.ContactForReturns(model.SellerContact{Email: nil, Phone: &phone}, nil)
		})

		It("returns the phone number and leaves the email absent", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(contact.Phone).To(HaveValue(Equal("+359888123456")))
			Expect(contact.Email).To(BeNil())
		})
	})

	// The default row in db/migrations/002_users.sql. "Nothing to show" is a
	// state of the account, not a failure to read it (000-principles.md
	// section 8b).
	When("the seller shared nothing", func() {
		BeforeEach(func() {
			fakeSellers.ContactForReturns(model.SellerContact{Email: nil, Phone: nil}, nil)
		})

		It("succeeds with nothing to show, and is not an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(contact.Email).To(BeNil())
			Expect(contact.Phone).To(BeNil())
		})
	})

	When("there is no such user at all", func() {
		BeforeEach(func() {
			fakeSellers.ContactForReturns(model.SellerContact{Email: nil, Phone: nil},
				dataerror.WrapMissingEntityError("user", errUsers))
		})

		It("is not found, which an unshared contact deliberately is not", func() {
			Expect(err).To(MatchError(service.ErrSellerNotFound))
		})
	})

	When("the user domain cannot answer", func() {
		BeforeEach(func() {
			fakeSellers.ContactForReturns(model.SellerContact{Email: nil, Phone: nil}, errUsers)
		})

		It("fails with the cause attached rather than as a sentinel", func() {
			Expect(err).To(MatchError(errUsers))
			Expect(err).NotTo(MatchError(service.ErrSellerNotFound))
		})
	})
})
