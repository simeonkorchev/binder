import { act, renderHook, waitFor } from '@testing-library/react-native'

import type { ScannedCard, SearchCardsBody } from '../types'

import { useCardSearch, type CardSearch } from './useCardSearch'

const apiBaseUrl = 'https://api.binder.test'
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const card = (name: string): ScannedCard => ({ id: name, name, imageObjectKey: null })

/**
 * A `Response` body reads once, so every answer is built fresh per call rather
 * than handed out from `mockResolvedValue`.
 */
const found = (...cards: ScannedCard[]): Response =>
  new Response(JSON.stringify({ cards } satisfies SearchCardsBody), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })

/** The query string the hook sent, read back off the URL it called. */
const queriedFor = (call: number): string => {
  const url = mockFetch.mock.calls[call]?.[0]
  if (typeof url !== 'string') throw new Error('the card search must call a URL string')
  return new URL(url).searchParams.get('q') ?? ''
}

const renderSearch = async (): Promise<{ current: CardSearch }> => {
  const { result } = await renderHook(() => useCardSearch())
  return result
}

const searchFor = async (search: CardSearch, query: string): Promise<void> => {
  await act(async () => {
    search.search(query)
  })
}

describe('useCardSearch', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(found(card('Dark Magician'))))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = apiBaseUrl
  })

  it('has asked nothing before the user searches', async () => {
    const search = await renderSearch()

    expect(search.current.results).toEqual([])
    expect(search.current.hasSearched).toBe(false)
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('asks the card search for the name the user typed', async () => {
    const search = await renderSearch()

    await searchFor(search.current, 'Dark Magician')

    await waitFor(() => expect(search.current.results).toEqual([card('Dark Magician')]))
    expect(queriedFor(0)).toBe('Dark Magician')
  })

  it('escapes a name the query string would otherwise cut short', async () => {
    const search = await renderSearch()

    await searchFor(search.current, 'Gate Guardian & Co')

    expect(queriedFor(0)).toBe('Gate Guardian & Co')
  })

  it('ignores a blank query rather than ask for the whole database', async () => {
    const search = await renderSearch()

    await searchFor(search.current, '   ')

    expect(mockFetch).not.toHaveBeenCalled()
    expect(search.current.isSearching).toBe(false)
  })

  it('searches for the trimmed name', async () => {
    const search = await renderSearch()

    await searchFor(search.current, '  Dark Magician  ')

    expect(queriedFor(0)).toBe('Dark Magician')
  })

  it('says it is searching until the answer lands', async () => {
    let land: (response: Response) => void = () => undefined
    mockFetch.mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        land = resolve
      }),
    )
    const search = await renderSearch()

    await searchFor(search.current, 'Dark')
    expect(search.current.isSearching).toBe(true)

    await act(async () => {
      land(found(card('Dark Magician')))
    })

    expect(search.current.isSearching).toBe(false)
    expect(search.current.hasSearched).toBe(true)
  })

  it('tells a search that found nothing from one that was never run', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(found()))
    const search = await renderSearch()

    await searchFor(search.current, 'Nothing At All')

    await waitFor(() => expect(search.current.hasSearched).toBe(true))
    expect(search.current.results).toEqual([])
    expect(search.current.hasFailed).toBe(false)
  })

  it('reports a dropped connection instead of an empty shelf', async () => {
    mockFetch.mockImplementation(() => Promise.reject(new Error('network down')))
    const search = await renderSearch()

    await searchFor(search.current, 'Dark Magician')

    await waitFor(() => expect(search.current.hasFailed).toBe(true))
    expect(search.current.isSearching).toBe(false)
  })

  it('reports a server that refused the search the same way', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('nope', { status: 500 })))
    const search = await renderSearch()

    await searchFor(search.current, 'Dark Magician')

    await waitFor(() => expect(search.current.hasFailed).toBe(true))
  })

  it('clears an earlier failure when the next search is asked for', async () => {
    mockFetch.mockImplementationOnce(() => Promise.reject(new Error('network down')))
    const search = await renderSearch()

    await searchFor(search.current, 'Dark Magician')
    await waitFor(() => expect(search.current.hasFailed).toBe(true))

    await searchFor(search.current, 'Dark Magician')

    await waitFor(() => expect(search.current.results).toEqual([card('Dark Magician')]))
    expect(search.current.hasFailed).toBe(false)
  })

  // Two searches in flight can land in either order. The older one landing last
  // would replace the user's current question with the answer to the previous.
  it('ignores an answer to a question the user has already replaced', async () => {
    let landFirst: (response: Response) => void = () => undefined
    mockFetch.mockReturnValueOnce(
      new Promise<Response>((resolve) => {
        landFirst = resolve
      }),
    )
    mockFetch.mockImplementationOnce(() => Promise.resolve(found(card('Mirror Force'))))
    const search = await renderSearch()

    await searchFor(search.current, 'Dark')
    await searchFor(search.current, 'Mirror')
    await waitFor(() => expect(search.current.results).toEqual([card('Mirror Force')]))

    await act(async () => {
      landFirst(found(card('Dark Magician')))
    })

    expect(search.current.results).toEqual([card('Mirror Force')])
  })

  it('refuses to search against an unset API URL rather than call an empty host', async () => {
    delete process.env.EXPO_PUBLIC_API_URL
    const search = await renderSearch()

    await searchFor(search.current, 'Dark Magician')

    await waitFor(() => expect(search.current.hasFailed).toBe(true))
    expect(mockFetch).not.toHaveBeenCalled()
  })
})
