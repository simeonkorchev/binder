import type { RejectedScan, ResolvedScan, ScanCandidate, ScannedCard } from '../types'

/**
 * Why the ladder could not settle a scan. These are the four things the review
 * sheet exists to put in front of the user (US3): every one of them is a card
 * that would otherwise land in a binder as a guess, or not land at all.
 */
export type FlagReason =
  /** The resolver refused the scan, so no card was ever looked up for it. */
  | 'refused'
  /** The resolver answered and nothing matched: no card, no candidates. */
  | 'unresolved'
  /** Several cards or printings matched and the ladder refused to pick. */
  | 'ambiguous'
  /**
   * A card, but no set. The name rung identified the card and the printed code
   * told it nothing — the case the brief calls out, because a binder slot with
   * no printing is a card whose set nobody will ever recover from the shelf.
   */
  | 'no_set'

/** One scan the sweep could not settle, as the review sheet asks about it. */
export interface FlaggedScan {
  /** The printed code the scanner read. Unique in the list — one question per code. */
  code: string
  reason: FlagReason
  /**
   * How many copies of this code the resolver has answered about. A capture
   * still waiting in the queue is not here yet, which is why the sheet is
   * opened after the sweep rather than during it.
   */
  copies: number
  /** The card the ladder named, or null when it named none. */
  card: ScannedCard | null
  /** What the ladder could not choose between. Empty unless `ambiguous`. */
  candidates: ScanCandidate[]
}

/**
 * Reads one answer's flag, or null when the ladder settled it.
 *
 * `outcome` is the discriminator, and `resolution` deliberately is not:
 * `internal/card/service/match.go` answers `unresolved` both when several rows
 * matched and when none did, which are opposite situations for the user — one
 * is a choice, the other is a search. The API carries `outcome` to say which,
 * and its own description says to read it rather than `resolution`.
 *
 * This used to infer the same thing from the response's shape. That worked,
 * but it meant the rule lived in two places and only one of them was the
 * contract.
 */
const flagReason = (scan: ResolvedScan): FlagReason | null => {
  switch (scan.outcome) {
    case 'resolved':
      return null
    case 'ambiguous':
      return 'ambiguous'
    case 'card_only':
      return 'no_set'
    case 'no_match':
      return 'unresolved'
  }
}

/**
 * Gathers everything the sweep could not settle, oldest first.
 *
 * One row per **code**, not per capture. Two captures of one code are two
 * physical cards but one question — they send the same request to a resolver
 * that answers it the same way — so the user decides once and `copies` says how
 * many cards that decision covers. `capturedCards` joins the strip on the same
 * rule, for the same reason.
 *
 * A refusal is a row like any other: `useResolveScan` keeps refused scans
 * precisely so the card the user swept past is never silently gone, and a list
 * nobody shows would be the same thing as dropping it. An answer outranks a
 * refusal for the same code — the resolver did eventually say something about
 * that card.
 */
export const flaggedScans = (
  resolved: readonly ResolvedScan[],
  rejected: readonly RejectedScan[],
): FlaggedScan[] => {
  const flagged = new Map<string, FlaggedScan>()

  const add = (row: FlaggedScan): void => {
    const already = flagged.get(row.code)
    flagged.set(row.code, already === undefined ? row : { ...already, copies: already.copies + 1 })
  }

  for (const scan of resolved) {
    const reason = flagReason(scan)
    if (reason === null) continue
    add({
      code: scan.code,
      reason,
      copies: 1,
      card: scan.card,
      candidates: scan.candidates,
    })
  }

  const answered = new Set(resolved.map((scan) => scan.code))
  for (const scan of rejected) {
    if (answered.has(scan.code)) continue
    add({ code: scan.code, reason: 'refused', copies: 1, card: null, candidates: [] })
  }

  return [...flagged.values()]
}
