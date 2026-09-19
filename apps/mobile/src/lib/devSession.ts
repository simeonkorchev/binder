import type { Session } from './storedSession'

/**
 * A session taken from the build's environment instead of a sign-in.
 *
 * It exists for one situation: exercising the app against a local `binderd`
 * before a Google OAuth client is configured. `EXPO_PUBLIC_GOOGLE_CLIENT_ID_*`
 * is what the sign-in screen needs, and obtaining one requires the release
 * keystore's fingerprint — which is a slower errand than the first run of the
 * app should have to wait for. `make seed-user && make token` mints a token the
 * server already trusts; this turns it into a session.
 *
 * **It cannot exist in a release build.** `__DEV__` is a compile-time constant
 * that Metro substitutes — `false` for a production bundle — so the whole body
 * is dead code the minifier removes. Combined with the fact that
 * `EXPO_PUBLIC_*` is inlined at bundle time, a release build has neither the
 * branch nor the value.
 *
 * Nothing here is written to the keychain. The session lives for the run, so
 * removing the variable from `.env` and restarting Metro puts the sign-in
 * screen back with no state to clear (004-security.md).
 */
export const devSession = (): Session | null => {
  if (!__DEV__) return null

  const token = process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN
  if (token === undefined || token === '') return null

  const claims = sessionClaims(token)
  if (claims === null) return null

  return { token, expiresAt: claims.expiresAt, userId: claims.userId }
}

/**
 * The `sub` and `exp` the server signed, read back off the token.
 *
 * Read rather than configured separately: a userId typed into `.env` beside the
 * token can disagree with it, and then the app acts as one user while the API
 * answers for another — a confusing failure for something whose only job is to
 * save time. The signature is not checked here; `binderd` does that on every
 * request, and this code only ever runs against a token the developer minted.
 */
const sessionClaims = (token: string): { userId: string; expiresAt: string } | null => {
  const payload = token.split('.')[1]
  if (payload === undefined) return null

  try {
    const decoded: unknown = JSON.parse(base64UrlDecode(payload))
    if (decoded === null || typeof decoded !== 'object') return null
    if (!('sub' in decoded) || !('exp' in decoded)) return null

    const { sub, exp } = decoded
    if (typeof sub !== 'string' || sub === '') return null
    if (typeof exp !== 'number') return null

    return { userId: sub, expiresAt: new Date(exp * 1000).toISOString() }
  } catch {
    // Anything that is not a JWT is a typo in .env, not a session. The token is
    // credential material, so the parse error itself is dropped rather than
    // logged (004-security.md).
    return null
  }
}

const base64UrlDecode = (segment: string): string => {
  const base64 = segment.replaceAll('-', '+').replaceAll('_', '/')
  return atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, '='))
}
