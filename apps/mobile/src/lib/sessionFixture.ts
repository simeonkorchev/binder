import type { Session } from './storedSession'

/**
 * A session the app would still accept, for the suites that launch `<App />`.
 *
 * The expiry is relative to the clock the test runs on, and that is the whole
 * point. These fixtures used to carry a fixed timestamp, and every suite that
 * renders the app went red the hour that timestamp passed: `restoreSession`
 * drops a session that has run out, so the app launched on the sign-in screen
 * and fourteen cases lost the tab bar they were looking for. A fixture that
 * means "not expired" has to say so against the clock, not against a date
 * somebody typed while it was still in the future.
 *
 * A test that is *about* expiry pins the clock instead and keeps its own
 * literal — see `lib/sessionStore.test.ts`. This is for the ones that render
 * against the real one.
 *
 * A function rather than a constant so the value is read when the test asks for
 * it, not when the module is imported.
 */
export const liveSession = (): Session => ({
  token: 'session.jwt.signature',
  // An hour: longer than any suite here takes, short enough to read as a session.
  expiresAt: new Date(Date.now() + 60 * 60 * 1000).toISOString(),
  userId: 'user-1',
})
