package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

// SQLSTATE classes this package can act on. Postgres guarantees these codes;
// the human-readable message is localised and is not matched on.
const (
	pgForeignKeyViolation = "23503"
	pgUniqueViolation     = "23505"
)

// Constraint names from db/migrations/003_binders.sql. They live here, next to
// the SQL, because the store is what knows the schema — a service that branched
// on a constraint name would break the day one is renamed.
const (
	constraintSlotPositionUnique = "binder_slots_binder_position_key"
	constraintSlotCardFK         = "binder_slots_card_id_fkey"
	constraintSlotPrintingFK     = "binder_slots_printing_belongs_to_card"
)

// The subjects and references the constraints above translate into. These are
// the vocabulary the service matches on.
const (
	referenceCard         = "card"
	referenceCardPrinting = "card printing"
	subjectSlotPosition   = "binder slot position"
)

// translateSlotWriteError turns a driver error from a binder_slots write into
// the typed error the service branches on, and returns err unchanged when it is
// not one this package has a decision for.
//
// The CHECK constraints are deliberately absent: the service rejects a
// resolution that disagrees with its printing, and a negative position cannot
// be produced by any code path here, so either one firing is a bug in this
// repo rather than a bad request. Those stay opaque 500s, which is what a bug
// should look like (000-principles.md section 8c).
func translateSlotWriteError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}

	switch pgErr.Code {
	case pgUniqueViolation:
		if pgErr.ConstraintName == constraintSlotPositionUnique {
			return dataerror.WrapConflictError(subjectSlotPosition, err)
		}
	case pgForeignKeyViolation:
		switch pgErr.ConstraintName {
		case constraintSlotCardFK:
			return dataerror.WrapInvalidReferenceError(referenceCard, err)
		case constraintSlotPrintingFK:
			return dataerror.WrapInvalidReferenceError(referenceCardPrinting, err)
		}
	}

	return err
}
