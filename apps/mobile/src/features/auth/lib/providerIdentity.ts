import * as Crypto from 'expo-crypto'

import type { AuthProvider, ProviderIdentity } from '../types'

import { appleIdentity } from './appleIdentity'
import { googleIdentity } from './googleIdentity'

/**
 * The one place that knows Apple from Google, mirroring `internal/user/identity`
 * on the server: everything above this line takes a provider name and gets an
 * identity back, and nothing above it branches on which provider was named.
 *
 * The nonce is minted here, once per attempt, so both providers are bound the
 * same way and the value the backend is told is the value the provider was given.
 */
export const requestProviderIdentity = (provider: AuthProvider): Promise<ProviderIdentity> => {
  const nonce = newNonce()

  return provider === 'apple' ? appleIdentity(nonce) : googleIdentity(nonce)
}

/**
 * A fresh, unguessable value per sign-in attempt.
 *
 * `randomUUID` is the platform CSPRNG — 122 random bits, which is what a nonce
 * needs to be worth binding a token to. It is never reused and never stored: a
 * new attempt mints a new one, and the previous one becomes worthless the moment
 * the token that carries it is exchanged.
 */
const newNonce = (): string => Crypto.randomUUID()
