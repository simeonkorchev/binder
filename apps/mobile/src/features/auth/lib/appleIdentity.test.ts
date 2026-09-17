import * as AppleAuthentication from 'expo-apple-authentication'

import { appleIdentity } from './appleIdentity'

// The provider SDK, mocked at the module boundary: it is a native module, and
// no live Apple sign-in is possible in this environment.
jest.mock('expo-apple-authentication', () => ({
  isAvailableAsync: jest.fn(),
  signInAsync: jest.fn(),
  AppleAuthenticationUserDetectionStatus: { UNSUPPORTED: 0 },
}))

const mockIsAvailable = jest.mocked(AppleAuthentication.isAvailableAsync)
const mockSignIn = jest.mocked(AppleAuthentication.signInAsync)

/** The sheet's answer, with only the fields this flow reads filled in. */
const credential = (
  identityToken: string | null,
): AppleAuthentication.AppleAuthenticationCredential => ({
  user: 'apple-subject',
  state: null,
  fullName: null,
  email: null,
  realUserStatus: AppleAuthentication.AppleAuthenticationUserDetectionStatus.UNSUPPORTED,
  identityToken,
  authorizationCode: null,
})

/** What Apple's sheet rejects with when the user dismisses it. */
const cancellation = (): Error & { code: string } =>
  Object.assign(new Error('The user canceled the authorization attempt.'), {
    code: 'ERR_REQUEST_CANCELED',
  })

describe('appleIdentity', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockIsAvailable.mockResolvedValue(true)
    mockSignIn.mockResolvedValue(credential('apple.identity.token'))
  })

  it('answers with the token and the nonce it was bound to', async () => {
    await expect(appleIdentity('nonce-1')).resolves.toEqual({
      status: 'obtained',
      identityToken: 'apple.identity.token',
      nonce: 'nonce-1',
    })
  })

  // The binding the backend checks: the nonce in the token's claim has to equal
  // the one sent beside it, so passing a different value to the sheet than the
  // one reported would refuse every sign-in.
  it('asks the sheet to bind the token to that same nonce, and for no scopes', async () => {
    await appleIdentity('nonce-1')

    expect(mockSignIn).toHaveBeenCalledWith({ nonce: 'nonce-1' })
  })

  it('is unavailable on a phone that cannot sign in with Apple', async () => {
    mockIsAvailable.mockResolvedValue(false)

    await expect(appleIdentity('nonce-1')).resolves.toEqual({ status: 'unavailable' })
    expect(mockSignIn).not.toHaveBeenCalled()
  })

  it('reports a dismissed sheet as a cancellation, not as a failure', async () => {
    mockSignIn.mockRejectedValue(cancellation())

    await expect(appleIdentity('nonce-1')).resolves.toEqual({ status: 'cancelled' })
  })

  it('reports anything else the sheet throws as a failure', async () => {
    mockSignIn.mockRejectedValue(new Error('the authorization attempt failed'))

    await expect(appleIdentity('nonce-1')).resolves.toEqual({ status: 'failed' })
  })

  it('fails rather than exchanging a credential with no token in it', async () => {
    mockSignIn.mockResolvedValue(credential(null))

    await expect(appleIdentity('nonce-1')).resolves.toEqual({ status: 'failed' })
  })
})
