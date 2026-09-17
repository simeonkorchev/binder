package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

const (
	// maxJWKSBytes caps what a JWKS response may be. A key set is a few
	// hundred bytes; the limit is what stops a compromised or confused endpoint
	// from feeding this process an unbounded body.
	maxJWKSBytes = 1 << 20
	// fetchTimeout bounds one JWKS request. CachedKeys serialises fetches, so a
	// provider that accepts a connection and never answers would otherwise hold
	// every sign-in behind it for as long as the client's context allows.
	fetchTimeout = 10 * time.Second
	// defaultTTL is how long a fetched key set is served without asking again.
	// Providers publish a new key well before they sign with it, so this is
	// about how quickly a *withdrawn* key stops being trusted, not about
	// catching a rotation in time — defaultMinRefetchInterval does that.
	defaultTTL = 15 * time.Minute
	// defaultMinRefetchInterval is the floor between two fetches. It is what
	// stops a stream of invented key ids from turning every request into a
	// request against the provider.
	defaultMinRefetchInterval = time.Minute
)

var (
	errNoFetcher       = errors.New("a fetcher is required")
	errRefetchAboveTTL = errors.New("the refetch floor must not exceed the TTL")
	errJWKSUnusable    = errors.New("provider key set is unusable")
)

// Fetcher reads a provider's JWKS document.
//
// It is the only part of this package that touches the network, which is why it
// is an interface: a spec serves its own key set instead, and none of them
// depends on reaching a provider.
type Fetcher interface {
	Fetch(ctx context.Context) (jose.JSONWebKeySet, error)
}

// HTTPFetcher reads a JWKS from a URL.
type HTTPFetcher struct {
	url    string
	client *http.Client
}

// NewHTTPFetcher returns a fetcher for url. A nil client gets one with
// fetchTimeout; TLS verification is never relaxed, here or anywhere.
func NewHTTPFetcher(url string, client *http.Client) *HTTPFetcher {
	if client == nil {
		client = &http.Client{Timeout: fetchTimeout}
	}
	return &HTTPFetcher{url: url, client: client}
}

// Fetch reads and parses the key set.
func (f *HTTPFetcher) Fetch(ctx context.Context) (jose.JSONWebKeySet, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("building the key set request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("fetching the provider key set: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return jose.JSONWebKeySet{}, fmt.Errorf("%w: the key set endpoint answered %s", errJWKSUnusable, resp.Status)
	}

	var set jose.JSONWebKeySet
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJWKSBytes)).Decode(&set); err != nil {
		return jose.JSONWebKeySet{}, fmt.Errorf("decoding the provider key set: %w", err)
	}
	if len(set.Keys) == 0 {
		return jose.JSONWebKeySet{}, fmt.Errorf("%w: it holds no key", errJWKSUnusable)
	}

	return set, nil
}

// CacheConfig configures CachedKeys. Every duration has a default, so the zero
// value of everything but Fetcher is usable.
type CacheConfig struct {
	Fetcher Fetcher
	// TTL is how long a key set is served before it is fetched again.
	TTL time.Duration
	// MinRefetchInterval is the shortest time between two fetches. It must not
	// exceed TTL, or a stale set could never be replaced.
	MinRefetchInterval time.Duration
	// Now is the clock. nil means time.Now.
	Now func() time.Time
}

// CachedKeys serves a provider's signing keys from memory, fetching them again
// when they go stale and when it is asked for a key id it does not hold.
//
// The second case is what survives a key rotation: a provider that starts
// signing with a new key mid-TTL would otherwise fail every sign-in until the
// TTL ran out. MinRefetchInterval is what keeps that from being a way to make
// this process hammer the provider.
type CachedKeys struct {
	fetcher            Fetcher
	ttl                time.Duration
	minRefetchInterval time.Duration
	now                func() time.Time

	// mu guards keys and fetchedAt, and makes a fetch single-flight: the second
	// caller to find the cache stale waits here and then finds it filled,
	// rather than opening a second request to the provider.
	mu        sync.Mutex
	keys      jose.JSONWebKeySet
	fetchedAt time.Time
}

// NewCachedKeys returns a cache over cfg.Fetcher.
func NewCachedKeys(cfg CacheConfig) (*CachedKeys, error) {
	if cfg.Fetcher == nil {
		return nil, errNoFetcher
	}

	keys := &CachedKeys{
		fetcher:            cfg.Fetcher,
		ttl:                cfg.TTL,
		minRefetchInterval: cfg.MinRefetchInterval,
		now:                cfg.Now,
		mu:                 sync.Mutex{},
		keys:               jose.JSONWebKeySet{},
		// The zero time makes the first lookup arbitrarily old, which is what
		// makes it fetch.
		fetchedAt: time.Time{},
	}
	if keys.ttl == 0 {
		keys.ttl = defaultTTL
	}
	if keys.minRefetchInterval == 0 {
		keys.minRefetchInterval = defaultMinRefetchInterval
	}
	if keys.now == nil {
		keys.now = time.Now
	}
	if keys.minRefetchInterval > keys.ttl {
		return nil, errRefetchAboveTTL
	}

	return keys, nil
}

// Key returns the key with that id, fetching the provider's key set when the
// cached one is stale or does not hold it.
func (c *CachedKeys) Key(ctx context.Context, keyID string) (jose.JSONWebKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	age := c.now().Sub(c.fetchedAt)
	key, found := findKey(c.keys, keyID)
	switch {
	case found && age < c.ttl:
		return key, nil
	case !found && age < c.minRefetchInterval:
		// Asked for a key id we have not got, too soon after the last fetch to
		// look again. This is the ordinary answer to a forged key id.
		return jose.JSONWebKey{}, fmt.Errorf("%w: %q", ErrKeyUnknown, keyID)
	}

	set, err := c.fetcher.Fetch(ctx)
	if err != nil {
		return jose.JSONWebKey{}, fmt.Errorf("refreshing the provider key set: %w", err)
	}
	c.keys = set
	c.fetchedAt = c.now()

	key, found = findKey(c.keys, keyID)
	if !found {
		return jose.JSONWebKey{}, fmt.Errorf("%w: %q", ErrKeyUnknown, keyID)
	}

	return key, nil
}

// findKey picks the key with that id out of a set. A set holding two keys under
// one id is a provider error we cannot resolve, so neither is used: verifying
// against an arbitrary one of them would be a coin toss.
func findKey(set jose.JSONWebKeySet, keyID string) (jose.JSONWebKey, bool) {
	matches := set.Key(keyID)
	if len(matches) != 1 {
		return jose.JSONWebKey{}, false
	}
	return matches[0], true
}
