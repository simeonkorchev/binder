import { writeJson } from '@/lib/apiRequest'
import type { Session } from '@/lib/storedSession'

import type { SessionBody, SignInBody } from '../types'

/**
 * The session as this app keeps it, from the session the server issued.
 *
 * Two of the response's fields are deliberately not carried: `user.contactEmail`
 * and `user.contactPhone` are the details the account publishes to buyers, no
 * screen reads them off a stored session, and keeping them would write an email
 * address into the keychain for nothing (000-principles.md §9,
 * 004-security.md).
 */
export const toSession = (body: SessionBody): Session => ({
  token: body.token,
  expiresAt: body.expiresAt,
  userId: body.user.id,
})

/**
 * Trades a provider's identity token for this backend's session.
 *
 * The identity token is not inspected here, and could not be usefully: the
 * server verifies it against the provider's published keys — signature, issuer,
 * audience, expiry and the nonce this attempt was bound to — and answers with
 * the session JWT (`pkg/oidc`, `internal/user/service`). A token it refuses
 * comes back as an `ApiError`, which the hook turns into one sentence.
 */
export const exchangeIdentityToken = async (input: SignInBody): Promise<Session> =>
  toSession(await writeJson<SessionBody>('POST', '/auth/sessions', input))
