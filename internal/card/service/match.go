package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/simeonkorchev/binder/internal/card/model"
)

const (
	// nameSimilarityThreshold is pg_trgm's own default, restated so the name
	// rung behaves the same whatever the cluster's similarity_threshold is.
	nameSimilarityThreshold = 0.3
	// nameCandidateLimit caps what the name rung reads back. It only ever needs
	// the best score and its runner-up to tell a winner from a tie; the rest are
	// what the review sheet offers the user when it is a tie.
	nameCandidateLimit = 5
)

// rungOutcome is what one rung of the ladder concluded. decided is false only
// when the rung matched nothing at all and the ladder must carry on down.
type rungOutcome struct {
	match   model.MatchResult
	decided bool
}

// ResolveScan walks the match ladder for one scanned card:
//
//  1. the whole printed code,
//  2. the number plus a prefix the set's own prefix starts with,
//  3. the number alone,
//  4. the card name, by trigram similarity,
//  5. unresolved.
//
// A rung that matches exactly one printing settles the set and stops the
// ladder. A rung that matches several stops it too, and settles nothing: those
// printings come back as candidates for the user to choose between, because
// guessing between them is the silent wrongness US3 exists to prevent. A rung
// that matches nothing hands over to the next one.
func (s *Service) ResolveScan(ctx context.Context, scan model.ScanInput) (model.MatchResult, error) {
	code := parseScanCode(scan.Code)
	name := strings.TrimSpace(scan.Name)
	if code.number == "" && name == "" {
		return model.MatchResult{}, ErrEmptyScan
	}

	outcome, err := s.matchByCode(ctx, code)
	if err != nil {
		return model.MatchResult{}, err
	}
	if outcome.decided {
		return outcome.match, nil
	}

	return s.matchByName(ctx, name)
}

// matchByCode runs the three code rungs. Each rung's result set contains the
// one below it, so the first rung to match anything is also the most precise
// one that can: there is nothing for a later rung to add.
func (s *Service) matchByCode(ctx context.Context, code scanCode) (rungOutcome, error) {
	if code.number == "" {
		return rungOutcome{match: model.MatchResult{}, decided: false}, nil
	}

	if code.prefix != "" {
		printings, err := s.store.FindPrintingsBySetCode(ctx, code.setCode())
		if err != nil {
			return rungOutcome{}, fmt.Errorf("matching the scanned code exactly: %w", err)
		}
		if outcome := decideCodeRung(printings, model.SetResolutionExact); outcome.decided {
			return outcome, nil
		}

		printings, err = s.store.FindPrintingsBySetPrefixAndNumber(ctx, code.prefix, code.number)
		if err != nil {
			return rungOutcome{}, fmt.Errorf("matching the scanned set prefix and number: %w", err)
		}
		if outcome := decideCodeRung(printings, model.SetResolutionByPrefixAndNumber); outcome.decided {
			return outcome, nil
		}
	}

	printings, err := s.store.FindPrintingsBySetNumber(ctx, code.number)
	if err != nil {
		return rungOutcome{}, fmt.Errorf("matching the scanned set number: %w", err)
	}
	return decideCodeRung(printings, model.SetResolutionByNumber), nil
}

// decideCodeRung turns one code rung's printings into its conclusion.
func decideCodeRung(printings []model.PrintedCard, resolution model.SetResolution) rungOutcome {
	switch len(printings) {
	case 0:
		return rungOutcome{match: model.MatchResult{}, decided: false}
	case 1:
		return rungOutcome{match: model.MatchResult{
			Resolution: resolution,
			Card:       &printings[0].Card,
			Printing:   &printings[0].Printing,
			Candidates: nil,
		}, decided: true}
	default:
		return rungOutcome{match: ambiguousPrintings(printings), decided: true}
	}
}

// ambiguousPrintings is the conclusion for a rung that matched more than one
// printing. The set is not determined, so no printing is returned and the
// resolution is unresolved. The card still can be: when every candidate is a
// printing of the same card, the scan identified the card and only its set is
// open — which is precisely what a binder slot with a null card_printing_id
// records (003_binders.sql).
func ambiguousPrintings(printings []model.PrintedCard) model.MatchResult {
	result := model.MatchResult{
		Resolution: model.SetResolutionUnresolved,
		Card:       nil,
		Printing:   nil,
		Candidates: printingCandidates(printings),
	}
	if card, ok := soleCard(printings); ok {
		result.Card = card
	}
	return result
}

// soleCard returns the card every printing belongs to, if they all belong to
// one.
func soleCard(printings []model.PrintedCard) (*model.Card, bool) {
	for _, printing := range printings[1:] {
		if printing.Card.ID != printings[0].Card.ID {
			return nil, false
		}
	}
	return &printings[0].Card, true
}

// matchByName is the last rung: the card name, fuzzily. It identifies a card
// and never a set, so it never carries a printing.
func (s *Service) matchByName(ctx context.Context, name string) (model.MatchResult, error) {
	if name == "" {
		return unresolved(nil), nil
	}

	matches, err := s.store.FindCardsByName(ctx, name, nameSimilarityThreshold, nameCandidateLimit)
	if err != nil {
		return model.MatchResult{}, fmt.Errorf("matching the scanned name: %w", err)
	}
	if len(matches) == 0 {
		return unresolved(nil), nil
	}

	// Card names repeat in the imported dump (001_cards.sql), so two cards can
	// score identically against one scanned name. A best score that is only
	// tied is not a winner; anything strictly better is.
	if len(matches) > 1 && matches[0].Similarity == matches[1].Similarity {
		return unresolved(nameCandidates(matches)), nil
	}

	return model.MatchResult{
		Resolution: model.SetResolutionByName,
		Card:       &matches[0].Card,
		Printing:   nil,
		Candidates: nil,
	}, nil
}

func unresolved(candidates []model.Candidate) model.MatchResult {
	return model.MatchResult{
		Resolution: model.SetResolutionUnresolved,
		Card:       nil,
		Printing:   nil,
		Candidates: candidates,
	}
}

func printingCandidates(printings []model.PrintedCard) []model.Candidate {
	candidates := make([]model.Candidate, 0, len(printings))
	for i := range printings {
		candidates = append(candidates, model.Candidate{
			Card:     printings[i].Card,
			Printing: &printings[i].Printing,
		})
	}
	return candidates
}

func nameCandidates(matches []model.NameMatch) []model.Candidate {
	candidates := make([]model.Candidate, 0, len(matches))
	for i := range matches {
		candidates = append(candidates, model.Candidate{
			Card:     matches[i].Card,
			Printing: nil,
		})
	}
	return candidates
}
