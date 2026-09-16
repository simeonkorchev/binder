// Package store holds the two queries the card-image pipeline runs against
// `cards`: which rows still need an image, and recording the key of one that
// now has it.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	// Registers the "pgx" driver used by Connect. Importing it here keeps the
	// driver choice in one place instead of in every binary and suite.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/simeonkorchev/binder/cmd/cardimages/model"
)

// ErrCardNotFound reports that the card a key was recorded against no longer
// exists. Without it a delete that races the run would look like a successful
// store while the key went nowhere.
var ErrCardNotFound = errors.New("card not found")

const driverName = "pgx"

// Connect opens a pool against url and verifies it, so a bad URL fails at
// startup rather than on the first query.
func Connect(ctx context.Context, url string) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, driverName, url)
	if err != nil {
		return nil, fmt.Errorf("connecting to the card database: %w", err)
	}
	return db, nil
}

// Store reads and writes the image bookkeeping on `cards`.
type Store struct {
	db *sqlx.DB
}

// NewStore returns a store over an already-open pool; the caller owns closing it.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// cardRow is the row shape of cardsNeedingImageQuery.
type cardRow struct {
	ID           uuid.UUID `db:"id"`
	YGOProDeckID int64     `db:"ygoprodeck_id"`
}

// toPendingCard is the row→model converter; card_internal_test.go pins that it
// carries every field.
func (r cardRow) toPendingCard() model.PendingCard {
	return model.PendingCard{
		ID:           r.ID,
		YGOProDeckID: r.YGOProDeckID,
	}
}

// The `image_object_key IS NULL` predicate is the whole of the pipeline's
// resumability: a card that was never fetched, or whose fetch failed, is still
// selected on the next run, and one that succeeded never is. The cursor on
// ygoprodeck_id walks the table exactly once within a run, so the cards that
// failed this time are not retried until the next one.
const cardsNeedingImageQuery = `
	SELECT id, ygoprodeck_id
	FROM cards
	WHERE image_object_key IS NULL
	  AND ygoprodeck_id > $1
	ORDER BY ygoprodeck_id
	LIMIT $2`

// CardsNeedingImage returns up to limit cards with no image, in ygoprodeck_id
// order, starting strictly after afterYGOProDeckID. No matching row is an
// empty slice, not an error (000-principles.md §8b) — a fully imaged database
// is the normal steady state.
func (s *Store) CardsNeedingImage(
	ctx context.Context,
	afterYGOProDeckID int64,
	limit int,
) ([]model.PendingCard, error) {
	var rows []cardRow
	if err := s.db.SelectContext(ctx, &rows, cardsNeedingImageQuery, afterYGOProDeckID, limit); err != nil {
		return nil, fmt.Errorf("selecting cards needing an image: %w", err)
	}

	cards := make([]model.PendingCard, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, row.toPendingCard())
	}
	return cards, nil
}

const setImageObjectKeyQuery = `
	UPDATE cards
	SET image_object_key = $2,
	    updated_at = now()
	WHERE id = $1`

// SetImageObjectKey records the key of a stored image, which is what takes the
// card out of the pipeline's work set for good.
func (s *Store) SetImageObjectKey(ctx context.Context, cardID uuid.UUID, key string) error {
	result, err := s.db.ExecContext(ctx, setImageObjectKeyQuery, cardID, key)
	if err != nil {
		return fmt.Errorf("recording image object key: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("reading affected rows after recording image object key: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("recording image object key: %w", ErrCardNotFound)
	}
	return nil
}
