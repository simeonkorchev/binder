// Command cardimages fetches the card art for every card that does not have it
// yet, stores each image under an object key and records that key on the card.
//
// It is safe to run repeatedly and safe to kill: the work set is "cards with no
// image object key", so a restarted run re-fetches nothing it already
// finished, and a card that failed is simply picked up next time.
//
//	CARD_IMAGES_BUCKET=binder-card-images \
//	DATABASE_URL=postgres://... go run ./cmd/cardimages
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/jmoiron/sqlx"

	"github.com/simeonkorchev/binder/cmd/cardimages/imagefetch"
	"github.com/simeonkorchev/binder/cmd/cardimages/pipeline"
	"github.com/simeonkorchev/binder/cmd/cardimages/store"
	"github.com/simeonkorchev/binder/pkg/objectstore"
)

var errStorageTarget = errors.New("set exactly one of CARD_IMAGES_BUCKET (GCS) and CARD_IMAGES_DIR (local directory)")

type config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`

	// Bucket selects GCS (D3). Dir selects a directory on disk, which is what
	// makes the command runnable without cloud credentials. Exactly one.
	Bucket string `env:"CARD_IMAGES_BUCKET"`
	Dir    string `env:"CARD_IMAGES_DIR"`

	// BaseURL is where images are fetched from. It is configuration rather
	// than a constant because the host this points at is unreachable from the
	// environments this repository is developed in, and because W2 may find
	// the real path differs from the one this plan assumed.
	BaseURL string `env:"CARD_IMAGES_BASE_URL" envDefault:"https://images.ygoprodeck.com/images/cards"`

	// Interval is deliberately far below the ~20 req/s the plan believes
	// upstream allows: five requests a second walks ~13k cards in about three
	// quarters of an hour, which is cheap insurance on a free API. Raise it
	// only once W2 has verified the real limit.
	Interval    time.Duration `env:"CARD_IMAGES_INTERVAL"    envDefault:"200ms"`
	Concurrency int           `env:"CARD_IMAGES_CONCURRENCY" envDefault:"4"`
	BatchSize   int           `env:"CARD_IMAGES_BATCH_SIZE"  envDefault:"200"`
	HTTPTimeout time.Duration `env:"CARD_IMAGES_HTTP_TIMEOUT" envDefault:"30s"`
}

// storageTarget enforces "exactly one destination" at the boundary it enters,
// rather than letting a run that configured both silently prefer one.
func (c config) storageTarget() error {
	if (c.Bucket == "") == (c.Dir == "") {
		return errStorageTarget
	}
	return nil
}

func main() {
	// SIGTERM and Ctrl-C cancel the run instead of killing it: in-flight
	// writes finish, and everything else is already resumable.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.ErrorContext(ctx, "card image run failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := env.ParseAs[config]()
	if err != nil {
		return fmt.Errorf("reading configuration: %w", err)
	}
	if err := cfg.storageTarget(); err != nil {
		return err
	}

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer closeQuietly(ctx, "database", db)

	storage, closeStorage, err := openStorage(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStorage()

	fetcher, err := imagefetch.NewFetcher(&http.Client{Timeout: cfg.HTTPTimeout}, cfg.BaseURL)
	if err != nil {
		return err
	}

	runner, err := pipeline.New(pipeline.Config{
		Store:       store.NewStore(db),
		Fetcher:     fetcher,
		Storage:     storage,
		Clock:       pipeline.SystemClock{},
		Concurrency: cfg.Concurrency,
		Interval:    cfg.Interval,
		BatchSize:   cfg.BatchSize,
	})
	if err != nil {
		return err
	}

	// Partial failure is not a failed run: every card that did not get an
	// image is still NULL and will be retried next time, and exiting non-zero
	// for one card upstream has never had would keep a scheduled job
	// permanently red.
	_, err = runner.Run(ctx)
	return err
}

// openStorage returns the configured destination and the function that
// releases it, so run() does not branch on which one it got.
func openStorage(ctx context.Context, cfg config) (pipeline.Storage, func(), error) {
	if cfg.Dir != "" {
		dir, err := objectstore.NewDir(cfg.Dir)
		if err != nil {
			return nil, nil, err
		}
		return dir, func() {}, nil
	}

	gcs, err := objectstore.NewGCS(ctx, cfg.Bucket)
	if err != nil {
		return nil, nil, err
	}
	return gcs, func() { closeQuietly(ctx, "gcs client", gcs) }, nil
}

// closeQuietly reports a close failure rather than returning it: the work is
// already done by the time anything here is closed, and overwriting the run's
// outcome with a teardown error would hide it.
func closeQuietly(ctx context.Context, what string, closer interface{ Close() error }) {
	if err := closer.Close(); err != nil {
		slog.WarnContext(ctx, "closing a resource failed",
			slog.String("resource", what),
			slog.String("error", err.Error()),
		)
	}
}

// compile-time proof that the pool satisfies what closeQuietly needs.
var _ interface{ Close() error } = (*sqlx.DB)(nil)
