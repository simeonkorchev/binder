import type { SessionBody } from '../types'

import { exchangeIdentityToken, toSession } from './exchangeSession'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

/** Every field the contract has, each with a distinct value. */
const issued: SessionBody = {
  token: 'session.jwt.signature',
  expiresAt: '2026-09-18T10:00:00Z',
  user: {
    id: 'f1f0c0de-0000-4000-8000-000000000001',
    contactEmail: 'seller@example.test',
    contactPhone: '+359888000111',
  },
}

/** What `POST /auth/sessions` was sent, read back off the call `fetch` received. */
const sentBody = (): unknown => {
  const body = mockFetch.mock.calls[0]?.[1]?.body
  if (typeof body !== 'string') throw new Error('the exchange must send a JSON body')
  return JSON.parse(body)
}

describe('toSession', () => {
  // The completeness test for this hop (000-principles.md §9). The two fields it
  // does not carry are named here on purpose: an omission has to be a decision,
  // and a field added to the contract and forgotten below fails this assertion.
  it('carries the token, the expiry and the account id, and no contact detail', () => {
    expect(toSession(issued)).toEqual({
      token: 'session.jwt.signature',
      expiresAt: '2026-09-18T10:00:00Z',
      userId: 'f1f0c0de-0000-4000-8000-000000000001',
    })
  })

  it('keeps the published contact details out of what gets stored', () => {
    const stored: unknown = toSession(issued)

    expect(JSON.stringify(stored)).not.toContain('seller@example.test')
    expect(JSON.stringify(stored)).not.toContain('+359888000111')
  })
})

describe('exchangeIdentityToken', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(() =>
      Promise.resolve(new Response(JSON.stringify(issued), { status: 201 })),
    )
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('posts the provider, the token and the nonce to the sign-in endpoint', async () => {
    await exchangeIdentityToken({
      provider: 'apple',
      identityToken: 'apple.identity.token',
      nonce: 'nonce-1',
    })

    expect(mockFetch.mock.calls[0]?.[0]).toBe('https://api.binder.test/auth/sessions')
    expect(sentBody()).toEqual({
      provider: 'apple',
      identityToken: 'apple.identity.token',
      nonce: 'nonce-1',
    })
  })

  it('answers with the session the server issued', async () => {
    await expect(
      exchangeIdentityToken({
        provider: 'google',
        identityToken: 'google.id.token',
        nonce: 'nonce-1',
      }),
    ).resolves.toEqual({
      token: 'session.jwt.signature',
      expiresAt: '2026-09-18T10:00:00Z',
      userId: 'f1f0c0de-0000-4000-8000-000000000001',
    })
  })

  it('throws when the server refuses the provider token', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 401 })))

    await expect(
      exchangeIdentityToken({
        provider: 'google',
        identityToken: 'forged.id.token',
        nonce: 'nonce-1',
      }),
    ).rejects.toHaveProperty('status', 401)
  })
})
