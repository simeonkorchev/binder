package pipeline

import (
	"log/slog"
	"maps"
	"sync"
)

// FailureReason names why one card's image did not get stored. It is a named
// type rather than a bare string because it is both a log value and a map key
// in the run summary, and the set is closed.
type FailureReason string

const (
	// FailureNone is the zero value and means the image was stored.
	FailureNone FailureReason = ""
	// FailureNotFound is the expected one: upstream simply has no image for
	// this card. It is the only reason logged below ERROR.
	FailureNotFound FailureReason = "not_found"
	// FailureFetch covers every other upstream problem — timeout, 5xx, a body
	// that never arrived.
	FailureFetch FailureReason = "fetch"
	// FailureUnsupportedType means upstream served bytes we will not label, so
	// no object was written rather than one whose extension lies.
	FailureUnsupportedType FailureReason = "unsupported_content_type"
	// FailureStore means the object store refused the write.
	FailureStore FailureReason = "store"
	// FailureRecord means the image was stored but the key could not be
	// written back. The card stays NULL and is fetched again next run; the
	// object it already wrote is simply overwritten.
	FailureRecord FailureReason = "record"
)

// Level is where 002-go-conventions.md §4a is honoured: a card upstream does
// not have an image for is an expected outcome of walking 13k cards, and
// logging it at ERROR would bury the failures that mean something.
func (r FailureReason) Level() slog.Level {
	if r == FailureNotFound {
		return slog.LevelWarn
	}
	return slog.LevelError
}

// Result is what one run did. Only the two independent numbers are stored;
// the totals are derived so they cannot drift from the counts they summarise.
type Result struct {
	// Stored is the number of cards whose image was fetched, written and
	// recorded in this run.
	Stored int
	// Failures counts the cards left for the next run, by reason.
	Failures map[FailureReason]int
}

// Failed is the number of cards this run left without an image.
func (r Result) Failed() int {
	total := 0
	for _, count := range r.Failures {
		total += count
	}
	return total
}

// Considered is how many cards the run attempted.
func (r Result) Considered() int {
	return r.Stored + r.Failed()
}

// tally accumulates a Result across the workers of a run.
type tally struct {
	mu       sync.Mutex
	stored   int
	failures map[FailureReason]int
}

func newTally() *tally {
	return &tally{failures: make(map[FailureReason]int)}
}

// record books one card's outcome; FailureNone is a success.
func (t *tally) record(reason FailureReason) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if reason == FailureNone {
		t.stored++
		return
	}
	t.failures[reason]++
}

// result snapshots the tally. The map is cloned so a caller holding a Result
// cannot see it change under a still-running batch.
func (t *tally) result() Result {
	t.mu.Lock()
	defer t.mu.Unlock()

	return Result{Stored: t.stored, Failures: maps.Clone(t.failures)}
}
