package identity_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/user/identity"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

var _ = Describe("Verifiers", func() {
	const (
		appleSubject  = "001234.apple.subject.5678"
		googleSubject = "109876543210987654321"
	)

	var (
		apple     *stubVerifier
		google    *stubVerifier
		verifiers *identity.Verifiers
	)

	BeforeEach(func() {
		apple = &stubVerifier{identity: oidc.Identity{Subject: appleSubject}, err: nil}
		google = &stubVerifier{identity: oidc.Identity{Subject: googleSubject}, err: nil}
		verifiers = identity.New(map[model.AuthProvider]identity.ProviderVerifier{
			model.ProviderApple:  apple,
			model.ProviderGoogle: google,
		})
	})

	Describe("Verify", func() {
		var (
			provider  model.AuthProvider
			resolved  model.Identity
			verifyErr error
		)

		JustBeforeEach(func() {
			resolved, verifyErr = verifiers.Verify(context.Background(), provider, "the-identity-token", "the-nonce")
		})

		When("the sign-in names Apple", func() {
			BeforeEach(func() {
				provider = model.ProviderApple
			})

			It("verifies against Apple and carries the provider into the identity", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(resolved).To(Equal(model.Identity{
					Provider: model.ProviderApple,
					Subject:  appleSubject,
				}))
			})

			It("hands Apple the token and the nonce unchanged", func() {
				Expect(apple.calls).To(Equal(1))
				Expect(apple.gotToken).To(Equal("the-identity-token"))
				Expect(apple.gotNonce).To(Equal("the-nonce"))
			})

			It("does not go to Google", func() {
				Expect(google.calls).To(BeZero())
			})
		})

		When("the sign-in names Google", func() {
			BeforeEach(func() {
				provider = model.ProviderGoogle
			})

			It("verifies against Google", func() {
				Expect(verifyErr).NotTo(HaveOccurred())
				Expect(resolved).To(Equal(model.Identity{
					Provider: model.ProviderGoogle,
					Subject:  googleSubject,
				}))
				Expect(apple.calls).To(BeZero())
			})
		})

		When("the sign-in names a provider this backend is not configured for", func() {
			BeforeEach(func() {
				provider = model.AuthProvider("facebook")
			})

			It("reports the provider unknown without verifying anything", func() {
				Expect(verifyErr).To(MatchError(identity.ErrProviderUnknown))
				Expect(apple.calls).To(BeZero())
				Expect(google.calls).To(BeZero())
			})
		})

		When("the sign-in names no provider at all", func() {
			BeforeEach(func() {
				provider = ""
			})

			It("reports the provider unknown", func() {
				Expect(verifyErr).To(MatchError(identity.ErrProviderUnknown))
			})
		})

		When("the provider rejects the token", func() {
			BeforeEach(func() {
				provider = model.ProviderApple
				apple.err = oidc.ErrTokenRejected
			})

			It("passes the rejection up so a caller can still match it", func() {
				Expect(verifyErr).To(MatchError(oidc.ErrTokenRejected))
				Expect(resolved).To(Equal(model.Identity{}))
			})
		})

		When("the provider cannot be reached", func() {
			BeforeEach(func() {
				provider = model.ProviderGoogle
				google.err = errProviderDown
			})

			It("passes the failure up as itself, not as a rejected token", func() {
				Expect(verifyErr).To(MatchError(errProviderDown))
				Expect(verifyErr).NotTo(MatchError(oidc.ErrTokenRejected))
			})
		})
	})

	Describe("New", func() {
		It("keeps its own copy of the map, so a later mutation cannot add a provider", func() {
			supplied := map[model.AuthProvider]identity.ProviderVerifier{model.ProviderApple: apple}
			built := identity.New(supplied)

			supplied[model.ProviderGoogle] = google
			_, err := built.Verify(context.Background(), model.ProviderGoogle, "token", "")
			Expect(err).To(MatchError(identity.ErrProviderUnknown))
		})
	})
})

var _ = Describe("LoadConfig", func() {
	When("both providers' client ids are set", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("APPLE_CLIENT_IDS", "com.example.binder")
			GinkgoT().Setenv("GOOGLE_CLIENT_IDS", "ios.apps.googleusercontent.com,android.apps.googleusercontent.com")
		})

		It("reads them, splitting a comma-separated list into the audiences", func() {
			cfg, err := identity.LoadConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.AppleClientIDs).To(ConsistOf("com.example.binder"))
			Expect(cfg.GoogleClientIDs).To(ConsistOf(
				"ios.apps.googleusercontent.com", "android.apps.googleusercontent.com"))
		})

		It("builds both providers' verifiers", func() {
			cfg, err := identity.LoadConfig()
			Expect(err).NotTo(HaveOccurred())

			// No request is made here: NewFromConfig only builds the caches, and
			// a key set is fetched on the first token that needs one.
			verifiers, err := identity.NewFromConfig(cfg, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(verifiers).NotTo(BeNil())
		})
	})

	When("APPLE_CLIENT_IDS is not set", func() {
		BeforeEach(func() {
			withoutEnv("APPLE_CLIENT_IDS")
			GinkgoT().Setenv("GOOGLE_CLIENT_IDS", "ios.apps.googleusercontent.com")
		})

		It("fails: a backend that cannot name its own audience would accept another app's tokens", func() {
			_, err := identity.LoadConfig()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("APPLE_CLIENT_IDS"))
		})
	})

	When("GOOGLE_CLIENT_IDS is not set", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("APPLE_CLIENT_IDS", "com.example.binder")
			withoutEnv("GOOGLE_CLIENT_IDS")
		})

		It("fails", func() {
			_, err := identity.LoadConfig()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("GOOGLE_CLIENT_IDS"))
		})
	})
})

var _ = Describe("NewFromConfig", func() {
	It("refuses an empty Apple audience list", func() {
		_, err := identity.NewFromConfig(identity.Config{
			AppleClientIDs:  nil,
			GoogleClientIDs: []string{"ios.apps.googleusercontent.com"},
		}, nil)
		Expect(err).To(HaveOccurred())
	})

	It("refuses an empty Google audience list", func() {
		_, err := identity.NewFromConfig(identity.Config{
			AppleClientIDs:  []string{"com.example.binder"},
			GoogleClientIDs: nil,
		}, nil)
		Expect(err).To(HaveOccurred())
	})
})
