import { act, renderHook, waitFor } from '@testing-library/react-native'

import type { ListedCard } from '../types'

import { useListings, type ListingsBrowse } from './useListings'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const listed = (over: Partial<ListedCard> = {}): ListedCard => ({
  listingId: 'listing-1',
  sellerId: 'seller-1',
  cardId: 'card-1',
  cardName: 'Dark Magician',
  imageObjectKey: null,
  setCode: 'LOB',
  listedAt: '2026-09-17T10:00:00Z',
  ...over,
})

/** A `Response` body reads once, so every answer is built fresh per call. */
const feed = (...listings: ListedCard[]): Response =>
  new Response(JSON.stringify({ listings }), { status: 200 })

/** The path the hook asked for, read back off the URL it called. */
const askedFor = (call: number): string => {
  const url = mockFetch.mock.calls[call]?.[0]
  if (typeof url !== 'string') throw new Error('the browse must call a URL string')
  return url.replace('https://api.binder.test', '')
}

const renderBrowse = async (): Promise<{ current: ListingsBrowse }> => {
  const { result } = await renderHook(() => useListings())
  await waitFor(() => {
    expect(result.current.state.status).not.toBe('loading')
  })
  return result
}

describe('useListings', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(feed(listed())))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('opens on the whole market, unfiltered', async () => {
    const browse = await renderBrowse()

    expect(browse.current.state).toEqual({
      status: 'ready',
      listings: [listed()],
      filter: { query: '', setCode: '' },
    })
    expect(askedFor(0)).toBe('/listings')
  })

  // The backend goes out of its way to answer `200 {"listings":[]}` here rather
  // than a 404 or a null, and this is the test that the client treats it as the
  // ordinary answer it is (000-principles.md §8b).
  it('reads a market with nothing for sale as an empty feed, not a failure', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(feed()))

    const browse = await renderBrowse()

    expect(browse.current.state).toEqual({
      status: 'ready',
      listings: [],
      filter: { query: '', setCode: '' },
    })
  })

  it('carries the filter back with an empty result, so "no match" is not "nothing for sale"', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(feed()))

    const browse = await renderBrowse()
    await act(async () => {
      browse.current.browse({ query: 'Kuriboh', setCode: 'LOB' })
    })

    await waitFor(() => {
      expect(browse.current.state).toEqual({
        status: 'ready',
        listings: [],
        filter: { query: 'Kuriboh', setCode: 'LOB' },
      })
    })
    expect(askedFor(1)).toBe('/listings?q=Kuriboh&set=LOB')
  })

  it('reports a failed read as a failure rather than as an empty market', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 500 })))

    const browse = await renderBrowse()

    expect(browse.current.state).toEqual({ status: 'error' })
  })

  it('reads the same filter again when a failed read is retried', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 500 })))

    const browse = await renderBrowse()
    await act(async () => {
      browse.current.browse({ query: 'Kuriboh', setCode: '' })
    })
    await waitFor(() => {
      expect(askedFor(1)).toBe('/listings?q=Kuriboh')
    })

    await act(async () => {
      browse.current.reload()
    })

    await waitFor(() => {
      expect(askedFor(2)).toBe('/listings?q=Kuriboh')
    })
  })

  it('is loading again while a new filter is in flight', async () => {
    const browse = await renderBrowse()
    // The answer is held back deliberately: "loading" is the state between the
    // question and the answer, and a mock that resolves at once has no between.
    let answer = (): void => {}
    mockFetch.mockImplementation(
      () =>
        new Promise<Response>((resolve) => {
          answer = (): void => resolve(feed())
        }),
    )

    await act(async () => {
      browse.current.browse({ query: 'Kuriboh', setCode: '' })
    })

    expect(browse.current.state).toEqual({ status: 'loading' })

    await act(async () => {
      answer()
    })

    expect(browse.current.state.status).toBe('ready')
  })
})
