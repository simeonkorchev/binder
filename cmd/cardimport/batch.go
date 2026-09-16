package main

import (
	"strings"

	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

// skipReason names why a printing in the dump was not imported. Each one is a
// row the database would either reject outright or accept with a blank key
// column — and a blank row that saved is worse than one that failed
// (000-principles.md section 10). They are counted and logged rather than
// dropped, so a change in the dump's format is visible on the next run instead
// of quietly shrinking the card database.
type skipReason string

const (
	// reasonMalformedSetCode: card_printings_set_code_shape requires
	// PREFIX-NUMBER with both halves non-empty (db/migrations/001_cards.sql).
	reasonMalformedSetCode skipReason = "malformed_set_code"
	// reasonMissingRarity: rarity is part of the printing's identity — two
	// rarities of one code are two printings — so a blank one is not a
	// printing we can tell apart from any other blank one.
	reasonMissingRarity skipReason = "missing_rarity"
	// reasonMissingSetName: card_sets.name is NOT NULL and is what the app
	// shows; a set row named "" is not worth creating.
	reasonMissingSetName skipReason = "missing_set_name"
)

// setRow is a row of card_sets. Code is the set prefix ("LOB"), because that
// is the table's natural key and the target of card_printings' foreign key.
type setRow struct {
	Code string
	Name string
}

type cardRow struct {
	YgoprodeckID int64
	Name         string
}

// printingRow is a row of card_printings before its card's uuid is known; it
// is also the de-duplication key inside a batch, which is why it is comparable.
type printingRow struct {
	YgoprodeckID int64
	SetCode      string
	Rarity       string
}

type skippedPrinting struct {
	YgoprodeckID int64
	SetCode      string
	Rarity       string
	Reason       skipReason
}

// batch is one transaction's worth of rows, in the order they must be written:
// Sets first, because card_printings.set_prefix is generated from set_code and
// is the foreign key to card_sets.code, so a printing whose set row does not
// exist yet is rejected.
type batch struct {
	Sets      []setRow
	Cards     []cardRow
	Printings []printingRow
	Skipped   []skippedPrinting
}

// buildBatch turns dump cards into the rows to upsert. It is pure, and it is
// where the dump's duplication is resolved: every card in a set repeats that
// set, and one card can list the same code and rarity twice. Both have to go,
// because ON CONFLICT DO UPDATE cannot touch the same row twice in one
// statement.
func buildBatch(cards []ygoprodeck.Card) batch {
	b := newBatchBuilder(len(cards))
	for _, card := range cards {
		b.addCard(card)
	}

	return b.batch
}

type batchBuilder struct {
	batch
	seenCards     map[int64]struct{}
	seenSets      map[string]struct{}
	seenPrintings map[printingRow]struct{}
}

func newBatchBuilder(cards int) *batchBuilder {
	return &batchBuilder{
		batch:         batch{Cards: make([]cardRow, 0, cards)},
		seenCards:     make(map[int64]struct{}, cards),
		seenSets:      make(map[string]struct{}),
		seenPrintings: make(map[printingRow]struct{}),
	}
}

func (b *batchBuilder) addCard(card ygoprodeck.Card) {
	if _, seen := b.seenCards[card.ID]; !seen {
		b.seenCards[card.ID] = struct{}{}
		b.Cards = append(b.Cards, cardRow{YgoprodeckID: card.ID, Name: strings.TrimSpace(card.Name)})
	}

	for _, set := range card.Sets {
		b.addPrinting(card.ID, set)
	}
}

func (b *batchBuilder) addPrinting(cardID int64, set ygoprodeck.CardSet) {
	code, ok := normaliseSetCode(set.Code)
	if !ok {
		b.skip(cardID, set, reasonMalformedSetCode)

		return
	}

	rarity := strings.TrimSpace(set.Rarity)
	if rarity == "" {
		b.skip(cardID, set, reasonMissingRarity)

		return
	}

	setName := strings.TrimSpace(set.Name)
	if setName == "" {
		b.skip(cardID, set, reasonMissingSetName)

		return
	}

	printing := printingRow{YgoprodeckID: cardID, SetCode: code, Rarity: rarity}
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
		b.Sets = append(b.Sets, setRow{Code: prefix, Name: setName})
	}
}

func (b *batchBuilder) skip(cardID int64, set ygoprodeck.CardSet, reason skipReason) {
	b.Skipped = append(b.Skipped, skippedPrinting{
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
