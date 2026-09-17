import { act, renderHook } from '@testing-library/react-native'

import type { RejectedScan, ResolvedScan, ScannedCard, ScannedPrinting } from './types'
import { useScanReview, type ScanReview } from './useScanReview'

const card = (id: string): ScannedCard => ({ id, name: `Card ${id}`, imageObjectKey: null })

const printing = (id: string, cardId: string): ScannedPrinting => ({
  id,
  cardId,
  setCode: 'LOB-EN001',
  rarity: 'Ultra Rare',
})

/** A card the name rung found without a set — the case US3 names. */
const byName = (code: string): ResolvedScan => ({
  code,
  resolution: 'by_name',
  card: card('blue-eyes'),
  printing: null,
  candidates: [],
})

/** A code nothing matched. */
const nothing = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  card: null,
  printing: null,
  candidates: [],
})

/** A scan the ladder settled: it must never reach the sheet. */
const settled = (code: string): ResolvedScan => ({
  code,
  resolution: 'exact',
  card: card('dark-magician'),
  printing: printing('lob-005', 'dark-magician'),
  candidates: [],
})

const renderReview = async (
  resolved: ResolvedScan[],
  rejected: RejectedScan[] = [],
): Promise<{ current: ScanReview }> => {
  const { result } = await renderHook(() => useScanReview(resolved, rejected))
  return result
}

describe('useScanReview', () => {
  it('has nothing to ask about when the sweep resolved cleanly', async () => {
    const review = await renderReview([settled('LOB-EN005')])

    expect(review.current.rows).toEqual([])
    expect(review.current.undecidedCount).toBe(0)
  })

  it('opens every flagged scan undecided', async () => {
    const review = await renderReview([byName('LOB-EN002'), nothing('SMUDGE')])

    expect(review.current.rows.map((row) => row.decision)).toEqual([null, null])
    expect(review.current.undecidedCount).toBe(2)
  })

  it('records the card the user confirmed, with the set still open', async () => {
    const review = await renderReview([byName('LOB-EN002')])

    await act(async () => {
      review.current.decide('LOB-EN002', { kind: 'kept', card: card('blue-eyes'), printing: null })
    })

    expect(review.current.rows[0]?.decision).toEqual({
      kind: 'kept',
      card: card('blue-eyes'),
      printing: null,
    })
    expect(review.current.undecidedCount).toBe(0)
  })

  it('records the printing the user picked out of the candidates', async () => {
    const review = await renderReview([byName('LOB-EN002')])
    const chosen = printing('lob-001', 'blue-eyes')

    await act(async () => {
      review.current.decide('LOB-EN002', {
        kind: 'kept',
        card: card('blue-eyes'),
        printing: chosen,
      })
    })

    expect(review.current.rows[0]?.decision).toEqual({
      kind: 'kept',
      card: card('blue-eyes'),
      printing: chosen,
    })
  })

  it('lets a scan that named no card be left out of the binder', async () => {
    const review = await renderReview([nothing('SMUDGE')])

    await act(async () => {
      review.current.decide('SMUDGE', { kind: 'discarded' })
    })

    expect(review.current.rows[0]?.decision).toEqual({ kind: 'discarded' })
    expect(review.current.undecidedCount).toBe(0)
  })

  it('settles every copy of one card with a single decision', async () => {
    const review = await renderReview([byName('LOB-EN002'), byName('LOB-EN002')])

    expect(review.current.rows).toHaveLength(1)
    expect(review.current.rows[0]?.copies).toBe(2)

    await act(async () => {
      review.current.decide('LOB-EN002', { kind: 'kept', card: card('blue-eyes'), printing: null })
    })

    expect(review.current.undecidedCount).toBe(0)
  })

  it('leaves the other flagged scans alone when one is decided', async () => {
    const review = await renderReview([byName('LOB-EN002'), nothing('SMUDGE')])

    await act(async () => {
      review.current.decide('SMUDGE', { kind: 'discarded' })
    })

    expect(review.current.rows[0]?.decision).toBeNull()
    expect(review.current.undecidedCount).toBe(1)
  })

  it('replaces a decision when the user answers the same scan again', async () => {
    const review = await renderReview([byName('LOB-EN002')])

    await act(async () => {
      review.current.decide('LOB-EN002', { kind: 'discarded' })
    })
    await act(async () => {
      review.current.decide('LOB-EN002', { kind: 'kept', card: card('blue-eyes'), printing: null })
    })

    expect(review.current.rows[0]?.decision).toEqual({
      kind: 'kept',
      card: card('blue-eyes'),
      printing: null,
    })
    expect(review.current.undecidedCount).toBe(0)
  })

  it('puts a settled scan back in question, so a wrong tap is not permanent', async () => {
    const review = await renderReview([byName('LOB-EN002')])

    await act(async () => {
      review.current.decide('LOB-EN002', { kind: 'discarded' })
    })
    await act(async () => {
      review.current.reopen('LOB-EN002')
    })

    expect(review.current.rows[0]?.decision).toBeNull()
    expect(review.current.undecidedCount).toBe(1)
  })

  // The sheet can be open while the queue is still draining, so the rows have
  // to be derived rather than copied into state when the hook first ran.
  it('picks up a scan flagged after the review was already open', async () => {
    const answers: ResolvedScan[] = [byName('LOB-EN002')]
    const { result, rerender } = await renderHook(() => useScanReview(answers, []))

    await act(async () => {
      result.current.decide('LOB-EN002', { kind: 'discarded' })
    })
    answers.push(nothing('SMUDGE'))
    await act(async () => {
      rerender(undefined)
    })

    expect(result.current.rows.map((row) => row.code)).toEqual(['LOB-EN002', 'SMUDGE'])
    expect(result.current.undecidedCount).toBe(1)
  })
})
