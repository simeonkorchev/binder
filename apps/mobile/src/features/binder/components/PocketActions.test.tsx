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
