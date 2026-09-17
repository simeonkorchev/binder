package oidc_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

// errFetch is the don't-care fetcher failure: the specs using it assert that an
// unreachable provider is not reported as a rejected token, never this text.
var errFetch = errors.New("the provider could not be reached")

// rsaKeyBits is deliberately the smallest size Go will sign RS256 with: these
// keys are generated per spec run and thrown away, and 2048-bit generation is
// slow enough to be felt across a suite.
const rsaKeyBits = 2048

func TestOIDC(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OIDC Suite")
}

// signingKey is a throwaway key pair with a key id, standing in for a
// provider's. Every spec in this suite signs with one of these: no spec reaches
// appleid.apple.com or googleapis.com, and appleid.apple.com is in fact blocked
// from this environment.
type signingKey struct {
	keyID   string
	private *rsa.PrivateKey
}

func newSigningKey(keyID string) signingKey {
	private, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	Expect(err).NotTo(HaveOccurred())
	return signingKey{keyID: keyID, private: private}
}

// publicJWK is the key as the provider would publish it.
func (k signingKey) publicJWK() jose.JSONWebKey {
	return jose.JSONWebKey{
		Key:       k.private.Public(),
		KeyID:     k.keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}
}

// keySet is a JWKS holding these keys.
func keySet(keys ...signingKey) jose.JSONWebKeySet {
	published := make([]jose.JSONWebKey, 0, len(keys))
	for _, key := range keys {
		published = append(published, key.publicJWK())
	}
	return jose.JSONWebKeySet{Keys: published}
}

// claims is a provider's claim set, built field by field so a spec can leave
// exactly one of them wrong.
type claims struct {
	issuer   string
	subject  string
	audience string
	expiry   time.Time
	issuedAt time.Time
	nonce    string
}

// sign serialises the claims as a token signed by key.
func (c claims) sign(key signingKey) string {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key.private},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader(jose.HeaderKey("kid"), key.keyID),
	)
	Expect(err).NotTo(HaveOccurred())

	registered := jwt.Claims{
		Issuer:   c.issuer,
		Subject:  c.subject,
		Audience: jwt.Audience{c.audience},
		Expiry:   jwt.NewNumericDate(c.expiry),
		IssuedAt: jwt.NewNumericDate(c.issuedAt),
	}

	raw, err := jwt.Signed(signer).Claims(registered).Claims(map[string]any{"nonce": c.nonce}).Serialize()
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// signWithoutClaim serialises the claims with one registered claim left out, for
// the specs that pin that an absent claim is refused rather than skipped.
func (c claims) signWithoutClaim(key signingKey, omit string) string {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key.private},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader(jose.HeaderKey("kid"), key.keyID),
	)
	Expect(err).NotTo(HaveOccurred())

	body := map[string]any{
		"iss": c.issuer,
		"sub": c.subject,
		"aud": c.audience,
		"exp": c.expiry.Unix(),
		"iat": c.issuedAt.Unix(),
	}
	delete(body, omit)

	raw, err := jwt.Signed(signer).Claims(body).Serialize()
	Expect(err).NotTo(HaveOccurred())
	return raw
}

// staticKeys is a KeyLookup over a fixed set, for the specs that are about the
// verifier rather than about the cache.
type staticKeys struct {
	set jose.JSONWebKeySet
}

func (s staticKeys) Key(_ context.Context, keyID string) (jose.JSONWebKey, error) {
	matches := s.set.Key(keyID)
	if len(matches) != 1 {
		return jose.JSONWebKey{}, fmt.Errorf("%w: %q", oidc.ErrKeyUnknown, keyID)
	}
	return matches[0], nil
}

// countingFetcher answers with whatever set it is pointed at and records how
// often it was asked, which is how the cache specs tell a cache hit from a
// fetch.
type countingFetcher struct {
	set   jose.JSONWebKeySet
	err   error
	calls int
}

func (f *countingFetcher) Fetch(context.Context) (jose.JSONWebKeySet, error) {
	f.calls++
	if f.err != nil {
		return jose.JSONWebKeySet{}, f.err
	}
	return f.set, nil
}

// jwksServer serves a key set over HTTP, which is how the HTTPFetcher spec gets
// a provider endpoint without a provider.
func jwksServer(status int, body any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			Expect(json.NewEncoder(w).Encode(body)).To(Succeed())
		}
	}))
}
