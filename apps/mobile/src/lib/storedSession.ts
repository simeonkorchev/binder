/**
 * The session as it sits on disk, and the two pure questions asked of it:
 * is this really a session, and is it still worth sending?
 *
 * Kept apart from the store that reads and writes it so both can be tested
 * without a keychain: the store owns the SecureStore calls and the React
 * subscription, this file owns the shape (003-frontend.md §9).
 */

/**
 * What the app keeps after a sign-in.
 *
 * The token and nothing more of the account than its id: the `user` the server
 * sends back also carries the contact details it publishes, and there is no
 * screen here that reads them off a stored session. Persisting them would put
 * an email address in the keychain for nothing (004-security.md).
 */
export interface Session {
  /** The bearer token. Credential material: never logged, never in an error message. */
  token: string
  /** When the server stops accepting the token, ISO-8601, exactly as it sent it. */
  expiresAt: string
  userId: string
}

/** What goes into SecureStore under one key — one value, so one read restores it. */
export const serializeSession = (session: Session): string =>
  JSON.stringify({
    token: session.token,
    expiresAt: session.expiresAt,
    userId: session.userId,
  })

/**
 * Reads a stored session back, or answers `null` for anything that is not one.
 *
 * Anything: an empty keychain, a value written by an older version of this app,
 * a half-written string. A restore cannot trust what it finds any more than a
 * request can trust its body, and the failure mode has to be "sign in again"
 * rather than an `Authorization: Bearer undefined` on every later call
 * (000-principles.md §10).
 */
export const parseStoredSession = (raw: string | null): Session | null => {
  if (raw === null) return null

  const parsed: unknown = tryParse(raw)
  if (parsed === null || typeof parsed !== 'object') return null
  if (!('token' in parsed) || !('expiresAt' in parsed) || !('userId' in parsed)) return null

  const { token, expiresAt, userId } = parsed
  if (typeof token !== 'string' || token === '') return null
  if (typeof expiresAt !== 'string' || Number.isNaN(Date.parse(expiresAt))) return null
  if (typeof userId !== 'string' || userId === '') return null

  return { token, expiresAt, userId }
}

/**
 * True once the server would refuse the token.
 *
 * The session JWT lasts 24 hours and there is no refresh token and no
 * revocation list, so an expired session is a normal end of the day, not a
 * failure: the app signs the user in again. Checked on the way out of the
 * keychain as well as reacting to a 401, because a token we already know is
 * dead should not be sent at all.
 */
export const isExpired = (session: Session, now: Date): boolean =>
  Date.parse(session.expiresAt) <= now.getTime()

const tryParse = (raw: string): unknown => {
  try {
    return JSON.parse(raw)
  } catch {
    // A keychain value that is not JSON is not a session. Which byte was wrong
    // is not something a user can act on, and the string is credential
    // material, so the parse error itself is deliberately dropped.
    return null
  }
}
