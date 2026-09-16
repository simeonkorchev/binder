package ygoprodeck

// The wire types. Everything this package reads out of a YGOPRODeck response
// is declared in this file, so a contract that turns out to differ from the
// assumptions in the package doc is corrected in one place. Field names are
// ours (no stutter); the json tags carry the upstream spelling and are the
// part that has to match the API.

// cardInfoResponse is the envelope around the dump (assumption A2). It is
// unexported because the envelope is an implementation detail of the
// transport: callers get the cards.
type cardInfoResponse struct {
	Data []Card `json:"data"`
}

// Card is one card in the dump (assumptions A3, A4). ID is the passcode
// printed on the cardboard and is the importer's upsert key — names repeat in
// the dump, ids do not.
type Card struct {
	ID     int64       `json:"id"`
	Name   string      `json:"name"`
	Sets   []CardSet   `json:"card_sets"`
	Images []CardImage `json:"card_images"`
}

// CardSet is one printing of a card: the card as it appears in one set, with
// the code printed on it and the rarity it was printed at. A card reprinted at
// two rarities under the same code appears as two entries, which is why rarity
// is part of the printing's identity.
type CardSet struct {
	Name   string `json:"set_name"`
	Code   string `json:"set_code"`
	Rarity string `json:"set_rarity"`
}

// CardImage is one artwork of a card. URL and SmallURL are upstream locations
// to download from, never to link to: assumption A6 forbids hotlinking, and
// cards.image_object_key holds our own copy instead (db/migrations/001_cards.sql).
// Consumed by cmd/cardimages, not by cmd/cardimport.
type CardImage struct {
	ID       int64  `json:"id"`
	URL      string `json:"image_url"`
	SmallURL string `json:"image_url_small"`
}
