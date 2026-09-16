// Package service holds the card domain's business logic: the match ladder
// that turns OCR text into a printing, and the card search behind GET /cards.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/simeonkorchev/binder/internal/card/model"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// ErrEmptyScan is returned when a scan carries neither a usable printed code
// nor a name, so no rung of the ladder has anything to match on. A code with no
// number is not usable: every code rung keys on the number.
var ErrEmptyScan = errors.New("scan has no usable code or name")

// Store reads the imported card database.
//
// Every method answers with an empty slice when nothing matched; absence is not
// an error here, because a scan that matches nothing is the ordinary case the
// ladder exists to handle (000-principles.md section 8b).
//
//counterfeiter:generate . Store
type Store interface {
	// FindPrintingsBySetCode returns every printing whose whole printed code is
	// setCode.
	FindPrintingsBySetCode(ctx context.Context, setCode string) ([]model.PrintedCard, error)
	// FindPrintingsBySetPrefixAndNumber returns every printing whose set number
	// is number and whose set prefix starts with prefix.
	FindPrintingsBySetPrefixAndNumber(ctx context.Context, prefix, number string) ([]model.PrintedCard, error)
	// FindPrintingsBySetNumber returns every printing whose set number is
	// number, in any set.
	FindPrintingsBySetNumber(ctx context.Context, number string) ([]model.PrintedCard, error)
	// FindCardsByName returns at most limit cards whose name has at least
	// threshold trigram similarity to name, best first.
	FindCardsByName(ctx context.Context, name string, threshold float64, limit int) ([]model.NameMatch, error)
	// SearchCards returns one page of cards matching search.
	SearchCards(ctx context.Context, search model.CardSearch) ([]model.Card, error)
}

// Service resolves scans and searches cards.
type Service struct {
	store Store
}

// NewService returns a service reading through store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// SearchCards returns one page of cards matching search. A page with no cards
// on it is an empty slice, not an error.
func (s *Service) SearchCards(ctx context.Context, search model.CardSearch) ([]model.Card, error) {
	cards, err := s.store.SearchCards(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("searching cards: %w", err)
	}
	return cards, nil
}
