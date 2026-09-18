package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// bearerPrefix is the scheme an Authorization header carries a session under.
const bearerPrefix = "Bearer "

// ErrNoActor means the request carries no session this server issued and is still
// accepting. Every handler that needs a user answers it with a 401.
//
// It is one error for "no header", "a header we cannot parse" and "a token that
// does not verify" on purpose: all three are the same request from the client's
// point of view, and the reason a token was refused is not something to tell the
// holder of a forged one.
var ErrNoActor = errors.New("request has no usable session")

// actorKey is the context key the signed-in user id is stored under. An
// unexported zero-sized type cannot collide with a key from another package and
// needs no package-level variable.
//
// It stays in this package. Identity is the user domain's business, and a key
// copied into the binder or listing domain would be two declarations that have to
// stay equal — which is why those domains take an ActorFunc instead
// (002-go-conventions.md section 3a: a cross-domain dependency is a consumer-side
// interface, never a reach into another domain's package).
type actorKey struct{}

// SessionVerifier resolves a session token to the user it was issued to. It is
// declared here, next to the middleware that is its only caller; one concrete
// *session.Tokens satisfies it.
//
// The contract: any token that is not an acceptable session — malformed, forged,
// expired, or not issued by this server — is an error. A nil error means this
// user id is authenticated.
type SessionVerifier interface {
	Verify(rawToken string) (uuid.UUID, error)
}

// Middleware resolves the session on each request and puts the user it belongs to
// on the request context.
//
// It authenticates; it does not authorise. A request with no session, or with one
// that does not verify, is passed through with no actor on its context rather than
// rejected here — because some endpoints have no actor by design (the sign-in
// itself, browsing the marketplace), and the ones that need one answer 401
// themselves through ActorFromContext. Rejecting here would mean maintaining a
// list of public paths inside the middleware, which is a second place for the
// routing table to be wrong.
//
// Nothing is logged. A rejected token is attacker-controlled bytes and an
// accepted one is a bearer credential, so neither belongs in a log line
// (004-security.md); the 401 the handler returns is the record that it happened.
func Middleware(verifier SessionVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			userID, err := verifier.Verify(token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), userID)))
		})
	}
}

// WithActor returns a context carrying userID as the signed-in user.
//
// Production code reaches this through Middleware; it is exported so that a test
// of another domain can build an authenticated context without minting a real
// session, and so main.go can wire a request-scoped actor in if it ever needs to.
func WithActor(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, actorKey{}, userID)
}

// ActorFromContext names the signed-in user behind a request, and is the
// ActorFunc every domain's handlers resolve their user through: main.go passes it
// to internal/binder/api.RegisterEndpoints, internal/listing/api.RegisterEndpoints
// and this package's own.
//
// A request that Middleware found no usable session on has no actor, which is
// ErrNoActor and a 401 at the handler.
func ActorFromContext(ctx context.Context) (uuid.UUID, error) {
	userID, ok := ctx.Value(actorKey{}).(uuid.UUID)
	if !ok {
		return uuid.Nil, ErrNoActor
	}
	// The nil uuid is not a user. It cannot come from Middleware, which only
	// stores what a verified session named, but it is what a zero value looks
	// like, and letting it through would make it the actor for every row keyed on
	// owner_id.
	if userID == uuid.Nil {
		return uuid.Nil, ErrNoActor
	}
	return userID, nil
}

// bearerToken pulls the session out of an Authorization header, reporting whether
// there was one to pull. The scheme is matched case-insensitively because RFC 7235
// says it is case-insensitive, and an app that sends "bearer" is not wrong.
func bearerToken(header string) (string, bool) {
	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(bearerPrefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
