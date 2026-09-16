package model

import (
	"github.com/google/uuid"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

// AddSlotInput is a request to put a card into a binder.
type AddSlotInput struct {
	CardID uuid.UUID
	// CardPrintingID must be set exactly when RequiresPrinting(SetResolution);
	// the service rejects any other combination rather than letting the
	// database CHECK turn it into a 500.
	CardPrintingID *uuid.UUID
	SetResolution  cardmodel.SetResolution
	// Position is where the card goes. Nil appends it after the last card,
	// which is what a scan does; a value inserts at that position and shifts
	// everything from there on one place along.
	Position *int
}

// MoveSlotInput is a request to move one card to a different position, taking
// every card between its old and new position one place along. It is expressed
// as a move rather than as a list of (slot, position) pairs so that a dense
// binder stays dense by construction: there is no arrangement a client can ask
// for that leaves a hole.
type MoveSlotInput struct {
	SlotID     uuid.UUID
	ToPosition int
}
