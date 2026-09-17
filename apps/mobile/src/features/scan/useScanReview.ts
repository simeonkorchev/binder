import { useState } from 'react'

import { flaggedScans, type FlaggedScan } from './lib/flaggedScans'
import type { RejectedScan, ResolvedScan, ScannedCard, ScannedPrinting } from './types'

/**
 * What the user settled a flagged scan as.
 *
 * `kept` carries the card itself rather than an id because the sheet has to
 * show what was decided, and a row that read "kept card 8f3a-…" would be a
 * decision the user cannot check. A null printing is a real answer, not a
 * missing one: it says the card is known and its set is not, which is exactly
 * what a binder slot with no printing records.
 */
export type ReviewDecision =
  | { kind: 'kept'; card: ScannedCard; printing: ScannedPrinting | null }
  /** Left out of the binder. The only move for a scan that named no card at all. */
  | { kind: 'discarded' }

/** One flagged scan and what the user has made of it so far. */
export interface ReviewRow extends FlaggedScan {
  /** Null while the scan is still open — this is what `undecidedCount` counts. */
  decision: ReviewDecision | null
}

export interface ScanReview {
  /** Every flagged scan, oldest first, with the decision it carries. */
  rows: ReviewRow[]
  /** How many flagged scans the user has not settled yet. */
  undecidedCount: number
  /** Settles one flagged scan. Deciding it again replaces the previous answer. */
  decide: (code: string, decision: ReviewDecision) => void
  /** Puts one settled scan back in question, so a wrong tap is not permanent. */
  reopen: (code: string) => void
}

/**
 * The review: what the sweep could not settle, and what the user says about it.
 *
 * It owns one thing — the decisions — and derives everything else during
 * render. The flagged rows are a pure function of the queue's answers, so
 * holding them in state beside the decisions would be a copy that can go stale
 * the moment one more scan lands while the sheet is open.
 *
 * Decisions are keyed by **code**, matching the one-question-per-code rule
 * `flaggedScans` groups on: deciding a card settles every copy of it the sweep
 * captured.
 */
export const useScanReview = (
  resolved: readonly ResolvedScan[],
  rejected: readonly RejectedScan[],
): ScanReview => {
  const [decisions, setDecisions] = useState<Record<string, ReviewDecision>>({})

  const rows = flaggedScans(resolved, rejected).map(
    (flagged): ReviewRow => ({ ...flagged, decision: decisions[flagged.code] ?? null }),
  )

  return {
    rows,
    undecidedCount: rows.filter((row) => row.decision === null).length,
    decide: (code: string, decision: ReviewDecision): void => {
      setDecisions((settled) => ({ ...settled, [code]: decision }))
    },
    reopen: (code: string): void => {
      setDecisions(({ [code]: _reopened, ...rest }) => rest)
    },
  }
}
