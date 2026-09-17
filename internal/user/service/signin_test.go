package service_test

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/internal/user/service"
	"github.com/simeonkorchev/binder/internal/user/service/servicefakes"
	"github.com/simeonkorchev/binder/internal/user/session"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

var _ = Describe("SignIn", func() {
	ctx := context.Background()
	userID := uuid.New()
	mintedID := uuid.New()
	expiresAt := time.Date(2026, time.September, 18, 12, 0, 0, 0, time.UTC)

	appleIdentity := model.Identity{Provider: model.ProviderApple, Subject: "001234.apple.subject.5678"}
	account := model.User{
		ID:        userID,
		Identity:  appleIdentity,
		Contact:   model.Contact{Email: nil, Phone: nil},
		CreatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
	}
	validSignIn := model.SignIn{
		Provider:      model.ProviderApple,
		IdentityToken: "the-providers-identity-token",
		Nonce:         "the-nonce-this-sign-in-was-bound-to",
	}

	var (
		fakeStore    *servicefakes.FakeStore
		fakeVerifier *servicefakes.FakeIdentityVerifier
		fakeSessions *servicefakes.FakeSessionIssuer
		subject      *service.Service

		input   model.SignIn
		result  model.Session
		signErr error
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		fakeVerifier = new(servicefakes.FakeIdentityVerifier)
		fakeSessions = new(servicefakes.FakeSessionIssuer)

		fakeVerifier.VerifyReturns(appleIdentity, nil)
		fakeStore.EnsureUserByIdentityReturns(account, nil)
		fakeSessions.IssueReturns(session.Token{Value: "the-session-token", ExpiresAt: expiresAt}, nil)

		subject = service.NewService(fakeStore, fakeVerifier, fakeSessions, func() uuid.UUID { return mintedID })
		input = validSignIn
	})

	JustBeforeEach(func() {
		result, signErr = subject.SignIn(ctx, input)
	})

	When("the provider's token verifies", func() {
		It("returns a session for the account", func() {
			Expect(signErr).NotTo(HaveOccurred())
			Expect(result.Token).To(Equal("the-session-token"))
			Expect(result.ExpiresAt).To(Equal(expiresAt))
			Expect(result.User).To(Equal(account))
		})

		It("verifies against the provider the sign-in named, with the token and nonce unchanged", func() {
			Expect(fakeVerifier.VerifyCallCount()).To(Equal(1))
			_, provider, token, nonce := fakeVerifier.VerifyArgsForCall(0)
			Expect(provider).To(Equal(model.ProviderApple))
			Expect(token).To(Equal("the-providers-identity-token"))
			Expect(nonce).To(Equal("the-nonce-this-sign-in-was-bound-to"))
		})

		It("looks the account up by the identity the provider vouched for, not by anything the client sent", func() {
			Expect(fakeStore.EnsureUserByIdentityCallCount()).To(Equal(1))
			_, newID, identity := fakeStore.EnsureUserByIdentityArgsForCall(0)
			Expect(identity).To(Equal(appleIdentity))
			Expect(newID).To(Equal(mintedID), "the id of a new account is minted server-side")
		})

		It("issues the session for the account the store resolved", func() {
			Expect(fakeSessions.IssueCallCount()).To(Equal(1))
			Expect(fakeSessions.IssueArgsForCall(0)).To(Equal(userID))
		})
	})

	When("the sign-in names Google", func() {
		BeforeEach(func() {
			input.Provider = model.ProviderGoogle
			fakeVerifier.VerifyReturns(model.Identity{
				Provider: model.ProviderGoogle,
				Subject:  "109876543210987654321",
			}, nil)
		})

		It("verifies against Google and stores the Google identity", func() {
			Expect(signErr).NotTo(HaveOccurred())

			_, provider, _, _ := fakeVerifier.VerifyArgsForCall(0)
			Expect(provider).To(Equal(model.ProviderGoogle))

			_, _, identity := fakeStore.EnsureUserByIdentityArgsForCall(0)
			Expect(identity.Provider).To(Equal(model.ProviderGoogle))
		})
	})

	When("the provider rejects the token", func() {
		BeforeEach(func() {
			fakeVerifier.VerifyReturns(model.Identity{}, oidc.ErrTokenRejected)
		})

		It("reports a rejected identity", func() {
			Expect(signErr).To(MatchError(service.ErrIdentityRejected))
			Expect(result).To(Equal(model.Session{}))
		})

		It("creates no account and issues no session", func() {
			Expect(fakeStore.EnsureUserByIdentityCallCount()).To(BeZero(),
				"a forged or expired token must never reach the database")
			Expect(fakeSessions.IssueCallCount()).To(BeZero())
		})

		It("keeps the provider's reason, so a diagnosis is still possible", func() {
			Expect(signErr).To(MatchError(oidc.ErrTokenRejected))
		})
	})

	When("the provider's keys cannot be fetched", func() {
		BeforeEach(func() {
			fakeVerifier.VerifyReturns(model.Identity{}, errProviderDown)
		})

		It("does not report a rejected identity: our outage is not the client's bad token", func() {
			Expect(signErr).To(MatchError(errProviderDown))
			Expect(signErr).NotTo(MatchError(service.ErrIdentityRejected))
		})

		It("creates no account", func() {
			Expect(fakeStore.EnsureUserByIdentityCallCount()).To(BeZero())
		})
	})

	When("the sign-in names a provider this backend does not support", func() {
		BeforeEach(func() {
			input.Provider = model.AuthProvider("facebook")
		})

		It("reports the provider unsupported without verifying anything", func() {
			Expect(signErr).To(MatchError(service.ErrProviderUnknown))
			Expect(fakeVerifier.VerifyCallCount()).To(BeZero())
			Expect(fakeStore.EnsureUserByIdentityCallCount()).To(BeZero())
		})
	})

	When("the sign-in names no provider", func() {
		BeforeEach(func() {
			input.Provider = ""
		})

		It("reports the provider unsupported: the zero value is not a provider", func() {
			Expect(signErr).To(MatchError(service.ErrProviderUnknown))
			Expect(fakeVerifier.VerifyCallCount()).To(BeZero())
		})
	})

	When("the store fails", func() {
		BeforeEach(func() {
			fakeStore.EnsureUserByIdentityReturns(model.User{}, errDB)
		})

		It("reports the failure and issues no session", func() {
			Expect(signErr).To(MatchError(errDB))
			Expect(signErr).NotTo(MatchError(service.ErrIdentityRejected))
			Expect(fakeSessions.IssueCallCount()).To(BeZero())
		})
	})

	When("the session cannot be signed", func() {
		BeforeEach(func() {
			fakeSessions.IssueReturns(session.Token{}, errSigning)
		})

		It("reports the failure rather than a session with an empty token", func() {
			Expect(signErr).To(MatchError(errSigning))
			Expect(result).To(Equal(model.Session{}))
		})
	})

	When("the same provider identity signs in again", func() {
		It("passes a fresh candidate id each time and lets the store decide", func() {
			_, err := subject.SignIn(ctx, validSignIn)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeStore.EnsureUserByIdentityCallCount()).To(Equal(2))
			_, _, firstIdentity := fakeStore.EnsureUserByIdentityArgsForCall(0)
			_, _, secondIdentity := fakeStore.EnsureUserByIdentityArgsForCall(1)
			Expect(secondIdentity).To(Equal(firstIdentity),
				"whether this is the same person is the unique index's decision, not the service's")
		})
	})

	// This domain handles nothing but personal data, so "no PII in the logs" is
	// pinned rather than reviewed: the spec reads the log line the sign-in
	// actually emitted.
	When("a sign-in succeeds", func() {
		var logged *bytes.Buffer

		BeforeEach(func() {
			logged = &bytes.Buffer{}
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
			DeferCleanup(func() { slog.SetDefault(previous) })

			fakeStore.EnsureUserByIdentityReturns(model.User{
				ID:        userID,
				Identity:  appleIdentity,
				Contact:   model.Contact{Email: ptr("seller@example.com"), Phone: ptr("+359888123456")},
				CreatedAt: account.CreatedAt,
				UpdatedAt: account.UpdatedAt,
			}, nil)
		})

		It("records the account and the provider", func() {
			Expect(signErr).NotTo(HaveOccurred())
			Expect(logged.String()).To(ContainSubstring("user_id=" + userID.String()))
			Expect(logged.String()).To(ContainSubstring("auth_provider=apple"))
		})

		It("records no identity token, no provider subject and no contact detail", func() {
			Expect(signErr).NotTo(HaveOccurred())
			Expect(logged.String()).NotTo(ContainSubstring("the-providers-identity-token"))
			Expect(logged.String()).NotTo(ContainSubstring("the-nonce-this-sign-in-was-bound-to"))
			Expect(logged.String()).NotTo(ContainSubstring(appleIdentity.Subject))
			Expect(logged.String()).NotTo(ContainSubstring("seller@example.com"))
			Expect(logged.String()).NotTo(ContainSubstring("+359888123456"))
			Expect(logged.String()).NotTo(ContainSubstring("the-session-token"))
		})
	})

	When("NewService is given no id source", func() {
		It("mints one, rather than storing the nil uuid", func() {
			defaulted := service.NewService(fakeStore, fakeVerifier, fakeSessions, nil)

			_, err := defaulted.SignIn(ctx, validSignIn)
			Expect(err).NotTo(HaveOccurred())

			_, newID, _ := fakeStore.EnsureUserByIdentityArgsForCall(fakeStore.EnsureUserByIdentityCallCount() - 1)
			Expect(newID).NotTo(Equal(uuid.Nil))
		})
	})
})

// ptr is a pointer to a copy of v, for the optional contact fields.
func ptr[T any](v T) *T {
	return &v
}
