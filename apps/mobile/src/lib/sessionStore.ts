import * as SecureStore from 'expo-secure-store'

import { isExpired, parseStoredSession, serializeSession, type Session } from './storedSession'

/**
 * The one place the session lives, for the two things that need it: the request
 * helper, which is a plain module and cannot read a React context, and the
 * navigator, which has to re-render when the session appears or goes away.
 *
 * It is a store rather than a context for exactly that reason — `apiRequest.ts`
 * attaches the bearer token on every call and must not become a hook. React
 * reads this through `useSyncExternalStore` in `features/auth/useSession.ts`,
 * so there is still one source of truth and no copy to drift.
 *
 * The token is held in memory and in **SecureStore**, never in AsyncStorage: it
 * is a bearer credential and AsyncStorage is plaintext on disk
 * (004-security.md).
 */

/**
 * `binder.` prefixed so a key from another library in the same keychain cannot
 * collide with it. Changing it signs everybody out once, which is why the shape
 * behind it is guarded rather than versioned.
 */
const sessionKey = 'binder.session'

/**
 * Three states, and `restoring` is not a detail: on launch the app does not yet
 * know whether it has a session, and showing the sign-in screen for the length
 * of a keychain read would flash it at somebody who is signed in.
 */
export type SessionState =
  | { status: 'restoring' }
  | { status: 'signed-out' }
  | { status: 'signed-in'; session: Session }

// Both stateless states are single values, so `useSyncExternalStore` compares
// snapshots by identity and a restore that finds nothing does not re-render
// every screen.
const restoring: SessionState = { status: 'restoring' }
const signedOut: SessionState = { status: 'signed-out' }

let state: SessionState = restoring
const listeners = new Set<() => void>()

/** The current session state. The `getSnapshot` half of `useSyncExternalStore`. */
export const sessionState = (): SessionState => state

/** Subscribes to sign-in, sign-out and the 401 that forces one. */
export const subscribeToSession = (listener: () => void): (() => void) => {
  listeners.add(listener)
  return (): void => {
    listeners.delete(listener)
  }
}

/**
 * The credential to put on a request, or `null` when there is none.
 *
 * Signed out is not an error here: `GET /listings` is public by design, so a
 * request with no token is a normal request (specs/001-binder-mvp, W11).
 */
export const bearerToken = (): string | null =>
  state.status === 'signed-in' ? state.session.token : null

/**
 * Reads the session back at launch.
 *
 * A token that has already run out is dropped rather than sent: the session JWT
 * lasts 24 hours and the backend has no refresh token, so yesterday's session
 * is a sign-in, not a failure.
 */
export const restoreSession = async (): Promise<void> => {
  const stored = parseStoredSession(await readStored())
  if (stored === null) {
    publish(signedOut)
    return
  }
  if (isExpired(stored, new Date())) {
    await forgetSession()
    return
  }
  publish({ status: 'signed-in', session: stored })
}

/** Keeps a session a sign-in just obtained, in memory and in the keychain. */
export const rememberSession = async (session: Session): Promise<void> => {
  await quietly(() => SecureStore.setItemAsync(sessionKey, serializeSession(session)))
  publish({ status: 'signed-in', session })
}

/**
 * Drops the session: the sign-out button, and any 401.
 *
 * A 401 means the token is not accepted any more — expired, or issued by a
 * server that no longer knows it — and keeping it would put a dead credential
 * on every later request. Publishing `signed-out` is what sends the user back
 * to the sign-in screen: the navigator renders from this state, so nothing has
 * to be pushed or popped.
 */
export const forgetSession = async (): Promise<void> => {
  await quietly(() => SecureStore.deleteItemAsync(sessionKey))
  publish(signedOut)
}

/**
 * Runs a keychain write and treats a refusal as "not stored".
 *
 * A keychain that will not take the value costs the user a sign-in on the next
 * launch; it must not fail the sign-in they just completed, and it must not
 * fail a sign-out, which is the one thing that has to always work. The error is
 * dropped rather than logged because the value in hand is the token
 * (004-security.md).
 */
const quietly = async (write: () => Promise<void>): Promise<void> => {
  try {
    await write()
  } catch {
    /* empty */
  }
}

const readStored = async (): Promise<string | null> => {
  try {
    return await SecureStore.getItemAsync(sessionKey)
  } catch {
    // A keychain this app cannot read is the same situation as an empty one:
    // sign in again.
    return null
  }
}

const publish = (next: SessionState): void => {
  if (next === state) return

  state = next
  for (const listener of listeners) listener()
}
