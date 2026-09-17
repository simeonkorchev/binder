// Package objectstore writes blobs to a bucket-like store under a
// caller-chosen object key.
//
// The store is named by URL and opened through gocloud.dev/blob, so the same
// code path serves every environment and only the URL changes:
//
//	gs://binder-card-images      deployment (D3)
//	file:///var/lib/binder/cards local development, no cloud credentials
//	mem://                       the suites
//
// That is the point of the indirection. The previous shape had two
// hand-written implementations, and the GCS one could not be exercised
// anywhere this repository is developed — it had no test at all, and its first
// proof would have been a deployment. Now local development and the suites run
// the same Put through the same library, and `gs://` swaps in a driver that
// gocloud tests rather than code this repository owns.
//
// It deliberately exposes no interface of its own: the consumer declares the
// one method it calls and *Bucket satisfies it structurally
// (002-go-conventions.md §3).
package objectstore

import (
	"errors"
	"path"
	"strings"
)

// ErrInvalidKey rejects a key that is not a clean relative path. An object key
// travels from a pipeline into a URL and, under file://, into a filesystem
// path; neither place may receive ".." or a leading slash. fileblob does its
// own escaping, but this is the repository's own invariant and is checked
// before the key reaches any driver, so every backend rejects the same keys.
var ErrInvalidKey = errors.New("object key must be a clean relative path")

// validateKey is the one definition of a legal key, so no backend can disagree
// with another about what it accepts.
func validateKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || path.Clean(key) != key {
		return ErrInvalidKey
	}
	for element := range strings.SplitSeq(key, "/") {
		if element == "" || element == "." || element == ".." {
			return ErrInvalidKey
		}
	}
	return nil
}
