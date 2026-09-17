import { useRef, useState } from 'react'
import {
  StyleSheet,
  View,
  type GestureResponderEvent,
  type LayoutChangeEvent,
} from 'react-native'

import {
  columns,
  dropDestination,
  dropTargetAt,
  type DropTarget,
  type GridGeometry,
} from '../lib/dropTarget'
import { addablePocket, pocketsPerPage, type PageShape } from '../lib/positions'
import type { SlotBody } from '../types'

import { BinderPocket } from './BinderPocket'
import { PageEdgeStrip } from './PageEdgeStrip'

/**
 * How long a finger has to rest on a card before moving it picks the card up.
 * Below it the same movement turns the page, which is what a flick across a
 * physical binder does — and it is why a card can be dragged from anywhere on
 * the grid without costing the page its swipe.
 *
 * It is measured with the touches' own timestamps rather than `Date.now()`:
 * the platform stamps the events, so the rest is the time between two real
 * touches and not between two readings of a clock that a test or a suspended
 * app can move independently.
 */
const dragHoldMs = 220
/** How far a finger travels before the gesture commits to being one or the other. */
const decideDistance = 8
/** How far a swipe has to carry to turn the page rather than settle back. */
const swipeDistance = 56
const rows = pocketsPerPage / columns

type GestureMode = 'undecided' | 'drag' | 'swipe'

/** The touch in progress. Never rendered — only the drag it may turn into is. */
interface Gesture {
  /** The platform's own timestamp for the touch that started it, not wall-clock. */
  startedAt: number
  /** Where the touch went down, in the grid's own coordinates. */
  startX: number
  startY: number
  pocket: number | null
  mode: GestureMode
}

/**
 * A card in the air. It carries everything the grid draws while the finger
 * moves — how far the card has travelled, which strip it is over, and the size
 * of a pocket — so that rendering never reads the measurement ref.
 */
interface Drag {
  pocket: number
  dx: number
  dy: number
  edge: Edge
  cellWidth: number
  cellHeight: number
}

type Edge = 'previous' | 'next' | null

/** Which strip, if either, a dragged card is currently over. */
const edgeOf = (target: DropTarget): Edge => {
  if (target.kind === 'previousPage') return 'previous'
  if (target.kind === 'nextPage') return 'next'
  return null
}

interface BinderPageGridProps {
  pockets: (SlotBody | null)[]
  shape: PageShape
  onOpenCard: (slot: SlotBody) => void
  onAddCard: () => void
  /** One move, one position. The grid never walks the pockets it displaced. */
  onMove: (slot: SlotBody, toPosition: number) => void
  onTurnPage: (step: -1 | 1) => void
}

/**
 * The page itself: nine pockets, the two strips a card crosses a page boundary
 * through, and the one gesture that drives both.
 *
 * All touch handling is here rather than on each pocket, so there is no
 * responder negotiation between nine children and their parent: the grid reads
 * where the finger went down, and how long it rested there decides whether the
 * movement carries a card or turns the page. Every move it can express is also
 * a button in `PocketActions` — a card reachable only by dragging is a card
 * some people cannot move at all (003-frontend.md §10).
 *
 * The responder props are the platform's own rather than `PanResponder`: the
 * gesture in progress belongs in a ref, and building a `PanResponder` during
 * render means handing that ref to a function call during render, which
 * `react-hooks/refs` refuses and the rules forbid suppressing. The card in the
 * air is state, so the ghost's offset and the strip under the finger are both
 * derived while rendering instead of being animated from a ref.
 */
export const BinderPageGrid = ({
  pockets,
  shape,
  onOpenCard,
  onAddCard,
  onMove,
  onTurnPage,
}: BinderPageGridProps): React.JSX.Element => {
  const [drag, setDrag] = useState<Drag | null>(null)
  const gridRef = useRef<View>(null)
  // The measurement is a ref and not state: nothing renders differently because
  // the grid was measured, and a `setState` here would be a second render on
  // every layout pass for a value only the touch handlers read. Touches carry
  // page coordinates and land on whichever pocket is under the finger, so the
  // grid's own origin is what makes them comparable.
  const geometry = useRef<GridGeometry>({ width: 0, height: 0 })
  const origin = useRef({ x: 0, y: 0 })
  const gesture = useRef<Gesture | null>(null)

  const addable = addablePocket(shape)

  const pointOf = (event: GestureResponderEvent): { x: number; y: number } => ({
    x: event.nativeEvent.pageX - origin.current.x,
    y: event.nativeEvent.pageY - origin.current.y,
  })

  const activate = (pocket: number): void => {
    const slot = pockets[pocket] ?? null
    if (slot !== null) {
      onOpenCard(slot)
      return
    }
    if (pocket === addable) onAddCard()
  }

  const grant = (event: GestureResponderEvent): void => {
    const point = pointOf(event)
    const target = dropTargetAt(point.x, point.y, geometry.current)

    gesture.current = {
      startedAt: event.nativeEvent.timestamp,
      startX: point.x,
      startY: point.y,
      pocket: target.kind === 'pocket' ? target.pocket : null,
      mode: 'undecided',
    }
  }

  const follow = (event: GestureResponderEvent): void => {
    const current = gesture.current
    if (current === null) return

    const point = pointOf(event)
    if (current.mode === 'undecided') {
      const travelled =
        Math.abs(point.x - current.startX) >= decideDistance ||
        Math.abs(point.y - current.startY) >= decideDistance
      if (!travelled) return

      const carriesCard =
        current.pocket !== null &&
        (pockets[current.pocket] ?? null) !== null &&
        event.nativeEvent.timestamp - current.startedAt >= dragHoldMs
      current.mode = carriesCard ? 'drag' : 'swipe'
    }

    if (current.mode !== 'drag' || current.pocket === null) return
    setDrag({
      pocket: current.pocket,
      dx: point.x - current.startX,
      dy: point.y - current.startY,
      edge: edgeOf(dropTargetAt(point.x, point.y, geometry.current)),
      cellWidth: geometry.current.width / columns,
      cellHeight: geometry.current.height / rows,
    })
  }

  const release = (event: GestureResponderEvent): void => {
    const current = gesture.current
    gesture.current = null
    setDrag(null)
    if (current === null) return

    const point = pointOf(event)
    if (current.mode === 'drag') {
      const slot = current.pocket === null ? null : pockets[current.pocket] ?? null
      if (slot === null) return

      const destination = dropDestination(
        dropTargetAt(point.x, point.y, geometry.current),
        slot.position,
        shape,
      )
      if (destination !== null) onMove(slot, destination)
      return
    }

    if (current.mode === 'swipe') {
      // Carrying the page leftwards brings the next one in, as turning a
      // physical page does.
      if (point.x - current.startX <= -swipeDistance) onTurnPage(1)
      else if (point.x - current.startX >= swipeDistance) onTurnPage(-1)
      return
    }

    if (current.pocket !== null) activate(current.pocket)
  }

  const measure = (event: LayoutChangeEvent): void => {
    const { width, height } = event.nativeEvent.layout
    geometry.current = { width, height }
    gridRef.current?.measureInWindow((x, y) => {
      origin.current = { x, y }
    })
  }

  const draggedSlot = drag === null ? null : pockets[drag.pocket] ?? null

  return (
    <View style={styles.stage}>
      <PageEdgeStrip labelKey="binder.page.previous" isActive={drag?.edge === 'previous'} />

      <View
        ref={gridRef}
        // The grid is one gesture surface with no accessible name of its own —
        // its pockets carry those — so a test reaches it by id.
        testID="binder-page-grid"
        onLayout={measure}
        style={styles.grid}
        onStartShouldSetResponder={() => true}
        onResponderGrant={grant}
        onResponderMove={follow}
        onResponderRelease={release}
        onResponderTerminate={() => {
          gesture.current = null
          setDrag(null)
        }}
      >
        {Array.from({ length: rows }, (_unused, row) => (
          <View key={row} style={styles.row}>
            {Array.from({ length: columns }, (_empty, column) => row * columns + column).map(
              (pocket) => (
                <BinderPocket
                  key={pocket}
                  pocket={pocket}
                  slot={pockets[pocket] ?? null}
                  canAdd={pocket === addable}
                  isLifted={pocket === drag?.pocket}
                  onActivate={() => activate(pocket)}
                />
              ),
            )}
          </View>
        ))}

        {drag !== null && draggedSlot !== null ? (
          <View
            testID="binder-page-ghost"
            pointerEvents="none"
            accessibilityElementsHidden
            importantForAccessibility="no-hide-descendants"
            style={[
              styles.ghost,
              {
                left: (drag.pocket % columns) * drag.cellWidth,
                top: Math.floor(drag.pocket / columns) * drag.cellHeight,
                width: drag.cellWidth,
                height: drag.cellHeight,
                transform: [{ translateX: drag.dx }, { translateY: drag.dy }],
              },
            ]}
          >
            <BinderPocket
              pocket={drag.pocket}
              slot={draggedSlot}
              canAdd={false}
              isLifted={false}
              onActivate={() => activate(drag.pocket)}
            />
          </View>
        ) : null}
      </View>

      <PageEdgeStrip labelKey="binder.page.next" isActive={drag?.edge === 'next'} />
    </View>
  )
}

const styles = StyleSheet.create({
  ghost: { position: 'absolute' },
  grid: { flex: 1 },
  row: { flex: 1, flexDirection: 'row' },
  stage: { flex: 1, flexDirection: 'row' },
})
