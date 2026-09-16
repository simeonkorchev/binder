// Command cardimport loads the YGOPRODeck card database into Postgres: one
// request for the full dump, then a transaction per slice of cards.
//
// It is safe to run repeatedly and safe to kill. Every write is an upsert on a
// natural key — `cards.ygoprodeck_id`, `card_sets.code`,
// `(card_id, set_code, rarity)` on `card_printings` — so a second run over the
// same dump writes the same rows and mints no new ids, and a run that was
// killed has committed every batch up to the one it was in.
//
//	DATABASE_URL=postgres://... go run ./cmd/cardimport
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v11"
	"github.com/simeonkorchev/binder/cmd/cardimport/importer"
	"github.com/simeonkorchev/binder/cmd/cardimport/store"
	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

type config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`

	// BatchSize is how many dump cards one transaction carries. Five hundred
	// cards is a few thousand rows — small enough that a killed run loses
	// little work, large enough that ~13k cards are not 13k transactions.
	BatchSize int `env:"CARD_IMPORT_BATCH_SIZE" envDefault:"500"`
}

func main() {
	// SIGTERM and Ctrl-C cancel the run rather than killing it: the in-flight
	// transaction rolls back, every batch before it stays committed, and the
	// next run picks the dump up again.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Released before the exit below rather than deferred: os.Exit runs no
	// deferred function, so a `defer stop()` here would be dead on the one
	// path that matters.
	err := run(ctx)
	stop()

	if err != nil {
		slog.ErrorContext(ctx, "card import failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := env.ParseAs[config]()
	if err != nil {
		return fmt.Errorf("reading configuration: %w", err)
	}

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer closeQuietly(ctx, "database", db)

	// The client's own defaults carry the endpoint and the request pacing:
	// this command makes exactly one request, so neither is worth an
	// environment variable until something asks for it.
	runner, err := importer.New(importer.Config{
		Fetcher:   ygoprodeck.NewClient(ygoprodeck.Config{}),
		Store:     store.NewStore(db),
		BatchSize: cfg.BatchSize,
	})
	if err != nil {
		return err
	}

	// A batch that fails stops the import and exits non-zero: unlike a missing
	// card image, a batch that did not commit means cards the app cannot
	// resolve, and a scheduled run that swallowed it would look healthy.
	_, err = runner.Import(ctx)

	return err
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
