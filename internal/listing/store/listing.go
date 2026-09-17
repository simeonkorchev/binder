package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// Names for the entities these reads can fail to find.
const (
	entityListing    = "listing"
	entityBinderSlot = "binder slot"
)

// listingColumns is the select list every listing read shares, in the order
// listingRow declares its fields. One constant keeps the reads from drifting
// apart.
//
// cardmarket_listing_id is deliberately not read: it is reserved for the
// phase-2 Cardmarket publisher, nothing in the MVP writes it, and a field on
// the model that is NULL for every row would read as data that exists.
const listingColumns = `id, binder_slot_id, created_at, updated_at`

// listingRow is the listings table as it is read.
type listingRow struct {
	ID           uuid.UUID `db:"id"`
	BinderSlotID uuid.UUID `db:"binder_slot_id"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// toListing converts a row to the domain model, mapping every field.
func toListing(r listingRow) model.Listing {
	return model.Listing{
		ID:           r.ID,
		BinderSlotID: r.BinderSlotID,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

const insertListingQuery = `
INSERT INTO listings (id, binder_slot_id)
VALUES ($1, $2)
RETURNING ` + listingColumns

// InsertListing marks one slot for sale and returns the listing as stored.
//
// It returns a dataerror.ConflictError when the slot is already listed and a
// dataerror.InvalidReferenceError over model.ReferenceBinderSlot when there is
// no such slot. Both are decided by the database rather than by a read first:
// a check-then-insert would let a concurrent writer slip between the two.
func (s *Store) InsertListing(ctx context.Context, listing model.Listing) (model.Listing, error) {
	var row listingRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, insertListingQuery, listing.ID, listing.BinderSlotID)
	})
	if err != nil {
		return model.Listing{}, fmt.Errorf("inserting listing: %w", translateListingWriteError(err))
	}

	return toListing(row), nil
}

const deleteListingByIDQuery = `DELETE FROM listings WHERE id = $1`

// DeleteListingByID unlists a slot. Deleting a listing that is not there is a
// MissingEntityError rather than a silent success, so a caller cannot report an
// unlisting that did not happen — including the case where the card was removed
// from the binder meanwhile and the listing cascaded away with it.
func (s *Store) DeleteListingByID(ctx context.Context, id uuid.UUID) error {
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		result, err := tx.ExecContext(ctx, deleteListingByIDQuery, id)
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return dataerror.WrapMissingEntityError(entityListing, sql.ErrNoRows)
		}

		return nil
	})
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return err
		}
		return fmt.Errorf("deleting listing: %w", err)
	}

	return nil
}

const slotSellerQuery = `
SELECT b.owner_id
FROM binder_slots s
JOIN binders b ON b.id = s.binder_id
WHERE s.id = $1`

// SlotSeller is the user who would be selling the card in that slot: the owner
// of the binder it is in. There is no seller column on a listing, so this join
// is the one answer to "whose slot is this", and the service compares it with
// the actor rather than the store deciding who may list what.
func (s *Store) SlotSeller(ctx context.Context, binderSlotID uuid.UUID) (uuid.UUID, error) {
	return s.sellerOf(ctx, slotSellerQuery, binderSlotID, entityBinderSlot)
}

const listingSellerQuery = `
SELECT b.owner_id
FROM listings l
JOIN binder_slots s ON s.id = l.binder_slot_id
JOIN binders b ON b.id = s.binder_id
WHERE l.id = $1`

// ListingSeller is the user selling what a listing lists, reached the same way:
// listing to slot to binder to owner.
func (s *Store) ListingSeller(ctx context.Context, listingID uuid.UUID) (uuid.UUID, error) {
	return s.sellerOf(ctx, listingSellerQuery, listingID, entityListing)
}

// sellerOf runs one of the two owner lookups above. Both are a single uuid read
// that has to turn "no such row" into a MissingEntityError naming what was
// looked for, and that translation is the part worth having in one place.
func (s *Store) sellerOf(ctx context.Context, query string, id uuid.UUID, entity string) (uuid.UUID, error) {
	var seller uuid.UUID
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &seller, query, id)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, dataerror.WrapMissingEntityError(entity, err)
		}
		return uuid.Nil, fmt.Errorf("getting the seller of a %s: %w", entity, err)
	}

	return seller, nil
}
