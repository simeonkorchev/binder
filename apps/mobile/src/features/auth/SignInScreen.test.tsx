import { render, screen, userEvent } from '@testing-library/react-native'

import App from '@/App'
import { forgetSession } from '@/lib/sessionStore'

import { requestProviderIdentity } from './lib/providerIdentity'

// The provider seam, mocked at the module boundary: both SDKs are native, and no
// live Apple or Google sign-in is possible in this environment. Everything below
// the seam — the exchange, the keychain, the navigator — is the real thing.
jest.mock('./lib/providerIdentity', () => ({ requestProviderIdentity: jest.fn() }))

const mockRequestIdentity = jest.mocked(requestProviderIdentity)
const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const issued = {
  token: 'session.jwt.signature',
  expiresAt: '2026-09-18T10:00:00.000Z',
  user: { id: 'user-1', contactEmail: null, contactPhone: null },
}

/** The sign-in endpoint issues a session; everything else answers empty. */
const server = (signIn: () => Response) =>
  (input: RequestInfo | URL): Promise<Response> =>
    Promise.resolve(
      String(input).includes('/auth/sessions')
        ? signIn()
        : new Response(JSON.stringify({ binders: [], listings: [] }), { status: 200 }),
    )

const pressGoogle = async (user: ReturnType<typeof userEvent.setup>): Promise<void> => {
  await render(<App />)
  await user.press(await screen.findByRole('button', { name: 'Sign in with Google' }))
}

describe('SignInScreen', () => {
  beforeEach(async () => {
    await forgetSession()
    jest.clearAllMocks()
    mockRequestIdentity.mockResolvedValue({
      status: 'obtained',
      identityToken: 'google.id.token',
      nonce: 'nonce-1',
    })
    mockFetch.mockReset()
    mockFetch.mockImplementation(
      server(() => new Response(JSON.stringify(issued), { status: 201 })),
    )
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  // The whole journey T080 exists for: a press, a provider token, one exchange,
  // and the app is somebody's own app.
  it('signs a collector in and lands them on their binders', async () => {
    const user = userEvent.setup()

    await pressGoogle(user)

    expect(await screen.findByRole('button', { name: 'Binders, tab, 2 of 3' })).toBeSelected()
  })

  it('sends the provider that was pressed, with the token and the nonce it was bound to', async () => {
    const user = userEvent.setup()

    await pressGoogle(user)
    await screen.findByRole('button', { name: 'Binders, tab, 2 of 3' })

    expect(mockRequestIdentity).toHaveBeenCalledWith('google')
    expect(JSON.parse(String(mockFetch.mock.calls[0]?.[1]?.body))).toEqual({
      provider: 'google',
      identityToken: 'google.id.token',
      nonce: 'nonce-1',
    })
  })

  it('asks Apple when the Apple button is pressed', async () => {
    const user = userEvent.setup()
    await render(<App />)

    await user.press(await screen.findByRole('button', { name: 'Sign in with Apple' }))

    expect(mockRequestIdentity).toHaveBeenCalledWith('apple')
  })

  // One sentence and nothing else. The server refuses a forged, replayed and
  // expired token identically on purpose, and none of what the provider handed
  // over may reach the screen (004-security.md).
  it('says one sentence and stays on the way in when the server refuses the token', async () => {
    mockFetch.mockImplementation(server(() => new Response('{}', { status: 401 })))
    const user = userEvent.setup()

    await pressGoogle(user)

    expect(await screen.findByText('Signing in did not work. Try again.')).toBeOnTheScreen()
    expect(screen.queryByText(/google.id.token/)).not.toBeOnTheScreen()
    expect(screen.queryByText(/nonce-1/)).not.toBeOnTheScreen()
    expect(screen.getByRole('button', { name: 'Sign in with Google' })).toBeOnTheScreen()
  })

  it('says nothing at all when the collector closes the provider sheet', async () => {
    mockRequestIdentity.mockResolvedValue({ status: 'cancelled' })
    const user = userEvent.setup()

    await pressGoogle(user)

    expect(await screen.findByRole('button', { name: 'Sign in with Google' })).toBeOnTheScreen()
    expect(screen.queryByText('Signing in did not work. Try again.')).not.toBeOnTheScreen()
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('says a provider this build cannot offer is unavailable', async () => {
    mockRequestIdentity.mockResolvedValue({ status: 'unavailable' })
    const user = userEvent.setup()

    await pressGoogle(user)

    expect(
      await screen.findByText('This phone cannot sign in that way. Use the other way in.'),
    ).toBeOnTheScreen()
  })
})
