import type { SlotCardBody } from '@/features/binder/types'

import type { RejectedScan, ResolvedScan } from '../types'
import type { ReviewDecision, ReviewRow } from '../useScanReview'

/**
 * Everything one commit is built out of: what the resolver answered about the
 * sweep, and what the user made of the part of it the ladder could not settle.
 *
 * The three lists are the session's own, passed as they are rather than
 * pre-joined: `reviewed` is `ScanReview.rows`, so the decisions the mapper reads
 * are literally the ones the user was shown.
 */
export interface ReviewedSweep {
  /** Every answer the resolver gave, one per captured card, in sweep order. */
  resolved: readonly ResolvedScan[]
  /** Every scan the resolver refused, one per captured card. */
  rejected: readonly RejectedScan[]
  /** Every flagged code and the decision it carries, undecided ones included. */
  reviewed: readonly ReviewRow[]
}

/**
 * The reviewed sweep as the batch write takes it: one entry per physical card,
 * in the order the cards were swept.
 *
 * **One entry per card, not per decision.** Two captures of one code are two
 * cards in the user's hand but one question in the sheet, so the decision the
 * user made once is expanded back out here — which is exactly the `copies`
 * number the row showed them. The count comes from the resolver's answers for
 * the same reason `flaggedScans` counts them: one answer is one capture, and a
 * refusal for a code that was also answered is not a second card.
 *
 * **Where the resolution comes from** is the whole point of this mapper, and it
 * is two rules, never mixed:
 *
 * - A card the review never asked about was settled by the ladder, and keeps the
 *   rung the ladder answered with — `exact` stays `exact`.
 * - A card the user settled is theirs: a printing they picked is `manual`, and a
 *   card they kept with its set left open is `by_name`.
 *
 * Nothing is sent for a card that was left out, or for a flagged row the user
 * never answered: to a binder those are the same thing, and a sweep where every
 * card was left out is an empty commit, which the endpoint accepts.
 */
export const toSlotCards = ({ resolved, rejected, reviewed }: ReviewedSweep): SlotCardBody[] => {
  const decisions = new Map(reviewed.map((row) => [row.code, row.decision]))
  const answered = new Set(resolved.map((scan) => scan.code))
  const cards: SlotCardBody[] = []

  for (const scan of resolved) {
    const decision = decisions.get(scan.code)
    // Absent from the map, rather than null in it: the review never asked about
    // this scan, so the ladder settled it.
    const card = decision === undefined ? laddered(scan) : decided(decision)
    if (card !== null) cards.push(card)
  }

  for (const refusal of rejected) {
    if (answered.has(refusal.code)) continue
    const card = decided(decisions.get(refusal.code) ?? null)
    if (card !== null) cards.push(card)
  }

  return cards
}

/**
 * A card the user settled in the review sheet.
 *
 * A printing they picked is filed as `manual`, never as `exact`: `exact` says
 * the whole printed code matched one printing, and a slot written that way would
 * claim forever after that the scanner read a code it never read
 * (`specs/001-binder-mvp/addendum-batch-commit.md` §2). A card they kept with
 * its set still open is `by_name` — the card is known and the set is not, which
 * is precisely what `by_name` records.
 */
const decided = (decision: ReviewDecision | null): SlotCardBody | null => {
  if (decision === null || decision.kind === 'discarded') return null
  if (decision.printing === null) return withoutSet(decision.card.id)

  return {
    cardId: decision.card.id,
    cardPrintingId: decision.printing.id,
    setResolution: 'manual',
  }
}

/**
 * A card the review never asked about, filed on the rung the ladder reached it
 * on. This is the half of the rule that must *not* say `manual`: the scanner
 * really did match the code, and the slot has to keep saying which way.
 *
 * The two refusals below are for shapes the contract rules out — a scan the
 * review left alone has `outcome: 'resolved'`, which carries both a card and a
 * printing — but the response type does not. They are checked here because a
 * slot whose resolution and printing disagree fails the `binder_slots` CHECK,
 * and one rejected row rolls the entire sweep back.
 */
const laddered = (scan: ResolvedScan): SlotCardBody | null => {
  const { card, printing } = scan
  if (card === null) return null

  switch (scan.resolution) {
    case 'exact':
    case 'by_prefix_and_number':
    case 'by_number':
      if (printing === null) return null
      return { cardId: card.id, cardPrintingId: printing.id, setResolution: scan.resolution }
    case 'by_name':
    case 'unresolved':
      // The ladder determined no set, so the review did ask about this scan and
      // this rung is unreachable through the sheet. Filed as the card it names
      // with no set rather than dropped: the card was really there.
      return withoutSet(card.id)
  }
}

/**
 * A card whose set nobody determined. `by_name` is the rung that says so, and it
 * is one of the two that may not carry a printing — the CHECK in
 * `db/migrations/005_manual_set_resolution.sql` is the same equivalence in SQL.
 */
const withoutSet = (cardId: string): SlotCardBody => ({
  cardId,
  cardPrintingId: null,
  setResolution: 'by_name',
})
