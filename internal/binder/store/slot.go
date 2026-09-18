package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// entityBinderSlot names a slot in a not-found error.
const entityBinderSlot = "binder slot"

// slotColumns is the select list every slot read shares: every column slotRow
// has a field for, in the order it declares them. One constant keeps the reads
// from drifting apart, and it is deliberately not slotInsertColumns below —
// these two are equal only where a written column is also a read one, and the
// timestamps are neither written nor equal.
const slotColumns = `id, binder_id, "position", card_id, card_printing_id, set_resolution, created_at, updated_at`

// slotRow is the binder_slots table as it is read.
//
// card_printing_id is a NullUUID rather than a *uuid.UUID because NULL is a
// normal value here — the name rung identifies a card and no printing — and
// NullUUID is what scans it without the driver having to guess.
type slotRow struct {
	ID             uuid.UUID     `db:"id"`
	BinderID       uuid.UUID     `db:"binder_id"`
	Position       int           `db:"position"`
	CardID         uuid.UUID     `db:"card_id"`
	CardPrintingID uuid.NullUUID `db:"card_printing_id"`
	SetResolution  string        `db:"set_resolution"`
	CreatedAt      time.Time     `db:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"`
}

// toSlot converts a row to the domain model, mapping every field. An absent
// printing becomes a nil pointer, which is what the model means by "the set is
// unknown"; a present one is copied so the caller cannot reach the row.
func toSlot(r slotRow) model.Slot {
	var printingID *uuid.UUID
	if r.CardPrintingID.Valid {
		id := r.CardPrintingID.UUID
		printingID = &id
	}

	return model.Slot{
		ID:             r.ID,
		BinderID:       r.BinderID,
		Position:       r.Position,
		CardID:         r.CardID,
		CardPrintingID: printingID,
		SetResolution:  cardmodel.SetResolution(r.SetResolution),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func toSlots(rows []slotRow) []model.Slot {
	slots := make([]model.Slot, 0, len(rows))
	for _, row := range rows {
		slots = append(slots, toSlot(row))
	}
	return slots
}

const countSlotsQuery = `SELECT count(*) FROM binder_slots WHERE binder_id = $1`

// CountSlots is how many cards the binder holds. Because positions are dense it
// is also one past the last position, which is what an append and a page count
// are derived from.
func (s *Store) CountSlots(ctx context.Context, binderID uuid.UUID) (int, error) {
	var count int
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &count, countSlotsQuery, binderID)
	})
	if err != nil {
		return 0, fmt.Errorf("counting binder slots: %w", err)
	}

	return count, nil
}

const listSlotsInRangeQuery = `
SELECT ` + slotColumns + `
FROM binder_slots
WHERE binder_id = $1 AND "position" BETWEEN $2 AND $3
ORDER BY "position"`

// ListSlotsInRange returns the slots whose position falls between from and to
// inclusive, in position order. A range with nothing in it — a page past the
// end of the binder, or an empty binder — is an empty slice and a nil error.
func (s *Store) ListSlotsInRange(ctx context.Context, binderID uuid.UUID, from, to int) ([]model.Slot, error) {
	var rows []slotRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.SelectContext(ctx, &rows, listSlotsInRangeQuery, binderID, from, to)
	})
	if err != nil {
		return nil, fmt.Errorf("listing binder slots in range: %w", err)
	}

	return toSlots(rows), nil
}

const getSlotByIDQuery = `
SELECT ` + slotColumns + `
FROM binder_slots
WHERE binder_id = $1 AND id = $2`

// GetSlotByID fetches one slot of one binder. The binder is part of the lookup
// so that a slot id belonging to somebody else's binder is simply not found,
// rather than found and then refused — which would confirm it exists.
func (s *Store) GetSlotByID(ctx context.Context, binderID, slotID uuid.UUID) (model.Slot, error) {
	var row slotRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, getSlotByIDQuery, binderID, slotID)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Slot{}, dataerror.WrapMissingEntityError(entityBinderSlot, err)
		}
		return model.Slot{}, fmt.Errorf("getting binder slot by id: %w", err)
	}

	return toSlot(row), nil
}

// slotInsertColumns is the column list both inserts write, and slotValueTuple
// the placeholder tuple for one row of it. They are one constant each because
// the two have to agree on order and on count, and InsertSlots derives its
// placeholder numbers from slotInsertColumnCount.
const (
	slotInsertColumns     = `id, binder_id, "position", card_id, card_printing_id, set_resolution`
	slotInsertColumnCount = 6
	// slotValuesTuple is one row's placeholder tuple, rendered with that row's
	// placeholder numbers. Its verbs take positions, never values.
	slotValuesTuple = `($%d, $%d, $%d, $%d, $%d, $%d::set_resolution)`
)

const insertSlotQuery = `
INSERT INTO binder_slots (` + slotInsertColumns + `)
VALUES ($1, $2, $3, $4, $5, $6::set_resolution)
RETURNING ` + slotColumns

// InsertSlot writes one slot and returns it as stored. The position must
// already be free; opening a gap for it is OpenPositionGap's job.
func (s *Store) InsertSlot(ctx context.Context, slot model.Slot) (model.Slot, error) {
	var row slotRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.GetContext(ctx, &row, insertSlotQuery, slotInsertArgs(slot)...)
	})
	if err != nil {
		return model.Slot{}, fmt.Errorf("inserting binder slot: %w", translateSlotWriteError(err))
	}

	return toSlot(row), nil
}

// The two halves of the batch insert that do not depend on how many rows there
// are. The VALUES tuples between them are generated by InsertSlots; the outer
// SELECT is what lets the result be ordered, which RETURNING on its own cannot
// be, so the caller gets its slots back in position order rather than in
// whatever order the executor produced them.
const (
	insertSlotsQueryHead = `
WITH inserted AS (
    INSERT INTO binder_slots (` + slotInsertColumns + `)
    VALUES `
	insertSlotsQueryTail = `
    RETURNING ` + slotColumns + `
)
SELECT ` + slotColumns + ` FROM inserted ORDER BY "position"`
)

// InsertSlots writes every slot in one statement and returns them as stored, in
// position order. Every position must already be free.
//
// One statement, so the batch is all-or-nothing however it fails: a foreign key
// a card does not satisfy, or a position a concurrent writer took, aborts the
// INSERT and no row of it is written. The caller's transaction is what carries
// that guarantee across the rest of its work.
//
// An empty batch writes nothing and is not an error: it returns an empty slice
// without going to the database (000-principles.md section 8b).
func (s *Store) InsertSlots(ctx context.Context, slots []model.Slot) ([]model.Slot, error) {
	if len(slots) == 0 {
		return []model.Slot{}, nil
	}

	query, args := buildInsertSlotsQuery(slots)

	var rows []slotRow
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		return tx.SelectContext(ctx, &rows, query, args...)
	})
	if err != nil {
		return nil, fmt.Errorf("inserting %d binder slots: %w", len(slots), translateSlotWriteError(err))
	}

	return toSlots(rows), nil
}

// buildInsertSlotsQuery renders the VALUES tuples for slots and collects their
// arguments.
//
// The only thing interpolated into the SQL is a placeholder number derived from
// a loop index; every value travels as a bind parameter in args. That is the
// one string-building exception 004-security.md permits, and this comment is
// the part of it that says so.
func buildInsertSlotsQuery(slots []model.Slot) (string, []any) {
	tuples := make([]string, 0, len(slots))
	args := make([]any, 0, len(slots)*slotInsertColumnCount)

	for i, slot := range slots {
		base := i * slotInsertColumnCount
		numbers := make([]any, 0, slotInsertColumnCount)
		for column := range slotInsertColumnCount {
			numbers = append(numbers, base+column+1)
		}

		tuples = append(tuples, fmt.Sprintf(slotValuesTuple, numbers...))
		args = append(args, slotInsertArgs(slot)...)
	}

	return insertSlotsQueryHead + strings.Join(tuples, ",\n           ") + insertSlotsQueryTail, args
}

// slotInsertArgs is one slot as the bind parameters of slotInsertColumns, in
// that order. Both inserts go through it, so they cannot disagree about which
// value belongs to which column.
func slotInsertArgs(slot model.Slot) []any {
	printingID := uuid.NullUUID{UUID: uuid.Nil, Valid: false}
	if slot.CardPrintingID != nil {
		printingID = uuid.NullUUID{UUID: *slot.CardPrintingID, Valid: true}
	}

	return []any{
		slot.ID, slot.BinderID, slot.Position, slot.CardID, printingID, string(slot.SetResolution),
	}
}

const deleteSlotByIDQuery = `DELETE FROM binder_slots WHERE binder_id = $1 AND id = $2`

// DeleteSlotByID removes one slot and leaves a hole at its position;
// ClosePositionGap is what closes it. Deleting a slot that is not there is a
// MissingEntityError rather than a silent success, so a caller cannot report a
// removal that did not happen.
func (s *Store) DeleteSlotByID(ctx context.Context, binderID, slotID uuid.UUID) error {
	err := s.InTx(ctx, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)
		result, err := tx.ExecContext(ctx, deleteSlotByIDQuery, binderID, slotID)
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return dataerror.WrapMissingEntityError(entityBinderSlot, sql.ErrNoRows)
		}

		return nil
	})
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return err
		}
		return fmt.Errorf("deleting binder slot: %w", err)
	}

	return nil
}
