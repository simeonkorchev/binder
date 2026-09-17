import { act, renderHook, waitFor } from '@testing-library/react-native'

import type { ScanMatchBody } from '../types'

import { useResolveScan, type ResolveScanQueue } from './useResolveScan'

const apiBaseUrl = 'https://api.binder.test'
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

/** The code a queued request carried, read back out of its JSON body. */
const codeOf = (init: RequestInit | undefined): string => {
  const body = init?.body
  if (typeof body !== 'string') throw new Error('a scan request must carry a JSON body')
  const parsed: { code: string } = JSON.parse(body)
  return parsed.code
}

const matchFor = (code: string): ScanMatchBody => ({
  resolution: 'exact',
  outcome: 'resolved',
  card: { id: `card-${code}`, name: `Card ${code}`, imageObjectKey: null },
  printing: { id: `printing-${code}`, cardId: `card-${code}`, setCode: code, rarity: 'Common' },
  candidates: [],
})

const jsonResponse = (body: ScanMatchBody): Response =>
  new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })

/** The resolver, working: every scan gets the match for the code it carried. */
const resolverAnswers = async (
  _input: RequestInfo | URL,
  init?: RequestInit,
): Promise<Response> => jsonResponse(matchFor(codeOf(init)))

/** A request that has left but not landed, so the queue can be caught mid-flight. */
const inFlight = (): { response: Promise<Response>; land: (body: ScanMatchBody) => void } => {
  let settle: (response: Response) => void = () => undefined
  const response = new Promise<Response>((resolve) => {
    settle = resolve
  })
  return { response, land: (body) => settle(jsonResponse(body)) }
}

const renderQueue = async (): Promise<{ current: ResolveScanQueue }> => {
  const { result } = await renderHook(() => useResolveScan())
  return result
}

const scan = async (queue: ResolveScanQueue, code: string): Promise<void> => {
  await act(async () => {
    queue.resolve(code)
  })
}

describe('useResolveScan', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(resolverAnswers)
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = apiBaseUrl
  })

  describe('the scan loop never waits', () => {
    it('queues a scan and returns before the resolver has answered', async () => {
      const flight = inFlight()
      mockFetch.mockReturnValueOnce(flight.response)
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')

      expect(queue.current.queuedCount).toBe(1)
      expect(queue.current.resolved).toEqual([])
    })

    it('asks about one scan at a time, so a slow answer does not fan out', async () => {
      const flight = inFlight()
      mockFetch.mockReturnValueOnce(flight.response)
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')
      await scan(queue.current, 'LOB-EN005')

      expect(mockFetch).toHaveBeenCalledTimes(1)
      expect(queue.current.queuedCount).toBe(2)

      await act(async () => {
        flight.land(matchFor('LOB-EN001'))
      })

      await waitFor(() => expect(queue.current.queuedCount).toBe(0))
      expect(mockFetch).toHaveBeenCalledTimes(2)
    })
  })

  describe('resolving', () => {
    it('posts the scanned code to the resolver', async () => {
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')

      await waitFor(() => expect(queue.current.resolved).toHaveLength(1))
      expect(mockFetch).toHaveBeenCalledWith(
        `${apiBaseUrl}/scans/resolve`,
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ code: 'LOB-EN001' }),
        }),
      )
    })

    it('keeps the answers in the order the cards were swept', async () => {
      const queue = await renderQueue()

      for (const code of ['LOB-EN001', 'LOB-EN005', 'SDK-001']) {
        await scan(queue.current, code)
      }

      await waitFor(() => expect(queue.current.resolved).toHaveLength(3))
      expect(queue.current.resolved.map((resolved) => resolved.code)).toEqual([
        'LOB-EN001',
        'LOB-EN005',
        'SDK-001',
      ])
      expect(queue.current.resolved[0]?.card?.name).toBe('Card LOB-EN001')
      expect(queue.current.isOffline).toBe(false)
    })
  })

  describe('a dropped connection', () => {
    it('keeps the scan queued and says the resolver is out of reach', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network request failed'))
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')

      await waitFor(() => expect(queue.current.isOffline).toBe(true))
      expect(queue.current.queuedCount).toBe(1)
      expect(queue.current.resolved).toEqual([])
      expect(queue.current.rejected).toEqual([])
    })

    it('resolves the scan it was holding when the next card is swept', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network request failed'))
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')
      await waitFor(() => expect(queue.current.isOffline).toBe(true))

      await scan(queue.current, 'LOB-EN005')

      await waitFor(() => expect(queue.current.resolved).toHaveLength(2))
      expect(queue.current.resolved.map((resolved) => resolved.code)).toEqual([
        'LOB-EN001',
        'LOB-EN005',
      ])
      expect(queue.current.queuedCount).toBe(0)
      expect(queue.current.isOffline).toBe(false)
    })

    it('resolves what it was holding on retry, with no further scan', async () => {
      mockFetch.mockRejectedValueOnce(new Error('Network request failed'))
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')
      await waitFor(() => expect(queue.current.isOffline).toBe(true))

      await act(async () => {
        queue.current.retryQueued()
      })

      await waitFor(() => expect(queue.current.resolved).toHaveLength(1))
      expect(queue.current.resolved[0]?.code).toBe('LOB-EN001')
      expect(queue.current.isOffline).toBe(false)
    })

    it('holds a scan the server broke on, because the trip can be made again', async () => {
      mockFetch.mockResolvedValueOnce(new Response('', { status: 502 }))
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')

      await waitFor(() => expect(queue.current.isOffline).toBe(true))
      expect(queue.current.queuedCount).toBe(1)
      expect(queue.current.rejected).toEqual([])
    })

    it('holds the queue when the API URL was never configured', async () => {
      delete process.env.EXPO_PUBLIC_API_URL
      const queue = await renderQueue()

      await scan(queue.current, 'LOB-EN001')

      await waitFor(() => expect(queue.current.isOffline).toBe(true))
      expect(queue.current.queuedCount).toBe(1)
      expect(mockFetch).not.toHaveBeenCalled()
    })
  })

  describe('a scan the resolver refused', () => {
    it('records the refusal with the code that was read', async () => {
      mockFetch.mockResolvedValueOnce(new Response('', { status: 422 }))
      const queue = await renderQueue()

      await scan(queue.current, 'ATK-2500')

      await waitFor(() => expect(queue.current.rejected).toHaveLength(1))
      expect(queue.current.rejected[0]).toEqual({ code: 'ATK-2500', status: 422 })
      expect(queue.current.isOffline).toBe(false)
    })

    it('does not wedge the queue behind it', async () => {
      mockFetch.mockResolvedValueOnce(new Response('', { status: 422 }))
      const queue = await renderQueue()

      await scan(queue.current, 'ATK-2500')
      await scan(queue.current, 'LOB-EN001')

      await waitFor(() => expect(queue.current.resolved).toHaveLength(1))
      expect(queue.current.resolved[0]?.code).toBe('LOB-EN001')
      expect(queue.current.queuedCount).toBe(0)
    })
  })
})
