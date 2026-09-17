/**
 * Where a card is, and where a move would put it.
 *
 * A position is one flat integer over the whole binder; the page and the pocket
 * are `position / 9` and `position % 9`. Two facts from the server's contract
 * decide every rule in this file, and neither is visible from the grid alone:
 *
 * - **The binder is dense.** `RemoveSlot` closes the gap it leaves, so the
 *   occupied positions are always `0 … count-1`. An empty pocket is therefore
 *   never in the middle of the binder — it is always past the last card.
 * - **A move cannot extend the binder.** `MoveSlot` refuses a destination
 *   outside `0 … count-1`, so a drop onto an empty pocket has to be clamped to
 *   the last card rather than sent as the pocket the finger was over.
 */

/** Three by three, the shape of a physical binder page. */
export const pocketsPerPage = 9

/** What a page screen knows about where it sits in the binder. */
export interface PageShape {
  /** Zero-based page number, as the API numbers pages. */
  page: number
  /** How many pages the binder has. Zero for a binder with no cards at all. */
  pageCount: number
  /** How many of this page's pockets hold a card — the first `filled` of them. */
  filled: number
}

export const positionOf = (page: number, pocket: number): number => page * pocketsPerPage + pocket

export const pageOf = (position: number): number => Math.floor(position / pocketsPerPage)

export const pocketOf = (position: number): number => position % pocketsPerPage

/**
 * How many pages the pager offers. An empty binder reports no pages at all and
 * still has to show its one empty page, which is where its first card goes.
 */
export const pagerLength = (pageCount: number): number => Math.max(pageCount, 1)

/** Keeps a page number inside the pager after a removal has shortened the binder. */
export const clampPage = (page: number, pageCount: number): number =>
  Math.min(Math.max(page, 0), pagerLength(pageCount) - 1)

/** True when nothing after this page holds a card. */
export const isLastPage = (shape: PageShape): boolean => shape.page >= shape.pageCount - 1

/**
 * The pocket a card added to this page would land in, or null when this page is
 * full or is not the page the binder ends on. Only that one pocket offers to
 * take a card: a card is always appended, so any other empty pocket would take
 * it somewhere the user did not point at.
 */
export const addablePocket = (shape: PageShape): number | null => {
  if (!isLastPage(shape) || shape.filled >= pocketsPerPage) return null
  return shape.filled
}

/**
 * How many of a page's pockets hold a card. The binder is dense, so they are
 * the first ones — which is what lets a page describe itself with one number.
 */
export const filledPockets = <T,>(pockets: readonly (T | null)[]): number =>
  pockets.filter((pocket) => pocket !== null).length

/** The moves a pocket offers as buttons — the same set a drag can express. */
export type MoveAction = 'pocketBack' | 'pocketForward' | 'previousPage' | 'nextPage'

/**
 * Where a move sends the card, or null when this binder cannot take it there.
 *
 * `previousPage` and `nextPage` are the page boundary: they hand the card to
 * the last pocket of the page before, or the first pocket of the page after,
 * and the server shifts everything in between one place along.
 */
export const moveDestination = (
  action: MoveAction,
  from: number,
  shape: PageShape,
): number | null => {
  switch (action) {
    case 'pocketBack':
      return from > 0 ? from - 1 : null
    case 'pocketForward':
      // Off the end of the last page there is no card to swap with, and the
      // server refuses a destination past the last one.
      if (isLastPage(shape) && pocketOf(from) + 1 >= shape.filled) return null
      return from + 1
    case 'previousPage':
      return shape.page > 0 ? positionOf(shape.page, 0) - 1 : null
    case 'nextPage':
      return shape.page + 1 <= shape.pageCount - 1 ? positionOf(shape.page + 1, 0) : null
  }
}

/**
 * The position a card dropped on one of this page's pockets takes.
 *
 * A drop on an empty pocket lands on the last card of the page rather than
 * nowhere: the pockets past the end are not destinations, and refusing the drop
 * would make the gesture feel broken over most of a half-filled page.
 * Returns null when the page holds no card to land beside.
 */
export const pocketDestination = (pocket: number, shape: PageShape): number | null => {
  if (shape.filled === 0) return null
  return positionOf(shape.page, Math.min(pocket, shape.filled - 1))
}
