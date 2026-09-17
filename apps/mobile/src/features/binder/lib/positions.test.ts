import {
  addablePocket,
  filledPockets,
  clampPage,
  isLastPage,
  moveDestination,
  pageOf,
  pagerLength,
  pocketDestination,
  pocketOf,
  positionOf,
  type MoveAction,
  type PageShape,
} from './positions'

const shape = (over: Partial<PageShape> = {}): PageShape => ({
  page: 0,
  pageCount: 1,
  filled: 9,
  ...over,
})

describe('position arithmetic', () => {
  it.each([
    [0, 0, 0],
    [0, 8, 8],
    [1, 0, 9],
    [3, 4, 31],
  ])('page %i pocket %i is position %i', (page, pocket, position) => {
    expect(positionOf(page, pocket)).toBe(position)
    expect(pageOf(position)).toBe(page)
    expect(pocketOf(position)).toBe(pocket)
  })
})

describe('pagerLength', () => {
  it('shows the one empty page of a binder the server reports no pages for', () => {
    expect(pagerLength(0)).toBe(1)
  })

  it('shows every page the binder has', () => {
    expect(pagerLength(4)).toBe(4)
  })
})

describe('clampPage', () => {
  it('keeps a page inside the pager after a removal shortened the binder', () => {
    expect(clampPage(3, 2)).toBe(1)
  })

  it('never goes before the first page', () => {
    expect(clampPage(-1, 3)).toBe(0)
  })

  it('leaves a page that is still there alone', () => {
    expect(clampPage(1, 3)).toBe(1)
  })
})

describe('isLastPage', () => {
  it('is true for the page the binder ends on', () => {
    expect(isLastPage(shape({ page: 2, pageCount: 3 }))).toBe(true)
  })

  it('is true for the single empty page of an empty binder', () => {
    expect(isLastPage(shape({ page: 0, pageCount: 0, filled: 0 }))).toBe(true)
  })

  it('is false while pages follow', () => {
    expect(isLastPage(shape({ page: 0, pageCount: 3 }))).toBe(false)
  })
})

describe('addablePocket', () => {
  it('is the first empty pocket of the last page', () => {
    expect(addablePocket(shape({ page: 1, pageCount: 2, filled: 4 }))).toBe(4)
  })

  it('is the first pocket of an empty binder', () => {
    expect(addablePocket(shape({ page: 0, pageCount: 0, filled: 0 }))).toBe(0)
  })

  it('is nowhere on a full page, because the card would land on the next one', () => {
    expect(addablePocket(shape({ page: 1, pageCount: 2, filled: 9 }))).toBeNull()
  })

  it('is nowhere on a page with pages after it — that page has no empty pocket', () => {
    expect(addablePocket(shape({ page: 0, pageCount: 3 }))).toBeNull()
  })
})

describe('filledPockets', () => {
  it('counts the cards on a page', () => {
    expect(filledPockets([{}, {}, null, null, null, null, null, null, null])).toBe(2)
  })

  it('counts an empty page as none', () => {
    expect(filledPockets([null, null, null, null, null, null, null, null, null])).toBe(0)
  })

  it('counts a full page as nine', () => {
    expect(filledPockets(Array.from({ length: 9 }, () => ({})))).toBe(9)
  })
})

describe('moveDestination', () => {
  it('sends a card one pocket back', () => {
    expect(moveDestination('pocketBack', 4, shape())).toBe(3)
  })

  it('refuses to move the very first card back', () => {
    expect(moveDestination('pocketBack', 0, shape())).toBeNull()
  })

  it('sends a card one pocket forward', () => {
    expect(moveDestination('pocketForward', 4, shape())).toBe(5)
  })

  it('refuses to move the last card of the binder forward', () => {
    expect(moveDestination('pocketForward', 12, shape({ page: 1, pageCount: 2, filled: 4 }))).toBeNull()
  })

  it('moves a card forward off a full page onto the next one', () => {
    expect(moveDestination('pocketForward', 8, shape({ page: 0, pageCount: 2 }))).toBe(9)
  })

  it('hands a card to the last pocket of the previous page', () => {
    expect(moveDestination('previousPage', 12, shape({ page: 1, pageCount: 3 }))).toBe(8)
  })

  it('has no previous page on the first page', () => {
    expect(moveDestination('previousPage', 2, shape({ page: 0, pageCount: 3 }))).toBeNull()
  })

  it('hands a card to the first pocket of the next page', () => {
    expect(moveDestination('nextPage', 2, shape({ page: 0, pageCount: 3 }))).toBe(9)
  })

  it('has no next page on the last page', () => {
    expect(moveDestination('nextPage', 20, shape({ page: 2, pageCount: 3, filled: 3 }))).toBeNull()
  })

  it.each<MoveAction>(['pocketBack', 'pocketForward', 'previousPage', 'nextPage'])(
    'offers %s nowhere in an empty binder',
    (action) => {
      expect(moveDestination(action, 0, shape({ page: 0, pageCount: 0, filled: 0 }))).toBeNull()
    },
  )
})

describe('pocketDestination', () => {
  it('is the position of the pocket the card was dropped on', () => {
    expect(pocketDestination(5, shape({ page: 1, pageCount: 2 }))).toBe(14)
  })

  it('clamps a drop past the last card onto the last card', () => {
    expect(pocketDestination(7, shape({ page: 1, pageCount: 2, filled: 3 }))).toBe(11)
  })

  it('has nowhere to land on a page with no cards', () => {
    expect(pocketDestination(0, shape({ page: 0, pageCount: 0, filled: 0 }))).toBeNull()
  })
})
