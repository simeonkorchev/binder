package api_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/listing/api"
	"github.com/simeonkorchev/binder/internal/listing/api/apifakes"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/internal/listing/service"
)

var _ = Describe("GET /sellers/{sellerId}/contact", func() {
	var (
		buyerID  = uuid.New()
		sellerID = uuid.New()
	)

	var (
		fakeSvc *apifakes.FakeService
		actor   api.ActorFunc
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		actor = signedInAs(buyerID)
	})

	JustBeforeEach(func() {
		rec = do(fakeSvc, actor, http.MethodGet, "/sellers/"+sellerID.String()+"/contact", nil)
	})

	When("the seller shared both an email and a phone number", func() {
		BeforeEach(func() {
			email := "seller@example.test"
			phone := "+359888123456"
			fakeSvc.SellerContactReturns(model.SellerContact{Email: &email, Phone: &phone}, nil)
		})

		It("answers 200 with both, for the seller in the path", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))

			var got struct {
				Email *string `json:"email"`
				Phone *string `json:"phone"`
			}
			decodeBody(rec, &got)
			Expect(got.Email).To(HaveValue(Equal("seller@example.test")))
			Expect(got.Phone).To(HaveValue(Equal("+359888123456")))

			Expect(fakeSvc.SellerContactCallCount()).To(Equal(1))
			_, asked := fakeSvc.SellerContactArgsForCall(0)
			Expect(asked).To(Equal(sellerID))
		})
	})

	When("the seller shared only a phone number", func() {
		BeforeEach(func() {
			phone := "+359888123456"
			fakeSvc.SellerContactReturns(model.SellerContact{Email: nil, Phone: &phone}, nil)
		})

		It("answers 200 with the phone number and a null email", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))

			var got struct {
				Email *string `json:"email"`
				Phone *string `json:"phone"`
			}
			decodeBody(rec, &got)
			Expect(got.Email).To(BeNil())
			Expect(got.Phone).To(HaveValue(Equal("+359888123456")))
		})
	})

	// D4: a seller who opted into nothing simply has no contact button. The
	// client has to be able to tell that apart from a failure, so it is a 200
	// with both fields present and null (000-principles.md section 8b).
	When("the seller shared nothing", func() {
		BeforeEach(func() {
			fakeSvc.SellerContactReturns(model.SellerContact{Email: nil, Phone: nil}, nil)
		})

		It("answers 200 with both fields null, not 404 and not an error", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(ContainSubstring(`"email":null`))
			Expect(rec.Body.String()).To(ContainSubstring(`"phone":null`))
		})
	})

	When("there is no such seller", func() {
		BeforeEach(func() {
			fakeSvc.SellerContactReturns(model.SellerContact{Email: nil, Phone: nil}, service.ErrSellerNotFound)
		})

		It("answers 404", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the service fails for an unforeseen reason", func() {
		BeforeEach(func() {
			fakeSvc.SellerContactReturns(model.SellerContact{Email: nil, Phone: nil}, errService)
		})

		It("answers 500 without the detail", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			Expect(rec.Body.String()).NotTo(ContainSubstring(errService.Error()))
		})
	})

	// Contact details are personal data the seller published to buyers, not to
	// anyone who can spell a uuid: a session is what separates a buyer asking
	// about a card from a script reading every seller in the database.
	When("nobody is signed in", func() {
		BeforeEach(func() {
			actor = signedOut()
		})

		It("answers 401 and never reaches the service", func() {
			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			Expect(fakeSvc.SellerContactCallCount()).To(BeZero())
		})
	})
})
