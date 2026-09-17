import { render, screen } from '@testing-library/react-native'

import App from '@/App'

// Only the launch read is stubbed, so the store stays in the state it starts in:
// the app has read nothing yet and does not know whether it has a session. That
// is the one moment the third state exists for, and it cannot be observed with
// the real read, which lands in the same effect that starts it.
jest.mock('@/lib/sessionStore', () => ({
  ...jest.requireActual<typeof import('@/lib/sessionStore')>('@/lib/sessionStore'),
  restoreSession: jest.fn(),
}))

describe('the session on launch', () => {
  it('shows neither the way in nor somebody\'s binders until the keychain has answered', async () => {
    await render(<App />)

    expect(await screen.findByText('Opening Binder…')).toBeOnTheScreen()
    expect(screen.queryByRole('button', { name: 'Sign in with Google' })).not.toBeOnTheScreen()
    expect(screen.queryByRole('button', { name: 'Binders, tab, 2 of 3' })).not.toBeOnTheScreen()
  })
})
