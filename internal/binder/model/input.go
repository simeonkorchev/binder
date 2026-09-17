package model

import (
	"github.com/google/uuid"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

// SlotCard is a card and how its set was determined: everything a slot records
// except where in the binder it goes. Both writes take it — one card at a
// chosen position, or a batch appended in order — so the pairing rule below is
// stated, and validated, once.
type SlotCard struct {
	CardID uuid.UUID
	// CardPrintingID must be set exactly when RequiresPrinting(SetResolution);
	// the service rejects any other combination rather than letting the
	// database CHECK turn it into a 500.
	CardPrintingID *uuid.UUID
	SetResolution  cardmodel.SetResolution
}

// AddSlotInput is a request to put one card into a binder.
type AddSlotInput struct {
	SlotCard
	// Position is where the card goes. Nil appends it after the last card,
	// which is what a scan does; a value inserts at that position and shifts
	// everything from there on one place along.
	Position *int
}

// AddSlotsInput is a request to put a reviewed scan session into a binder in
// one go: every card or none, appended in order after the binder's last one.
//
// There is no per-card position. A batch appends, which is what committing a
// sweep does; letting one request name positions would let it ask for an
// arrangement with a hole in it, and density is this domain's whole contract.
//
// There is no copies count either: a decision covering three physical cards is
// three entries. The client already holds that count, expanding it is one loop,
// and a second way to say the same thing is a second thing the batch bound
// would have to be computed over.
type AddSlotsInput struct {
	Cards []SlotCard
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

// ReferenceCard and ReferenceCardPrinting name the rows a slot points at, in
// the vocabulary a dataerror.InvalidReferenceError carries. The store produces
// them when a foreign key rejects a write and the service matches on them to
// decide which sentinel the caller gets, so they are one fact here rather than
// two string literals in two layers that can drift apart.
const (
	ReferenceCard         = "card"
	ReferenceCardPrinting = "card printing"
)
