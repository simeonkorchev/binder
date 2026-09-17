import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Animated, PanResponder, StyleSheet, Text, View, type LayoutChangeEvent } from 'react-native'

import { useTheme } from '@/theme/useTheme'

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

/**
 * How long a finger has to rest on a card before moving it picks the card up.
 * Below it the same movement turns the page, which is what a flick across a
 * physical binder does.
 */
const dragHoldMs = 220
/** How far a finger travels before the gesture commits to being one or the other. */
const decideDistance = 8
/** How far a swipe has to carry to turn the page rather than settle back. */
const swipeDistance = 56

type GestureMode = 'undecided' | 'drag' | 'swipe'

interface Gesture {
  startedAt: number
  /** Where the touch went down, in the grid's own coordinates. */
  x: number
  y: number
  pocket: number | null
  mode: GestureMode
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
 */
export const BinderPageGrid = ({
  pockets,
  shape,
  onOpenCard,
  onAddCard,
  onMove,
  onTurnPage,
}: BinderPageGridProps): React.JSX.Element => {
  const [geometry, setGeometry] = useState<GridGeometry>({ width: 0, height: 0 })
  const [draggedPocket, setDraggedPocket] = useState<number | null>(null)
  const [hoveredEdge, setHoveredEdge] = useState<Edge>(null)
  const translation = useRef(new Animated.ValueXY()).current
  const gridRef = useRef<View>(null)
  // Where the grid sits on the screen. Touches carry page coordinates, and the
  // node that receives one is whichever pocket is under the finger, so the
  // grid's own origin is the only thing that makes them comparable.
  const origin = useRef({ x: 0, y: 0 })
  const gesture = useRef<Gesture | null>(null)

  const addable = addablePocket(shape)

  const activate = (pocket: number): void => {
    const slot = pockets[pocket] ?? null
    if (slot !== null) {
      onOpenCard(slot)
      return
    }
    if (pocket === addable) onAddCard()
  }

  const endGesture = (): void => {
    gesture.current = null
    translation.setValue({ x: 0, y: 0 })
    setDraggedPocket(null)
    setHoveredEdge(null)
  }

  const pan = PanResponder.create({
    onStartShouldSetPanResponder: () => true,
    onPanResponderGrant: (event) => {
      const x = event.nativeEvent.pageX - origin.current.x
      const y = event.nativeEvent.pageY - origin.current.y
      const target = dropTargetAt(x, y, geometry)

      gesture.current = {
        startedAt: Date.now(),
        x,
        y,
        pocket: target.kind === 'pocket' ? target.pocket : null,
        mode: 'undecided',
      }
      translation.setValue({ x: 0, y: 0 })
    },
    onPanResponderMove: (_event, state) => {
      const current = gesture.current
      if (current === null) return

      if (current.mode === 'undecided') {
        if (Math.abs(state.dx) < decideDistance && Math.abs(state.dy) < decideDistance) return

        const carriesCard =
          current.pocket !== null &&
          (pockets[current.pocket] ?? null) !== null &&
          Date.now() - current.startedAt >= dragHoldMs
        current.mode = carriesCard ? 'drag' : 'swipe'
        if (carriesCard) setDraggedPocket(current.pocket)
      }

      if (current.mode !== 'drag') return
      translation.setValue({ x: state.dx, y: state.dy })

      setHoveredEdge(edgeOf(dropTargetAt(current.x + state.dx, current.y + state.dy, geometry)))
    },
    onPanResponderRelease: (_event, state) => {
      const current = gesture.current
      if (current === null) return

      if (current.mode === 'drag') {
        const slot = current.pocket === null ? null : pockets[current.pocket] ?? null
        const target = dropTargetAt(current.x + state.dx, current.y + state.dy, geometry)
        endGesture()
        if (slot === null) return

        const destination = dropDestination(target, slot.position, shape)
        if (destination !== null) onMove(slot, destination)
        return
      }

      const pocket = current.pocket
      const swiped = current.mode === 'swipe'
      endGesture()

      if (swiped) {
        // Carrying the page leftwards brings the next one in, as turning a
        // physical page does.
        if (state.dx <= -swipeDistance) onTurnPage(1)
        else if (state.dx >= swipeDistance) onTurnPage(-1)
        return
      }
      if (pocket !== null) activate(pocket)
    },
    onPanResponderTerminate: endGesture,
  })

  const measure = (event: LayoutChangeEvent): void => {
    const { width, height } = event.nativeEvent.layout
    setGeometry({ width, height })
    gridRef.current?.measureInWindow((x, y) => {
      origin.current = { x, y }
    })
  }

  const draggedSlot = draggedPocket === null ? null : pockets[draggedPocket] ?? null

  return (
    <View style={styles.stage}>
      <EdgeStrip labelKey="binder.page.previous" isActive={hoveredEdge === 'previous'} />

      <View ref={gridRef} onLayout={measure} style={styles.grid} {...pan.panHandlers}>
        {[0, 1, 2].map((row) => (
          <View key={row} style={styles.row}>
            {Array.from({ length: columns }, (_unused, column) => row * columns + column).map(
              (pocket) => (
                <BinderPocket
                  key={pocket}
                  pocket={pocket}
                  slot={pockets[pocket] ?? null}
                  canAdd={pocket === addable}
                  isLifted={pocket === draggedPocket}
                  onActivate={() => activate(pocket)}
                />
              ),
            )}
          </View>
        ))}

        {draggedPocket !== null && draggedSlot !== null ? (
          <Animated.View
            pointerEvents="none"
            importantForAccessibility="no-hide-descendants"
            accessibilityElementsHidden
            style={[
              styles.ghost,
              {
                left: (draggedPocket % columns) * (geometry.width / columns),
                top: Math.floor(draggedPocket / columns) * (geometry.height / (pocketsPerPage / columns)),
                width: geometry.width / columns,
                height: geometry.height / (pocketsPerPage / columns),
                transform: translation.getTranslateTransform(),
              },
            ]}
          >
            <BinderPocket
              pocket={draggedPocket}
              slot={draggedSlot}
              canAdd={false}
              isLifted={false}
              onActivate={() => activate(draggedPocket)}
            />
          </Animated.View>
        ) : null}
      </View>

      <EdgeStrip labelKey="binder.page.next" isActive={hoveredEdge === 'next'} />
    </View>
  )
}

interface EdgeStripProps {
  labelKey: 'binder.page.previous' | 'binder.page.next'
  /** True while a dragged card is over the strip, so the drop reads as armed. */
  isActive: boolean
}

/** The page boundary, made droppable: the only way a drag reaches a page the grid is not showing. */
const EdgeStrip = ({ labelKey, isActive }: EdgeStripProps): React.JSX.Element => {
  const { t } = useTranslation()
  const { colors } = useTheme()

  return (
    <View
      style={[styles.strip, { backgroundColor: isActive ? colors.accent : colors.surfaceMuted }]}
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
    >
      {isActive ? (
        <Text style={[styles.stripLabel, { color: colors.onAccent }]} numberOfLines={3}>
          {t(labelKey)}
        </Text>
      ) : null}
    </View>
  )
}

const styles = StyleSheet.create({
  ghost: { position: 'absolute' },
  grid: { flex: 1 },
  row: { flexDirection: 'row', flex: 1 },
  stage: { flexDirection: 'row', flex: 1 },
  strip: { borderRadius: 6, justifyContent: 'center', marginVertical: 5, paddingHorizontal: 2, width: 24 },
  stripLabel: { fontSize: 9, fontWeight: '700', textAlign: 'center' },
})
