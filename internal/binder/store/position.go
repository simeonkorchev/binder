package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// Rearranging positions has to get past UNIQUE (binder_id, position), and that
// constraint is not deferrable: Postgres checks it as each row version enters
// the index, not at the end of the statement. So even a single
//
//	UPDATE binder_slots SET position = position + 1 WHERE position >= 3
//
// fails — the row moving 3 -> 4 collides with the row still at 4, which has not
// been updated yet. The order rows are updated in is not something SQL lets us
// choose, and for a move there is no order that works anyway: every position in
// the affected range is occupied, so every intermediate step collides.
//
// Every rearrangement here is therefore two statements inside one transaction:
// park the affected rows above every occupied position, then bring them back at
// their final positions. The parked range is disjoint from the occupied one by
// construction, and the positions the rows come back to were all vacated by the
// first statement, so no intermediate state ever holds two rows at one position.
//
// parkOffsetQuery is what makes the parked range disjoint: one past the highest
// position in the binder. A binder with no slots answers 0, which no rearranging
// caller reaches — there is nothing to shift.
const parkOffsetQuery = `
SELECT coalesce(max("position"), -1) + 1
FROM binder_slots
WHERE binder_id = $1`

// parkPositionsFromQuery lifts every slot at or after a position out of the way.
const parkPositionsFromQuery = `
UPDATE binder_slots
SET "position" = "position" + $2, updated_at = now()
WHERE binder_id = $1 AND "position" >= $3`

// unparkPositionsQuery brings the parked rows back, shifted by delta. Only
// parked rows are at or above the offset, so the predicate needs nothing else.
const unparkPositionsQuery = `
UPDATE binder_slots
SET "position" = "position" - $2 + $3, updated_at = now()
WHERE binder_id = $1 AND "position" >= $2`

// OpenPositionGap frees the position at by shifting it, and everything after
// it, one place along. Opening a gap at the end of the binder is a no-op.
func (s *Store) OpenPositionGap(ctx context.Context, binderID uuid.UUID, at int) error {
	if err := s.shiftPositionsFrom(ctx, binderID, at, 1); err != nil {
		return fmt.Errorf("opening a gap at position %d: %w", at, err)
	}
	return nil
}

// ClosePositionGap closes the hole a removed slot left at at, by shifting
// everything after it one place back. The database cannot enforce that
// positions are dense, so this is what keeps them so.
func (s *Store) ClosePositionGap(ctx context.Context, binderID uuid.UUID, at int) error {
	if err := s.shiftPositionsFrom(ctx, binderID, at+1, -1); err != nil {
		return fmt.Errorf("closing the gap at position %d: %w", at, err)
	}
	return nil
}

// shiftPositionsFrom moves every slot at or after from by delta, parking them
// clear of the occupied range on the way.
func (s *Store) shiftPositionsFrom(ctx context.Context, binderID uuid.UUID, from, delta int) error {
	return s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)

		park, err := parkOffset(ctx, tx, binderID)
		if err != nil {
			return err
		}
		if park == 0 {
			return nil
		}

		if _, err := tx.ExecContext(ctx, parkPositionsFromQuery, binderID, park, from); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, unparkPositionsQuery, binderID, park, delta); err != nil {
			return err
		}
		return nil
	})
}

// parkPositionsInRangeQuery lifts the window a move rearranges out of the way,
// leaving the slots outside it untouched.
const parkPositionsInRangeQuery = `
UPDATE binder_slots
SET "position" = "position" + $2, updated_at = now()
WHERE binder_id = $1 AND "position" BETWEEN $3 AND $4`

// moveSlotQuery brings the parked window back with the moving slot at its
// destination and everything else closed up behind it. delta is -1 when the
// slot moves later in the binder (the slots it passes each move one back) and
// +1 when it moves earlier.
const moveSlotQuery = `
UPDATE binder_slots
SET "position" = CASE WHEN id = $4 THEN $5 ELSE "position" - $2 + $3 END,
    updated_at = now()
WHERE binder_id = $1 AND "position" >= $2`

// MoveSlotPosition moves one slot from one position to another and takes every
// slot between the two one place along, so the binder stays dense.
func (s *Store) MoveSlotPosition(
	ctx context.Context,
	binderID uuid.UUID,
	slotID uuid.UUID,
	from int,
	to int,
) error {
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)

		park, err := parkOffset(ctx, tx, binderID)
		if err != nil {
			return err
		}

		low, high := min(from, to), max(from, to)
		if _, err := tx.ExecContext(ctx, parkPositionsInRangeQuery, binderID, park, low, high); err != nil {
			return err
		}

		// The slots the mover passes shift the opposite way to the mover.
		delta := 1
		if from < to {
			delta = -1
		}
		if _, err := tx.ExecContext(ctx, moveSlotQuery, binderID, park, delta, slotID, to); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("moving binder slot from position %d to %d: %w", from, to, err)
	}
	return nil
}

// parkOffset is one past the highest position in the binder, which is what
// makes a parked range disjoint from the occupied one.
func parkOffset(ctx context.Context, tx *sqlx.Tx, binderID uuid.UUID) (int, error) {
	var offset int
	if err := tx.GetContext(ctx, &offset, parkOffsetQuery, binderID); err != nil {
		return 0, err
	}
	return offset, nil
}
