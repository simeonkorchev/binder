import { act, fireEvent, render, screen } from '@testing-library/react-native'

import '@/i18n/i18n'

import type { PageShape } from '../lib/positions'
import type { SlotBody } from '../types'

import { BinderPageGrid } from './BinderPageGrid'

const width = 300
const height = 420
/** Longer than the grid's hold threshold, so the movement carries a card. */
const heldMs = 400
/** Shorter than it, so the same movement turns the page. */
const flickedMs = 50

const card = (pocket: number, page = 0): SlotBody => ({
  id: `slot-${page * 9 + pocket}`,
  cardId: `card-${page * 9 + pocket}`,
  cardPrintingId: 'printing-1',
  page,
  position: page * 9 + pocket,
  slotOnPage: pocket,
  setResolution: 'exact',
})

const pageOfCards = (page = 0): (SlotBody | null)[] =>
  Array.from({ length: 9 }, (_unused, pocket) => card(pocket, page))

const shape = (over: Partial<PageShape> = {}): PageShape => ({
  page: 0,
  pageCount: 2,
  filled: 9,
  ...over,
})

const props = {
  onOpenCard: jest.fn(),
  onAddCard: jest.fn(),
  onMove: jest.fn(),
  onTurnPage: jest.fn(),
}

interface Point {
  x: number
  y: number
}

/** The middle of pocket 5, the strips either side of the grid, and pocket 1. */
const middle: Point = { x: 150, y: 210 }
const beyondRightEdge: Point = { x: 340, y: 210 }
const beyondLeftEdge: Point = { x: -30, y: 210 }
const topLeftPocket: Point = { x: 50, y: 70 }

const touch = (
  name: 'responderGrant' | 'responderMove' | 'responderRelease',
  point: Point,
  timestamp: number,
): void => {
  fireEvent(screen.getByTestId('binder-page-grid'), name, {
    nativeEvent: { pageX: point.x, pageY: point.y, timestamp },
  })
}

/**
 * The grid has no drop targets until it has been measured. The measurement is a
 * ref, so this changes nothing on screen and needs no flush — which is the
 * reason it is a ref.
 */
const layOut = (): void => {
  fireEvent(screen.getByTestId('binder-page-grid'), 'layout', {
    nativeEvent: { layout: { width, height, x: 0, y: 0 } },
  })
}

/**
 * A card picked up at `from`, carried to `to` and dropped. The rest before the
 * movement is expressed in the touches' own timestamps, which is what the grid
 * measures it with — no clock is mocked and nothing sleeps.
 */
const drag = async (from: Point, to: Point): Promise<void> => {
  await act(async () => {
    touch('responderGrant', from, 0)
    touch('responderMove', to, heldMs)
    touch('responderRelease', to, heldMs)
  })
}

/** The same movement, too quick to pick a card up: nothing goes in the air. */
const flick = (from: Point, to: Point): void => {
  touch('responderGrant', from, 0)
  touch('responderMove', to, flickedMs)
  touch('responderRelease', to, flickedMs)
}

const tap = (point: Point): void => {
  touch('responderGrant', point, 0)
  touch('responderRelease', point, 30)
}

/**
 * One test, and one render inside it.
 *
 * A drag has no `userEvent` equivalent — the library models presses and typing,
 * not a finger held on a card and carried off the edge of the page — so the
 * responder events the platform would send are fired directly. Everything the
 * gesture *decides* is pure and tested case by case in `lib/dropTarget.test.ts`
 * and `lib/positions.test.ts`; what this pins is the wiring between the touches
 * and those decisions.
 *
 * It is one test because under RNTL 14 on React 19 the first test in a file
 * that fires an event at this tree leaves the renderer producing an **empty**
 * tree for every `render` after it. `rerender` is unaffected, so the page the
 * grid is showing is changed that way.
 */
describe('BinderPageGrid', () => {
  it('reads a gesture as a tap, a page turn or a card carried to where it was let go', async () => {
    const halfFilled: (SlotBody | null)[] = [card(0), card(1), null, null, null, null, null, null, null]
    const { rerender } = await render(
      <BinderPageGrid pockets={halfFilled} shape={shape({ pageCount: 1, filled: 2 })} {...props} />,
    )

    expect(screen.getByRole('button', { name: 'Pocket 1, card, Set confirmed' })).toBeOnTheScreen()
    expect(screen.getByRole('button', { name: 'Pocket 3, empty. Add a card here.' })).toBeOnTheScreen()
    // The empty pockets after it are pockets, not buttons: a card is appended,
    // so any other one would take it somewhere nobody pointed at.
    expect(screen.getByLabelText('Pocket 4, empty')).toBeOnTheScreen()

    // A screen reader activates a pocket without a touch, which is the route
    // that has to work when a hold-and-drag cannot (003-frontend.md §10).
    fireEvent(
      screen.getByRole('button', { name: 'Pocket 3, empty. Add a card here.' }),
      'accessibilityTap',
    )
    expect(props.onAddCard).toHaveBeenCalled()

    await act(async () => {
      rerender(
        <BinderPageGrid pockets={pageOfCards(1)} shape={shape({ page: 1, pageCount: 3 })} {...props} />,
      )
    })
    layOut()

    // A touch that goes nowhere opens the card's actions — the route to every
    // move without a drag.
    tap(middle)
    expect(props.onOpenCard).toHaveBeenCalledWith(card(4, 1))
    expect(props.onMove).not.toHaveBeenCalled()

    // The same movement, too quick to pick a card up, turns the page. That is
    // what keeps a swipe available over a page with a card in every pocket.
    flick(middle, topLeftPocket)
    expect(props.onTurnPage).toHaveBeenCalledWith(1)
    expect(props.onMove).not.toHaveBeenCalled()

    // Held first, the same movement carries the card.
    await drag(middle, topLeftPocket)
    expect(props.onMove).toHaveBeenCalledTimes(1)
    expect(props.onMove).toHaveBeenCalledWith(card(4, 1), 9)

    // The page boundary: carried past the right edge, the card asks for the
    // first position of the page the grid is not showing — and asks once,
    // because the cards it displaces are the server's to shift.
    props.onMove.mockReset()
    await drag(middle, beyondRightEdge)
    expect(props.onMove).toHaveBeenCalledTimes(1)
    expect(props.onMove).toHaveBeenCalledWith(card(4, 1), 18)

    // And back the other way, onto the last pocket of the page before.
    props.onMove.mockReset()
    await drag(middle, beyondLeftEdge)
    expect(props.onMove).toHaveBeenCalledTimes(1)
    expect(props.onMove).toHaveBeenCalledWith(card(4, 1), 8)
  })
})
