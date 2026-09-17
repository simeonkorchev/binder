import { ApiError, readJson, writeEmpty, writeJson } from './apiRequest'
import { forgetSession, rememberSession, sessionState } from './sessionStore'
import type { Session } from './storedSession'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const session: Session = {
  token: 'header.payload.signature',
  expiresAt: '2026-09-18T10:00:00.000Z',
  userId: 'f1f0c0de-0000-4000-8000-000000000001',
}

/** The headers of the nth request, read back off the call `fetch` received. */
const sentHeaders = (call = 0): Record<string, string> => {
  const init = mockFetch.mock.calls[call]?.[1]
  const sent = init?.headers
  if (sent === undefined) return {}
  if (sent instanceof Headers || Array.isArray(sent)) {
    throw new Error('the request helper builds a plain header object')
  }
  return sent
}

describe('apiRequest', () => {
  beforeEach(async () => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 200 })))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
    await forgetSession()
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  describe('signed out', () => {
    // Browse is public by design (W11): the feed has to be readable with no
    // credential at all, so a missing session is not a missing header value —
    // there is no header.
    it('sends no Authorization header at all, so the public feed stays readable', async () => {
      await readJson('/listings')

      expect(sentHeaders()).toEqual({})
    })

    it('sends only the content type on a write', async () => {
      await writeJson('POST', '/auth/sessions', { provider: 'google' })

      expect(sentHeaders()).toEqual({ 'Content-Type': 'application/json' })
    })
  })

  describe('signed in', () => {
    beforeEach(async () => {
      await rememberSession(session)
    })

    it('attaches the session as a bearer token on a read', async () => {
      await readJson('/binders')

      expect(sentHeaders()).toEqual({ Authorization: `Bearer ${session.token}` })
    })

    it('attaches it beside the content type on a write with a body', async () => {
      await writeJson('POST', '/binders', { name: 'Blue-Eyes' })

      expect(sentHeaders()).toEqual({
        Authorization: `Bearer ${session.token}`,
        'Content-Type': 'application/json',
      })
    })

    it('attaches it on a bodiless write too', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(new Response(null, { status: 204 })))

      await writeEmpty('DELETE', '/listings/listing-1')

      expect(sentHeaders()).toEqual({ Authorization: `Bearer ${session.token}` })
    })
  })

  describe('a refused status', () => {
    it('throws with the status, and nothing of what the request carried', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 409 })))
      await rememberSession(session)

      const failure = await readJson('/binders').catch((error: unknown) => error)

      expect(failure).toBeInstanceOf(ApiError)
      expect(failure).toHaveProperty('status', 409)
      expect(String(failure)).toBe('ApiError: GET /binders answered 409')
      expect(String(failure)).not.toContain(session.token)
    })

    it('keeps the session when the status is not a 401', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 500 })))
      await rememberSession(session)

      await expect(readJson('/binders')).rejects.toBeInstanceOf(ApiError)

      expect(sessionState()).toEqual({ status: 'signed-in', session })
    })
  })

  // The 401 path, which is the whole point of T082: the token is gone, so the
  // app has to stop sending it and the user has to be taken back to sign-in.
  // Publishing `signed-out` is what does the second half — the navigator
  // renders from that state.
  describe('a 401', () => {
    it.each([
      ['a read', (): Promise<unknown> => readJson('/binders')],
      ['a write', (): Promise<unknown> => writeJson('POST', '/binders', { name: 'x' })],
      ['a bodiless write', (): Promise<unknown> => writeEmpty('DELETE', '/listings/listing-1')],
    ])('clears the session on %s', async (_kind, call) => {
      mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 401 })))
      await rememberSession(session)

      await expect(call()).rejects.toBeInstanceOf(ApiError)

      expect(sessionState()).toEqual({ status: 'signed-out' })
    })

    it('still throws, so the caller is never told the request succeeded', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 401 })))
      await rememberSession(session)

      await expect(readJson('/binders')).rejects.toHaveProperty('status', 401)
    })

    it('sends no stale token on the next request', async () => {
      mockFetch.mockImplementationOnce(() => Promise.resolve(new Response('{}', { status: 401 })))
      await rememberSession(session)

      await expect(readJson('/binders')).rejects.toBeInstanceOf(ApiError)
      await readJson('/listings')

      expect(sentHeaders(1)).toEqual({})
    })
  })
})
