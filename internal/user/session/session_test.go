package session_test

import (
	"os"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/user/session"
)

var _ = Describe("Tokens", func() {
	// secret is exactly at the floor, so a spec that shortens it by one byte is
	// testing the floor rather than a number nobody chose.
	secret := strings.Repeat("s", session.MinSecretBytes)
	userID := uuid.New()

	var (
		clock  time.Time
		tokens *session.Tokens
	)

	BeforeEach(func() {
		clock = time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)

		var err error
		tokens, err = session.New(
			session.Config{Secret: secret, TTL: time.Hour},
			func() time.Time { return clock },
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Issue then Verify", func() {
		It("round-trips the user id", func() {
			issued, err := tokens.Issue(userID)
			Expect(err).NotTo(HaveOccurred())

			verified, err := tokens.Verify(issued.Value)
			Expect(err).NotTo(HaveOccurred())
			Expect(verified).To(Equal(userID))
		})

		It("reports when the session stops being accepted", func() {
			issued, err := tokens.Issue(userID)
			Expect(err).NotTo(HaveOccurred())
			Expect(issued.ExpiresAt).To(Equal(clock.Add(time.Hour)))
		})

		It("does not carry the token value anywhere but the token", func() {
			issued, err := tokens.Issue(userID)
			Expect(err).NotTo(HaveOccurred())
			// A session token is a bearer credential: the secret that signed it
			// must not be recoverable from it.
			Expect(issued.Value).NotTo(ContainSubstring(secret))
		})
	})

	Describe("Verify", func() {
		var (
			rawToken string
			verified uuid.UUID
			err      error
		)

		JustBeforeEach(func() {
			verified, err = tokens.Verify(rawToken)
		})

		When("the session has expired", func() {
			BeforeEach(func() {
				issued, issueErr := tokens.Issue(userID)
				Expect(issueErr).NotTo(HaveOccurred())
				rawToken = issued.Value
				clock = clock.Add(time.Hour + time.Minute)
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
				Expect(verified).To(Equal(uuid.Nil))
			})
		})

		When("the token was signed with a different secret", func() {
			BeforeEach(func() {
				other, newErr := session.New(
					session.Config{Secret: strings.Repeat("x", session.MinSecretBytes), TTL: time.Hour},
					func() time.Time { return clock },
				)
				Expect(newErr).NotTo(HaveOccurred())

				issued, issueErr := other.Issue(userID)
				Expect(issueErr).NotTo(HaveOccurred())
				rawToken = issued.Value
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
			})
		})

		When("the signature was tampered with", func() {
			BeforeEach(func() {
				issued, issueErr := tokens.Issue(userID)
				Expect(issueErr).NotTo(HaveOccurred())
				rawToken = issued.Value[:len(issued.Value)-1] + "A"
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
			})
		})

		When("the token was issued by something else with our secret", func() {
			BeforeEach(func() {
				// Same secret, wrong issuer: a token minted for another service
				// that happens to share a secret is not a session here.
				rawToken = signWith(secret, jwt.Claims{
					Issuer:   "somebody-else",
					Subject:  userID.String(),
					Audience: jwt.Audience{"binder-app"},
					Expiry:   jwt.NewNumericDate(clock.Add(time.Hour)),
				})
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
			})
		})

		When("the token carries no expiry", func() {
			BeforeEach(func() {
				rawToken = signWith(secret, jwt.Claims{
					Issuer:   "binder-api",
					Subject:  userID.String(),
					Audience: jwt.Audience{"binder-app"},
				})
			})

			It("rejects it rather than accepting a session that never ends", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
			})
		})

		When("the subject is not a user id", func() {
			BeforeEach(func() {
				rawToken = signWith(secret, jwt.Claims{
					Issuer:   "binder-api",
					Subject:  "admin",
					Audience: jwt.Audience{"binder-app"},
					Expiry:   jwt.NewNumericDate(clock.Add(time.Hour)),
				})
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
			})
		})

		When("the token is not a JWT", func() {
			BeforeEach(func() {
				rawToken = "Bearer nonsense"
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
			})
		})

		When("there is no token", func() {
			BeforeEach(func() {
				rawToken = ""
			})

			It("rejects it", func() {
				Expect(err).To(MatchError(session.ErrSessionRejected))
			})
		})
	})

	Describe("New", func() {
		It("refuses a missing secret, with no fallback of any kind", func() {
			_, err := session.New(session.Config{Secret: "", TTL: time.Hour}, nil)
			Expect(err).To(MatchError(session.ErrSecretMissing))
		})

		It("refuses a secret one byte below the floor", func() {
			_, err := session.New(
				session.Config{Secret: strings.Repeat("s", session.MinSecretBytes-1), TTL: time.Hour},
				nil,
			)
			Expect(err).To(MatchError(session.ErrSecretTooShort))
		})

		It("defaults the TTL, so only the secret has to be configured", func() {
			defaulted, err := session.New(session.Config{Secret: secret, TTL: 0}, func() time.Time { return clock })
			Expect(err).NotTo(HaveOccurred())

			issued, err := defaulted.Issue(userID)
			Expect(err).NotTo(HaveOccurred())
			Expect(issued.ExpiresAt).To(Equal(clock.Add(session.DefaultTTL)))
		})
	})

	Describe("LoadConfig", func() {
		When("SESSION_JWT_SECRET is not set", func() {
			BeforeEach(func() {
				// Unset rather than trusting the ambient environment: this spec
				// is the exit-if-missing guarantee, so it must be the absence
				// that fails it and not a variable a CI runner happened to
				// export.
				previous, wasSet := os.LookupEnv("SESSION_JWT_SECRET")
				Expect(os.Unsetenv("SESSION_JWT_SECRET")).To(Succeed())
				DeferCleanup(func() {
					if wasSet {
						Expect(os.Setenv("SESSION_JWT_SECRET", previous)).To(Succeed())
					}
				})
			})

			It("fails, so a process with no signing secret never serves a request", func() {
				_, err := session.LoadConfig()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("SESSION_JWT_SECRET"))
			})
		})

		When("SESSION_JWT_SECRET is set but empty", func() {
			BeforeEach(func() {
				GinkgoT().Setenv("SESSION_JWT_SECRET", "")
			})

			It("loads, and New is what refuses it: env only checks presence", func() {
				cfg, err := session.LoadConfig()
				Expect(err).NotTo(HaveOccurred())

				_, err = session.New(cfg, nil)
				Expect(err).To(MatchError(session.ErrSecretMissing))
			})
		})

		When("SESSION_JWT_SECRET is set", func() {
			BeforeEach(func() {
				GinkgoT().Setenv("SESSION_JWT_SECRET", secret)
			})

			It("defaults the TTL to a day", func() {
				cfg, err := session.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.Secret).To(Equal(secret))
				Expect(cfg.TTL).To(Equal(session.DefaultTTL))
			})

			It("takes a shorter TTL from the environment", func() {
				GinkgoT().Setenv("SESSION_TTL", "15m")

				cfg, err := session.LoadConfig()
				Expect(err).NotTo(HaveOccurred())
				Expect(cfg.TTL).To(Equal(15 * time.Minute))
			})
		})
	})
})

// signWith mints a token with arbitrary claims, for the specs that need a
// well-signed token this server would not have issued.
func signWith(secret string, claims jwt.Claims) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.HS256, Key: []byte(secret)}, nil)
	Expect(err).NotTo(HaveOccurred())

	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	Expect(err).NotTo(HaveOccurred())
	return raw
}
