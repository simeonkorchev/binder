package model

// SetResolution says how a slot's *set* was determined. The values are the
// `set_resolution` enum of db/migrations/003_binders.sql, as amended by
// 005_manual_set_resolution.sql, spelled identically — a binder slot stores this
// value verbatim.
//
// The migration constrains the pairing and every producer must honour it:
//
//	(set_resolution NOT IN ('by_name','unresolved')) = (card_printing_id IS NOT NULL)
//
// so the three code rungs and the manual one always carry a printing, and the
// two that determine no set never do.
//
// It answers one question — how the set was settled — and not the neighbouring
// one, "what should the client do with this scan": that is ScanOutcome.
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
	// SetResolutionManual — a person settled the set, by choosing one of the
	// printings an ambiguous scan offered. It is not a rung of the ladder and
	// ResolveScan never returns it; it exists so that a set a human read off the
	// card is not recorded as one the scanner matched
	// (specs/001-binder-mvp/addendum-batch-commit.md section 2).
	SetResolutionManual SetResolution = "manual"
)

// Candidate is one card the ladder could not rule out. Printing is nil for a
// candidate the name rung produced, which identifies a card and no set.
type Candidate struct {
	Card     Card
	Printing *CardPrinting
}

// ScanOutcome says what happened to one scan, and so what the client has to do
// about it next. It is deliberately not SetResolution: that field answers how
// the *set* was settled, and two of its values answer the same way for scans
// that need opposite reactions from the user — `unresolved` is both "several
// matched, choose one" and "nothing matched at all".
//
// A review UI reads this field. Before it existed the mobile app had to
// classify on the response's *shape* (candidates, then card, then printing),
// which is a derivation every client re-invented and any of them could get
// wrong (specs/001-binder-mvp/addendum-batch-commit.md section 1).
type ScanOutcome string

const (
	// ScanOutcomeUnset is the zero value: no rung has concluded yet. It never
	// leaves the ladder.
	ScanOutcomeUnset ScanOutcome = ""
	// ScanOutcomeResolved — the card and the set are both settled. Card and
	// Printing are both non-nil and there are no candidates.
	ScanOutcomeResolved ScanOutcome = "resolved"
	// ScanOutcomeCardOnly — the card is settled and its set is not. Card is
	// non-nil, Printing is nil. The user may keep it as it is; the slot will
	// record no printing.
	ScanOutcomeCardOnly ScanOutcome = "card_only"
	// ScanOutcomeAmbiguous — more than one row matched and the ladder refused to
	// pick. Candidates is non-empty and the user chooses from it; Card is
	// non-nil when every candidate is a printing of the same card.
	ScanOutcomeAmbiguous ScanOutcome = "ambiguous"
	// ScanOutcomeNoMatch — nothing matched. No card, no printing, no
	// candidates: the user searches by name or leaves the card out.
	ScanOutcomeNoMatch ScanOutcome = "no_match"
)

// ScanOutcomes lists every outcome a scan can end in, most settled first. The
// OpenAPI enum derives from this list, so a new outcome cannot reach the wire
// undocumented. It omits ScanOutcomeUnset, which is never returned.
func ScanOutcomes() []ScanOutcome {
	return []ScanOutcome{
		ScanOutcomeResolved,
		ScanOutcomeCardOnly,
		ScanOutcomeAmbiguous,
		ScanOutcomeNoMatch,
	}
}

// MatchResult is what the match ladder concluded about one scan.
//
//	Outcome    Resolution            Card  Printing  Candidates
//	resolved   exact | by_prefix_and_number | by_number
//	                                 set   set       none
//	card_only  by_name               set   nil       none
//	ambiguous  unresolved            maybe nil       some — Card is set when
//	                                                 they are all printings of
//	                                                 one card
//	no_match   unresolved            nil   nil       none
//
// Outcome is stated by the rung that concludes, never derived here from the
// other four fields: the whole point of it is that the two `unresolved` rows
// above are two different answers, and a derivation would have to guess which.
//
// Candidates carries the printings or cards a rung could not choose between,
// so the review sheet can ask the user instead of guessing (US3).
type MatchResult struct {
	Outcome    ScanOutcome
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

// SetResolutions lists every value a binder slot may store, most certain first:
// the `set_resolution` enum, verbatim. Boundary validation and the OpenAPI enum
// of the binder's writes both derive from this one list, so a new value cannot
// reach the wire through one of them and not the other.
//
// It deliberately omits SetResolutionUnset: the zero value means "nobody
// decided", which is never something a client may send.
func SetResolutions() []SetResolution {
	return []SetResolution{
		SetResolutionExact,
		SetResolutionByPrefixAndNumber,
		SetResolutionByNumber,
		SetResolutionByName,
		SetResolutionUnresolved,
		SetResolutionManual,
	}
}

// ScanResolutions lists what ResolveScan can answer: the rungs of the ladder.
// It is SetResolutions minus SetResolutionManual, which only a person can
// produce — publishing `manual` on the scan endpoint's enum would advertise an
// answer the ladder cannot give.
func ScanResolutions() []SetResolution {
	ladder := make([]SetResolution, 0, len(SetResolutions()))
	for _, resolution := range SetResolutions() {
		if resolution != SetResolutionManual {
			ladder = append(ladder, resolution)
		}
	}
	return ladder
}

// ValidSetResolution reports whether r is a value a binder slot may store.
func ValidSetResolution(r SetResolution) bool {
	for _, known := range SetResolutions() {
		if r == known {
			return true
		}
	}
	return false
}

// RequiresPrinting reports whether a scan resolved this way names a card
// printing. It is one fact stated in two places: the ladder never returns a
// printing for a rung this rejects, and a binder slot may not store one — the
// binder_slots_printing_matches_resolution CHECK, as
// db/migrations/005_manual_set_resolution.sql leaves it, is the same
// equivalence in SQL.
//
// The CHECK is the last line of defence, not the first. A slot whose resolution
// and printing disagree is rejected before the INSERT, because a CHECK violation
// surfacing from the driver is a 500 telling the client the server broke when in
// fact their request was wrong.
func RequiresPrinting(r SetResolution) bool {
	switch r {
	case SetResolutionExact,
		SetResolutionByPrefixAndNumber,
		SetResolutionByNumber,
		SetResolutionManual:
		return true
	case SetResolutionByName,
		SetResolutionUnresolved,
		SetResolutionUnset:
		return false
	}
	return false
}
