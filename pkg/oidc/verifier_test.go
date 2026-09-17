package oidc_test

import (
	"context"
	"crypto/x509"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

var _ = Describe("Verifier", func() {
	const (
		issuer   = "https://provider.example"
		audience = "com.example.binder.ios"
		subject  = "provider-subject-000123"
	)

	// now is fixed so "expired" and "still valid" are facts of the spec rather
	// than of how long the suite took to run.
	now := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)

	var (
		provider signingKey
		valid    claims
		verifier *oidc.Verifier
		keys     oidc.KeyLookup
	)

	BeforeEach(func() {
		provider = newSigningKey("provider-key-1")
		keys = staticKeys{set: keySet(provider)}
		valid = claims{
			issuer:   issuer,
			subject:  subject,
			audience: audience,
			expiry:   now.Add(time.Hour),
			issuedAt: now.Add(-time.Minute),
			nonce:    "",
		}
	})

	// buildVerifier is called by every JustBeforeEach below, after the spec's
	// BeforeEach has had its say about the keys.
	buildVerifier := func() {
		var err error
		verifier, err = oidc.NewVerifier(oidc.Config{
			Issuers:   []string{issuer, "provider.example"},
			Audiences: []string{audience, "com.example.binder.android"},
			Keys:      keys,
			Now:       func() time.Time { return now },
		})
		Expect(err).NotTo(HaveOccurred())
	}

	Describe("Verify", func() {
		var (
			rawToken      string
			expectedNonce string
			identity      oidc.Identity
			err           error
		)

		BeforeEach(func() {
			expectedNonce = ""
		})

		JustBeforeEach(func() {
			buildVerifier()
			identity, err = verifier.Verify(context.Background(), rawToken, expectedNonce)
		})

		When("the token is the provider's, current and addressed to this app", func() {
			BeforeEach(func() {
				rawToken = valid.sign(provider)
			})

			It("returns the provider's subject", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(identity.Subject).To(Equal(subject))
			})
		})

		When("the token names the provider's other issuer spelling", func() {
			BeforeEach(func() {
				other := valid
				other.issuer = "provider.example"
				rawToken = other.sign(provider)
			})

			It("accepts it: a provider may spell its own issuer more than one way", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(identity.Subject).To(Equal(subject))
			})
		})

		When("the token names the second accepted audience", func() {
			BeforeEach(func() {
				other := valid
				other.audience = "com.example.binder.android"
				rawToken = other.sign(provider)
			})

			It("accepts it: one account is reached from several platform clients", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(identity.Subject).To(Equal(subject))
			})
		})

		When("the token has expired", func() {
			BeforeEach(func() {
				expired := valid
				expired.expiry = now.Add(-time.Hour)
				rawToken = expired.sign(provider)
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
				Expect(identity).To(Equal(oidc.Identity{}))
			})
		})

		When("the token was issued for a different app", func() {
			BeforeEach(func() {
				wrongAudience := valid
				wrongAudience.audience = "com.attacker.app"
				rawToken = wrongAudience.sign(provider)
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		When("the token was issued by a different provider", func() {
			BeforeEach(func() {
				wrongIssuer := valid
				wrongIssuer.issuer = "https://attacker.example"
				rawToken = wrongIssuer.sign(provider)
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		When("the signature was tampered with", func() {
			BeforeEach(func() {
				rawToken = tamperSignature(valid.sign(provider))
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		When("the token was signed by a key the provider does not publish", func() {
			BeforeEach(func() {
				// A well-formed token, correctly signed — by somebody else, who
				// also chose the key id.
				impostor := newSigningKey("provider-key-1")
				rawToken = valid.sign(impostor)
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		When("the token names a key id the provider does not publish", func() {
			BeforeEach(func() {
				unpublished := newSigningKey("no-such-key")
				rawToken = valid.sign(unpublished)
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
				Expect(err).To(MatchError(oidc.ErrKeyUnknown))
			})
		})

		When("the token carries no key id at all", func() {
			BeforeEach(func() {
				anonymous := newSigningKey("")
				rawToken = valid.sign(anonymous)
			})

			It("rejects it rather than trying every published key", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		When("the token is signed with the provider's public key as an HMAC secret", func() {
			BeforeEach(func() {
				// The algorithm-confusion attack: the attacker has the
				// provider's public key, because it is public, and uses its
				// bytes as the shared secret.
				publicKey, err := x509.MarshalPKIXPublicKey(provider.private.Public())
				Expect(err).NotTo(HaveOccurred())

				signer, err := jose.NewSigner(
					jose.SigningKey{Algorithm: jose.HS256, Key: publicKey},
					(&jose.SignerOptions{}).WithHeader(jose.HeaderKey("kid"), provider.keyID),
				)
				Expect(err).NotTo(HaveOccurred())

				rawToken, err = jwt.Signed(signer).Claims(jwt.Claims{
					Issuer:   issuer,
					Subject:  subject,
					Audience: jwt.Audience{audience},
					Expiry:   jwt.NewNumericDate(now.Add(time.Hour)),
				}).Serialize()
				Expect(err).NotTo(HaveOccurred())
			})

			It("rejects it: only asymmetric algorithms are accepted", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		When("the token is not a JWT at all", func() {
			BeforeEach(func() {
				rawToken = "not-a-token"
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		When("the token is empty", func() {
			BeforeEach(func() {
				rawToken = ""
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})

		DescribeTable("a registered claim the provider left out",
			func(omitted string) {
				verifier = nil
				rawToken = valid.signWithoutClaim(provider, omitted)
				buildVerifier()

				_, err := verifier.Verify(context.Background(), rawToken, "")
				Expect(err).To(MatchError(oidc.ErrTokenRejected),
					"an absent %q is not validated by go-jose, so it has to be refused explicitly", omitted)
			},
			Entry("no expiry, which would otherwise never expire", "exp"),
			Entry("no issuer, which would otherwise match anything", "iss"),
			Entry("no audience, which would otherwise match anything", "aud"),
			Entry("no subject, which would otherwise be an empty identity", "sub"),
		)

		When("the provider cannot be reached to look the key up", func() {
			BeforeEach(func() {
				failing, err := oidc.NewCachedKeys(oidc.CacheConfig{
					Fetcher:            &countingFetcher{err: errFetch},
					TTL:                0,
					MinRefetchInterval: 0,
					Now:                nil,
				})
				Expect(err).NotTo(HaveOccurred())
				keys = failing
				rawToken = valid.sign(provider)
			})

			It("does not report a rejected token: the outage is ours, not the client's", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).NotTo(MatchError(oidc.ErrTokenRejected))
				Expect(err).To(MatchError(errFetch))
			})
		})

		Context("when the app bound the sign-in to a nonce", func() {
			BeforeEach(func() {
				withNonce := valid
				withNonce.nonce = "nonce-from-this-sign-in"
				rawToken = withNonce.sign(provider)
			})

			When("the caller names the same nonce", func() {
				BeforeEach(func() {
					expectedNonce = "nonce-from-this-sign-in"
				})

				It("accepts it", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(identity.Subject).To(Equal(subject))
				})
			})

			When("the caller names a different nonce", func() {
				BeforeEach(func() {
					expectedNonce = "some-other-nonce"
				})

				It("rejects it", func() {
					Expect(err).To(MatchError(oidc.ErrTokenRejected))
				})
			})

			When("the caller names no nonce", func() {
				It("rejects it: a replayed token must not lose its binding", func() {
					Expect(err).To(MatchError(oidc.ErrTokenRejected))
				})
			})
		})

		When("the caller names a nonce the token is not bound to", func() {
			BeforeEach(func() {
				rawToken = valid.sign(provider)
				expectedNonce = "nonce-the-provider-never-saw"
			})

			It("rejects it rather than reporting a binding that was never made", func() {
				Expect(err).To(MatchError(oidc.ErrTokenRejected))
			})
		})
	})

	Describe("NewVerifier", func() {
		DescribeTable("a configuration that cannot verify anything",
			func(cfg oidc.Config) {
				_, err := oidc.NewVerifier(cfg)
				Expect(err).To(HaveOccurred())
			},
			Entry("no issuer, which would accept a token from anybody",
				oidc.Config{Issuers: nil, Audiences: []string{audience}, Keys: staticKeys{}, Now: nil}),
			Entry("no audience, which makes go-jose skip the audience check",
				oidc.Config{Issuers: []string{issuer}, Audiences: nil, Keys: staticKeys{}, Now: nil}),
			Entry("no key lookup",
				oidc.Config{Issuers: []string{issuer}, Audiences: []string{audience}, Keys: nil, Now: nil}),
		)
	})
})

// tamperSignature changes one byte of a token's signature, leaving its header and
// payload — and so its key id — alone.
//
// It flips the *first* character of the signature segment rather than the last,
// and that is the whole point of the helper. A JWS signature is base64url of a
// byte count that is not a multiple of three, so its final character carries
// padding bits the decoder throws away: changing "A" to "B" there decodes to a
// byte-identical signature, the token verifies, and the spec asserting it is
// rejected fails. RS256's 256-byte signature leaves four padding bits, which
// made this a one-in-four flake — measured at 5 failures in 20 runs. The first
// character of the segment is always fully significant.
func tamperSignature(token string) string {
	GinkgoHelper()

	dot := strings.LastIndex(token, ".")
	Expect(dot).To(BeNumerically(">", 0), "token has no signature segment")
	Expect(len(token)).To(BeNumerically(">", dot+1), "token has an empty signature segment")

	flipped := []byte(token)
	if flipped[dot+1] == 'A' {
		flipped[dot+1] = 'B'
	} else {
		flipped[dot+1] = 'A'
	}
	return string(flipped)
}
