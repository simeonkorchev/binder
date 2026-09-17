import {
  moveDestination,
  pocketDestination,
  pocketsPerPage,
  type PageShape,
} from './positions'

/** Three across and three down, measured in points once the grid has laid out. */
export const columns = 3
const rows = pocketsPerPage / columns

/** The grid's own box, as `onLayout` reports it. */
export interface GridGeometry {
  width: number
  height: number
}

/**
 * Where a dragged card would land if the finger lifted now.
 *
 * `previousPage` and `nextPage` are the strips either side of the grid: they
 * are how a drag reaches a page it cannot see, which a grid that only accepted
 * drops on its own nine pockets could never do.
 */
export type DropTarget =
  | { kind: 'pocket'; pocket: number }
  | { kind: 'previousPage' }
  | { kind: 'nextPage' }
  | { kind: 'none' }

/** The middle of a pocket, which is where a drag starts from. */
export const pocketCentre = (
  pocket: number,
  geometry: GridGeometry,
): { x: number; y: number } => ({
  x: ((pocket % columns) + 0.5) * (geometry.width / columns),
  y: (Math.floor(pocket / columns) + 0.5) * (geometry.height / rows),
})

const cell = (value: number, size: number, count: number): number =>
  Math.min(Math.max(Math.floor(value / (size / count)), 0), count - 1)

/**
 * Reads a point in the grid's coordinates as a drop target. The cells divide
 * the grid exactly, gaps included, so a finger resting over a gap lands on one
 * of the two pockets beside it rather than on "nowhere".
 */
export const dropTargetAt = (x: number, y: number, geometry: GridGeometry): DropTarget => {
  // Before the first layout there is no grid to drop on, and dividing by zero
  // would put every point in the first pocket.
  if (geometry.width <= 0 || geometry.height <= 0) return { kind: 'none' }
  if (y < 0 || y > geometry.height) return { kind: 'none' }
  if (x < 0) return { kind: 'previousPage' }
  if (x > geometry.width) return { kind: 'nextPage' }

  return {
    kind: 'pocket',
    pocket: cell(y, geometry.height, rows) * columns + cell(x, geometry.width, columns),
  }
}

/**
 * The position a drop asks the server for, or null when the drop changes
 * nothing and no request should be made. A drag and the pocket's move buttons
 * come through the same two functions, so the two can never disagree about
 * where a card goes.
 */
export const dropDestination = (
  target: DropTarget,
  from: number,
  shape: PageShape,
): number | null => {
  const destination = ((): number | null => {
    switch (target.kind) {
      case 'pocket':
        return pocketDestination(target.pocket, shape)
      case 'previousPage':
        return moveDestination('previousPage', from, shape)
      case 'nextPage':
        return moveDestination('nextPage', from, shape)
      case 'none':
        return null
    }
  })()

  return destination === from ? null : destination
}
