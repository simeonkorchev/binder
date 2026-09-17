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
 * **The resolution field cannot be this discriminator**, and that is the trap
 * worth stating: `internal/card/service/match.go` returns `unresolved` for two
 * completely different outcomes — a rung that matched *several* printings
 * (candidates, and the card too when they are all printings of one card) and a
 * ladder that matched *nothing at all*. The user's next move differs entirely
 * between them: one is a choice, the other is a search. What separates them is
 * `candidates`, so that is what is read first.
 *
 * After that it is the shape, not the rung: no card is unresolved, and a card
 * with no printing is the `by_name` case.
 */
const flagReason = (scan: ResolvedScan): FlagReason | null => {
  if (scan.candidates.length > 0) return 'ambiguous'
  if (scan.card === null) return 'unresolved'
  if (scan.printing === null) return 'no_set'
  return null
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
