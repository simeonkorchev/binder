// Package pipeline walks the cards that have no image yet, fetches each one at
// a rate the upstream host is happy with, stores it, and records the object
// key.
//
// Two properties shape everything here. It is **resumable**: the work set is
// "cards.image_object_key IS NULL", so a run that is killed halfway and
// restarted picks up exactly where it stopped and re-fetches nothing. And it
// **survives partial failure**: one 404 or one timeout is counted and the card
// is left for the next run, never aborting the other ~13,000.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/simeonkorchev/binder/cmd/cardimages/model"
)

var (
	errNilStore               = errors.New("store is nil")
	errNilFetcher             = errors.New("fetcher is nil")
	errNilStorage             = errors.New("storage is nil")
	errNilClock               = errors.New("clock is nil")
	errNonPositiveConcurrency = errors.New("concurrency must be positive")
	errNonPositiveBatchSize   = errors.New("batch size must be positive")
)

// Config is the pipeline's dependencies and settings. It is a struct rather
// than a parameter list because every one of these is required and a
// seven-argument constructor is unreadable (002-go-conventions.md §1.2).
type Config struct {
	Store   Store
	Fetcher Fetcher
	Storage Storage
	Clock   Clock

	// Concurrency bounds the images in flight at once. It is a bound on
	// memory and sockets, not the throughput knob — Interval is that.
	Concurrency int
	// Interval is the minimum gap between two upstream requests across all
	// workers.
	Interval time.Duration
	// BatchSize is how many pending cards one database round-trip claims.
	BatchSize int
}

// Pipeline runs the fetch-store-record loop.
type Pipeline struct {
	store       Store
	fetcher     Fetcher
	storage     Storage
	limiter     *Limiter
	concurrency int
	batchSize   int
}

// New validates the configuration at the one boundary it enters, so a
// misconfigured run fails at startup instead of panicking on the first card
// (000-principles.md §10).
func New(cfg Config) (*Pipeline, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	limiter, err := NewLimiter(cfg.Clock, cfg.Interval)
	if err != nil {
		return nil, err
	}
	return &Pipeline{
		store:       cfg.Store,
		fetcher:     cfg.Fetcher,
		storage:     cfg.Storage,
		limiter:     limiter,
		concurrency: cfg.Concurrency,
		batchSize:   cfg.BatchSize,
	}, nil
}

func (c Config) validate() error {
	const wrapper = "creating the card image pipeline: %w"
	switch {
	case c.Store == nil:
		return fmt.Errorf(wrapper, errNilStore)
	case c.Fetcher == nil:
		return fmt.Errorf(wrapper, errNilFetcher)
	case c.Storage == nil:
		return fmt.Errorf(wrapper, errNilStorage)
	case c.Clock == nil:
		return fmt.Errorf(wrapper, errNilClock)
	case c.Concurrency <= 0:
		return fmt.Errorf(wrapper, errNonPositiveConcurrency)
	case c.BatchSize <= 0:
		return fmt.Errorf(wrapper, errNonPositiveBatchSize)
	default:
		return nil
	}
}

// Run processes every card that still needs an image and returns what it did.
//
// The returned Result is meaningful even alongside an error: the run stops on
// a database or cancellation failure, and the caller still wants to know how
// many cards it got through first.
func (p *Pipeline) Run(ctx context.Context) (Result, error) {
	slog.InfoContext(ctx, "card image run started",
		slog.Int("concurrency", p.concurrency),
		slog.Int("batch_size", p.batchSize),
	)

	counts := newTally()
	// The cursor is what makes the run terminate. Successes leave the work
	// set by getting a key; failures do not, so without it a batch of
	// failures would be selected again forever.
	var cursor int64

	for {
		cards, err := p.store.CardsNeedingImage(ctx, cursor, p.batchSize)
		if err != nil {
			return counts.result(), fmt.Errorf("listing cards needing an image: %w", err)
		}
		if len(cards) == 0 {
			break
		}
		cursor = cards[len(cards)-1].YGOProDeckID

		if err := p.processBatch(ctx, cards, counts); err != nil {
			return counts.result(), err
		}
	}

	result := counts.result()
	logSummary(ctx, result)
	return result, nil
}

// processBatch fans the batch out over at most Concurrency workers and waits
// for all of them, so no goroutine outlives the call (002 §5).
func (p *Pipeline) processBatch(ctx context.Context, cards []model.PendingCard, counts *tally) error {
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(p.concurrency)

	for _, card := range cards {
		group.Go(func() error {
			reason, err := p.processCard(groupCtx, card)
			if err != nil {
				return err
			}
			counts.record(reason)
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return fmt.Errorf("processing a batch of card images: %w", err)
	}
	return nil
}

// processCard returns the reason this card was left alone, or FailureNone if
// its image was stored. It returns an error only when the whole run must stop.
func (p *Pipeline) processCard(ctx context.Context, card model.PendingCard) (FailureReason, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		return FailureNone, err
	}

	image, err := p.fetcher.Fetch(ctx, card.YGOProDeckID)
	if err != nil {
		return failCard(ctx, card, fetchReason(err), err)
	}

	key, err := objectKey(card.YGOProDeckID, image.ContentType)
	if err != nil {
		return failCard(ctx, card, FailureUnsupportedType, err)
	}

	if err := p.storage.Put(ctx, key, image.Body, image.ContentType); err != nil {
		return failCard(ctx, card, FailureStore, err)
	}

	if err := p.store.SetImageObjectKey(ctx, card.ID, key); err != nil {
		return failCard(ctx, card, FailureRecord, err)
	}
	return FailureNone, nil
}

// failCard is the single place a per-card error is turned into a counted,
// logged outcome. A cancelled run is the one thing it will not absorb: that is
// not this card failing, it is the process going away.
func failCard(
	ctx context.Context,
	card model.PendingCard,
	reason FailureReason,
	cause error,
) (FailureReason, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return FailureNone, fmt.Errorf("processing card image: %w", ctxErr)
	}

	slog.LogAttrs(ctx, reason.Level(), "card image not stored",
		slog.String("card_id", card.ID.String()),
		slog.Int64("ygoprodeck_id", card.YGOProDeckID),
		slog.String("failure_reason", string(reason)),
		slog.String("error", cause.Error()),
	)
	return reason, nil
}

// fetchReason separates the one expected upstream outcome from everything else
// so the summary — and the log level — can tell them apart.
func fetchReason(err error) FailureReason {
	if errors.Is(err, model.ErrImageNotFound) {
		return FailureNotFound
	}
	return FailureFetch
}

// logSummary is the run's one report: totals plus a breakdown per reason, so
// an operator can see at a glance whether 400 failures were 400 missing cards
// or 400 timeouts.
func logSummary(ctx context.Context, result Result) {
	failures := make([]any, 0, len(result.Failures))
	for reason, count := range result.Failures {
		failures = append(failures, slog.Int(string(reason), count))
	}

	slog.InfoContext(ctx, "card image run finished",
		slog.Int("considered", result.Considered()),
		slog.Int("stored", result.Stored),
		slog.Int("failed", result.Failed()),
		slog.Group("failures", failures...),
	)
}
