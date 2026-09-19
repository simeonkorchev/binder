import { devSession } from './devSession'

/** A token shaped like the one `cmd/devtoken` prints: header.payload.signature. */
const tokenFor = (claims: object): string =>
  ['eyJhbGciOiJIUzI1NiJ9', base64Url(JSON.stringify(claims)), 'signature'].join('.')

const base64Url = (value: string): string =>
  btoa(value).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '')

const userId = 'f1f0c0de-0000-4000-8000-000000000001'
const expiresAt = Date.UTC(2026, 8, 19, 10, 0, 0) / 1000

describe('devSession', () => {
  afterEach(() => {
    delete process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN
  })

  it('is absent when the build supplies no token — the normal case', () => {
    expect(devSession()).toBeNull()
  })

  it('reads the user and the expiry the server signed, rather than a second variable that could disagree', () => {
    const token = tokenFor({ sub: userId, exp: expiresAt })
    process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN = token

    expect(devSession()).toEqual({ token, userId, expiresAt: '2026-09-19T10:00:00.000Z' })
  })

  // Each of these is a typo in .env. None of them may produce a session that
  // sends `Authorization: Bearer undefined` on every later call.
  it.each([
    ['not a JWT at all', 'hunter2'],
    ['a JWT with no payload segment', 'header'],
    ['a payload that is not base64', 'header.!!!!.signature'],
    ['a payload that is not JSON', ['header', base64Url('nope'), 'signature'].join('.')],
    ['no subject', tokenFor({ exp: expiresAt })],
    ['an empty subject', tokenFor({ sub: '', exp: expiresAt })],
    ['no expiry', tokenFor({ sub: userId })],
    ['an expiry that is not a number', tokenFor({ sub: userId, exp: '2026-09-19' })],
  ])('refuses %s', (_case, token) => {
    process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN = token

    expect(devSession()).toBeNull()
  })

  it('is absent when the variable is set but empty', () => {
    process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN = ''

    expect(devSession()).toBeNull()
  })

  // The claim the doc comment makes, and the only one that matters for
  // security: a release bundle has no such branch. Metro substitutes __DEV__ at
  // build time, so this is what a production build evaluates.
  it('is absent in a release build, however the token is set', () => {
    process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN = tokenFor({ sub: userId, exp: expiresAt })
    // `__DEV__` is declared as a bare `const` global, so it cannot be assigned
    // through its own name; the intersection names the one property being set
    // rather than widening globalThis.
    const globals = globalThis as typeof globalThis & { __DEV__: boolean }
    const wasDev = globals.__DEV__
    globals.__DEV__ = false

    try {
      expect(devSession()).toBeNull()
    } finally {
      globals.__DEV__ = wasDev
    }
  })
})
