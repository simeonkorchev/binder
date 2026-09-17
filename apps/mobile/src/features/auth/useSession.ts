import { useEffect, useSyncExternalStore } from 'react'

import {
  restoreSession,
  sessionState,
  subscribeToSession,
  type SessionState,
} from '@/lib/sessionStore'

/**
 * The session, as React sees it: `restoring` on launch, then `signed-out` or
 * `signed-in`.
 *
 * `useSyncExternalStore` rather than a context because the store has a second
 * reader that is not a component — `lib/apiRequest.ts` attaches the bearer token
 * on every call. Subscribing here means a sign-in, a sign-out and the 401 that
 * forces one all reach the navigator the same way, without anything pushing a
 * route (T083).
 *
 * The launch read is started from here because this hook is where the app first
 * needs an answer. It runs once: only the navigator calls this hook.
 */
export const useSession = (): SessionState => {
  const state = useSyncExternalStore(subscribeToSession, sessionState)

  useEffect(restoreSession, [])

  return state
}
