package model

import (
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"time"

	"github.com/google/uuid"
)

// SlotsPerPage is the 3x3 grid of a physical binder page. Every page/slot
// derivation in the codebase goes through this constant rather than a literal 9
// so that the grid is one fact.
const SlotsPerPage = 9

// Binder is one collector's binder.
type Binder struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Slot is one card in one pocket of a binder.
//
// Position is a single flat integer, not a (page, row, column) triple: page and
// slot are derived from it, so a page is a range scan and a reorder is a bulk
// update of integers. The database enforces that a position is unique within a
// binder and not negative; it cannot enforce that positions are *dense*, and
// keeping them gap-free is the service's contract.
type Slot struct {
	ID       uuid.UUID
	BinderID uuid.UUID
	Position int
	CardID   uuid.UUID
	// CardPrintingID is nil exactly when SetResolution does not require a
	// printing: the name rung identifies the card but not the set it was
	// printed in.
	CardPrintingID *uuid.UUID
	SetResolution  cardmodel.SetResolution
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Page is the page this slot falls on, counted from zero.
func (s Slot) Page() int {
	return PageOfPosition(s.Position)
}

// SlotOnPage is the index of this slot within its page, 0 to SlotsPerPage-1,
// reading left to right and top to bottom.
func (s Slot) SlotOnPage() int {
	return s.Position % SlotsPerPage
}

// PageOfPosition is the page a position falls on, counted from zero. Pages are
// zero-based throughout — the API, the model and the SQL — because that is what
// makes the derivation position/SlotsPerPage exact and leaves nowhere for an
// off-by-one to hide.
func PageOfPosition(position int) int {
	return position / SlotsPerPage
}

// FirstPositionOnPage is the lowest position that falls on the given page; the
// page covers FirstPositionOnPage(page) through that plus SlotsPerPage-1.
func FirstPositionOnPage(page int) int {
	return page * SlotsPerPage
}

// PageCount is how many pages it takes to hold slotCount cards. An empty binder
// has no pages: "nothing to show" is a count of zero, not an empty page the
// client has to recognise as a special case.
func PageCount(slotCount int) int {
	return (slotCount + SlotsPerPage - 1) / SlotsPerPage
}

// Page is one page of a binder: the slots that are actually on it, in position
// order, and how many pages the binder has.
//
// Slots holds only occupied positions, so it is shorter than SlotsPerPage on a
// partly filled last page and empty on a binder with no cards. It is the API
// layer that lays these out into a fixed 3x3 grid, because the grid is how the
// page is drawn, not what it is.
type Page struct {
	Number    int
	Slots     []Slot
	PageCount int
}
