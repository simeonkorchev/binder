import { act, renderHook, waitFor } from '@testing-library/react-native'

import type { PageBody } from '../types'

import { useBinderPage } from './useBinderPage'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()
globalThis.fetch = mockFetch

const emptyPage = (over: Partial<PageBody> = {}): PageBody => ({
  page: 0,
  pageCount: 0,
  slots: [null, null, null, null, null, null, null, null, null],
  ...over,
})

const answer = (body: PageBody, status = 200): Response =>
  new Response(JSON.stringify(body), { status })

describe('useBinderPage', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('asks for the page of the binder it was given', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer(emptyPage({ page: 2, pageCount: 5 }))))

    const { result } = await renderHook(() => useBinderPage('binder-1', 2))

    await waitFor(() => {
      expect(result.current.state).toEqual({
        status: 'ready',
        page: { page: 2, pageCount: 5, pockets: Array.from({ length: 9 }, () => null) },
      })
    })
    expect(mockFetch).toHaveBeenCalledWith('https://api.binder.test/binders/binder-1?page=2', {
      headers: {},
    })
  })

  it('reads a binder with no cards as an empty page, not as a failure', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer(emptyPage())))

    const { result } = await renderHook(() => useBinderPage('binder-1', 0))

    await waitFor(() => {
      expect(result.current.state.status).toBe('ready')
    })
  })

  it('reports a failed read as a failure rather than as an empty binder', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer(emptyPage(), 500)))

    const { result } = await renderHook(() => useBinderPage('binder-1', 0))

    await waitFor(() => {
      expect(result.current.state).toEqual({ status: 'error' })
    })
  })

  it('reports a trip that never reached the server as a failure', async () => {
    mockFetch.mockImplementation(() => Promise.reject(new Error('offline')))

    const { result } = await renderHook(() => useBinderPage('binder-1', 0))

    await waitFor(() => {
      expect(result.current.state).toEqual({ status: 'error' })
    })
  })

  it('re-reads the page when asked, so a move shows where the server put the card', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer(emptyPage())))

    const { result } = await renderHook(() => useBinderPage('binder-1', 0))
    await waitFor(() => {
      expect(result.current.state.status).toBe('ready')
    })

    await act(async () => {
      result.current.reload()
    })

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(2)
    })
  })

  it('asks again when the page turns', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer(emptyPage())))

    const { rerender } = await renderHook(({ page }: { page: number }) => useBinderPage('binder-1', page), {
      initialProps: { page: 0 },
    })
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(1)
    })

    await act(async () => {
      rerender({ page: 1 })
    })

    await waitFor(() => {
      expect(mockFetch).toHaveBeenLastCalledWith(
        'https://api.binder.test/binders/binder-1?page=1',
        { headers: {} },
      )
    })
  })
})
