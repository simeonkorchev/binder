import { dropDestination, dropTargetAt, pocketCentre, type GridGeometry } from './dropTarget'
import type { PageShape } from './positions'

const geometry: GridGeometry = { width: 300, height: 420 }

const shape = (over: Partial<PageShape> = {}): PageShape => ({
  page: 0,
  pageCount: 1,
  filled: 9,
  ...over,
})

describe('dropTargetAt', () => {
  it.each([
    ['the first pocket', 10, 10, 0],
    ['the middle pocket', 150, 210, 4],
    ['the last pocket', 290, 410, 8],
    ['a point just across a pocket boundary', 101, 139, 1],
  ])('reads %s', (_name, x, y, pocket) => {
    expect(dropTargetAt(x, y, geometry)).toEqual({ kind: 'pocket', pocket })
  })

  it('reads the strip left of the grid as the previous page', () => {
    expect(dropTargetAt(-20, 200, geometry)).toEqual({ kind: 'previousPage' })
  })

  it('reads the strip right of the grid as the next page', () => {
    expect(dropTargetAt(320, 200, geometry)).toEqual({ kind: 'nextPage' })
  })

  it('reads a point above or below the grid as no target', () => {
    expect(dropTargetAt(150, -5, geometry)).toEqual({ kind: 'none' })
    expect(dropTargetAt(150, 500, geometry)).toEqual({ kind: 'none' })
  })

  it('has no target before the grid has been measured', () => {
    expect(dropTargetAt(10, 10, { width: 0, height: 0 })).toEqual({ kind: 'none' })
  })
})

describe('pocketCentre', () => {
  it('is the middle of the pocket, so a drag starts where the card is', () => {
    expect(pocketCentre(0, geometry)).toEqual({ x: 50, y: 70 })
    expect(pocketCentre(4, geometry)).toEqual({ x: 150, y: 210 })
    expect(pocketCentre(8, geometry)).toEqual({ x: 250, y: 350 })
  })

  it('lands inside the pocket it names', () => {
    for (let pocket = 0; pocket < 9; pocket += 1) {
      const centre = pocketCentre(pocket, geometry)

      expect(dropTargetAt(centre.x, centre.y, geometry)).toEqual({ kind: 'pocket', pocket })
    }
  })
})

describe('dropDestination', () => {
  it('takes a card to the pocket it was dropped on', () => {
    expect(dropDestination({ kind: 'pocket', pocket: 7 }, 2, shape())).toBe(7)
  })

  it('makes no request when the card was dropped back where it came from', () => {
    expect(dropDestination({ kind: 'pocket', pocket: 2 }, 2, shape())).toBeNull()
  })

  it('makes no request for a drop that hit nothing', () => {
    expect(dropDestination({ kind: 'none' }, 2, shape())).toBeNull()
  })

  // The page boundary: a card dragged to the right edge of page 0 reaches
  // page 1 without the grid ever showing it, and the position it asks for is a
  // position on that page.
  it('crosses forward to the first pocket of the next page', () => {
    expect(dropDestination({ kind: 'nextPage' }, 2, shape({ page: 0, pageCount: 3 }))).toBe(9)
  })

  it('crosses back to the last pocket of the previous page', () => {
    expect(dropDestination({ kind: 'previousPage' }, 11, shape({ page: 1, pageCount: 3 }))).toBe(8)
  })

  it('has no page to cross to beyond the last one', () => {
    expect(
      dropDestination({ kind: 'nextPage' }, 20, shape({ page: 2, pageCount: 3, filled: 3 })),
    ).toBeNull()
  })

  it('has no page to cross back to from the first one', () => {
    expect(dropDestination({ kind: 'previousPage' }, 2, shape({ page: 0, pageCount: 3 }))).toBeNull()
  })

  it('clamps a drop past the last card of a half-filled page onto that card', () => {
    expect(
      dropDestination({ kind: 'pocket', pocket: 8 }, 9, shape({ page: 1, pageCount: 2, filled: 3 })),
    ).toBe(11)
  })
})
