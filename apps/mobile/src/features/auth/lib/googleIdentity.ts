import * as Application from 'expo-application'
import * as AuthSession from 'expo-auth-session'
import { Platform } from 'react-native'

import type { ProviderIdentity } from '../types'

/**
 * Sign in with Google, through Expo AuthSession (D1).
 *
 * The code flow with PKCE, not the implicit one: an installed app has no client
 * secret, and Google only returns an ID token to a native client by exchanging
 * the code. The exchange happens on the phone because there is nothing secret in
 * it — the code verifier proves the exchange belongs to the same request that
 * started, which is what PKCE is for.
 *
 * Google's endpoints are constants rather than a discovery fetch: they are facts
 * about the provider, and a sign-in that starts with a network round trip to
 * fetch them fails for a reason the user cannot act on.
 *
 * **Only `openid` is requested.** `pkg/oidc.Identity` carries the subject and
 * nothing else, so asking for an email address the backend never reads would
 * collect PII for nothing (004-security.md).
 */
const discovery = {
  authorizationEndpoint: 'https://accounts.google.com/o/oauth2/v2/auth',
  tokenEndpoint: 'https://oauth2.googleapis.com/token',
}

export const googleIdentity = async (nonce: string): Promise<ProviderIdentity> => {
  const clientId = googleClientId()
  if (clientId === null) return { status: 'unavailable' }

  // The app's own id, which is the redirect Google accepts for an installed
  // client and the default expo-auth-session's Google provider uses. Reading it
  // rather than repeating the bundle identifier from app.config.ts keeps the two
  // from drifting.
  const redirectUri = AuthSession.makeRedirectUri({
    native: `${Application.applicationId ?? ''}:/oauthredirect`,
  })

  try {
    const request = await AuthSession.loadAsync(
      {
        clientId,
        redirectUri,
        scopes: ['openid'],
        usePKCE: true,
        // Google copies this into the ID token's `nonce` claim, and the backend
        // requires it to equal the one sent beside the token, so a token bound
        // to one sign-in cannot be replayed into another (pkg/oidc.checkNonce).
        extraParams: { nonce },
      },
      discovery,
    )

    const result = await request.promptAsync(discovery)
    if (result.type === 'cancel' || result.type === 'dismiss') return { status: 'cancelled' }
    if (result.type !== 'success') return { status: 'failed' }

    const code = result.params.code
    if (code === undefined || code === '') return { status: 'failed' }

    const exchanged = await AuthSession.exchangeCodeAsync(
      {
        clientId,
        code,
        redirectUri,
        extraParams: { code_verifier: request.codeVerifier ?? '' },
      },
      discovery,
    )
    if (exchanged.idToken === undefined || exchanged.idToken === '') return { status: 'failed' }

    return { status: 'obtained', identityToken: exchanged.idToken, nonce }
  } catch {
    // Whatever went wrong is dropped rather than logged or attached: the objects
    // in play here hold the authorization code and the tokens
    // (004-security.md).
    return { status: 'failed' }
  }
}

/**
 * The OAuth client this build signs in with, or `null` when it has none.
 *
 * Google issues one client id per platform, so the iOS and the Android build of
 * this app are different audiences for the same account — the backend's
 * `GOOGLE_CLIENT_IDS` accepts both for exactly that reason. They are read as two
 * literal `process.env` lookups because that is what Metro inlines
 * `EXPO_PUBLIC_*` values into.
 */
const googleClientId = (): string | null => {
  const clientId = Platform.select({
    ios: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS,
    default: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_ANDROID,
  })

  return clientId === undefined || clientId === '' ? null : clientId
}
