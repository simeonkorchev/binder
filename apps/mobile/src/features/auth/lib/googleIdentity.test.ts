import * as AuthSession from 'expo-auth-session'

import { googleIdentity } from './googleIdentity'

// The provider SDK, mocked at the module boundary: it opens a browser through a
// native module, and no live Google sign-in is possible in this environment.
jest.mock('expo-auth-session', () => ({
  makeRedirectUri: jest.fn(),
  loadAsync: jest.fn(),
  exchangeCodeAsync: jest.fn(),
}))

jest.mock('expo-application', () => ({ applicationId: 'com.simeonkorchev.binder' }))

const mockMakeRedirectUri = jest.mocked(AuthSession.makeRedirectUri)
const mockLoadAsync = jest.mocked(AuthSession.loadAsync)
const mockExchangeCodeAsync = jest.mocked(AuthSession.exchangeCodeAsync)
const mockPromptAsync = jest.fn<Promise<AuthSession.AuthSessionResult>, []>()

/**
 * The loaded authorization request, carrying the two members this flow reads.
 *
 * `AuthRequest` is a class with a dozen more of them, so the fake is declared as
 * a `Partial` and narrowed once — spelling out the other eleven would assert
 * nothing this test is about.
 */
const loadedRequest = (): AuthSession.AuthRequest => {
  const request: Partial<AuthSession.AuthRequest> = {
    codeVerifier: 'the-code-verifier',
    promptAsync: mockPromptAsync,
  }
  return request as AuthSession.AuthRequest
}

const authorized = (params: Record<string, string>): AuthSession.AuthSessionResult => ({
  type: 'success',
  errorCode: null,
  params,
  authentication: null,
  url: 'com.simeonkorchev.binder:/oauthredirect',
})

/** The token endpoint's answer, with the one field the flow reads off it. */
const exchanged = (idToken: string | undefined): AuthSession.TokenResponse => {
  const response: Partial<AuthSession.TokenResponse> = {
    accessToken: 'the-access-token',
    idToken,
  }
  return response as AuthSession.TokenResponse
}

/** What the authorization request was asked for, read off the config it was loaded with. */
const requestedConfig = (): AuthSession.AuthRequestConfig => {
  const config = mockLoadAsync.mock.calls[0]?.[0]
  if (config === undefined) throw new Error('no authorization request was loaded')
  return config
}

describe('googleIdentity', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    // Google issues one client id per platform and this flow picks the one for
    // the platform it runs on. jest-expo runs the iOS platform, so both are set
    // to distinct values and the assertions below name the iOS one.
    process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS = 'ios-client.apps.googleusercontent.com'
    process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_ANDROID = 'android-client.apps.googleusercontent.com'
    mockMakeRedirectUri.mockReturnValue('com.simeonkorchev.binder:/oauthredirect')
    mockLoadAsync.mockResolvedValue(loadedRequest())
    mockPromptAsync.mockResolvedValue(authorized({ code: 'the-authorization-code' }))
    mockExchangeCodeAsync.mockResolvedValue(exchanged('google.id.token'))
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS
    delete process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_ANDROID
  })

  it('answers with the ID token from the exchange and the nonce it was bound to', async () => {
    await expect(googleIdentity('nonce-1')).resolves.toEqual({
      status: 'obtained',
      identityToken: 'google.id.token',
      nonce: 'nonce-1',
    })
  })

  // The binding the backend checks: the token's `nonce` claim has to equal the
  // one sent beside it, so the value asked for and the value reported are one.
  it('binds the authorization request to that nonce', async () => {
    await googleIdentity('nonce-1')

    expect(requestedConfig().extraParams).toEqual({ nonce: 'nonce-1' })
  })

  it('asks only for openid — the backend reads the subject and nothing else', async () => {
    await googleIdentity('nonce-1')

    expect(requestedConfig().scopes).toEqual(['openid'])
  })

  it('uses the code flow with PKCE, which is the only one that yields an ID token here', async () => {
    await googleIdentity('nonce-1')

    expect(requestedConfig().usePKCE).toBe(true)
    expect(mockExchangeCodeAsync).toHaveBeenCalledWith(
      {
        clientId: 'ios-client.apps.googleusercontent.com',
        code: 'the-authorization-code',
        redirectUri: 'com.simeonkorchev.binder:/oauthredirect',
        extraParams: { code_verifier: 'the-code-verifier' },
      },
      expect.objectContaining({ tokenEndpoint: 'https://oauth2.googleapis.com/token' }),
    )
  })

  it('redirects back to the app itself, which is what Google accepts for an installed client', async () => {
    await googleIdentity('nonce-1')

    expect(mockMakeRedirectUri).toHaveBeenCalledWith({
      native: 'com.simeonkorchev.binder:/oauthredirect',
    })
  })

  it.each([['cancel'], ['dismiss']] as const)(
    'reports a %s as a cancellation, not as a failure',
    async (type) => {
      mockPromptAsync.mockResolvedValue({ type })

      await expect(googleIdentity('nonce-1')).resolves.toEqual({ status: 'cancelled' })
      expect(mockExchangeCodeAsync).not.toHaveBeenCalled()
    },
  )

  it('reports a refused authorization as a failure', async () => {
    mockPromptAsync.mockResolvedValue({
      type: 'error',
      errorCode: 'access_denied',
      params: {},
      authentication: null,
      url: 'com.simeonkorchev.binder:/oauthredirect',
    })

    await expect(googleIdentity('nonce-1')).resolves.toEqual({ status: 'failed' })
  })

  it('fails rather than exchanging a success that carries no code', async () => {
    mockPromptAsync.mockResolvedValue(authorized({}))

    await expect(googleIdentity('nonce-1')).resolves.toEqual({ status: 'failed' })
    expect(mockExchangeCodeAsync).not.toHaveBeenCalled()
  })

  it('fails when the exchange comes back without an ID token', async () => {
    mockExchangeCodeAsync.mockResolvedValue(exchanged(undefined))

    await expect(googleIdentity('nonce-1')).resolves.toEqual({ status: 'failed' })
  })

  it('fails when the token endpoint cannot be reached', async () => {
    mockExchangeCodeAsync.mockRejectedValue(new Error('network request failed'))

    await expect(googleIdentity('nonce-1')).resolves.toEqual({ status: 'failed' })
  })

  it('is unavailable in a build with no Google client id, without opening a browser', async () => {
    delete process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID_IOS

    await expect(googleIdentity('nonce-1')).resolves.toEqual({ status: 'unavailable' })
    expect(mockLoadAsync).not.toHaveBeenCalled()
  })
})
