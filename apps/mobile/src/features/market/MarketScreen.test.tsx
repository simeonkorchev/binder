import { render, screen, userEvent } from '@testing-library/react-native'

import App from '@/App'

import type { ListedCard } from './types'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const MARKET_TAB = 'Market, tab, 3 of 3'

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

/** Opens the market the way a buyer does: from the tab bar. */
const openMarket = async (user: ReturnType<typeof userEvent.setup>): Promise<void> => {
  await render(<App />)
  await user.press(screen.getByRole('button', { name: MARKET_TAB }))
}

// The empty feed is the case the backend went out of its way to get right —
// `200 {"listings":[]}`, never a 404 and never null — and these are the two
// sentences a buyer is owed for it. Collapsing them would tell somebody who
// searched for one card that the whole market is empty.
describe('MarketScreen', () => {
  beforeEach(() => {
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
})
