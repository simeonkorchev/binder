// Package model holds the user domain's types: one account, the provider
// identity that lets us recognise it again, and the contact details its owner
// chose to publish. See db/migrations/002_users.sql.
package model

import (
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AuthProvider is who vouched for an account. The values are the auth_provider
// enum in the database, so the string form is the storage form and there is no
// second table of translations to keep in step.
type AuthProvider string

const (
	// ProviderApple is Sign in with Apple.
	ProviderApple AuthProvider = "apple"
	// ProviderGoogle is Google Sign-In.
	ProviderGoogle AuthProvider = "google"
)

// AuthProviders is every provider an account can be signed in with, in the
// order the enum declares them. Everything that needs the list — the OpenAPI
// enum, the validation below — reads it from here, so adding a third provider
// is one line.
func AuthProviders() []AuthProvider {
	return []AuthProvider{ProviderApple, ProviderGoogle}
}

// ValidAuthProvider reports whether p is a provider this backend knows. The zero
// value is not one.
func ValidAuthProvider(p AuthProvider) bool {
	return slices.Contains(AuthProviders(), p)
}

// Identity is the provider identity a sign-in presents: which provider, and the
// subject it calls this person. The pair is unique — a subject is opaque and
// unique only within its provider — which is why the same subject string under
// two providers is two accounts.
type Identity struct {
	Provider AuthProvider
	// Subject is the provider's "sub" claim. It is opaque to us and is never
	// shown to anyone.
	Subject string
}

// ContactField names one of the contact details a user can publish. It exists so
// the boundary that rejects a blank one can say which field was blank without
// re-typing the JSON key at each call site.
type ContactField string

const (
	// ContactFieldEmail is the published email address.
	ContactFieldEmail ContactField = "email"
	// ContactFieldPhone is the published phone number.
	ContactFieldPhone ContactField = "phone"
)

// Contact is the contact details a user chose to publish, and nothing else. A
// nil field means "not shared", which is the normal state: an account that has
// not opted in to being contacted has no contact button and is fully
// representable.
//
// The provider's own email address is deliberately not in here and is not
// stored: publishing an address to buyers is a separate decision with separate
// consent, so the two must not be the same value.
type Contact struct {
	Email *string
	Phone *string
}

// Normalise trims the whitespace off each shared field and reports the fields
// that were nothing but whitespace.
//
// It is one function with two callers on purpose. The API boundary uses blank to
// tell the client which field is wrong, and the service uses it to refuse the
// write; a blank string must not reach the database, where it would satisfy
// every "is it shared?" check that only tests for NULL — which is exactly what
// the users_contact_email_not_blank constraint exists to stop.
func (c Contact) Normalise() (normalised Contact, blank []ContactField) {
	normalised.Email, blank = trimShared(c.Email, ContactFieldEmail, blank)
	normalised.Phone, blank = trimShared(c.Phone, ContactFieldPhone, blank)
	return normalised, blank
}

// trimShared trims one optional field, appending its name to blank when there
// was nothing but whitespace in it. A field that was not shared at all stays
// nil and is not blank: not opting in is not an error.
func trimShared(value *string, field ContactField, blank []ContactField) (*string, []ContactField) {
	if value == nil {
		return nil, blank
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, append(blank, field)
	}
	return &trimmed, blank
}

// User is one account.
type User struct {
	ID       uuid.UUID
	Identity Identity
	Contact  Contact
	// CreatedAt is when the account first signed in.
	CreatedAt time.Time
	// UpdatedAt changes when the contact details do. A sign-in does not touch
	// it: repeat sign-ins are the common case and would make the column mean
	// "last seen", which nothing asks for.
	UpdatedAt time.Time
}

// SignIn is one attempt to exchange a provider's identity token for a session.
type SignIn struct {
	Provider AuthProvider
	// IdentityToken is the provider's ID token exactly as the app received it.
	// It is credential material: it is never logged, and it never leaves this
	// process (004-security.md).
	IdentityToken string
	// Nonce is what the app bound this sign-in to, when it bound it to
	// anything. Empty means the app sent no nonce to the provider, and the
	// token must then carry none either.
	Nonce string
}

// Session is what a successful sign-in returns: the token the app puts on every
// later request, when it stops being accepted, and the account it belongs to.
type Session struct {
	Token     string
	ExpiresAt time.Time
	User      User
}
