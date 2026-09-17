package api_test

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/user/api/apifakes"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/internal/user/service"
)

// userBodyShape is the account as this domain puts it on the wire.
type userBodyShape struct {
	ID           uuid.UUID `json:"id"`
	ContactEmail *string   `json:"contactEmail"`
	ContactPhone *string   `json:"contactPhone"`
}

var _ = Describe("The signed-in account", func() {
	userID := uuid.New()
	account := model.User{
		ID:        userID,
		Identity:  model.Identity{Provider: model.ProviderApple, Subject: "001234.apple.subject.5678"},
		Contact:   model.Contact{Email: ptr("seller@example.com"), Phone: ptr("+359888123456")},
		CreatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
	}

	var fakeSvc *apifakes.FakeService

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		fakeSvc.GetUserReturns(account, nil)
		fakeSvc.SetContactReturns(account, nil)
	})

	Describe("GET /me", func() {
		When("the request carries a session", func() {
			It("returns the account, with the contact details it shares", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodGet, "/me", nil)
				Expect(rec.Code).To(Equal(http.StatusOK))

				var body userBodyShape
				decodeBody(rec, &body)
				Expect(body.ID).To(Equal(userID))
				Expect(body.ContactEmail).To(HaveValue(Equal("seller@example.com")))
				Expect(body.ContactPhone).To(HaveValue(Equal("+359888123456")))
			})

			It("reads the account the session names, never one from the request", func() {
				do(fakeSvc, signedInAs(userID), http.MethodGet, "/me", nil)

				Expect(fakeSvc.GetUserCallCount()).To(Equal(1))
				_, asked := fakeSvc.GetUserArgsForCall(0)
				Expect(asked).To(Equal(userID))
			})

			It("does not put the provider subject or the timestamps on the wire", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodGet, "/me", nil)
				Expect(rec.Body.String()).NotTo(ContainSubstring("001234.apple.subject.5678"))
				Expect(rec.Body.String()).NotTo(ContainSubstring("apple"))
				Expect(rec.Body.String()).NotTo(ContainSubstring("2026-03-01"))
			})
		})

		When("the account shares nothing", func() {
			BeforeEach(func() {
				anonymous := account
				anonymous.Contact = model.Contact{Email: nil, Phone: nil}
				fakeSvc.GetUserReturns(anonymous, nil)
			})

			It("sends both fields as null rather than as empty strings", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodGet, "/me", nil)
				Expect(rec.Code).To(Equal(http.StatusOK))

				var body userBodyShape
				decodeBody(rec, &body)
				Expect(body.ContactEmail).To(BeNil())
				Expect(body.ContactPhone).To(BeNil())
				Expect(rec.Body.String()).To(ContainSubstring(`"contactEmail":null`))
			})
		})

		When("the request carries no session", func() {
			It("answers 401 without touching the service", func() {
				rec := do(fakeSvc, signedOut(), http.MethodGet, "/me", nil)
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.GetUserCallCount()).To(BeZero())
			})
		})

		When("the session names an account that is gone", func() {
			BeforeEach(func() {
				fakeSvc.GetUserReturns(model.User{}, service.ErrUserNotFound)
			})

			It("answers 404", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodGet, "/me", nil)
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		When("the service fails", func() {
			BeforeEach(func() {
				fakeSvc.GetUserReturns(model.User{}, errService)
			})

			It("answers 500 and keeps the detail on this side", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodGet, "/me", nil)
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
				Expect(rec.Body.String()).NotTo(ContainSubstring("service failed"))
			})
		})
	})

	Describe("PUT /me/contact", func() {
		When("the user opts in to both fields", func() {
			It("replaces the contact details", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", map[string]any{
					"email": "seller@example.com",
					"phone": "+359888123456",
				})
				Expect(rec.Code).To(Equal(http.StatusOK))

				Expect(fakeSvc.SetContactCallCount()).To(Equal(1))
				_, asked, contact := fakeSvc.SetContactArgsForCall(0)
				Expect(asked).To(Equal(userID))
				Expect(contact.Email).To(HaveValue(Equal("seller@example.com")))
				Expect(contact.Phone).To(HaveValue(Equal("+359888123456")))
			})
		})

		When("the user opts out of a field by leaving it out", func() {
			It("sends it as not shared, which is how a field is cleared", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", map[string]any{
					"email": "seller@example.com",
				})
				Expect(rec.Code).To(Equal(http.StatusOK))

				_, _, contact := fakeSvc.SetContactArgsForCall(0)
				Expect(contact.Phone).To(BeNil())
			})
		})

		When("the user opts out of a field explicitly", func() {
			It("treats null the same as leaving it out", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", map[string]any{
					"email": nil,
					"phone": "+359888123456",
				})
				Expect(rec.Code).To(Equal(http.StatusOK))

				_, _, contact := fakeSvc.SetContactArgsForCall(0)
				Expect(contact.Email).To(BeNil())
			})
		})

		When("the user opts out of everything", func() {
			It("accepts an empty body: sharing nothing is a legal preference", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", map[string]any{})
				Expect(rec.Code).To(Equal(http.StatusOK))

				_, _, contact := fakeSvc.SetContactArgsForCall(0)
				Expect(contact).To(Equal(model.Contact{Email: nil, Phone: nil}))
			})
		})

		DescribeTable("a field that is present but blank",
			func(body map[string]any, wantLocation string) {
				fakeSvc := new(apifakes.FakeService)

				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", body)
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(rec.Body.String()).To(ContainSubstring(wantLocation))
				Expect(fakeSvc.SetContactCallCount()).To(BeZero(),
					"a blank field is refused at the boundary, so nothing below has to cope with it")
			},
			Entry("an empty email", map[string]any{"email": ""}, "body.email"),
			Entry("a space-only email", map[string]any{"email": "   "}, "body.email"),
			Entry("a tab-only email", map[string]any{"email": "\t"}, "body.email"),
			Entry("an empty phone number", map[string]any{"phone": ""}, "body.phone"),
			Entry("a blank phone beside a good email",
				map[string]any{"email": "seller@example.com", "phone": " "}, "body.phone"),
		)

		When("a blank field is refused", func() {
			It("does not echo the value back", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", map[string]any{
					"email": "   ",
				})
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(rec.Body.String()).NotTo(ContainSubstring(`"value":"   "`))
			})
		})

		When("the request carries no session", func() {
			It("answers 401 without touching the service", func() {
				rec := do(fakeSvc, signedOut(), http.MethodPut, "/me/contact", map[string]any{
					"email": "seller@example.com",
				})
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.SetContactCallCount()).To(BeZero())
			})
		})

		When("the service refuses a blank field the boundary let through", func() {
			BeforeEach(func() {
				fakeSvc.SetContactReturns(model.User{}, service.ErrContactBlank)
			})

			It("answers 422", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", map[string]any{
					"email": "seller@example.com",
				})
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			})
		})

		When("the session names an account that is gone", func() {
			BeforeEach(func() {
				fakeSvc.SetContactReturns(model.User{}, service.ErrUserNotFound)
			})

			It("answers 404", func() {
				rec := do(fakeSvc, signedInAs(userID), http.MethodPut, "/me/contact", map[string]any{
					"email": "seller@example.com",
				})
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})
	})
})
