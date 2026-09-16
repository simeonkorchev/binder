package objectstore

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/storage"
)

// errNoBucket keeps "you forgot to configure a bucket" out of the credential
// error GCS would otherwise return, which reads like an auth problem.
var errNoBucket = errors.New("bucket name is empty")

// GCS stores objects in a Google Cloud Storage bucket — D3's choice for card
// images.
//
// Nothing in this repository's test suites exercises it: the environments the
// waves run in have no egress to storage.googleapis.com and no service
// account, so the first real proof of this path is a deployment. Behaviour it
// shares with Dir (key validation) therefore lives in objectstore.go, where
// the Dir suite does cover it.
type GCS struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

// NewGCS builds a client from the ambient credentials (workload identity in
// deployment, GOOGLE_APPLICATION_CREDENTIALS locally). The caller owns Close.
func NewGCS(ctx context.Context, bucket string) (*GCS, error) {
	if bucket == "" {
		return nil, fmt.Errorf("creating gcs object store: %w", errNoBucket)
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating gcs client: %w", err)
	}
	return &GCS{client: client, bucket: client.Bucket(bucket)}, nil
}

// Put writes body at key, overwriting any existing object. Overwriting is
// deliberate: a caller that retries a partially written object must be able to
// replace it.
func (g *GCS) Put(ctx context.Context, key string, body []byte, contentType string) error {
	if err := validateKey(key); err != nil {
		return fmt.Errorf("putting object %q: %w", key, err)
	}

	w := g.bucket.Object(key).NewWriter(ctx)
	w.ContentType = contentType
	if _, err := w.Write(body); err != nil {
		// Close releases the stream; the write error is the one that explains
		// the failure, so both are reported rather than either dropped.
		return fmt.Errorf("writing object %q to gcs: %w", key, errors.Join(err, w.Close()))
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("finalising object %q in gcs: %w", key, err)
	}
	return nil
}

// Close releases the underlying client's connections.
func (g *GCS) Close() error {
	if err := g.client.Close(); err != nil {
		return fmt.Errorf("closing gcs client: %w", err)
	}
	return nil
}
