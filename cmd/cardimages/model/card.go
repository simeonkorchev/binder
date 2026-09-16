// Package model holds the types the card-image pipeline moves between its
// store and its orchestration, so neither has to import the other
// (001-architecture.md: data flows one direction).
package model

import "github.com/google/uuid"

// PendingCard is a card whose image has not been fetched yet — a row of
// `cards` with `image_object_key IS NULL`, carrying only the two columns the
// pipeline reads. It is deliberately not the whole card: the image pipeline
// has no use for the name or the printings, and a wider type would invite a
// query that reads them.
type PendingCard struct {
	// ID is what the fetched key is recorded against.
	ID uuid.UUID
	// YGOProDeckID is both the upstream image's identity and the cursor the
	// run walks, so a killed run resumes from where it stopped.
	YGOProDeckID int64
}
