import * as Crypto from 'expo-crypto'

import { appleIdentity } from './appleIdentity'
import { googleIdentity } from './googleIdentity'
import { requestProviderIdentity } from './providerIdentity'

jest.mock('./appleIdentity', () => ({ appleIdentity: jest.fn() }))
jest.mock('./googleIdentity', () => ({ googleIdentity: jest.fn() }))
jest.mock('expo-crypto', () => ({ randomUUID: jest.fn() }))

const mockApple = jest.mocked(appleIdentity)
const mockGoogle = jest.mocked(googleIdentity)
const mockRandomUUID = jest.mocked(Crypto.randomUUID)

describe('requestProviderIdentity', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockRandomUUID.mockReturnValue('nonce-1')
    mockApple.mockResolvedValue({
      status: 'obtained',
      identityToken: 'apple.identity.token',
      nonce: 'nonce-1',
    })
    mockGoogle.mockResolvedValue({
      status: 'obtained',
      identityToken: 'google.id.token',
      nonce: 'nonce-1',
    })
  })

  it('sends an Apple sign-in to Apple, with a fresh nonce', async () => {
    const identity = await requestProviderIdentity('apple')

    expect(mockApple).toHaveBeenCalledWith('nonce-1')
    expect(mockGoogle).not.toHaveBeenCalled()
    expect(identity).toEqual({
      status: 'obtained',
      identityToken: 'apple.identity.token',
      nonce: 'nonce-1',
    })
  })

  it('sends a Google sign-in to Google, with a fresh nonce', async () => {
    const identity = await requestProviderIdentity('google')

    expect(mockGoogle).toHaveBeenCalledWith('nonce-1')
    expect(mockApple).not.toHaveBeenCalled()
    expect(identity).toEqual({
      status: 'obtained',
      identityToken: 'google.id.token',
      nonce: 'nonce-1',
    })
  })

  // A nonce binds one token to one attempt. Reusing it would let a token
  // obtained for an earlier attempt be replayed into a later one.
  it('mints a new nonce for every attempt', async () => {
    mockRandomUUID.mockReturnValueOnce('nonce-1').mockReturnValueOnce('nonce-2')

    await requestProviderIdentity('google')
    await requestProviderIdentity('google')

    expect(mockGoogle.mock.calls).toEqual([['nonce-1'], ['nonce-2']])
  })

  it('passes a cancellation back untouched', async () => {
    mockApple.mockResolvedValue({ status: 'cancelled' })

    await expect(requestProviderIdentity('apple')).resolves.toEqual({ status: 'cancelled' })
  })
})
