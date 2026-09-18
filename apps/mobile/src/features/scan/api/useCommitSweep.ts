import { useState } from 'react'

import type { AddSlotsBody, AddSlotsResult } from '@/features/binder/types'
import { writeJson } from '@/lib/apiRequest'

import { toSlotCards, type ReviewedSweep } from './toSlotCards'

/** Where one attempt at filing the sweep has got to. */
type CommitStatus =
  /** Nothing has been sent yet, or the last attempt was abandoned. */
  | 'idle'
  /** The batch is in flight. */
  | 'sending'
  /** The binder has the whole sweep. */
  | 'filed'
  /**
   * The batch did not land. The server wrote nothing — the endpoint is one
   * transaction — and nothing here was thrown away either.
   */
  | 'failed'

export interface SweepCommit {
  status: CommitStatus
  /**
   * How many cards the commit will file: one per physical card still being kept,
   * with the ones left out already gone. Zero is a real commit — a sweep where
   * every card was left out — and the endpoint answers it with an empty binder
   * page rather than an error.
   */
  cardCount: number
  /**
   * Sends the whole sweep to one binder as **one** request, and answers whether
   * the binder has it.
   */
  send: (binderId: string) => Promise<boolean>
}

/**
 * Files a reviewed sweep into a binder in one request.
 *
 * One request, because the endpoint is one transaction: a bad card, a taken
 * position or a dropped connection rolls the whole batch back, so the binder is
 * never left holding the prefix of a sweep the user has already put away. A loop
 * over the single-slot write would be exactly that prefix, which is why there is
 * no loop here (`specs/001-binder-mvp/addendum-batch-commit.md` §3).
 *
 * **A failure throws nothing away.** The cards are derived from the session's
 * decisions on every render, never copied into state here, so a failed attempt
 * leaves the review sheet holding every answer the user gave: pressing the same
 * binder again sends the same batch, and nothing has to be scanned twice. That
 * is the client half of the server's transaction — an all-or-nothing write whose
 * client loses the sweep on a bad signal is still a lost sweep.
 *
 * `onFiled` runs once the binder has the cards, and is where the session lets
 * the sweep go: a sweep that stayed in the strip after being filed could be
 * filed a second time.
 */
export const useCommitSweep = (sweep: ReviewedSweep, onFiled: () => void): SweepCommit => {
  const [status, setStatus] = useState<CommitStatus>('idle')
  const cards = toSlotCards(sweep)

  return {
    status,
    cardCount: cards.length,
    send: async (binderId: string): Promise<boolean> => {
      setStatus('sending')
      try {
        // The answer is one slot per card sent, which the binder page reads for
        // itself when the user lands on it — nothing here needs it.
        await writeJson<AddSlotsResult>('POST', `/binders/${binderId}/slots/batch`, {
          cards,
        } satisfies AddSlotsBody)
      } catch {
        setStatus('failed')
        return false
      }

      setStatus('filed')
      onFiled()
      return true
    },
  }
}
