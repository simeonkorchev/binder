package api_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	binderapi "github.com/simeonkorchev/binder/internal/binder/api"
	binderapifakes "github.com/simeonkorchev/binder/internal/binder/api/apifakes"
	bindermodel "github.com/simeonkorchev/binder/internal/binder/model"
	listingapi "github.com/simeonkorchev/binder/internal/listing/api"
	"github.com/simeonkorchev/binder/internal/user/api"
	"github.com/simeonkorchev/binder/internal/user/api/apifakes"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/internal/user/session"
	"github.com/simeonkorchev/binder/pkg/humaschema"
)

// The seam this domain exists to fill: one resolver serves every domain's
// handlers, so no other domain declares a context key of its own
// (.claude/memory/decisions.md, 2026-09-16-binder-api-takes-an-actorfunc-until-w7-lands).
// These fail to compile the day a signature drifts.
var (
	_ binderapi.ActorFunc  = api.ActorFromContext
	_ listingapi.ActorFunc = api.ActorFromContext
	_ api.ActorFunc        = api.ActorFromContext
)

var _ = Describe("Middleware and ActorFromContext", func() {
	userID := uuid.New()
	secret := strings.Repeat("s", session.MinSecretBytes)

	var (
		tokens  *session.Tokens
		valid   session.Token
		fakeSvc *apifakes.FakeService
		handler http.Handler
	)

	BeforeEach(func() {
		var err error
		tokens, err = session.New(session.Config{Secret: secret, TTL: time.Hour}, nil)
		Expect(err).NotTo(HaveOccurred())

		valid, err = tokens.Issue(userID)
		Expect(err).NotTo(HaveOccurred())

		fakeSvc = new(apifakes.FakeService)
		fakeSvc.GetUserReturns(model.User{
			ID:        userID,
			Identity:  model.Identity{Provider: model.ProviderApple, Subject: "001234.apple.subject.5678"},
			Contact:   model.Contact{Email: nil, Phone: nil},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil)

		// The real thing: this domain's endpoints behind this domain's
		// middleware, resolving the actor through this domain's resolver.
		handler = api.Middleware(tokens)(newTestHandler(fakeSvc, api.ActorFromContext))
	})

	// get drives one authenticated read of the account through the whole stack.
	get := func(authorization string) int {
		headers := map[string]string{}
		if authorization != "" {
			headers["Authorization"] = authorization
		}
		return doWithHeaders(handler, http.MethodGet, "/me", nil, headers).Code
	}

	When("the request carries a session this server issued", func() {
		It("resolves it to the user the token was issued to", func() {
			Expect(get("Bearer " + valid.Value)).To(Equal(http.StatusOK))

			Expect(fakeSvc.GetUserCallCount()).To(Equal(1))
			_, asked := fakeSvc.GetUserArgsForCall(0)
			Expect(asked).To(Equal(userID))
		})

		It("accepts the scheme in any case: RFC 7235 says it is case-insensitive", func() {
			Expect(get("bearer " + valid.Value)).To(Equal(http.StatusOK))
		})
	})

	DescribeTable("a request with no usable session",
		func(authorization string) {
			Expect(get(authorization)).To(Equal(http.StatusUnauthorized))
			Expect(fakeSvc.GetUserCallCount()).To(BeZero(),
				"an unauthenticated request must not reach the service")
		},
		Entry("no Authorization header at all", ""),
		Entry("an empty header", " "),
		Entry("another scheme", "Basic dXNlcjpwYXNz"),
		Entry("the scheme with nothing after it", "Bearer "),
		Entry("a token that is not a JWT", "Bearer not-a-token"),
		Entry("the token without its scheme", "eyJhbGciOiJIUzI1NiJ9.e30.x"),
	)

	When("the token was signed with another secret", func() {
		It("answers 401 and does not reach the service", func() {
			other, err := session.New(
				session.Config{Secret: strings.Repeat("x", session.MinSecretBytes), TTL: time.Hour}, nil)
			Expect(err).NotTo(HaveOccurred())

			forged, err := other.Issue(userID)
			Expect(err).NotTo(HaveOccurred())

			Expect(get("Bearer " + forged.Value)).To(Equal(http.StatusUnauthorized))
			Expect(fakeSvc.GetUserCallCount()).To(BeZero())
		})
	})

	When("the session has expired", func() {
		It("answers 401", func() {
			clock := time.Now()
			shortLived, err := session.New(
				session.Config{Secret: secret, TTL: time.Minute},
				func() time.Time { return clock },
			)
			Expect(err).NotTo(HaveOccurred())

			issued, err := shortLived.Issue(userID)
			Expect(err).NotTo(HaveOccurred())

			clock = clock.Add(time.Hour)
			expiredHandler := api.Middleware(shortLived)(newTestHandler(fakeSvc, api.ActorFromContext))
			rec := doWithHeaders(expiredHandler, http.MethodGet, "/me", nil,
				map[string]string{"Authorization": "Bearer " + issued.Value})

			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
		})
	})

	When("a token is rejected", func() {
		It("logs nothing: the token is attacker-controlled bytes and the 401 is the record", func() {
			logged := &bytes.Buffer{}
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
			DeferCleanup(func() { slog.SetDefault(previous) })

			Expect(get("Bearer forged.token.value")).To(Equal(http.StatusUnauthorized))
			Expect(logged.String()).NotTo(ContainSubstring("forged.token.value"))
			Expect(logged.String()).To(BeEmpty())
		})
	})

	When("an endpoint needs no actor", func() {
		It("still works without a session: the middleware authenticates, it does not authorise", func() {
			fakeSvc.SignInReturns(model.Session{
				Token:     "the-session-token",
				ExpiresAt: time.Now().Add(time.Hour),
				User:      model.User{ID: userID},
			}, nil)

			rec := doWithHeaders(handler, http.MethodPost, "/auth/sessions", map[string]any{
				"provider":      "apple",
				"identityToken": "the-providers-identity-token",
			}, nil)

			Expect(rec.Code).To(Equal(http.StatusCreated))
			Expect(fakeSvc.SignInCallCount()).To(Equal(1))
		})
	})

	Describe("ActorFromContext", func() {
		It("names the user WithActor put on the context", func() {
			resolved, err := api.ActorFromContext(api.WithActor(context.Background(), userID))
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(userID))
		})

		It("refuses a context with no actor on it", func() {
			_, err := api.ActorFromContext(context.Background())
			Expect(err).To(MatchError(api.ErrNoActor))
		})

		It("refuses the nil uuid, which would otherwise be the actor for every row", func() {
			_, err := api.ActorFromContext(api.WithActor(context.Background(), uuid.Nil))
			Expect(err).To(MatchError(api.ErrNoActor))
		})
	})
})

// The binder domain was built before this one and took an ActorFunc as the seam
// to fill later. This drives one of its real endpoints through this domain's
// middleware and resolver, so "wired up" is a passing spec rather than a claim.
var _ = Describe("Another domain behind this domain's middleware", func() {
	userID := uuid.New()
	secret := strings.Repeat("s", session.MinSecretBytes)

	var (
		tokens     *session.Tokens
		valid      session.Token
		fakeBinder *binderapifakes.FakeService
		handler    http.Handler
	)

	BeforeEach(func() {
		var err error
		tokens, err = session.New(session.Config{Secret: secret, TTL: time.Hour}, nil)
		Expect(err).NotTo(HaveOccurred())
		valid, err = tokens.Issue(userID)
		Expect(err).NotTo(HaveOccurred())

		fakeBinder = new(binderapifakes.FakeService)
		fakeBinder.CreateBinderReturns(bindermodel.Binder{
			ID:        uuid.New(),
			OwnerID:   userID,
			Name:      "Trade binder",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}, nil)

		mux := http.NewServeMux()
		binderapi.RegisterEndpoints(
			humago.New(mux, humaschema.Config("Binder", "test")), fakeBinder, api.ActorFromContext)
		handler = api.Middleware(tokens)(mux)
	})

	It("creates the binder for the user the session names", func() {
		rec := doWithHeaders(handler, http.MethodPost, "/binders", map[string]any{"name": "Trade binder"},
			map[string]string{"Authorization": "Bearer " + valid.Value})
		Expect(rec.Code).To(Equal(http.StatusCreated))

		Expect(fakeBinder.CreateBinderCallCount()).To(Equal(1))
		_, ownerID, _ := fakeBinder.CreateBinderArgsForCall(0)
		Expect(ownerID).To(Equal(userID))
	})

	It("answers 401 without a session, and creates nothing", func() {
		rec := doWithHeaders(handler, http.MethodPost, "/binders", map[string]any{"name": "Trade binder"}, nil)
		Expect(rec.Code).To(Equal(http.StatusUnauthorized))
		Expect(fakeBinder.CreateBinderCallCount()).To(BeZero())
	})
})
