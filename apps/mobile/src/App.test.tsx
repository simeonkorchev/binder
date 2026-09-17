import { render, screen, userEvent } from '@testing-library/react-native'

import App from './App'

// The tab bar builds each accessible name from the translated title, so these
// names double as proof that `t()` reached the navigator.
const SCAN_TAB = 'Scan, tab, 1 of 3'
const BINDERS_TAB = 'Binders, tab, 2 of 3'
const MARKET_TAB = 'Market, tab, 3 of 3'

describe('App', () => {
  it('opens on the scan tab', async () => {
    await render(<App />)

    expect(screen.getByRole('button', { name: SCAN_TAB })).toBeSelected()
  })

  it('moves to another top-level destination when its tab is pressed', async () => {
    const user = userEvent.setup()
    await render(<App />)

    await user.press(screen.getByRole('button', { name: MARKET_TAB }))

    expect(await screen.findByRole('button', { name: MARKET_TAB })).toBeSelected()
    expect(screen.getByRole('button', { name: SCAN_TAB })).not.toBeSelected()
  })

  it('names every tab for a screen reader', async () => {
    await render(<App />)

    for (const name of [SCAN_TAB, BINDERS_TAB, MARKET_TAB]) {
      expect(screen.getByRole('button', { name })).toBeOnTheScreen()
    }
  })

  it('says a screen is unbuilt rather than rendering a blank page', async () => {
    await render(<App />)

    expect(screen.getByText('This screen is not built yet.')).toBeOnTheScreen()
  })
})
