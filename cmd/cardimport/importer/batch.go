package importer

import (
	"strings"

	"github.com/simeonkorchev/binder/cmd/cardimport/model"
	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

// buildBatch turns dump cards into the rows to upsert. It is pure, and it is
// where the dump's duplication is resolved: every card in a set repeats that
// set, and one card can list the same code and rarity twice. Both have to go,
// because ON CONFLICT DO UPDATE cannot touch the same row twice in one
// statement.
func buildBatch(cards []ygoprodeck.Card) model.Batch {
	b := newBatchBuilder(len(cards))
	for _, card := range cards {
		b.addCard(card)
	}

	return b.Batch
}

type batchBuilder struct {
	model.Batch
	seenCards     map[int64]struct{}
	seenSets      map[string]struct{}
	seenPrintings map[model.PrintingRow]struct{}
}

func newBatchBuilder(cards int) *batchBuilder {
	return &batchBuilder{
		Batch:         model.Batch{Cards: make([]model.CardRow, 0, cards)},
		seenCards:     make(map[int64]struct{}, cards),
		seenSets:      make(map[string]struct{}),
		seenPrintings: make(map[model.PrintingRow]struct{}),
	}
}

func (b *batchBuilder) addCard(card ygoprodeck.Card) {
	if _, seen := b.seenCards[card.ID]; !seen {
		b.seenCards[card.ID] = struct{}{}
		b.Cards = append(b.Cards, model.CardRow{YgoprodeckID: card.ID, Name: strings.TrimSpace(card.Name)})
	}

	for _, set := range card.Sets {
		b.addPrinting(card.ID, set)
	}
}

func (b *batchBuilder) addPrinting(cardID int64, set ygoprodeck.CardSet) {
	code, ok := normaliseSetCode(set.Code)
	if !ok {
		b.skip(cardID, set, model.SkipMalformedSetCode)

		return
	}

	rarity := strings.TrimSpace(set.Rarity)
	if rarity == "" {
		b.skip(cardID, set, model.SkipMissingRarity)

		return
	}

	setName := strings.TrimSpace(set.Name)
	if setName == "" {
		b.skip(cardID, set, model.SkipMissingSetName)

		return
	}

	printing := model.PrintingRow{YgoprodeckID: cardID, SetCode: code, Rarity: rarity}
	if _, seen := b.seenPrintings[printing]; seen {
		return
	}
	b.seenPrintings[printing] = struct{}{}
	b.Printings = append(b.Printings, printing)

	// The prefix is the set. Only sets that survived the checks above get a
	// row: a set whose every printing was skipped is not one we have seen.
	// First name wins inside a batch, so the rows a run writes do not depend
	// on which card happened to be last.
	prefix, _, _ := strings.Cut(code, "-")
	if _, seen := b.seenSets[prefix]; !seen {
		b.seenSets[prefix] = struct{}{}
		b.Sets = append(b.Sets, model.SetRow{Code: prefix, Name: setName})
	}
}

func (b *batchBuilder) skip(cardID int64, set ygoprodeck.CardSet, reason model.SkipReason) {
	b.Skipped = append(b.Skipped, model.SkippedPrinting{
		YgoprodeckID: cardID,
		SetCode:      set.Code,
		Rarity:       set.Rarity,
		Reason:       reason,
	})
}

// normaliseSetCode upper-cases a printed code and reports whether it satisfies
// card_printings_set_code_shape — `^[^-]+-.+$`, a non-empty prefix, a hyphen,
// a non-empty remainder. The constraint is mirrored here rather than left to
// the database because one bad code in a batch of five hundred would otherwise
// take the whole transaction down with it. It is mirrored, never loosened: a
// code the database rejects is a dump we do not understand.
//
// Cut splits at the first hyphen, exactly as the generated columns do, so the
// remainder of a multi-hyphen code (SDK-EN-A01) stays in the number.
func normaliseSetCode(raw string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(raw))
	prefix, number, found := strings.Cut(code, "-")
	if !found || prefix == "" || number == "" {
		return "", false
	}

	return code, true
}
