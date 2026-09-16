// Package importer turns the YGOPRODeck dump into database rows: one request
// for the whole dump, then a transaction per slice of cards.
//
// Two properties shape it. It is **idempotent**: every write is an upsert on a
// natural key, so running it twice changes nothing the first run did not
// already do. And it is **resumable**: a run killed between two batches has
// committed every batch before it, and the next run rewrites them harmlessly
// rather than having to know where it stopped.
package importer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
)

var (
	errNilFetcher           = errors.New("fetcher is nil")
	errNilStore             = errors.New("store is nil")
	errNonPositiveBatchSize = errors.New("batch size must be positive")
)

// Config is the importer's dependencies and settings.
type Config struct {
	Fetcher Fetcher
	Store   Store

	// BatchSize is how many dump cards one transaction carries. It bounds the
	// transaction, not the request: the dump arrives whole (assumption A1) and
	// is sliced here.
	BatchSize int
}

// Importer runs the fetch-build-upsert loop.
type Importer struct {
	fetcher   Fetcher
	store     Store
	batchSize int
}

// New validates the configuration at the one boundary it enters, so a
// misconfigured run fails at startup instead of part-way through the dump
// (000-principles.md section 10).
func New(cfg Config) (*Importer, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &Importer{fetcher: cfg.Fetcher, store: cfg.Store, batchSize: cfg.BatchSize}, nil
}

func (c Config) validate() error {
	const wrapper = "creating the card importer: %w"
	switch {
	case c.Fetcher == nil:
		return fmt.Errorf(wrapper, errNilFetcher)
	case c.Store == nil:
		return fmt.Errorf(wrapper, errNilStore)
	case c.BatchSize <= 0:
		return fmt.Errorf(wrapper, errNonPositiveBatchSize)
	default:
		return nil
	}
}

// Import fetches the dump and writes it, returning what it wrote.
//
// The returned Result is meaningful even alongside an error: a batch that
// fails stops the run, and the caller still wants to know how much of the dump
// was committed before it did.
func (i *Importer) Import(ctx context.Context) (Result, error) {
	cards, err := i.fetcher.FetchAllCards(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("fetching the card dump: %w", err)
	}
	slog.InfoContext(ctx, "card dump fetched",
		slog.Int("cards", len(cards)),
		slog.Int("batch_size", i.batchSize),
	)

	counts := newTally()
	for chunk := range slices.Chunk(cards, i.batchSize) {
		batch := buildBatch(chunk)
		if err := i.store.UpsertBatch(ctx, batch); err != nil {
			return counts.result(), fmt.Errorf("importing a batch of cards: %w", err)
		}
		counts.record(batch)
	}

	result := counts.result()
	logResult(ctx, result)

	return result, nil
}
