// Package model holds the rows the card importer moves between the batch
// builder that shapes them and the store that writes them, so neither has to
// import the other (001-architecture.md: data flows one direction).
package model

// SkipReason names why a printing in the dump was not imported. Each one is a
// row the database would either reject outright or accept with a blank key
// column — and a blank row that saved is worse than one that failed
// (000-principles.md section 10). They are counted and logged rather than
// dropped, so a change in the dump's format is visible on the next run instead
// of quietly shrinking the card database.
//
// It is a named type rather than a bare string because it is both a log value
// and a map key in the run summary, and the set is closed.
type SkipReason string

const (
	// SkipMalformedSetCode: card_printings_set_code_shape requires
	// PREFIX-NUMBER with both halves non-empty (db/migrations/001_cards.sql).
	SkipMalformedSetCode SkipReason = "malformed_set_code"
	// SkipMissingRarity: rarity is part of the printing's identity — two
	// rarities of one code are two printings — so a blank one is not a
	// printing we can tell apart from any other blank one.
	SkipMissingRarity SkipReason = "missing_rarity"
	// SkipMissingSetName: card_sets.name is NOT NULL and is what the app
	// shows; a set row named "" is not worth creating.
	SkipMissingSetName SkipReason = "missing_set_name"
)

// SetRow is a row of card_sets. Code is the set prefix ("LOB"), because that
// is the table's natural key and the target of card_printings' foreign key.
type SetRow struct {
	Code string
	Name string
}

// CardRow is a row of cards. The uuid primary key is not here: it is minted by
// the store on insert and kept from the existing row on conflict, so a
// re-import never re-mints an id that binder slots already point at.
type CardRow struct {
	YgoprodeckID int64
	Name         string
}

// PrintingRow is a row of card_printings before its card's uuid is known; it
// is also the de-duplication key inside a batch, which is why it is comparable.
type PrintingRow struct {
	YgoprodeckID int64
	SetCode      string
	Rarity       string
}

// SkippedPrinting is one printing the batch refused, kept whole rather than
// counted only: the raw code is what tells an operator reading the log whether
// the dump changed shape or simply carries promo cards with no printed code.
type SkippedPrinting struct {
	YgoprodeckID int64
	SetCode      string
	Rarity       string
	Reason       SkipReason
}

// Batch is one transaction's worth of rows, in the order they must be written:
// Sets first, because card_printings.set_prefix is generated from set_code and
// is the foreign key to card_sets.code, so a printing whose set row does not
// exist yet is rejected.
type Batch struct {
	Sets      []SetRow
	Cards     []CardRow
	Printings []PrintingRow
	Skipped   []SkippedPrinting
}

// Empty reports that the batch would write nothing. An empty batch is not an
// error — a slice of the dump whose every printing was skipped still has cards
// to write, and a batch with nothing at all is simply a no-op
// (000-principles.md section 8b).
func (b Batch) Empty() bool {
	return len(b.Sets) == 0 && len(b.Cards) == 0 && len(b.Printings) == 0
}
