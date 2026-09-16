package ygoprodeck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// DefaultCardInfoURL is the full-dump endpoint (assumption A1).
	DefaultCardInfoURL = "https://db.ygoprodeck.com/api/v7/cardinfo.php"

	// DefaultRequestsPerSecond is the documented ceiling (assumption A5). It is
	// a default rather than a hard limit so an operator who has been told a
	// different number can set it without editing this package.
	DefaultRequestsPerSecond = 20

	// DefaultTimeout covers one response carrying the whole card database, not
	// one small request: the dump is tens of megabytes.
	DefaultTimeout = 2 * time.Minute

	// userAgent identifies this importer to the service. It names the project
	// rather than the operator: no PII goes upstream (004-security.md).
	userAgent = "binder-cardimport (+https://github.com/simeonkorchev/binder)"
)

// ErrUnexpectedStatus is returned when the API answers with anything but 200.
// Exported because the importer distinguishes "the service refused us" from
// "the body did not parse".
var ErrUnexpectedStatus = errors.New("unexpected response status")

// Doer performs an HTTP request. *http.Client satisfies it; tests substitute a
// stub, which is the only way this package can be exercised in an environment
// that cannot reach the API.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config configures a Client. Every field has a working zero value.
type Config struct {
	// CardInfoURL overrides DefaultCardInfoURL.
	CardInfoURL string
	// RequestsPerSecond overrides DefaultRequestsPerSecond.
	RequestsPerSecond float64
	// HTTPClient overrides an *http.Client with DefaultTimeout.
	HTTPClient Doer
	// Clock overrides the wall clock the rate limiter paces against.
	Clock Clock
}

// Client reads the card database. It is safe for concurrent use: the rate
// limiter it shares across calls is.
type Client struct {
	cardInfoURL string
	httpClient  Doer
	limiter     *RateLimiter
}

// NewClient builds a Client from cfg, filling in the documented defaults.
func NewClient(cfg Config) *Client {
	if cfg.CardInfoURL == "" {
		cfg.CardInfoURL = DefaultCardInfoURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: DefaultTimeout}
	}

	return &Client{
		cardInfoURL: cfg.CardInfoURL,
		httpClient:  cfg.HTTPClient,
		limiter:     NewRateLimiter(cfg.RequestsPerSecond, cfg.Clock),
	}
}

// FetchAllCards returns every card in the database in one request (assumption
// A1): one request beats thirteen thousand, and the importer needs all of them
// anyway. An empty database is an empty slice and no error.
//
// This is the only place in the package that decodes JSON, so a response shape
// that differs from the assumptions in the package doc is corrected here and
// in types.go, nowhere else.
func (c *Client) FetchAllCards(ctx context.Context) ([]Card, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("pacing the card dump request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cardInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building the card dump request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting the card dump: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	var body cardInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding the card dump: %w", err)
	}

	return body.Data, nil
}
