import { useState } from 'react'

import { rememberSession } from '@/lib/sessionStore'

import { requestProviderIdentity } from '../lib/providerIdentity'
import type { AuthProvider } from '../types'

import { exchangeIdentityToken } from './exchangeSession'

/**
 * Where a sign-in attempt is.
 *
 * `idle` is also where a cancelled attempt lands: closing the provider's sheet is
 * a decision, not a failure, and putting a red sentence under it would blame the
 * user for changing their mind.
 */
type SignInStatus = 'idle' | 'signing-in' | 'unavailable' | 'failed'

export interface SignInFlow {
  status: SignInStatus
  /** Runs the whole exchange: provider sheet, then `POST /auth/sessions`. */
  signIn: (provider: AuthProvider) => Promise<void>
}

/**
 * Signing in, from the button press to a session this app can use.
 *
 * Three steps, and the hook owns all three so the screen owns none: ask the
 * provider for an identity token, trade it at `POST /auth/sessions`, keep what
 * comes back. Keeping it is what ends the attempt — the session store publishes
 * `signed-in` and the navigator renders the tabs from that state, so this hook
 * never navigates (003-frontend.md §1).
 *
 * Nothing it touches is ever logged or put in a message: the identity token, the
 * nonce and the session token all pass through here, and `failed` is one
 * sentence with no detail behind it (004-security.md).
 */
export const useSignIn = (): SignInFlow => {
  const [status, setStatus] = useState<SignInStatus>('idle')

  const signIn = async (provider: AuthProvider): Promise<void> => {
    setStatus('signing-in')

    const identity = await requestProviderIdentity(provider)
    if (identity.status !== 'obtained') {
      setStatus(identity.status === 'cancelled' ? 'idle' : identity.status)
      return
    }

    try {
      const session = await exchangeIdentityToken({
        provider,
        identityToken: identity.identityToken,
        nonce: identity.nonce,
      })
      await rememberSession(session)
    } catch {
      // A token the server refused and a server that could not be reached are
      // the same sentence here. Which check failed is deliberately not told
      // apart — the backend does not say either, because saying it would help
      // somebody probing with forged tokens (internal/user/api/errors.go).
      setStatus('failed')
    }
  }

  return { status, signIn }
}
