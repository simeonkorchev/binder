package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/card/model"
)

// The three code rungs differ only in their WHERE clause, so they select the
// same columns and share one row type and one converter.
const printedCardColumns = `
	SELECT p.id AS printing_id,
	       p.card_id,
	       p.set_code,
	       p.rarity,
	       c.name AS card_name,
	       c.image_object_key
	  FROM card_printings p
	  JOIN cards c ON c.id = p.card_id`

// Deterministic across every rung, so two runs of the same ambiguous scan
// return the candidates in the same order.
const printedCardOrder = `
	 ORDER BY p.set_code, p.rarity, p.id`

const findPrintingsBySetCodeQuery = printedCardColumns + `
	 WHERE p.set_code = $1` + printedCardOrder

// set_prefix is a generated column, so "the set's prefix starts with what was
// read" is an index-friendly LIKE over it. The pattern is a parameter, never
// interpolated; the prefix reaching here has been normalised to A-Z0-9 by the
// service, so it can carry no LIKE metacharacter of its own.
const findPrintingsBySetPrefixAndNumberQuery = printedCardColumns + `
	 WHERE p.set_number = $1
	   AND p.set_prefix LIKE $2 || '%'` + printedCardOrder

const findPrintingsBySetNumberQuery = printedCardColumns + `
	 WHERE p.set_number = $1` + printedCardOrder

// printedCardRow is one row of the query above.
type printedCardRow struct {
	PrintingID     uuid.UUID `db:"printing_id"`
	CardID         uuid.UUID `db:"card_id"`
	SetCode        string    `db:"set_code"`
	Rarity         string    `db:"rarity"`
	CardName       string    `db:"card_name"`
	ImageObjectKey *string   `db:"image_object_key"`
}

func (r printedCardRow) toModel() model.PrintedCard {
	return model.PrintedCard{
		Card: model.Card{
			ID:             r.CardID,
			Name:           r.CardName,
			ImageObjectKey: r.ImageObjectKey,
		},
		Printing: model.CardPrinting{
			ID:      r.PrintingID,
			CardID:  r.CardID,
			SetCode: r.SetCode,
			Rarity:  r.Rarity,
		},
	}
}

func toPrintedCards(rows []printedCardRow) []model.PrintedCard {
	printings := make([]model.PrintedCard, 0, len(rows))
	for _, row := range rows {
		printings = append(printings, row.toModel())
	}
	return printings
}

// FindPrintingsBySetCode returns every printing whose whole printed code is
// setCode. No match is an empty slice, not an error.
func (s *Store) FindPrintingsBySetCode(ctx context.Context, setCode string) ([]model.PrintedCard, error) {
	printings, err := s.selectPrintedCards(ctx, findPrintingsBySetCodeQuery, setCode)
	if err != nil {
		return nil, fmt.Errorf("finding printings by set code: %w", err)
	}
	return printings, nil
}

// FindPrintingsBySetPrefixAndNumber returns every printing whose set number is
// number and whose set prefix starts with prefix. No match is an empty slice.
func (s *Store) FindPrintingsBySetPrefixAndNumber(
	ctx context.Context,
	prefix string,
	number string,
) ([]model.PrintedCard, error) {
	printings, err := s.selectPrintedCards(ctx, findPrintingsBySetPrefixAndNumberQuery, number, prefix)
	if err != nil {
		return nil, fmt.Errorf("finding printings by set prefix and number: %w", err)
	}
	return printings, nil
}

// FindPrintingsBySetNumber returns every printing whose set number is number,
// across all sets. No match is an empty slice.
func (s *Store) FindPrintingsBySetNumber(ctx context.Context, number string) ([]model.PrintedCard, error) {
	printings, err := s.selectPrintedCards(ctx, findPrintingsBySetNumberQuery, number)
	if err != nil {
		return nil, fmt.Errorf("finding printings by set number: %w", err)
	}
	return printings, nil
}

func (s *Store) selectPrintedCards(ctx context.Context, query string, args ...any) ([]model.PrintedCard, error) {
	var rows []printedCardRow
	// Unwrapped on purpose: every caller wraps with the operation it was
	// running, and wrapping twice inside one layer reads as two failures.
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return toPrintedCards(rows), nil
}
