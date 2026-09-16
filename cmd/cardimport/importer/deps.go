package importer

import (
	"context"

	"github.com/simeonkorchev/binder/cmd/cardimport/model"
	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Fetcher reads the card database from upstream.
//
// The importer consumes it through an interface because the API is unreachable
// from the containers this repository is built in — every spec drives a fake
// against the committed fixture, and nothing here has ever run against the
// live service (pkg/ygoprodeck's package doc, VERIFY-YGOPRODECK.md).
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	// FetchAllCards returns every card in one request. An empty database is an
	// empty slice and a nil error, never a not-found
	// (000-principles.md section 8b).
	FetchAllCards(ctx context.Context) ([]ygoprodeck.Card, error)
}

// Store writes one batch of rows.
//
//counterfeiter:generate . Store
type Store interface {
	// UpsertBatch writes sets, cards and printings in one transaction, in that
	// order, and is idempotent: the same batch written twice leaves the same
	// rows and mints no new ids. An empty batch is a no-op, not an error.
	UpsertBatch(ctx context.Context, batch model.Batch) error
}
