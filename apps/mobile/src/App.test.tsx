import { render, screen, userEvent } from '@testing-library/react-native'

import { forgetSession, rememberSession } from '@/lib/sessionStore'

import App from './App'

// The tab bar builds each accessible name from the translated title, so these
// names double as proof that `t()` reached the navigator.
const SCAN_TAB = 'Scan, tab, 1 of 3'
const BINDERS_TAB = 'Binders, tab, 2 of 3'
const MARKET_TAB = 'Market, tab, 3 of 3'

const GOOGLE = 'Sign in with Google'
const APPLE = 'Sign in with Apple'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const session = {
  token: 'session.jwt.signature',
  expiresAt: '2026-09-18T10:00:00.000Z',
  userId: 'user-1',
}

/** A collector who is already signed in when the app starts. */
const launchSignedIn = async (): Promise<void> => {
  await rememberSession(session)
  await render(<App />)
}

describe('App', () => {
  beforeEach(async () => {
    // Signed out before each case, after the previous test's tree is gone:
    // dropping the session while a screen is still mounted would re-render it
    // outside act.
    await forgetSession()
    mockFetch.mockReset()
    mockFetch.mockImplementation(() =>
      Promise.resolve(new Response(JSON.stringify({ binders: [], listings: [] }), { status: 200 })),
    )
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  describe('signed out', () => {
    it('opens on the way in, not on a binder list nobody can read', async () => {
      await render(<App />)

      expect(await screen.findByRole('button', { name: GOOGLE })).toBeOnTheScreen()
      expect(screen.queryByRole('button', { name: BINDERS_TAB })).not.toBeOnTheScreen()
    })

    // Offering Google without Apple is what the App Store refuses (D1). The
    // Apple button is iOS-only, and jest-expo runs the iOS platform.
    it('offers Apple beside Google', async () => {
      await render(<App />)

      expect(await screen.findByRole('button', { name: APPLE })).toBeOnTheScreen()
    })

    // Browse is public by design: `GET /listings` takes no actor, so the market
    // has to be reachable without an account (W11).
    it('lets a visitor browse what is for sale without signing in', async () => {
      const user = userEvent.setup()
      await render(<App />)

      await user.press(await screen.findByRole('button', { name: 'Browse what is for sale' }))

      expect(await screen.findByRole('button', { name: 'Search' })).toBeOnTheScreen()
      expect(mockFetch.mock.calls[0]?.[1]?.headers).toEqual({})
    })
  })

  describe('signed in', () => {
    it('skips the sign-in screen when a session was restored from the keychain', async () => {
      await launchSignedIn()

      expect(await screen.findByRole('button', { name: BINDERS_TAB })).toBeOnTheScreen()
      expect(screen.queryByRole('button', { name: GOOGLE })).not.toBeOnTheScreen()
    })

    it('lands on the binders, which is where a scanned card has to go', async () => {
      await launchSignedIn()

      expect(await screen.findByRole('button', { name: BINDERS_TAB })).toBeSelected()
      expect(await screen.findByRole('button', { name: 'New binder' })).toBeOnTheScreen()
    })

    it('names every tab for a screen reader', async () => {
      await launchSignedIn()

      for (const name of [SCAN_TAB, BINDERS_TAB, MARKET_TAB]) {
        expect(await screen.findByRole('button', { name })).toBeOnTheScreen()
      }
    })

    it('moves to another top-level destination when its tab is pressed', async () => {
      const user = userEvent.setup()
      await launchSignedIn()

      await user.press(await screen.findByRole('button', { name: MARKET_TAB }))

      expect(await screen.findByRole('button', { name: MARKET_TAB })).toBeSelected()
      expect(screen.getByRole('button', { name: BINDERS_TAB })).not.toBeSelected()
    })

    it('opens the market tab on what is for sale, not on a placeholder', async () => {
      const user = userEvent.setup()
      await launchSignedIn()

      await user.press(await screen.findByRole('button', { name: MARKET_TAB }))

      expect(await screen.findByRole('button', { name: 'Search' })).toBeOnTheScreen()
    })

    it('opens the scanner on a way to get the camera, not on a blank viewfinder', async () => {
      const user = userEvent.setup()
      await launchSignedIn()

      await user.press(await screen.findByRole('button', { name: SCAN_TAB }))

      expect(await screen.findByRole('button', { name: 'Allow camera' })).toBeOnTheScreen()
    })

    it('attaches the session to what it asks for', async () => {
      await launchSignedIn()
      await screen.findByRole('button', { name: 'New binder' })

      expect(mockFetch.mock.calls[0]?.[1]?.headers).toEqual({
        Authorization: `Bearer ${session.token}`,
      })
    })

    it('goes back to the way in when the collector signs out', async () => {
      const user = userEvent.setup()
      await launchSignedIn()

      await user.press(await screen.findByRole('button', { name: 'Sign out' }))

      expect(await screen.findByRole('button', { name: GOOGLE })).toBeOnTheScreen()
    })
  })

  // The session JWT lasts 24 hours and cannot be refreshed, so a token the
  // server stops accepting is the end of the session — not a screen that failed
  // to load. The app has to stop sending it and ask for a new one (T082).
  it('returns to the sign-in screen when a call comes back 401', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 401 })))

    await launchSignedIn()

    expect(await screen.findByRole('button', { name: GOOGLE })).toBeOnTheScreen()
    expect(screen.queryByRole('button', { name: BINDERS_TAB })).not.toBeOnTheScreen()
  })
})
