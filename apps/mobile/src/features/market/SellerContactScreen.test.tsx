import * as Clipboard from 'expo-clipboard'
import { render, screen, userEvent } from '@testing-library/react-native'
import { Linking } from 'react-native'

import { liveSession } from '@/lib/sessionFixture'
import { forgetSession, rememberSession } from '@/lib/sessionStore'

import App from '@/App'

import type { ListedCard, SellerContactBody } from './types'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const MARKET_TAB = 'Market, tab, 3 of 3'
const BROWSE = 'Browse what is for sale'

const session = liveSession()

const listed: ListedCard = {
  listingId: 'listing-1',
  sellerId: 'seller-1',
  cardId: 'card-1',
  cardName: 'Dark Magician',
  imageObjectKey: null,
  setCode: 'LOB',
  listedAt: '2026-09-17T10:00:00Z',
}

/**
 * The feed answers every browse, the seller's details answer their own path, and
 * the binder list answers the tab a signed-in collector lands on.
 */
const serverWhere =
  (contact: () => Response) =>
  (input: RequestInfo | URL): Promise<Response> => {
    const path = String(input)
    if (path.includes('/sellers/')) return Promise.resolve(contact())
    if (path.includes('/binders')) {
      return Promise.resolve(new Response(JSON.stringify({ binders: [] }), { status: 200 }))
    }
    return Promise.resolve(new Response(JSON.stringify({ listings: [listed] }), { status: 200 }))
  }

const shares = (contact: SellerContactBody): Response =>
  new Response(JSON.stringify(contact), { status: 200 })

/**
 * Reaches the seller the way a buyer does: the market, then the card they want.
 *
 * Signed in, because this is the one thing in the market that needs an account —
 * a signed-in caller is what separates a buyer from a script reading every
 * seller in the database (internal/listing/api/seller.go).
 */
const openSeller = async (user: ReturnType<typeof userEvent.setup>): Promise<void> => {
  await rememberSession(session)
  await render(<App />)
  await user.press(await screen.findByRole('button', { name: MARKET_TAB }))
  await user.press(
    await screen.findByRole('button', { name: 'Contact the seller of Dark Magician' }),
  )
}

describe('SellerContactScreen', () => {
  beforeEach(async () => {
    // Signed out before each case, after the previous test's tree is gone:
    // dropping the session while a screen is still mounted would re-render it
    // outside act.
    await forgetSession()
    mockFetch.mockReset()
    mockFetch.mockImplementation(serverWhere(() => shares({ email: null, phone: null })))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
    jest.spyOn(Linking, 'openURL').mockResolvedValue(true)
    jest.spyOn(Clipboard, 'setStringAsync').mockResolvedValue(true)
  })

  afterEach(() => {
    jest.restoreAllMocks()
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('reveals what the seller opted into sharing, each one actionable', async () => {
    mockFetch.mockImplementation(
      serverWhere(() => shares({ email: 'seller@binder.test', phone: '0888 123 456' })),
    )
    const user = userEvent.setup()

    await openSeller(user)

    expect(await screen.findByRole('button', { name: 'Write to seller@binder.test' })).toBeOnTheScreen()
    expect(screen.getByRole('button', { name: 'Call 0888 123 456' })).toBeOnTheScreen()
  })

  // The headline case of the contract: a seller who published neither field is
  // a 200 with two nulls. It is a sentence, never a failure and never a blank.
  it('says a seller shared nothing rather than showing a failure or an empty screen', async () => {
    const user = userEvent.setup()

    await openSeller(user)

    expect(
      await screen.findByText('This seller has not shared any contact details.'),
    ).toBeOnTheScreen()
    expect(screen.queryByText('The seller\'s details could not be loaded.')).not.toBeOnTheScreen()
  })

  it('hands an address to the app that writes mail', async () => {
    mockFetch.mockImplementation(serverWhere(() => shares({ email: 'seller@binder.test', phone: null })))
    const user = userEvent.setup()

    await openSeller(user)
    await user.press(await screen.findByRole('button', { name: 'Write to seller@binder.test' }))

    expect(Linking.openURL).toHaveBeenCalledWith('mailto:seller@binder.test')
  })

  it('copies a number a buyer would otherwise retype, and confirms it landed', async () => {
    mockFetch.mockImplementation(serverWhere(() => shares({ email: null, phone: '0888 123 456' })))
    const user = userEvent.setup()

    await openSeller(user)
    await user.press(await screen.findByRole('button', { name: 'Copy the phone number 0888 123 456' }))

    expect(Clipboard.setStringAsync).toHaveBeenCalledWith('0888 123 456')
    expect(await screen.findByText('Copied to the clipboard.')).toBeOnTheScreen()
  })

  it('says the details could not be read, which is not the same as none being shared', async () => {
    mockFetch.mockImplementation(serverWhere(() => new Response('{}', { status: 500 })))
    const user = userEvent.setup()

    await openSeller(user)

    expect(await screen.findByText('The seller\'s details could not be loaded.')).toBeOnTheScreen()
    expect(
      screen.queryByText('This seller has not shared any contact details.'),
    ).not.toBeOnTheScreen()
  })

  // A visitor who came in through the public feed: the contact reveal is the one
  // request in the market that needs an account, and "sign in" is something they
  // can act on where "could not be loaded" is not (T082).
  it('tells a signed-out visitor that reaching a seller needs an account', async () => {
    mockFetch.mockImplementation(serverWhere(() => new Response('{}', { status: 401 })))
    const user = userEvent.setup()

    await render(<App />)
    await user.press(await screen.findByRole('button', { name: BROWSE }))
    await user.press(
      await screen.findByRole('button', { name: 'Contact the seller of Dark Magician' }),
    )

    expect(
      await screen.findByText('Sign in to see how to reach this seller.'),
    ).toBeOnTheScreen()
    expect(screen.queryByText('The seller\'s details could not be loaded.')).not.toBeOnTheScreen()
  })
})
