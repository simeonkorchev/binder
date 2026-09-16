package model

import "errors"

// ErrImageNotFound is part of the Fetch contract, not one implementation's
// detail: every fetcher — the HTTP one, and the fakes the suites drive —
// reports "upstream has no image for this card" with this error, and the
// pipeline matches it to demote the failure below ERROR and count it
// separately. Declaring it here is what stops a fake and the real fetcher
// drifting on the one signal the pipeline branches on.
var ErrImageNotFound = errors.New("image not found upstream")

// Image is one fetched card image, held whole in memory: card art is on the
// order of a hundred kilobytes, and having the bytes rather than a stream
// means a failed store can be retried without re-fetching.
type Image struct {
	Body []byte
	// ContentType is the media type without parameters ("image/jpeg"). The
	// pipeline turns it into the object key's extension and hands it to the
	// object store, so an object cannot claim an extension its bytes deny.
	ContentType string
}
