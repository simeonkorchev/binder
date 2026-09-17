package objectstore

import (
	"context"
	"errors"
	"fmt"

	"gocloud.dev/blob"
	// The URL schemes this binary can open. Each blank import registers one
	// driver with blob.OpenBucket; without it the scheme is unknown at runtime
	// rather than at compile time, so adding a scheme to the config means
	// adding it here too.
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
)

// errNoURL keeps "you forgot to configure a store" out of the driver error the
// bucket would otherwise return, which for gs:// reads like an auth problem.
var errNoURL = errors.New("bucket url is empty")

// Bucket writes objects to the store its URL names.
type Bucket struct {
	bucket *blob.Bucket
}

// Open resolves bucketURL to a driver and opens it. For gs:// the credentials
// are ambient (workload identity in deployment, GOOGLE_APPLICATION_CREDENTIALS
// locally); for file:// the directory must already exist, which gocloud
// reports plainly if it does not. The caller owns Close.
func Open(ctx context.Context, bucketURL string) (*Bucket, error) {
	if bucketURL == "" {
		return nil, fmt.Errorf("opening object store: %w", errNoURL)
	}
	bucket, err := blob.OpenBucket(ctx, bucketURL)
	if err != nil {
		return nil, fmt.Errorf("opening object store %q: %w", bucketURL, err)
	}
	return &Bucket{bucket: bucket}, nil
}

// Put writes body at key, overwriting any existing object. Overwriting is
// deliberate: a caller that retries a partially written object must be able to
// replace it.
//
// contentType is recorded by every backend, including file://, which keeps it
// in a sidecar — so a local run and a deployment store the same metadata. The
// previous directory implementation discarded it, which meant local output was
// not a faithful rehearsal of the real bucket.
func (b *Bucket) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if err := validateKey(key); err != nil {
		return fmt.Errorf("putting object %q: %w", key, err)
	}
	if err := b.bucket.WriteAll(ctx, key, body, &blob.WriterOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("writing object %q: %w", key, err)
	}
	return nil
}

// Close releases the driver's resources.
func (b *Bucket) Close() error {
	if err := b.bucket.Close(); err != nil {
		return fmt.Errorf("closing object store: %w", err)
	}
	return nil
}
