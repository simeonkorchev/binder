import { isExpired, parseStoredSession, serializeSession, type Session } from './storedSession'

const session: Session = {
  token: 'header.payload.signature',
  expiresAt: '2026-09-18T10:00:00.000Z',
  userId: 'f1f0c0de-0000-4000-8000-000000000001',
}

describe('serializeSession / parseStoredSession', () => {
  // The completeness test for this hop (000-principles.md §9): every field is
  // distinct and non-empty, and the whole session is asserted on the way back,
  // so a field added to `Session` and forgotten in the serializer fails here
  // rather than signing somebody out a day later.
  it('carries every field of a session through the keychain and back', () => {
    expect(parseStoredSession(serializeSession(session))).toEqual(session)
  })

  it('finds no session in an empty keychain', () => {
    expect(parseStoredSession(null)).toBeNull()
  })

  it('refuses a value that is not JSON rather than throwing at launch', () => {
    expect(parseStoredSession('not a session')).toBeNull()
  })

  it('refuses JSON that is not an object', () => {
    expect(parseStoredSession('"just a string"')).toBeNull()
  })

  it.each([
    ['token', { expiresAt: session.expiresAt, userId: session.userId }],
    ['expiresAt', { token: session.token, userId: session.userId }],
    ['userId', { token: session.token, expiresAt: session.expiresAt }],
  ])('refuses a stored value with no %s', (_field, stored) => {
    expect(parseStoredSession(JSON.stringify(stored))).toBeNull()
  })

  it('refuses an empty token, which would be sent as a header that cannot work', () => {
    expect(parseStoredSession(JSON.stringify({ ...session, token: '' }))).toBeNull()
  })

  it('refuses an expiry that is not a date', () => {
    expect(parseStoredSession(JSON.stringify({ ...session, expiresAt: 'tomorrow' }))).toBeNull()
  })

  it('refuses fields of the wrong type', () => {
    expect(parseStoredSession(JSON.stringify({ ...session, userId: 7 }))).toBeNull()
  })
})

describe('isExpired', () => {
  it('is still live a second before the server stops accepting it', () => {
    expect(isExpired(session, new Date('2026-09-18T09:59:59.000Z'))).toBe(false)
  })

  it('is expired at the very moment it runs out', () => {
    expect(isExpired(session, new Date('2026-09-18T10:00:00.000Z'))).toBe(true)
  })

  it('is expired a day later', () => {
    expect(isExpired(session, new Date('2026-09-19T10:00:00.000Z'))).toBe(true)
  })
})
