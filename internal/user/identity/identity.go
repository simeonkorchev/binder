// Package identity is the one place that knows Apple from Google.
//
// pkg/oidc verifies a token against a provider described as an issuer, a set of
// accepted client ids and a key lookup. This package holds those descriptions
// for the two providers D1 chose, and multiplexes a sign-in onto the right one,
// so that neither the service nor the API layer ever branches on which provider
// a request named.
//
// The client ids come from the environment: they are per-app, they differ
// between a development build and the App Store build, and they are not
// secrets. The issuers and the key endpoints are facts about the providers and
// are constants.
package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/caarlos0/env/v11"
	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

const (
	// appleIssuer is the "iss" claim in a Sign in with Apple identity token.
	appleIssuer = "https://appleid.apple.com"
	// appleKeysURL is Apple's JWKS endpoint.
	//
	// It is unreachable from this project's agent containers (403 at the egress
	// proxy), which is why every spec in this repository serves its own key set
	// and no spec calls this URL. A deployed process does reach it.
	appleKeysURL = "https://appleid.apple.com/auth/keys"

	// googleIssuer is the "iss" claim in a Google ID token. Google issues both
	// this and the bare "accounts.google.com"; accepting only one of them would
	// refuse tokens at random, so both are configured.
	googleIssuer = "https://accounts.google.com"
	// googleIssuerLegacy is the same issuer without a scheme, which Google still
	// puts in some tokens.
	googleIssuerLegacy = "accounts.google.com"
	// googleKeysURL is Google's JWKS endpoint.
	googleKeysURL = "https://www.googleapis.com/oauth2/v3/certs"
)

var (
	// ErrProviderUnknown means the sign-in named a provider this backend is not
	// configured for.
	ErrProviderUnknown = errors.New("sign-in provider is not configured")

	errNoAppleAudience  = errors.New("APPLE_CLIENT_IDS is empty")
	errNoGoogleAudience = errors.New("GOOGLE_CLIENT_IDS is empty")
)

// Config is the per-app half of the provider descriptions, read from the
// environment at startup.
type Config struct {
	// AppleClientIDs are the Apple service/bundle ids a token may be addressed
	// to. A list because the iOS app and any web client are separate ids.
	AppleClientIDs []string `env:"APPLE_CLIENT_IDS,required" envSeparator:","`
	// GoogleClientIDs are the OAuth client ids a Google token may be addressed
	// to. Google issues one per platform, so the iOS and Android clients of one
	// app are different audiences for the same account.
	GoogleClientIDs []string `env:"GOOGLE_CLIENT_IDS,required" envSeparator:","`
}

// LoadConfig reads the provider client ids from the environment. Like the
// session secret, a missing one is a startup failure: a backend that cannot
// name its own audience would accept a token issued for somebody else's app.
func LoadConfig() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("reading the provider configuration: %w", err)
	}
	return cfg, nil
}

// ProviderVerifier is the part of oidc.Verifier this package uses. Declaring it
// here rather than taking the concrete type is what lets the multiplexing be
// driven without a network, a key pair or a clock the caller does not control.
type ProviderVerifier interface {
	Verify(ctx context.Context, rawToken, expectedNonce string) (oidc.Identity, error)
}

// Verifiers verifies an identity token against whichever provider issued it.
type Verifiers struct {
	byProvider map[model.AuthProvider]ProviderVerifier
}

// NewFromConfig builds the Apple and Google verifiers cfg describes. client is
// the HTTP client the key fetches use; nil gets one with a timeout.
func NewFromConfig(cfg Config, client *http.Client) (*Verifiers, error) {
	if len(cfg.AppleClientIDs) == 0 {
		return nil, errNoAppleAudience
	}
	if len(cfg.GoogleClientIDs) == 0 {
		return nil, errNoGoogleAudience
	}

	apple, err := newProviderVerifier([]string{appleIssuer}, cfg.AppleClientIDs, appleKeysURL, client)
	if err != nil {
		return nil, fmt.Errorf("configuring Apple sign-in: %w", err)
	}

	google, err := newProviderVerifier(
		[]string{googleIssuer, googleIssuerLegacy}, cfg.GoogleClientIDs, googleKeysURL, client,
	)
	if err != nil {
		return nil, fmt.Errorf("configuring Google sign-in: %w", err)
	}

	return New(map[model.AuthProvider]ProviderVerifier{
		model.ProviderApple:  apple,
		model.ProviderGoogle: google,
	}), nil
}

// New returns a multiplexer over one verifier per provider. NewFromConfig is
// what the bootstrap calls; this is the seam underneath it, so the multiplexing
// is covered without a provider and a third provider is a map entry rather than
// a branch.
func New(byProvider map[model.AuthProvider]ProviderVerifier) *Verifiers {
	// Copied rather than kept: a caller that mutated its map afterwards would be
	// changing which tokens this backend accepts.
	own := make(map[model.AuthProvider]ProviderVerifier, len(byProvider))
	for provider, v := range byProvider {
		own[provider] = v
	}
	return &Verifiers{byProvider: own}
}

// Verify returns the provider identity the token proves. It is the
// service.IdentityVerifier the user service consumes.
//
// The error is oidc.ErrTokenRejected for a token that is not acceptable, and
// ErrProviderUnknown for a provider this backend has no configuration for.
func (v *Verifiers) Verify(
	ctx context.Context, provider model.AuthProvider, rawToken, nonce string,
) (model.Identity, error) {
	chosen, ok := v.byProvider[provider]
	if !ok {
		return model.Identity{}, fmt.Errorf("%w: %q", ErrProviderUnknown, provider)
	}

	identity, err := chosen.Verify(ctx, rawToken, nonce)
	if err != nil {
		return model.Identity{}, fmt.Errorf("verifying the %s identity token: %w", provider, err)
	}

	return model.Identity{Provider: provider, Subject: identity.Subject}, nil
}

// newProviderVerifier builds one provider's verifier over its own cached key
// set. Each provider gets its own cache: the two rotate keys independently, and
// one shared cache would hold whichever set was fetched last.
func newProviderVerifier(
	issuers, audiences []string, keysURL string, client *http.Client,
) (*oidc.Verifier, error) {
	keys, err := oidc.NewCachedKeys(oidc.CacheConfig{
		Fetcher:            oidc.NewHTTPFetcher(keysURL, client),
		TTL:                0,
		MinRefetchInterval: 0,
		Now:                nil,
	})
	if err != nil {
		return nil, err
	}
	return oidc.NewVerifier(oidc.Config{Issuers: issuers, Audiences: audiences, Keys: keys, Now: nil})
}
