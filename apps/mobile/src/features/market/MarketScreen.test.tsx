import { render, screen, userEvent } from '@testing-library/react-native'

import { forgetSession, rememberSession } from '@/lib/sessionStore'

import App from '@/App'

import type { ListedCard } from './types'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const MARKET_TAB = 'Market, tab, 3 of 3'
const BROWSE = 'Browse what is for sale'

const session = {
  token: 'session.jwt.signature',
  expiresAt: '2026-09-18T10:00:00.000Z',
  userId: 'user-1',
}

const listed = (over: Partial<ListedCard> = {}): ListedCard => ({
  listingId: 'listing-1',
  sellerId: 'seller-1',
  cardId: 'card-1',
  cardName: 'Dark Magician',
  imageObjectKey: null,
  setCode: 'LOB',
  listedAt: '2026-09-17T10:00:00Z',
  ...over,
})

const feed = (...listings: ListedCard[]): Response =>
  new Response(JSON.stringify({ listings }), { status: 200 })

/**
 * Opens the market the way most buyers do: signed out, from the sign-in screen.
 *
 * Browse is the one public endpoint (W11), so every case below is also a case
 * that it stays reachable with no account.
 */
const openMarket = async (user: ReturnType<typeof userEvent.setup>): Promise<void> => {
  await render(<App />)
  await user.press(await screen.findByRole('button', { name: BROWSE }))
}

// The empty feed is the case the backend went out of its way to get right —
// `200 {"listings":[]}`, never a 404 and never null — and these are the two
// sentences a buyer is owed for it. Collapsing them would tell somebody who
// searched for one card that the whole market is empty.
describe('MarketScreen', () => {
  beforeEach(async () => {
    // Signed out before each case, after the previous test's tree is gone:
    // dropping the session while a screen is still mounted would re-render it
    // outside act.
    await forgetSession()
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(feed()))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('lists what is for sale, and says which set each card is from', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(feed(listed())))
    const user = userEvent.setup()

    await openMarket(user)

    expect(
      await screen.findByRole('button', { name: 'Contact the seller of Dark Magician' }),
    ).toBeOnTheScreen()
    expect(screen.getByText('LOB')).toBeOnTheScreen()
  })

  it('says a card whose set was never resolved has none, rather than leaving a gap', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(feed(listed({ setCode: null }))))
    const user = userEvent.setup()

    await openMarket(user)

    expect(await screen.findByText('Set unknown')).toBeOnTheScreen()
  })

  it('tells a buyer the market is empty when nothing at all is for sale', async () => {
    const user = userEvent.setup()

    await openMarket(user)

    expect(
      await screen.findByText(
        'Nothing is for sale yet. Mark a card in one of your binders and it shows up here.',
      ),
    ).toBeOnTheScreen()
  })

  it('tells a buyer their filter matched nothing, which is not the same as an empty market', async () => {
    const user = userEvent.setup()

    await openMarket(user)
    await user.type(screen.getByLabelText('Search cards for sale by name'), 'Kuriboh')
    await user.press(screen.getByRole('button', { name: 'Search' }))

    expect(
      await screen.findByText(
        'No card for sale matches that search. Try a shorter name, or another set.',
      ),
    ).toBeOnTheScreen()
  })

  it('goes back to the whole market when the filters are cleared', async () => {
    const user = userEvent.setup()

    await openMarket(user)
    await user.type(screen.getByLabelText('Search cards for sale by name'), 'Kuriboh')
    await user.press(screen.getByRole('button', { name: 'Search' }))
    await screen.findByText(
      'No card for sale matches that search. Try a shorter name, or another set.',
    )

    await user.press(screen.getByRole('button', { name: 'Clear the filters' }))

    expect(
      await screen.findByText(
        'Nothing is for sale yet. Mark a card in one of your binders and it shows up here.',
      ),
    ).toBeOnTheScreen()
  })

  it('says the feed could not be read rather than showing an empty market', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 500 })))
    const user = userEvent.setup()

    await openMarket(user)

    expect(await screen.findByText('The listings could not be loaded.')).toBeOnTheScreen()
  })

  // The header, not just the screen: signed out there is no credential to send,
  // and the browse feed is the one endpoint that needs none.
  it('reads the feed with no Authorization header at all', async () => {
    const user = userEvent.setup()

    await openMarket(user)
    await screen.findByText(
      'Nothing is for sale yet. Mark a card in one of your binders and it shows up here.',
    )

    expect(mockFetch.mock.calls[0]?.[1]?.headers).toEqual({})
  })

  it('shows the same market from the tab bar once a collector is signed in', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(feed(listed())))
    await rememberSession(session)
    const user = userEvent.setup()

    await render(<App />)
    await user.press(await screen.findByRole('button', { name: MARKET_TAB }))

    expect(
      await screen.findByRole('button', { name: 'Contact the seller of Dark Magician' }),
    ).toBeOnTheScreen()
  })
})
