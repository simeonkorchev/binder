// Package store writes one batch of dump rows into `cards`, `card_sets` and
// `card_printings`.
//
// Everything here is an upsert on a natural key, which is what makes the
// import **idempotent**: a second run over the same dump writes the same rows
// and mints no new ids, so a run that was killed halfway is simply run again.
// The card's uuid is minted on insert and kept on conflict, so binder slots
// that already point at a printing survive a re-import.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	// Registers the "pgx" driver used by Connect. Importing it here keeps the
	// driver choice in one place instead of in every binary and suite.
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/simeonkorchev/binder/cmd/cardimport/model"
	"github.com/simeonkorchev/binder/pkg/sqlxtx"
)

// errUnknownCard reports a printing whose card was not in the same batch.
// buildBatch cannot produce one — every printing is built from a card it also
// emitted — so this is a bug in the batch, and writing the printing with a
// blank card id would be worse than refusing it (000-principles.md section 10).
var errUnknownCard = errors.New("printing references a card that is not in the batch")

// maxBindParams is Postgres' hard limit on parameters in one statement. It is
// what decides how many rows a single INSERT can carry, so every bulk insert
// below is chunked by it rather than by a number someone guessed.
const maxBindParams = 65535

// Column counts of the three bulk inserts, used to turn maxBindParams into a
// row count per statement.
const (
	setColumns      = 2
	cardColumns     = 3
	printingColumns = 4
)

// bytesPerPlaceholder sizes the placeholder builder up front: "$12345," is
// seven characters at the widest a 65535-parameter statement can reach, plus
// the row's own parentheses. Over-reserving one buffer beats regrowing it once
// per row of a thirteen-thousand-card dump.
const bytesPerPlaceholder = 8

const driverName = "pgx"

// Connect opens a pool against url and verifies it, so a bad URL fails at
// startup rather than on the first query.
//
// cmd/cardimages/store has the twin of this function. Two copies of ten lines
// of boilerplate is cheaper than a package shared by two commands that
// otherwise have nothing in common; a third caller is the point at which it
// moves to pkg/.
func Connect(ctx context.Context, url string) (*sqlx.DB, error) {
	db, err := sqlx.ConnectContext(ctx, driverName, url)
	if err != nil {
		return nil, fmt.Errorf("connecting to the card database: %w", err)
	}

	return db, nil
}

// Store writes batches of imported rows.
type Store struct {
	db *sqlx.DB
}

// NewStore returns a store over an already-open pool; the caller owns closing it.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// UpsertBatch writes one batch in one transaction, in the only order the
// schema allows: sets, then cards, then the printings that reference both.
//
// `card_printings.set_prefix` is generated from `set_code` **and** is the
// foreign key to `card_sets.code`, so a printing whose set row does not exist
// yet is rejected outright — the ordering here is the schema's requirement,
// not a preference (db/migrations/001_cards.sql).
//
// An empty batch writes nothing and is not an error (000-principles.md
// section 8b): a slice of the dump whose every printing was skipped is a
// normal thing to be handed.
func (s *Store) UpsertBatch(ctx context.Context, batch model.Batch) error {
	if batch.Empty() {
		return nil
	}

	return sqlxtx.EnsureTx(ctx, s.db, func(ctx context.Context) error {
		tx := sqlxtx.MustGetTx(ctx)

		if err := upsertSets(ctx, tx, batch.Sets); err != nil {
			return err
		}
		cardIDs, err := upsertCards(ctx, tx, batch.Cards)
		if err != nil {
			return err
		}

		return upsertPrintings(ctx, tx, batch.Printings, cardIDs)
	}, sql.LevelReadCommitted)
}

// upsertSetsStmt refreshes the name of a set we have already seen: the dump is
// the source of truth for it, and a set renamed upstream should not stay wrong
// here until someone drops the table.
const upsertSetsStmt = `
	INSERT INTO card_sets (code, name)
	VALUES %s
	ON CONFLICT (code) DO UPDATE
	SET name = EXCLUDED.name,
	    updated_at = now()`

func upsertSets(ctx context.Context, tx *sqlx.Tx, sets []model.SetRow) error {
	for chunk := range slices.Chunk(sets, maxBindParams/setColumns) {
		args := make([]any, 0, len(chunk)*setColumns)
		for _, set := range chunk {
			args = append(args, set.Code, set.Name)
		}

		statement := fmt.Sprintf(upsertSetsStmt, placeholders(len(chunk), setColumns))
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return fmt.Errorf("upserting card sets: %w", err)
		}
	}

	return nil
}

// upsertCardsStmt returns every row it touched — inserted or updated — which
// is how the caller learns the uuid of a card that was already there. DO
// NOTHING would return nothing for exactly those rows, which is the case a
// re-import consists almost entirely of.
const upsertCardsStmt = `
	INSERT INTO cards (id, ygoprodeck_id, name)
	VALUES %s
	ON CONFLICT (ygoprodeck_id) DO UPDATE
	SET name = EXCLUDED.name,
	    updated_at = now()
	RETURNING id, ygoprodeck_id`

// cardIDRow is the row shape of upsertCardsStmt's RETURNING clause.
type cardIDRow struct {
	ID           uuid.UUID `db:"id"`
	YgoprodeckID int64     `db:"ygoprodeck_id"`
}

// upsertCards writes the cards and reports the uuid each upstream id now has,
// which is what the printings are hung off.
func upsertCards(ctx context.Context, tx *sqlx.Tx, cards []model.CardRow) (map[int64]uuid.UUID, error) {
	ids := make(map[int64]uuid.UUID, len(cards))

	for chunk := range slices.Chunk(cards, maxBindParams/cardColumns) {
		args := make([]any, 0, len(chunk)*cardColumns)
		for _, card := range chunk {
			// A fresh uuid for an insert; on conflict the row keeps the id it
			// already had and RETURNING reports that one instead.
			args = append(args, uuid.New(), card.YgoprodeckID, card.Name)
		}

		statement := fmt.Sprintf(upsertCardsStmt, placeholders(len(chunk), cardColumns))
		var rows []cardIDRow
		if err := tx.SelectContext(ctx, &rows, statement, args...); err != nil {
			return nil, fmt.Errorf("upserting cards: %w", err)
		}
		for _, row := range rows {
			ids[row.YgoprodeckID] = row.ID
		}
	}

	return ids, nil
}

// upsertPrintingsStmt conflicts on (card_id, set_code, rarity), the schema's
// unique key: alternate rarities of a reprint share one printed code, so
// (card_id, set_code) alone would collapse two real printings into one.
//
// DO NOTHING rather than DO UPDATE because those three columns are the whole
// row — there is nothing left to update, and touching updated_at on every
// re-import would rewrite the entire table for no information.
const upsertPrintingsStmt = `
	INSERT INTO card_printings (id, card_id, set_code, rarity)
	VALUES %s
	ON CONFLICT (card_id, set_code, rarity) DO NOTHING`

func upsertPrintings(
	ctx context.Context,
	tx *sqlx.Tx,
	printings []model.PrintingRow,
	cardIDs map[int64]uuid.UUID,
) error {
	for chunk := range slices.Chunk(printings, maxBindParams/printingColumns) {
		args := make([]any, 0, len(chunk)*printingColumns)
		for _, printing := range chunk {
			cardID, ok := cardIDs[printing.YgoprodeckID]
			if !ok {
				return fmt.Errorf("upserting card printings: %w (%d)", errUnknownCard, printing.YgoprodeckID)
			}
			args = append(args, uuid.New(), cardID, printing.SetCode, printing.Rarity)
		}

		statement := fmt.Sprintf(upsertPrintingsStmt, placeholders(len(chunk), printingColumns))
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return fmt.Errorf("upserting card printings: %w", err)
		}
	}

	return nil
}

// placeholders builds the VALUES list of a bulk insert — "($1,$2),($3,$4)" for
// two rows of two columns.
//
// This is the one exception 004-security.md permits to the ban on building SQL
// with fmt: everything interpolated here is a **loop index**, never a value, a
// column name or a table name. Every actual value travels separately through
// the statement's args.
func placeholders(rows, columns int) string {
	var b strings.Builder
	b.Grow(rows * columns * bytesPerPlaceholder)

	for row := range rows {
		if row > 0 {
			b.WriteString(",")
		}
		b.WriteString("(")
		for column := range columns {
			if column > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, "$%d", row*columns+column+1)
		}
		b.WriteString(")")
	}

	return b.String()
}
