package model

// SetResolution says how a scan's *set* was determined. The values are the
// db/migrations/003_binders.sql `set_resolution` enum, spelled identically,
// because a binder slot stores this value verbatim.
//
// 003_binders.sql constrains the pairing and the ladder must honour it:
//
//	(set_resolution IN ('exact','by_prefix_and_number','by_number'))
//	  = (card_printing_id IS NOT NULL)
//
// so the three code rungs always carry a printing and the other two never do.
type SetResolution string

const (
	// SetResolutionUnset is the zero value and is never a valid resolution;
	// it exists so an unset field is distinguishable from a decided one.
	SetResolutionUnset SetResolution = ""
	// SetResolutionExact — the whole printed code matched one printing.
	SetResolutionExact SetResolution = "exact"
	// SetResolutionByPrefixAndNumber — the number matched and the set's prefix
	// starts with the prefix that was read, which is what saves a misread
	// prefix tail.
	SetResolutionByPrefixAndNumber SetResolution = "by_prefix_and_number"
	// SetResolutionByNumber — the number alone matched one printing.
	SetResolutionByNumber SetResolution = "by_number"
	// SetResolutionByName — the name rung identified the card. Which set it was
	// printed in is unknown, so there is no printing.
	SetResolutionByName SetResolution = "by_name"
	// SetResolutionUnresolved — the set could not be determined. The card may
	// still be known (Card non-nil): several printings of one card share the
	// number that was read, so the card is settled and the set is not.
	SetResolutionUnresolved SetResolution = "unresolved"
)

// Candidate is one card the ladder could not rule out. Printing is nil for a
// candidate the name rung produced, which identifies a card and no set.
type Candidate struct {
	Card     Card
	Printing *CardPrinting
}

// MatchResult is what the match ladder concluded about one scan.
//
//	Resolution           Card  Printing  meaning
//	exact                set   set       the printed code named one printing
//	by_prefix_and_number set   set       number + a prefix of the set's prefix
//	by_number            set   set       the number alone named one printing
//	by_name              set   nil       the card is known, its set is not
//	unresolved           maybe nil       the set is not known; Card is set when
//	                                     the ambiguous printings are all of one
//	                                     card, nil when nothing matched at all
//
// Candidates carries the printings or cards a rung could not choose between,
// so the review sheet can ask the user instead of guessing (US3).
type MatchResult struct {
	Resolution SetResolution
	Card       *Card
	Printing   *CardPrinting
	Candidates []Candidate
}

// ScanInput is the OCR text of one scanned card. Both fields are as the camera
// read them: mangling, case and stray punctuation are the ladder's problem.
type ScanInput struct {
	// Code is the printed code line, ideally "{PREFIX}-{NUMBER}".
	Code string
	// Name is the card-name line.
	Name string
}
