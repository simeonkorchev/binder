import { act, renderHook, waitFor } from '@testing-library/react-native'

import { forgetSession, sessionState } from '@/lib/sessionStore'

import { requestProviderIdentity } from '../lib/providerIdentity'

import { useSignIn, type SignInFlow } from './useSignIn'

jest.mock('../lib/providerIdentity', () => ({ requestProviderIdentity: jest.fn() }))

const mockRequestIdentity = jest.mocked(requestProviderIdentity)
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const issued = {
  token: 'session.jwt.signature',
  expiresAt: '2026-09-18T10:00:00Z',
  user: { id: 'user-1', contactEmail: null, contactPhone: null },
}

const renderSignIn = async (): Promise<{ current: SignInFlow }> => {
  const { result } = await renderHook(() => useSignIn())
  return result
}

describe('useSignIn', () => {
  beforeEach(async () => {
    jest.clearAllMocks()
    mockRequestIdentity.mockResolvedValue({
      status: 'obtained',
      identityToken: 'apple.identity.token',
      nonce: 'nonce-1',
    })
    mockFetch.mockReset()
    mockFetch.mockImplementation(() =>
      Promise.resolve(new Response(JSON.stringify(issued), { status: 201 })),
    )
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
    await forgetSession()
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('starts with nothing to say', async () => {
    const flow = await renderSignIn()

    expect(flow.current.status).toBe('idle')
  })

  it('keeps the session the exchange returned, which is what signs the user in', async () => {
    const flow = await renderSignIn()

    await act(() => flow.current.signIn('apple'))

    expect(sessionState()).toEqual({
      status: 'signed-in',
      session: {
        token: 'session.jwt.signature',
        expiresAt: '2026-09-18T10:00:00Z',
        userId: 'user-1',
      },
    })
  })

  it('asks the provider the user picked', async () => {
    const flow = await renderSignIn()

    await act(() => flow.current.signIn('google'))

    expect(mockRequestIdentity).toHaveBeenCalledWith('google')
  })

  // A closed sheet is a decision, not a failure: there is nothing to tell the
  // user, and a red sentence under the buttons would blame them for it.
  it('goes quiet again when the user cancels, and stores nothing', async () => {
    mockRequestIdentity.mockResolvedValue({ status: 'cancelled' })
    const flow = await renderSignIn()

    await act(() => flow.current.signIn('apple'))

    expect(flow.current.status).toBe('idle')
    expect(sessionState()).toEqual({ status: 'signed-out' })
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('says a provider this build cannot offer is unavailable', async () => {
    mockRequestIdentity.mockResolvedValue({ status: 'unavailable' })
    const flow = await renderSignIn()

    await act(() => flow.current.signIn('apple'))

    expect(flow.current.status).toBe('unavailable')
  })

  it('fails when the provider itself failed', async () => {
    mockRequestIdentity.mockResolvedValue({ status: 'failed' })
    const flow = await renderSignIn()

    await act(() => flow.current.signIn('apple'))

    expect(flow.current.status).toBe('failed')
    expect(mockFetch).not.toHaveBeenCalled()
  })

  // One sentence and no detail: the server refuses a forged, replayed or expired
  // token the same way on purpose, and the token itself must never reach a
  // message (004-security.md).
  it('fails with nothing but a status when the server refuses the provider token', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 401 })))
    const flow = await renderSignIn()

    await act(() => flow.current.signIn('apple'))

    expect(flow.current).toEqual({ status: 'failed', signIn: expect.any(Function) })
    expect(sessionState()).toEqual({ status: 'signed-out' })
  })

  it('fails when the exchange cannot be reached at all', async () => {
    mockFetch.mockRejectedValue(new Error('network request failed'))
    const flow = await renderSignIn()

    await act(() => flow.current.signIn('apple'))

    expect(flow.current.status).toBe('failed')
    expect(sessionState()).toEqual({ status: 'signed-out' })
  })

  it('reports it is working while the provider sheet is open', async () => {
    let openSheet = (): void => {}
    mockRequestIdentity.mockImplementation(
      () =>
        new Promise((resolve) => {
          openSheet = (): void => resolve({ status: 'cancelled' })
        }),
    )
    const flow = await renderSignIn()

    const attempt = flow.current.signIn('apple')
    await waitFor(() => {
      expect(flow.current.status).toBe('signing-in')
    })

    openSheet()
    await act(() => attempt)
  })
})
