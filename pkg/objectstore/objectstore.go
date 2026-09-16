// Package objectstore writes blobs to a bucket-like store under a caller-chosen
// object key.
//
// It deliberately exposes no interface of its own: the consumer declares the
// one method it calls and both types here satisfy it structurally
// (002-go-conventions.md §3). Two implementations exist because the pipeline
// that uses it must be runnable without cloud credentials — GCS in deployment
// (D3), a directory on disk for local runs and for the suites.
package objectstore

import (
	"errors"
	"path"
	"strings"
)

// ErrInvalidKey rejects a key that is not a clean relative path. Both
// implementations check it: an object key travels from a pipeline into a URL
// and, for Dir, into a filesystem path, and neither place may receive ".." or
// a leading slash.
var ErrInvalidKey = errors.New("object key must be a clean relative path")

// validateKey is the one definition of a legal key, so Dir and GCS cannot
// disagree about what they accept.
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
