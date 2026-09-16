package model

import cardmodel "github.com/simeonkorchev/binder/internal/card/model"

// The match ladder and its SetResolution values belong to the card domain,
// which is what produces them; a binder slot only stores what the ladder
// concluded. Importing the card domain's model is the shape
// 002-go-conventions.md section 3a asks for — types travel between domains,
// concrete services do not.

// SetResolutions lists every rung of the ladder, most certain first. Boundary
// validation and the OpenAPI enum both derive from this one list, so a new rung
// cannot reach the wire through one of them and not the other.
//
// It deliberately omits SetResolutionUnset: the zero value means "nobody
// decided", which is never something a client may send.
func SetResolutions() []cardmodel.SetResolution {
	return []cardmodel.SetResolution{
		cardmodel.SetResolutionExact,
		cardmodel.SetResolutionByPrefixAndNumber,
		cardmodel.SetResolutionByNumber,
		cardmodel.SetResolutionByName,
		cardmodel.SetResolutionUnresolved,
	}
}

// ValidSetResolution reports whether r is a decided rung of the ladder.
func ValidSetResolution(r cardmodel.SetResolution) bool {
	for _, known := range SetResolutions() {
		if r == known {
			return true
		}
	}
	return false
}

// RequiresPrinting reports whether a slot resolved this way must name a card
// printing. It is the Go side of the binder_slots_printing_matches_resolution
// CHECK in db/migrations/003_binders.sql, which states the same thing as an
// equivalence: the three code rungs resolve to a printing, the name rung and an
// unresolved scan do not.
//
// The constraint is the last line of defence, not the first. A slot whose
// resolution and printing disagree is rejected before the INSERT, because a
// CHECK violation surfacing from the driver is a 500 telling the client the
// server broke when in fact their request was wrong.
func RequiresPrinting(r cardmodel.SetResolution) bool {
	switch r {
	case cardmodel.SetResolutionExact,
		cardmodel.SetResolutionByPrefixAndNumber,
		cardmodel.SetResolutionByNumber:
		return true
	case cardmodel.SetResolutionByName,
		cardmodel.SetResolutionUnresolved,
		cardmodel.SetResolutionUnset:
		return false
	}
	return false
}
