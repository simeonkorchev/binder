// Package service holds the user domain's business logic: turning a provider's
// identity token into a session on this backend, and keeping the contact details
// a user chose to publish distinguishable from the ones they did not.
//
// Nothing here logs a token, an email address or a phone number. The one thing
// worth recording about a sign-in is which account it was, so that is what is
// recorded (004-security.md).
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/internal/user/session"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

var (
	// ErrIdentityRejected is returned when the provider's identity token is not
	// acceptable proof of identity — forged, expired, addressed to another app,
	// signed by an unpublished key, or bound to a different nonce.
	//
	// It is one sentinel for all of those deliberately. Each of them is a
	// request that must not create or reach an account, the answer to every one
	// of them is the same 401, and a client told which check failed has been
	// handed an oracle for guessing at the next one.
	ErrIdentityRejected = errors.New("identity token rejected")

	// ErrProviderUnknown is returned when a sign-in names a provider this
	// backend does not support.
	ErrProviderUnknown = errors.New("sign-in provider is not supported")

	// ErrUserNotFound is returned for an account id that names no account.
	ErrUserNotFound = errors.New("user not found")

	// ErrContactBlank is returned when a contact detail arrives as whitespace.
	// It is refused rather than quietly treated as "not shared": the two are
	// different intentions, and a blank string stored in the column would read
	// as opted-in to everything that only tests for NULL.
	ErrContactBlank = errors.New("a shared contact detail cannot be blank")
)

// Store reads and writes accounts.
type Store interface {
	// EnsureUserByIdentity returns the account behind a provider identity,
	// creating it with newID on the first sign-in. newID is ignored when the
	// account already exists, so this is also how a repeat sign-in is resolved.
	EnsureUserByIdentity(ctx context.Context, newID uuid.UUID, identity model.Identity) (model.User, error)
	// GetUserByID returns a dataerror.MissingEntityError when there is no such
	// account.
	GetUserByID(ctx context.Context, id uuid.UUID) (model.User, error)
	// SetUserContact replaces both contact fields and returns the account as
	// stored, or a dataerror.MissingEntityError when there is no such account.
	// A blank string is not a legal value for either field.
	SetUserContact(ctx context.Context, id uuid.UUID, contact model.Contact) (model.User, error)
}

// IdentityVerifier proves that a provider's identity token belongs to the person
// presenting it.
//
// The contract, so a fake and the real verifier cannot drift: a token that is not
// acceptable — for any reason at all — is oidc.ErrTokenRejected. Anything else is
// an infrastructure failure, which must not be reported to the client as a bad
// token, and it is returned as itself.
//
//counterfeiter:generate . IdentityVerifier
type IdentityVerifier interface {
	Verify(ctx context.Context, provider model.AuthProvider, rawToken, nonce string) (model.Identity, error)
}

// SessionIssuer mints the session token that authenticates every request after
// the sign-in.
//
//counterfeiter:generate . SessionIssuer
type SessionIssuer interface {
	Issue(userID uuid.UUID) (session.Token, error)
}

//counterfeiter:generate . Store

// Service is the user domain's business logic.
type Service struct {
	store     Store
	verifier  IdentityVerifier
	sessions  SessionIssuer
	newUserID func() uuid.UUID
}

// NewService returns a service over store, verifying identity tokens through
// verifier and issuing sessions through sessions.
//
// newUserID mints the id a first sign-in gets. It is injected so a spec can
// assert which id was stored; nil means uuid.New.
func NewService(
	store Store, verifier IdentityVerifier, sessions SessionIssuer, newUserID func() uuid.UUID,
) *Service {
	if newUserID == nil {
		newUserID = uuid.New
	}
	return &Service{store: store, verifier: verifier, sessions: sessions, newUserID: newUserID}
}
