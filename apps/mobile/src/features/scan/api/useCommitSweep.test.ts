import { act, renderHook } from '@testing-library/react-native'

import type { AddSlotsBody, SlotCardBody } from '@/features/binder/types'
import { forgetSession, rememberSession } from '@/lib/sessionStore'

import type { ResolvedScan, ScannedCard, ScannedPrinting } from '../types'
import type { ReviewDecision, ReviewRow } from '../useScanReview'

import type { ReviewedSweep } from './toSlotCards'
import { useCommitSweep } from './useCommitSweep'

const apiBaseUrl = 'https://api.binder.test'
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const blueEyes: ScannedCard = { id: 'blue-eyes', name: 'Blue-Eyes', imageObjectKey: null }
const sdk: ScannedPrinting = {
  id: 'sdk-001',
  cardId: 'blue-eyes',
  setCode: 'SDK-001',
  rarity: 'Common',
}

const settled = (code: string): ResolvedScan => ({
  code,
  resolution: 'exact',
  outcome: 'resolved',
  card: blueEyes,
  printing: { id: `printing-${code}`, cardId: 'blue-eyes', setCode: 'LOB', rarity: 'Common' },
  candidates: [],
})

const ambiguous = (code: string): ResolvedScan => ({
  code,
  resolution: 'unresolved',
  outcome: 'ambiguous',
  card: blueEyes,
  printing: null,
  candidates: [{ card: blueEyes, printing: sdk }],
})

const row = (code: string, decision: ReviewDecision | null): ReviewRow => ({
  code,
  reason: 'ambiguous',
  copies: 1,
  card: blueEyes,
  candidates: [],
  decision,
})

/** A sweep of two cards: one the ladder settled, one the user picked a printing for. */
const reviewedSweep = (): ReviewedSweep => ({
  resolved: [settled('LOB-001'), ambiguous('SMUDGE-1')],
  rejected: [],
  reviewed: [row('SMUDGE-1', { kind: 'kept', card: blueEyes, printing: sdk })],
})

const created = (): Response => new Response(JSON.stringify({ slots: [] }), { status: 201 })

/** What one request carried: the path it went to and the cards it sent. */
const requested = (call: number): { url: string; body: AddSlotsBody } => {
  const [url, init] = mockFetch.mock.calls[call] ?? []
  if (typeof url !== 'string') throw new Error(`no request number ${call} was made`)
  if (typeof init?.body !== 'string') throw new Error('the batch must carry a JSON body')

  const body: AddSlotsBody = JSON.parse(init.body)
  return { url, body }
}

const session = { token: 'session-token', expiresAt: '2099-01-01T00:00:00Z', userId: 'user-1' }

describe('useCommitSweep', () => {
  beforeEach(async () => {
    await forgetSession()
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(created()))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = apiBaseUrl
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('sends the whole sweep as one request, not one request per card', async () => {
    const { result } = await renderHook(() => useCommitSweep(reviewedSweep(), jest.fn()))

    await act(async () => {
      await result.current.send('binder-1')
    })

    expect(mockFetch).toHaveBeenCalledTimes(1)
    const { url, body } = requested(0)
    expect(url).toBe(`${apiBaseUrl}/binders/binder-1/slots/batch`)
    expect(body.cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: 'printing-LOB-001', setResolution: 'exact' },
      { cardId: 'blue-eyes', cardPrintingId: 'sdk-001', setResolution: 'manual' },
    ] satisfies SlotCardBody[])
  })

  it('carries the collector as the bearer token, through the one request helper', async () => {
    await rememberSession(session)
    const { result } = await renderHook(() => useCommitSweep(reviewedSweep(), jest.fn()))

    await act(async () => {
      await result.current.send('binder-1')
    })

    expect(mockFetch.mock.calls[0]?.[1]?.headers).toEqual({
      'Content-Type': 'application/json',
      Authorization: 'Bearer session-token',
    })
  })

  it('counts the cards it will file, the ones left out already gone', async () => {
    const sweep: ReviewedSweep = {
      resolved: [settled('LOB-001'), ambiguous('SMUDGE-1'), ambiguous('SMUDGE-2')],
      rejected: [],
      reviewed: [
        row('SMUDGE-1', { kind: 'kept', card: blueEyes, printing: sdk }),
        row('SMUDGE-2', { kind: 'discarded' }),
      ],
    }

    const { result } = await renderHook(() => useCommitSweep(sweep, jest.fn()))

    expect(result.current.cardCount).toBe(2)
    expect(result.current.status).toBe('idle')
  })

  it('says the binder has the sweep, and lets the session go, once it lands', async () => {
    const onFiled = jest.fn()
    const { result } = await renderHook(() => useCommitSweep(reviewedSweep(), onFiled))

    let filed = false
    await act(async () => {
      filed = await result.current.send('binder-1')
    })

    expect(filed).toBe(true)
    expect(result.current.status).toBe('filed')
    expect(onFiled).toHaveBeenCalledTimes(1)
  })

  describe('a commit that did not land', () => {
    it('says so, and does not tell the session the sweep is filed', async () => {
      mockFetch.mockImplementation(() => Promise.reject(new Error('the connection dropped')))
      const onFiled = jest.fn()
      const { result } = await renderHook(() => useCommitSweep(reviewedSweep(), onFiled))

      let filed = true
      await act(async () => {
        filed = await result.current.send('binder-1')
      })

      expect(filed).toBe(false)
      expect(result.current.status).toBe('failed')
      expect(onFiled).not.toHaveBeenCalled()
    })

    // The harm the server's transaction exists to prevent, reintroduced on the
    // client: a sweep the user has reviewed and lost to a bad signal is a page
    // of cards to scan again.
    it('keeps every decision, so the same batch is retried without a re-scan', async () => {
      mockFetch.mockImplementationOnce(() => Promise.reject(new Error('the connection dropped')))
      const { result } = await renderHook(() => useCommitSweep(reviewedSweep(), jest.fn()))

      await act(async () => {
        await result.current.send('binder-1')
      })
      expect(result.current.cardCount).toBe(2)

      await act(async () => {
        await result.current.send('binder-1')
      })

      expect(mockFetch).toHaveBeenCalledTimes(2)
      expect(requested(1).body).toEqual(requested(0).body)
      expect(result.current.status).toBe('filed')
    })

    it('reports a refused batch as a failure rather than as a filed sweep', async () => {
      mockFetch.mockImplementation(() =>
        Promise.resolve(new Response(JSON.stringify({}), { status: 422 })),
      )
      const { result } = await renderHook(() => useCommitSweep(reviewedSweep(), jest.fn()))

      await act(async () => {
        await result.current.send('binder-1')
      })

      expect(result.current.status).toBe('failed')
    })
  })

  it('commits a sweep whose every card was left out as an empty batch', async () => {
    const sweep: ReviewedSweep = {
      resolved: [ambiguous('SMUDGE-1')],
      rejected: [],
      reviewed: [row('SMUDGE-1', { kind: 'discarded' })],
    }
    const { result } = await renderHook(() => useCommitSweep(sweep, jest.fn()))
    expect(result.current.cardCount).toBe(0)

    await act(async () => {
      await result.current.send('binder-1')
    })

    expect(mockFetch).toHaveBeenCalledTimes(1)
    expect(requested(0).body.cards).toEqual([])
    expect(result.current.status).toBe('filed')
  })
})
