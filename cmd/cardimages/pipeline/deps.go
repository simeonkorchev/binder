package pipeline

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/simeonkorchev/binder/cmd/cardimages/model"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// Store is the pipeline's view of the card database.
//
//counterfeiter:generate . Store
type Store interface {
	// CardsNeedingImage returns up to limit cards that have no image object
	// key yet, ordered by ygoprodeck_id and starting strictly after
	// afterYGOProDeckID. No such card is an empty slice and a nil error, never
	// a not-found (000-principles.md §8b) — the pipeline reaches that state on
	// every run once the backlog is drained.
	CardsNeedingImage(ctx context.Context, afterYGOProDeckID int64, limit int) ([]model.PendingCard, error)

	// SetImageObjectKey records where a card's image was stored. Until it
	// succeeds the card stays in the work set, which is how an interrupted run
	// resumes without re-fetching anything it finished.
	SetImageObjectKey(ctx context.Context, cardID uuid.UUID, key string) error
}

// Fetcher downloads one card image from upstream.
//
//counterfeiter:generate . Fetcher
type Fetcher interface {
	// Fetch returns model.ErrImageNotFound when upstream has no image for the
	// card — the one failure the pipeline treats as expected. Every other
	// error is unexpected and counted separately.
	Fetch(ctx context.Context, ygoProDeckID int64) (model.Image, error)
}

// Storage writes an image to the bucket under a key the pipeline chooses.
//
//counterfeiter:generate . Storage
type Storage interface {
	Put(ctx context.Context, key string, body []byte, contentType string) error
}

// Clock is the pipeline's whole dependency on real time, so the rate limiter
// can be proven in a suite instead of waited out.
//
//counterfeiter:generate . Clock
type Clock interface {
	Now() time.Time
	// Sleep returns early with the context's error when the run is cancelled,
	// so a shutdown is not held up by the longest pending rate-limit wait.
	Sleep(ctx context.Context, d time.Duration) error
}
