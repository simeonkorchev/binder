// Package imagefetch downloads one card image over HTTP.
//
// It is the only part of the card-image pipeline that talks to the network,
// which is why the pipeline consumes it through an interface: images.
// ygoprodeck.com is answered with 403 by the egress proxy in the environments
// this repository is built in, so every suite drives a fake and this package's
// own specs drive httptest. Nothing here has ever run against the real host.
package imagefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"

	"github.com/simeonkorchev/binder/cmd/cardimages/model"
)

var (
	errNoClient         = errors.New("http client is nil")
	errNoBaseURL        = errors.New("base url is empty")
	errUnexpectedStatus = errors.New("unexpected status from the image host")
	errImageTooLarge    = errors.New("image exceeds the size limit")
)

// MaxImageBytes caps what one response may spend of this process's memory. A
// card image is ~100 KB; a body two orders of magnitude larger than that is a
// wrong URL or a hostile response, not a card.
const MaxImageBytes = 8 << 20

// Fetcher downloads card images from a host that serves them at
// <baseURL>/<ygoprodeck id>.jpg.
type Fetcher struct {
	client  *http.Client
	baseURL string
}

// NewFetcher takes the client rather than building one so the timeout stays a
// deployment setting decided in main, where every other boundary timeout is.
func NewFetcher(client *http.Client, baseURL string) (*Fetcher, error) {
	if client == nil {
		return nil, fmt.Errorf("creating image fetcher: %w", errNoClient)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("creating image fetcher: %w", errNoBaseURL)
	}
	return &Fetcher{client: client, baseURL: baseURL}, nil
}

// Fetch returns the image for one card, or model.ErrImageNotFound when the
// host has none. Every other failure is returned wrapped: the caller counts it
// and moves on, and the card is retried on the next run.
func (f *Fetcher) Fetch(ctx context.Context, ygoProDeckID int64) (model.Image, error) {
	target, err := url.JoinPath(f.baseURL, strconv.FormatInt(ygoProDeckID, 10)+".jpg")
	if err != nil {
		return model.Image{}, fmt.Errorf("building image url for card %d: %w", ygoProDeckID, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return model.Image{}, fmt.Errorf("building image request for card %d: %w", ygoProDeckID, err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return model.Image{}, fmt.Errorf("fetching image for card %d: %w", ygoProDeckID, err)
	}
	defer func() { _ = resp.Body.Close() }() // nothing actionable is left once the body is read

	if err := statusError(resp.StatusCode, ygoProDeckID); err != nil {
		return model.Image{}, err
	}
	return readImage(resp, ygoProDeckID)
}

// statusError maps the response status onto the Fetch contract: 200 is an
// image, 404 is the one expected absence, anything else is unexpected.
func statusError(status int, ygoProDeckID int64) error {
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("fetching image for card %d: %w", ygoProDeckID, model.ErrImageNotFound)
	default:
		return fmt.Errorf("fetching image for card %d: %w (%d)", ygoProDeckID, errUnexpectedStatus, status)
	}
}

func readImage(resp *http.Response, ygoProDeckID int64) (model.Image, error) {
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return model.Image{}, fmt.Errorf("reading content type for card %d: %w", ygoProDeckID, err)
	}

	// One byte past the cap, so a body exactly at the limit is accepted and
	// anything larger is detected rather than silently truncated.
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return model.Image{}, fmt.Errorf("reading image for card %d: %w", ygoProDeckID, err)
	}
	if len(body) > MaxImageBytes {
		return model.Image{}, fmt.Errorf("reading image for card %d: %w", ygoProDeckID, errImageTooLarge)
	}

	return model.Image{Body: body, ContentType: contentType}, nil
}
