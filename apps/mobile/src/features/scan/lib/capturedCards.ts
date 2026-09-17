import type { RejectedScan, ResolvedScan } from '../types'

/** What the resolver has made of one captured card so far. */
export type CaptureStatus =
  /** Queued, or in flight — the resolver has not answered about it yet. */
  | 'pending'
  /** The ladder named a card. */
  | 'matched'
  /** The resolver answered and no card matched the code. */
  | 'unmatched'
  /** The resolver refused the scan — a 4xx, so sending it again would be refused again. */
  | 'refused'

export interface CapturedCard {
  /**
   * Stable per capture. The index is part of it because the code is not
   * unique: capturing the same code twice is a collector's second copy, and
   * keying on the code alone would collapse the two rows into one.
   */
  key: string
  code: string
  status: CaptureStatus
  /** The card's name once the ladder named it, and null until then. */
  name: string | null
}

/** An answer decides the status; without one, only a refusal does. */
const statusOf = (answer: ResolvedScan | undefined, isRefused: boolean): CaptureStatus => {
  if (answer === undefined) return isRefused ? 'refused' : 'pending'
  return answer.card === null ? 'unmatched' : 'matched'
}

/**
 * Joins the codes the sweep captured with the answers the resolver has sent
 * back so far, newest capture first.
 *
 * Answers are matched to captures **by code, taking the first answer for that
 * code**. Two captures of one code are two physical cards but one question —
 * they send the same request to a resolver that answers it the same way — so
 * pairing them off in order would be bookkeeping for a distinction that cannot
 * exist. The order is newest-first because the strip's job is to confirm the
 * card that was just swept past, and that one has to be the one on screen.
 */
export const capturedCards = (
  codes: readonly string[],
  resolved: readonly ResolvedScan[],
  rejected: readonly RejectedScan[],
): CapturedCard[] => {
  const answers = new Map<string, ResolvedScan>()
  for (const scan of resolved) if (!answers.has(scan.code)) answers.set(scan.code, scan)

  const refusals = new Set(rejected.map((scan) => scan.code))

  return codes
    .map((code, index): CapturedCard => {
      const answer = answers.get(code)
      return {
        key: `${index}:${code}`,
        code,
        status: statusOf(answer, refusals.has(code)),
        name: answer?.card?.name ?? null,
      }
    })
    .reverse()
}
