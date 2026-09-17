import * as AppleAuthentication from 'expo-apple-authentication'

import type { ProviderIdentity } from '../types'

/**
 * Sign in with Apple, through the system sheet.
 *
 * This is the one provider that does not go through Expo AuthSession, and the
 * reason is the backend rather than a preference: Apple's web flow returns the
 * identity token by `form_post` to an **https** redirect, which an app with a
 * custom URI scheme cannot receive and which this backend has no endpoint for.
 * The native sheet hands the token straight to the app, and it is the flow Apple
 * requires on iOS anyway. Everything after this function is provider-agnostic
 * (D1: Apple + Google, no third-party identity vendor).
 *
 * **No scopes are requested.** `pkg/oidc.Identity` carries the subject and
 * nothing else, so the name and the email Apple would share are data this
 * product has no use for — and the contact details a seller publishes are a
 * separate, deliberate opt-in through `PUT /me/contact` (004-security.md).
 */
export const appleIdentity = async (nonce: string): Promise<ProviderIdentity> => {
  if (!(await AppleAuthentication.isAvailableAsync())) return { status: 'unavailable' }

  try {
    // Apple copies this nonce into the token's `nonce` claim unchanged, and the
    // backend requires it to equal the one sent beside the token — a token bound
    // to one sign-in cannot be replayed into another (pkg/oidc.checkNonce).
    const credential = await AppleAuthentication.signInAsync({ nonce })
    if (credential.identityToken === null) return { status: 'failed' }

    return { status: 'obtained', identityToken: credential.identityToken, nonce }
  } catch (error: unknown) {
    return wasCancelled(error) ? { status: 'cancelled' } : { status: 'failed' }
  }
}

/**
 * Apple's sheet rejects with this code when the user dismisses it, which is the
 * only way to tell "no thanks" from "something broke".
 */
const wasCancelled = (error: unknown): boolean =>
  typeof error === 'object' &&
  error !== null &&
  'code' in error &&
  error.code === 'ERR_REQUEST_CANCELED'
