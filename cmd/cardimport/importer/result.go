package importer

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/simeonkorchev/binder/cmd/cardimport/model"
)

// skipExamplesPerReason caps how many raw codes one reason keeps. A handful is
// enough to tell "the dump carries promo cards with no printed code" from "the
// dump changed shape and we are now dropping everything"; thirteen thousand of
// them is a log nobody reads.
const skipExamplesPerReason = 5

// SkipSummary is what one skip reason cost this run.
type SkipSummary struct {
	Count int
	// Examples are the first few raw set codes skipped for this reason,
	// exactly as the dump spelled them.
	Examples []string
}

// Result is what one import did. The counts are rows *sent* to the database —
// every write is an upsert, so "inserted" and "updated" are deliberately not
// distinguished: on a re-import the number is the same and means the same
// thing, that the row is now what the dump says.
type Result struct {
	Sets      int
	Cards     int
	Printings int
	// Skipped counts the printings that were not written, by reason. A skip is
	// never silent: the constraint that would have rejected the row is
	// mirrored in the batch builder, and the count is what makes a dump that
	// changed shape visible on the next run.
	Skipped map[model.SkipReason]SkipSummary
}

// SkippedTotal is the number of printings this run refused.
func (r Result) SkippedTotal() int {
	total := 0
	for _, summary := range r.Skipped {
		total += summary.Count
	}

	return total
}

// tally accumulates a Result across the batches of a run. It carries no lock:
// batches are written one after another, because they share set rows and
// running them concurrently would have them conflict on the same card_sets
// row rather than go faster.
type tally struct {
	sets      int
	cards     int
	printings int
	skipped   map[model.SkipReason]SkipSummary
}

func newTally() *tally {
	return &tally{skipped: make(map[model.SkipReason]SkipSummary)}
}

// record books one written batch.
func (t *tally) record(batch model.Batch) {
	t.sets += len(batch.Sets)
	t.cards += len(batch.Cards)
	t.printings += len(batch.Printings)

	for _, skip := range batch.Skipped {
		summary := t.skipped[skip.Reason]
		summary.Count++
		if len(summary.Examples) < skipExamplesPerReason {
			summary.Examples = append(summary.Examples, skip.SetCode)
		}
		t.skipped[skip.Reason] = summary
	}
}

// result snapshots the tally. The map is cloned so a caller holding a Result
// cannot see it change under a later batch.
func (t *tally) result() Result {
	return Result{
		Sets:      t.sets,
		Cards:     t.cards,
		Printings: t.printings,
		Skipped:   maps.Clone(t.skipped),
	}
}

// logResult is the run's report. The totals go out at INFO; every skip reason
// gets its own WARN line naming the reason, the count and a few of the codes,
// because a skipped printing is a row the card database will not have and that
// has to be loud enough to notice (000-principles.md section 5).
func logResult(ctx context.Context, result Result) {
	slog.InfoContext(ctx, "card import finished",
		slog.Int("sets_upserted", result.Sets),
		slog.Int("cards_upserted", result.Cards),
		slog.Int("printings_upserted", result.Printings),
		slog.Int("printings_skipped", result.SkippedTotal()),
	)

	// Sorted so two runs over the same dump produce the same log, which is
	// what makes the lines diffable between runs.
	for _, reason := range slices.Sorted(maps.Keys(result.Skipped)) {
		summary := result.Skipped[reason]
		slog.WarnContext(ctx, "printings skipped, they are not in the card database",
			slog.String("skip_reason", string(reason)),
			slog.Int("count", summary.Count),
			slog.Any("example_set_codes", summary.Examples),
		)
	}
}
