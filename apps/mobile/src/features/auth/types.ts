import type { components } from '@binder/types'

/**
 * The feature's re-export of the generated contract: `@binder/types` is imported
 * here and in the API hooks, never in a screen or a component, so a regenerated
 * schema has one blast radius (003-frontend.md §5).
 */

/** `POST /auth/sessions` request body: a provider token and the nonce it is bound to. */
export type SignInBody = components['schemas']['SignInBody']

/** `POST /auth/sessions` response body: the session JWT, its expiry, and the account. */
export type SessionBody = components['schemas']['SessionBody']

/**
 * The two providers D1 chose. Taken off the contract rather than written out
 * again, so a provider the backend stops accepting stops compiling here.
 */
export type AuthProvider = SignInBody['provider']

/**
 * What asking a provider to sign somebody in ends in.
 *
 * Four outcomes, because three of them are not failures to report as one:
 *
 * - `obtained` — a provider token, and the nonce it was bound to. Both go
 *   straight to the backend, which verifies the token against the provider's
 *   published keys; nothing here reads inside it.
 * - `cancelled` — the user closed the sheet. Nothing to say, nothing went
 *   wrong, and showing an error for it would blame them for changing their mind.
 * - `unavailable` — this phone or this build cannot offer that provider: Apple
 *   sign-in on Android, or a build with no Google client id.
 * - `failed` — the provider refused, or could not be reached.
 *
 * `failed` deliberately carries no detail. The things that could be attached —
 * the token, the nonce, the authorization code, the provider's error body — are
 * credential material, and the user cannot act on any of it (004-security.md).
 */
export type ProviderIdentity =
  | { status: 'obtained'; identityToken: string; nonce: string }
  | { status: 'cancelled' }
  | { status: 'unavailable' }
  | { status: 'failed' }
