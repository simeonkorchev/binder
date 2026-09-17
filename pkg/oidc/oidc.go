// Package oidc verifies an OpenID Connect identity token against the keys the
// issuing provider publishes.
//
// It knows nothing about Apple or Google: a provider here is an issuer string,
// the set of client ids ("audiences") this backend accepts a token for, and a
// way to look up a signing key by its id. What the package does know is the
// shape of the check, and that the check has to happen on this side — a mobile
// app can post any JSON it likes, so a signature over the provider's own claims
// is the only thing that turns a subject into an identity.
//
// Nothing here logs. A token, and every claim in it, is credential material and
// personal data; the caller logs the user id it ended up with instead
// (.claude/rules/004-security.md).
package oidc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// clockSkew is how far this server's clock may run ahead of the provider's
// before a token that is genuinely still valid starts being refused. It is
// deliberately small: go-jose's own default leeway is a full minute, which
// keeps accepting a token for a minute after it expires.
const clockSkew = 30 * time.Second

var (
	// ErrTokenRejected means the token is not acceptable proof of identity: a
	// bad signature, an unknown signing key, the wrong issuer or audience, an
	// expired token, a nonce that does not match, or a required claim that is
	// not there.
	//
	// It is deliberately one error for all of those. A client told *which*
	// check failed has been handed an oracle, and the caller mapping this to a
	// status has nothing else to decide: every one of them is a 401.
	ErrTokenRejected = errors.New("identity token rejected")

	// ErrKeyUnknown means the provider does not publish a key with that id.
	// It is its own error because a KeyLookup returns it and the verifier is
	// what decides it means "rejected": failing to *reach* the provider is our
	// problem rather than the client's, and must never become a 401.
	ErrKeyUnknown = errors.New("signing key id is not published by the provider")

	errNoIssuer    = errors.New("an issuer is required")
	errNoAudience  = errors.New("at least one audience is required")
	errNoKeyLookup = errors.New("a key lookup is required")
)

// KeyLookup hands the verifier a provider's signing key for one key id.
//
// It is an interface because behind it is a network call to the provider's JWKS
// endpoint. CachedKeys is the implementation that ships; a test generates its
// own key pair and serves the key set from an httptest server, so no spec in
// this repository depends on reaching Apple or Google.
//
// A key id the provider does not publish is ErrKeyUnknown. Anything else is an
// infrastructure failure and is returned as itself.
type KeyLookup interface {
	Key(ctx context.Context, keyID string) (jose.JSONWebKey, error)
}

// Identity is what a verified token proves: this is the same person the
// provider called this subject last time.
//
// The provider's email and name claims are deliberately not carried across.
// db/migrations/002_users.sql stores no provider email, because the address a
// user publishes for buyers is a separate decision with separate consent, so
// reading one here would only invite storing it.
type Identity struct {
	Subject string
}

// Config describes one provider.
type Config struct {
	// Issuer is the exact "iss" claim the provider puts in its tokens.
	Issuer string
	// Audiences are the client ids a token may be addressed to. It is a list
	// because a provider issues one client id per platform — an iOS and an
	// Android client are different audiences for the same account — and a
	// token is accepted when it names any of them.
	Audiences []string
	// Keys resolves the key id in a token header to the key that signed it.
	Keys KeyLookup
	// Now is the clock. nil means time.Now; a test sets it to make a token
	// expired without waiting.
	Now func() time.Time
}

// Verifier checks tokens from one provider.
type Verifier struct {
	issuer    string
	audiences []string
	keys      KeyLookup
	now       func() time.Time
}

// NewVerifier returns a verifier for cfg, refusing a configuration that cannot
// verify anything: an empty issuer or audience list would make
// jwt.Expected.Validate skip that check entirely rather than fail it.
func NewVerifier(cfg Config) (*Verifier, error) {
	switch {
	case cfg.Issuer == "":
		return nil, errNoIssuer
	case len(cfg.Audiences) == 0:
		return nil, errNoAudience
	case cfg.Keys == nil:
		return nil, errNoKeyLookup
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Verifier{
		issuer:    cfg.Issuer,
		audiences: cfg.Audiences,
		keys:      cfg.Keys,
		now:       now,
	}, nil
}

// identityClaims is the registered claim set plus the one extra claim a sign-in
// cares about.
type identityClaims struct {
	jwt.Claims
	// Nonce is the value the app asked the provider to bind this token to. It
	// is present only when the app sent one.
	Nonce string `json:"nonce"`
}

// Verify returns the identity a token proves, or ErrTokenRejected.
//
// expectedNonce is the nonce the app says it generated for this sign-in. It has
// to equal the token's nonce claim, including both being empty: see checkNonce.
func (v *Verifier) Verify(ctx context.Context, rawToken, expectedNonce string) (Identity, error) {
	token, err := jwt.ParseSigned(rawToken, signatureAlgorithms())
	if err != nil {
		return Identity{}, fmt.Errorf("%w: not a signed JWT: %w", ErrTokenRejected, err)
	}

	key, err := v.signingKey(ctx, token.Headers)
	if err != nil {
		return Identity{}, err
	}

	var claims identityClaims
	if err := token.Claims(key.Key, &claims); err != nil {
		return Identity{}, fmt.Errorf("%w: the signature does not verify: %w", ErrTokenRejected, err)
	}
	if err := v.checkClaims(claims, expectedNonce); err != nil {
		return Identity{}, err
	}

	return Identity{Subject: claims.Subject}, nil
}

// signingKey resolves the key that signed the token from its header.
func (v *Verifier) signingKey(ctx context.Context, headers []jose.Header) (jose.JSONWebKey, error) {
	if len(headers) != 1 {
		return jose.JSONWebKey{}, fmt.Errorf("%w: a token must carry exactly one signature", ErrTokenRejected)
	}

	keyID := headers[0].KeyID
	if keyID == "" {
		return jose.JSONWebKey{}, fmt.Errorf("%w: the token header names no key id", ErrTokenRejected)
	}

	key, err := v.keys.Key(ctx, keyID)
	if err != nil {
		if errors.Is(err, ErrKeyUnknown) {
			return jose.JSONWebKey{}, fmt.Errorf("%w: %w", ErrTokenRejected, err)
		}
		// Not ErrTokenRejected: failing to reach the provider says nothing
		// about the token, and answering 401 would blame the client for our
		// outage.
		return jose.JSONWebKey{}, fmt.Errorf("looking up the provider's signing key: %w", err)
	}

	return key, nil
}

// checkClaims validates the claim set against this provider's configuration.
func (v *Verifier) checkClaims(claims identityClaims, expectedNonce string) error {
	// Every registered claim is optional in a JWT, and go-jose validates only
	// the ones it finds — a token with no "exp" passes its expiry check
	// forever. Their absence therefore has to be refused here, explicitly,
	// before the validation below is worth anything.
	switch {
	case claims.Issuer == "":
		return fmt.Errorf("%w: no issuer claim", ErrTokenRejected)
	case claims.Subject == "":
		return fmt.Errorf("%w: no subject claim", ErrTokenRejected)
	case len(claims.Audience) == 0:
		return fmt.Errorf("%w: no audience claim", ErrTokenRejected)
	case claims.Expiry == nil:
		return fmt.Errorf("%w: no expiry claim", ErrTokenRejected)
	}

	expected := jwt.Expected{
		Issuer:      v.issuer,
		AnyAudience: jwt.Audience(v.audiences),
		Time:        v.now(),
	}
	if err := claims.ValidateWithLeeway(expected, clockSkew); err != nil {
		return fmt.Errorf("%w: %w", ErrTokenRejected, err)
	}

	return checkNonce(claims.Nonce, expectedNonce)
}

// checkNonce requires the token's nonce and the one the app says it generated
// to agree, including when both are absent.
//
// A token that carries a nonce is bound to one specific sign-in attempt.
// Accepting it from a caller that cannot name that nonce throws the binding
// away, which is the replay this claim exists to stop; accepting a caller's
// nonce for a token that carries none reports a binding that was never made.
// Whatever the app did to the value before handing it to the provider — some
// SDKs send a hash of it — it sends us the same value, so plain equality is the
// whole check.
func checkNonce(tokenNonce, expectedNonce string) error {
	if tokenNonce == expectedNonce {
		return nil
	}
	return fmt.Errorf("%w: the nonce does not match this sign-in", ErrTokenRejected)
}

// signatureAlgorithms are the only algorithms a provider's token may be signed
// with.
//
// Both are asymmetric on purpose. Allowing an HMAC algorithm here is the
// algorithm-confusion attack: an attacker signs their own claims using the
// provider's *public* key as the shared secret, and a verifier that accepts
// HS256 verifies it happily. "none" is absent for the same reason.
func signatureAlgorithms() []jose.SignatureAlgorithm {
	return []jose.SignatureAlgorithm{jose.RS256, jose.ES256}
}
