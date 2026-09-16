package pipeline

import (
	"errors"
	"fmt"
)

var errUnsupportedContentType = errors.New("unsupported image content type")

// objectKeyPrefix namespaces card art inside the bucket, so the same bucket
// can later hold other kinds of object without a migration.
const objectKeyPrefix = "cards"

// objectKey is the one place a card becomes an object key. It is a key and not
// a URL on purpose: the bucket and the CDN host in front of it are deployment
// configuration, and the key has to survive both changing (db/migrations/
// 001_cards.sql, D3).
//
// Keying on the upstream id rather than our own uuid makes a re-import that
// re-mints card rows point at the objects that are already there.
func objectKey(ygoProDeckID int64, contentType string) (string, error) {
	extension, err := extensionFor(contentType)
	if err != nil {
		return "", fmt.Errorf("building object key for card %d: %w", ygoProDeckID, err)
	}
	return fmt.Sprintf("%s/%d%s", objectKeyPrefix, ygoProDeckID, extension), nil
}

// extensionFor refuses a content type it cannot name rather than guessing one.
// An object whose extension disagrees with its bytes is served with the wrong
// type forever and nothing downstream can tell.
func extensionFor(contentType string) (string, error) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", fmt.Errorf("%w: %q", errUnsupportedContentType, contentType)
	}
}
