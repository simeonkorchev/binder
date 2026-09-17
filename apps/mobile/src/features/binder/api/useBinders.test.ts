import { act, renderHook, waitFor } from '@testing-library/react-native'

import type { Binder } from '../types'

import { useBinders } from './useBinders'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()
globalThis.fetch = mockFetch

const binder = (over: Partial<Binder> = {}): Binder => ({
  id: 'binder-1',
  name: 'Blue-Eyes',
  createdAt: '2026-09-17T10:00:00Z',
  updatedAt: '2026-09-17T10:00:00Z',
  ...over,
})

const answer = (body: object, status = 200): Response =>
  new Response(JSON.stringify(body), { status })

describe('useBinders', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('lists the binders the collector owns', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer({ binders: [binder()] })))

    const { result } = await renderHook(() => useBinders())

    await waitFor(() => {
      expect(result.current.state).toEqual({ status: 'ready', binders: [binder()] })
    })
    expect(mockFetch).toHaveBeenCalledWith('https://api.binder.test/binders', {
      headers: {},
    })
  })

  it('reads a collector with no binders as an empty list', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer({ binders: [] })))

    const { result } = await renderHook(() => useBinders())

    await waitFor(() => {
      expect(result.current.state).toEqual({ status: 'ready', binders: [] })
    })
  })

  it('reports a failed read as a failure rather than as no binders', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer({}, 401)))

    const { result } = await renderHook(() => useBinders())

    await waitFor(() => {
      expect(result.current.state).toEqual({ status: 'error' })
    })
  })

  it('reads the list again when a failed read is retried', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(answer({}, 500)))

    const { result } = await renderHook(() => useBinders())
    await waitFor(() => {
      expect(result.current.state.status).toBe('error')
    })

    await act(async () => {
      result.current.reload()
    })

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(2)
    })
  })

  it('creates a binder and answers with the one the server made', async () => {
    const created = binder({ id: 'binder-2', name: 'Trades' })
    mockFetch.mockImplementation((_input, init) =>
      Promise.resolve(init?.method === 'POST' ? answer(created, 200) : answer({ binders: [] })),
    )

    const { result } = await renderHook(() => useBinders())
    await waitFor(() => {
      expect(result.current.state.status).toBe('ready')
    })

    let answered: Binder | null = null
    await act(async () => {
      answered = await result.current.createBinder('Trades')
    })

    expect(answered).toEqual(created)
    expect(mockFetch).toHaveBeenCalledWith(
      'https://api.binder.test/binders',
      expect.objectContaining({ method: 'POST', body: JSON.stringify({ name: 'Trades' }) }),
    )
  })

  it('re-reads the list after a create, so the new binder is on it', async () => {
    mockFetch.mockImplementation((_input, init) =>
      Promise.resolve(init?.method === 'POST' ? answer(binder()) : answer({ binders: [binder()] })),
    )

    const { result } = await renderHook(() => useBinders())
    await waitFor(() => {
      expect(result.current.state.status).toBe('ready')
    })

    await act(async () => {
      await result.current.createBinder('Trades')
    })

    await waitFor(() => {
      expect(mockFetch.mock.calls.filter(([, init]) => init?.method === undefined)).toHaveLength(2)
    })
  })

  it('says the create failed and answers with nothing to navigate to', async () => {
    mockFetch.mockImplementation((_input, init) =>
      Promise.resolve(init?.method === 'POST' ? answer({}, 500) : answer({ binders: [] })),
    )

    const { result } = await renderHook(() => useBinders())
    await waitFor(() => {
      expect(result.current.state.status).toBe('ready')
    })

    let answered: Binder | null = binder()
    await act(async () => {
      answered = await result.current.createBinder('Trades')
    })

    expect(answered).toBeNull()
    expect(result.current.createFailed).toBe(true)
  })
})
