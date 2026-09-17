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

var _ = Describe("POST /auth/sessions", func() {
	userID := uuid.New()
	expiresAt := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)
	session := model.Session{
		Token:     "the-session-token",
		ExpiresAt: expiresAt,
		User: model.User{
			ID:        userID,
			Identity:  model.Identity{Provider: model.ProviderApple, Subject: "001234.apple.subject.5678"},
			Contact:   model.Contact{Email: ptr("seller@example.com"), Phone: nil},
			CreatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		},
	}

	var fakeSvc *apifakes.FakeService

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		fakeSvc.SignInReturns(session, nil)
	})

	// Signing in is the one endpoint with no actor, so every spec below drives it
	// signed out: a sign-in that needed a session could never be the first call.

	When("the identity token verifies", func() {
		It("creates the session", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "the-providers-identity-token",
				"nonce":         "the-nonce",
			})
			Expect(rec.Code).To(Equal(http.StatusCreated))

			var body struct {
				Token     string    `json:"token"`
				ExpiresAt time.Time `json:"expiresAt"`
				User      struct {
					ID           uuid.UUID `json:"id"`
					ContactEmail *string   `json:"contactEmail"`
					ContactPhone *string   `json:"contactPhone"`
				} `json:"user"`
			}
			decodeBody(rec, &body)

			Expect(body.Token).To(Equal("the-session-token"))
			Expect(body.ExpiresAt).To(BeTemporally("==", expiresAt))
			Expect(body.User.ID).To(Equal(userID))
			Expect(body.User.ContactEmail).To(HaveValue(Equal("seller@example.com")))
			Expect(body.User.ContactPhone).To(BeNil())
		})

		It("passes the provider, the token and the nonce through unchanged", func() {
			do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "google",
				"identityToken": "the-providers-identity-token",
				"nonce":         "the-nonce",
			})

			Expect(fakeSvc.SignInCallCount()).To(Equal(1))
			_, input := fakeSvc.SignInArgsForCall(0)
			Expect(input).To(Equal(model.SignIn{
				Provider:      model.ProviderGoogle,
				IdentityToken: "the-providers-identity-token",
				Nonce:         "the-nonce",
			}))
		})

		It("never puts the provider's subject on the wire", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "the-providers-identity-token",
			})
			Expect(rec.Body.String()).NotTo(ContainSubstring("001234.apple.subject.5678"))
		})
	})

	When("the app sent no nonce", func() {
		It("accepts the sign-in with an empty one", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "the-providers-identity-token",
			})
			Expect(rec.Code).To(Equal(http.StatusCreated))

			_, input := fakeSvc.SignInArgsForCall(0)
			Expect(input.Nonce).To(BeEmpty())
		})
	})

	When("the identity token does not verify", func() {
		BeforeEach(func() {
			fakeSvc.SignInReturns(model.Session{}, service.ErrIdentityRejected)
		})

		It("answers 401 and says nothing about which check failed", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "a-forged-token",
			})
			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			Expect(rec.Body.String()).NotTo(ContainSubstring("signature"))
			Expect(rec.Body.String()).NotTo(ContainSubstring("expired"))
			Expect(rec.Body.String()).NotTo(ContainSubstring("audience"))
		})

		It("does not echo the token back", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "a-forged-token",
			})
			Expect(rec.Body.String()).NotTo(ContainSubstring("a-forged-token"))
		})
	})

	When("the body names a provider this app does not sign in with", func() {
		It("answers 422 naming the field, without reaching the service", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "facebook",
				"identityToken": "the-providers-identity-token",
			})
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(rec.Body.String()).To(ContainSubstring("provider"))
			Expect(fakeSvc.SignInCallCount()).To(BeZero())
		})
	})

	When("the body names no provider", func() {
		It("answers 422 without reaching the service", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"identityToken": "the-providers-identity-token",
			})
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(fakeSvc.SignInCallCount()).To(BeZero())
		})
	})

	When("the body carries no identity token", func() {
		It("answers 422 without reaching the service", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "",
			})
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(fakeSvc.SignInCallCount()).To(BeZero())
		})
	})

	When("the service fails for a reason the client cannot act on", func() {
		BeforeEach(func() {
			fakeSvc.SignInReturns(model.Session{}, errService)
		})

		It("answers 500 and keeps the detail on this side", func() {
			rec := do(fakeSvc, signedOut(), http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "the-providers-identity-token",
			})
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			Expect(rec.Body.String()).NotTo(ContainSubstring("service failed"))
		})
	})
})
