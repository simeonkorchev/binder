import { render, screen, userEvent } from '@testing-library/react-native'

import '@/i18n/i18n'

import type { PageShape } from '../lib/positions'
import type { SlotBody } from '../types'

import { PocketActions } from './PocketActions'

const slot = (over: Partial<SlotBody> = {}): SlotBody => ({
  id: 'slot-3',
  cardId: 'card-3',
  cardPrintingId: 'printing-3',
  page: 0,
  position: 2,
  slotOnPage: 2,
  setResolution: 'exact',
  ...over,
})

const shape = (over: Partial<PageShape> = {}): PageShape => ({
  page: 0,
  pageCount: 3,
  filled: 9,
  ...over,
})

const props = {
  onMove: jest.fn(),
  onRemove: jest.fn(),
  onClose: jest.fn(),
}

// This is the proof that every move the drag can make is also reachable without
// one. A control that only a hold-and-drag can reach is a control a screen
// reader and an unsteady hand cannot (003-frontend.md §10).
describe('PocketActions', () => {
  beforeEach(() => {
    props.onMove.mockReset()
    props.onRemove.mockReset()
    props.onClose.mockReset()
  })

  it('offers both single-pocket moves and both page moves as named buttons', async () => {
    await render(<PocketActions slot={slot()} shape={shape({ page: 1 })} {...props} />)

    for (const name of [
      'One pocket back',
      'One pocket forward',
      'To the previous page',
      'To the next page',
      'Remove from the binder',
    ]) {
      expect(screen.getByRole('button', { name })).toBeOnTheScreen()
    }
  })

  it('sends a card across the page boundary to the first pocket of the next page', async () => {
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)

    await user.press(screen.getByRole('button', { name: 'To the next page' }))

    expect(props.onMove).toHaveBeenCalledWith(9)
  })

  it('sends a card back across the boundary to the last pocket of the page before', async () => {
    const user = userEvent.setup()
    await render(
      <PocketActions slot={slot({ position: 11, page: 1, slotOnPage: 2 })} shape={shape({ page: 1 })} {...props} />,
    )

    await user.press(screen.getByRole('button', { name: 'To the previous page' }))

    expect(props.onMove).toHaveBeenCalledWith(8)
  })

  it('disables the moves this pocket cannot make instead of hiding them', async () => {
    const user = userEvent.setup()
    await render(
      <PocketActions slot={slot({ position: 0, slotOnPage: 0 })} shape={shape({ pageCount: 1 })} {...props} />,
    )

    const back = screen.getByRole('button', { name: 'One pocket back' })
    expect(back).toBeDisabled()
    expect(screen.getByRole('button', { name: 'To the next page' })).toBeDisabled()

    await user.press(back)

    expect(props.onMove).not.toHaveBeenCalled()
  })

  it('takes the card out of the binder', async () => {
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)

    await user.press(screen.getByRole('button', { name: 'Remove from the binder' }))

    expect(props.onRemove).toHaveBeenCalled()
  })

  it('says in words, not in colour, that a card has no set recorded', async () => {
    await render(<PocketActions slot={slot({ setResolution: 'unresolved', cardPrintingId: null })} shape={shape()} {...props} />)

    expect(screen.getByText(/No set/)).toBeOnTheScreen()
  })
})

// Selling a card is one of the things that can be done to a card, so it is in
// the sheet a card opens rather than behind an affordance of its own. What the
// requests do is `useSlotListing`'s to prove; this is that a seller can reach
// them, and that the server's refusal arrives as a sentence.
describe('PocketActions, marking the card for sale', () => {
  const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

  const listed = (): Response =>
    new Response(
      JSON.stringify({
        id: 'listing-1',
        binderSlotId: 'slot-3',
        createdAt: '2026-09-17T10:00:00Z',
        updatedAt: '2026-09-17T10:00:00Z',
      }),
      { status: 201 },
    )

  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(listed()))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('offers the card for sale from the same sheet that moves and removes it', async () => {
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)

    await user.press(screen.getByRole('button', { name: 'Mark this card for sale' }))

    expect(
      await screen.findByText('This card is for sale. Buyers find it in the Market tab.'),
    ).toBeOnTheScreen()
  })

  it('offers to take the card back off sale once it is listed', async () => {
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)
    await user.press(screen.getByRole('button', { name: 'Mark this card for sale' }))

    expect(
      await screen.findByRole('button', { name: 'Take this card off sale' }),
    ).toBeOnTheScreen()
  })

  // The 409 the server answers for a card that is already listed. A seller
  // reads a sentence about their card, never `POST /listings answered 409`.
  it('says a card is already for sale as a sentence, not as an error', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 409 })))
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)

    await user.press(screen.getByRole('button', { name: 'Mark this card for sale' }))

    expect(
      await screen.findByText('This card is already for sale, so nothing changed.'),
    ).toBeOnTheScreen()
    expect(screen.queryByText(/409/)).not.toBeOnTheScreen()
  })

  it('takes the card back off sale from the same button', async () => {
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)
    await user.press(screen.getByRole('button', { name: 'Mark this card for sale' }))

    mockFetch.mockImplementation(() => Promise.resolve(new Response(null, { status: 204 })))
    await user.press(await screen.findByRole('button', { name: 'Take this card off sale' }))

    expect(await screen.findByText('This card is no longer for sale.')).toBeOnTheScreen()
  })

  it('cannot be pressed twice while the first press is still in flight', async () => {
    // The answer is held back deliberately: "saving" is the state between the
    // press and the answer, and a mock that resolves at once has no between.
    mockFetch.mockImplementation(() => new Promise<Response>(() => {}))
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)

    await user.press(screen.getByRole('button', { name: 'Mark this card for sale' }))

    expect(await screen.findByText('Saving…')).toBeOnTheScreen()
    expect(screen.getByRole('button', { name: 'Mark this card for sale' })).toBeDisabled()
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  // Somebody else's slot is a 404 rather than a 403 so a stranger cannot learn
  // it exists; the sentence the seller reads must not give that back.
  it('says no more about a card that is not theirs than about any other failure', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 404 })))
    const user = userEvent.setup()
    await render(<PocketActions slot={slot()} shape={shape()} {...props} />)

    await user.press(screen.getByRole('button', { name: 'Mark this card for sale' }))

    expect(
      await screen.findByText('The card could not be put up for sale.'),
    ).toBeOnTheScreen()
  })
})
