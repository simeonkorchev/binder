import { act, renderHook, waitFor } from '@testing-library/react-native'
import type { Frame } from 'react-native-vision-camera'

import type { SlotCardBody } from '@/features/binder/types'

import type { ScanMatchBody } from './types'
import { absentReadsToLeaveFrame, stableReadsRequired } from './lib/useStableRead'
import { testFrame } from './testFrame'
import { useScanSession, type ScanSession } from './useScanSession'

const mockScanText = jest.fn<{ resultText: string }, [Frame]>()
const mockCaptureTick = jest.fn<Promise<void>, []>()

jest.mock('react-native-vision-camera', () => ({
  useFrameProcessor: (frameProcessor: (frame: Frame) => void) => ({
    frameProcessor,
    type: 'readonly',
  }),
  runAtTargetFps: (_fps: number, work: () => void) => {
    work()
  },
}))

jest.mock('react-native-vision-camera-text-recognition', () => ({
  useTextRecognition: () => ({ scanText: mockScanText }),
}))

jest.mock('react-native-worklets-core', () => ({
  useRunOnJS: (callback: (text: string | null) => void) => callback,
}))

jest.mock('./lib/captureTick', () => ({
  captureTick: () => mockCaptureTick(),
}))

const apiBaseUrl = 'https://api.binder.test'
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const matchFor = (code: string): ScanMatchBody => ({
  resolution: 'exact',
  outcome: 'resolved',
  card: { id: `card-${code}`, name: `Card ${code}`, imageObjectKey: null },
  printing: { id: `printing-${code}`, cardId: `card-${code}`, setCode: 'LOB', rarity: 'Common' },
  candidates: [],
})

const codeOf = (init: RequestInit | undefined): string => {
  const body = init?.body
  if (typeof body !== 'string') throw new Error('a scan request must carry a JSON body')
  const parsed: { code: string } = JSON.parse(body)
  return parsed.code
}

const resolverAnswers = async (_input: RequestInfo | URL, init?: RequestInit): Promise<Response> =>
  new Response(JSON.stringify(matchFor(codeOf(init))), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })

/** The OCR text a frame of a card in the guide frame carries. */
const cardInFrame = (code: string): string => `Blue-Eyes White Dragon\nATK/3000 DEF/2500\n${code}`

/** Nothing the camera can read — the wall between two cards on the table. */
const blankFrame = null

/**
 * Plays a run of frames through the camera, one text per processed frame and
 * `null` for a frame ML Kit found nothing in.
 */
const sweep = async (
  session: { current: ScanSession },
  frames: readonly (string | null)[],
): Promise<void> => {
  for (const text of frames) {
    mockScanText.mockReturnValue({ resultText: text ?? '' })
    await act(async () => {
      session.current.frameProcessor.frameProcessor(testFrame())
    })
  }
}

const repeat = (text: string | null, times: number): (string | null)[] =>
  Array.from({ length: times }, () => text)

const renderSession = async (): Promise<{ current: ScanSession }> => {
  const { result } = await renderHook(() => useScanSession())
  return result
}

describe('useScanSession', () => {
  beforeEach(() => {
    mockScanText.mockReset()
    mockCaptureTick.mockReset()
    mockCaptureTick.mockResolvedValue(undefined)
    mockFetch.mockReset()
    mockFetch.mockImplementation(resolverAnswers)
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = apiBaseUrl
  })

  it('starts with nothing captured', async () => {
    const session = await renderSession()

    expect(session.current.captured.length).toBe(0)
    expect(session.current.captured).toEqual([])
  })

  it('captures a card once it has been read enough times in a row', async () => {
    const session = await renderSession()

    await sweep(session, repeat(cardInFrame('LOB-EN001'), stableReadsRequired))

    expect(session.current.captured.length).toBe(1)
    expect(session.current.captured[0]?.code).toBe('LOB-EN001')
  })

  it('does not capture a card read fewer times than the rule requires', async () => {
    const session = await renderSession()

    await sweep(session, repeat(cardInFrame('LOB-EN001'), stableReadsRequired - 1))

    expect(session.current.captured.length).toBe(0)
  })

  it('captures a card held under the lens once, however long it is held there', async () => {
    const session = await renderSession()

    await sweep(session, repeat(cardInFrame('LOB-EN001'), stableReadsRequired * 4))

    expect(session.current.captured.length).toBe(1)
  })

  it('captures a second copy met after the first left the frame', async () => {
    const session = await renderSession()
    const card = cardInFrame('LOB-EN001')

    await sweep(session, [
      ...repeat(card, stableReadsRequired),
      ...repeat(blankFrame, absentReadsToLeaveFrame),
      ...repeat(card, stableReadsRequired),
    ])

    expect(session.current.captured.length).toBe(2)
    expect(session.current.captured.map((entry) => entry.code)).toEqual([
      'LOB-EN001',
      'LOB-EN001',
    ])
  })

  it('does not split one card into two when the blank frames are too few to leave the frame', async () => {
    const session = await renderSession()
    const card = cardInFrame('LOB-EN001')

    await sweep(session, [
      ...repeat(card, stableReadsRequired),
      ...repeat(blankFrame, absentReadsToLeaveFrame - 1),
      ...repeat(card, stableReadsRequired),
    ])

    expect(session.current.captured.length).toBe(1)
  })

  it('captures the next card of a sweep across a page', async () => {
    const session = await renderSession()

    await sweep(session, [
      ...repeat(cardInFrame('LOB-EN001'), stableReadsRequired),
      ...repeat(cardInFrame('SDK-002'), stableReadsRequired),
    ])

    expect(session.current.captured.map((entry) => entry.code)).toEqual(['SDK-002', 'LOB-EN001'])
  })

  it('ignores frames whose text holds no code at all', async () => {
    const session = await renderSession()

    await sweep(session, repeat('ATK/2500 DEF/2100', stableReadsRequired * 2))

    expect(session.current.captured.length).toBe(0)
  })

  it('ticks the phone once per captured card', async () => {
    const session = await renderSession()

    await sweep(session, repeat(cardInFrame('LOB-EN001'), stableReadsRequired * 3))

    expect(mockCaptureTick).toHaveBeenCalledTimes(1)
  })

  it('names a captured card once the resolver has answered', async () => {
    const session = await renderSession()

    await sweep(session, repeat(cardInFrame('LOB-EN001'), stableReadsRequired))

    await waitFor(() => {
      expect(session.current.captured[0]?.name).toBe('Card LOB-EN001')
    })
    expect(session.current.captured[0]?.status).toBe('matched')
  })

  it('keeps the card and says so when the resolver could not be reached', async () => {
    mockFetch.mockRejectedValue(new Error('the network is gone'))
    const session = await renderSession()

    await sweep(session, repeat(cardInFrame('LOB-EN001'), stableReadsRequired))

    await waitFor(() => {
      expect(session.current.isOffline).toBe(true)
    })
    expect(session.current.captured.length).toBe(1)
    expect(session.current.captured[0]?.status).toBe('pending')
  })

  it('resolves what was waiting once the connection is retried', async () => {
    mockFetch.mockRejectedValueOnce(new Error('the network is gone'))
    const session = await renderSession()

    await sweep(session, repeat(cardInFrame('LOB-EN001'), stableReadsRequired))
    await waitFor(() => {
      expect(session.current.isOffline).toBe(true)
    })

    await act(async () => {
      session.current.retryPending()
    })

    await waitFor(() => {
      expect(session.current.captured[0]?.status).toBe('matched')
    })
    expect(session.current.isOffline).toBe(false)
  })

  describe('filing the sweep', () => {
    /** The resolver for the scans, and the binder for the one batch that follows. */
    const resolverAndBinder = async (
      input: RequestInfo | URL,
      init?: RequestInit,
    ): Promise<Response> => {
      if (String(input).endsWith('/slots/batch')) {
        return new Response(JSON.stringify({ slots: [] }), { status: 201 })
      }
      return resolverAnswers(input, init)
    }

    const batchBody = (): { cards: SlotCardBody[] } => {
      const call = mockFetch.mock.calls.find(([input]) => String(input).endsWith('/slots/batch'))
      const body = call?.[1]?.body
      if (typeof body !== 'string') throw new Error('no batch was sent')

      const parsed: { cards: SlotCardBody[] } = JSON.parse(body)
      return parsed
    }

    const sweptPage = async (): Promise<{ current: ScanSession }> => {
      mockFetch.mockImplementation(resolverAndBinder)
      const session = await renderSession()

      await sweep(session, [
        ...repeat(cardInFrame('LOB-EN001'), stableReadsRequired),
        ...repeat(cardInFrame('SDK-002'), stableReadsRequired),
      ])
      await waitFor(() => {
        expect(session.current.commit.cardCount).toBe(2)
      })

      return session
    }

    it('sends every card the sweep resolved, on the rung the ladder answered with', async () => {
      const session = await sweptPage()

      await act(async () => {
        await session.current.commit.send('binder-1')
      })

      expect(batchBody().cards).toEqual([
        { cardId: 'card-LOB-EN001', cardPrintingId: 'printing-LOB-EN001', setResolution: 'exact' },
        { cardId: 'card-SDK-002', cardPrintingId: 'printing-SDK-002', setResolution: 'exact' },
      ] satisfies SlotCardBody[])
    })

    // The cards are in a binder now. A sweep left in the session after it was
    // filed is a sweep the next press files a second time.
    it('lets the sweep go once the binder has it', async () => {
      const session = await sweptPage()

      await act(async () => {
        await session.current.commit.send('binder-1')
      })

      expect(session.current.captured).toEqual([])
      expect(session.current.commit.cardCount).toBe(0)
      expect(session.current.review.rows).toEqual([])
    })

    it('keeps the sweep when the commit did not land, so nothing is scanned twice', async () => {
      const session = await sweptPage()
      mockFetch.mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) =>
        String(input).endsWith('/slots/batch')
          ? Promise.reject(new Error('the connection dropped'))
          : resolverAnswers(input, init),
      )

      await act(async () => {
        await session.current.commit.send('binder-1')
      })

      expect(session.current.commit.status).toBe('failed')
      expect(session.current.commit.cardCount).toBe(2)
      expect(session.current.captured.map((entry) => entry.code)).toEqual([
        'SDK-002',
        'LOB-EN001',
      ])
    })
  })
})
