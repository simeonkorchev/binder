package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/listing/model"
)

// SQLSTATE classes this package can act on. Postgres guarantees these codes;
// the human-readable message is localised and is not matched on.
const (
	pgForeignKeyViolation = "23503"
	pgUniqueViolation     = "23505"
)

// Constraint names from db/migrations/004_listings.sql. They live here, next to
// the SQL, because the store is what knows the schema — a service that branched
// on a constraint name would break the day one is renamed.
//
// listings_cardmarket_listing_id_key is deliberately absent: nothing in the MVP
// writes that column, so a violation of it would be a bug in this repo rather
// than a bad request, and a bug should look like an opaque 500
// (000-principles.md section 8c).
const (
	constraintListingSlotUnique = "listings_binder_slot_id_key"
	constraintListingSlotFK     = "listings_binder_slot_id_fkey"
)

// subjectListedSlot names what a listing conflict is a conflict over: the slot
// can only be listed once.
const subjectListedSlot = "binder slot listing"

// translateListingWriteError turns a driver error from a listings write into the
// typed error the service branches on, and returns err unchanged when it is not
// one this package has a decision for.
func translateListingWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Code {
	case pgUniqueViolation:
		if pgErr.ConstraintName == constraintListingSlotUnique {
			return dataerror.WrapConflictError(subjectListedSlot, err)
		}
	case pgForeignKeyViolation:
		if pgErr.ConstraintName == constraintListingSlotFK {
			return dataerror.WrapInvalidReferenceError(model.ReferenceBinderSlot, err)
		}
	}

	return err
}
