// Package model holds the card domain's types. A scan resolves to a *printing*
// — a card as printed in one set — because that is what the code on the
// cardboard names. See db/migrations/001_cards.sql.
package model

import "github.com/google/uuid"

// Card is the artwork-and-rules identity: one card, however many sets it was
// printed in.
type Card struct {
	ID   uuid.UUID
	Name string
	// ImageObjectKey is the object key inside our own bucket, not a URL: the
	// bucket and CDN host are deployment config. It is nil until cmd/cardimages
	// has fetched the image, which is also how that pipeline resumes.
	ImageObjectKey *string
}

// CardPrinting is one card as printed in one set. SetCode is the full printed
// code ("LOB-EN005"); the prefix and number the match ladder keys on are
// generated columns in the database, not fields here, because nothing outside
// a query needs them.
type CardPrinting struct {
	ID      uuid.UUID
	CardID  uuid.UUID
	SetCode string
	Rarity  string
}

// PrintedCard is a printing together with the card it prints — the shape every
// code rung of the match ladder reads, and the only shape in which a printing
// is useful to a caller (a printing alone cannot be shown to a user).
type PrintedCard struct {
	Card     Card
	Printing CardPrinting
}

// NameMatch is one card the name rung found, with the trigram similarity that
// found it. The ladder compares the best two scores to tell a winner from a
// tie, which is the only reason the score leaves the store.
type NameMatch struct {
	Card       Card
	Similarity float64
}

// CardSearch is the filter behind GET /cards. Page is 1-based.
type CardSearch struct {
	// Query matches anywhere in the card name, case-insensitively. Empty
	// matches every card.
	Query string
	// SetPrefix restricts the result to cards with a printing in that set
	// ("LOB"). Empty does not restrict.
	SetPrefix string
	Page      int
	PageSize  int
}
