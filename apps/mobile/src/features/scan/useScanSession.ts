import { useState } from 'react'
import type { ReadonlyFrameProcessor } from 'react-native-vision-camera'

import { useResolveScan } from './api/useResolveScan'
import { capturedCards, type CapturedCard } from './lib/capturedCards'
import { captureTick } from './lib/captureTick'
import { parseCardCode } from './lib/parseCardCode'
import { useStableRead } from './lib/useStableRead'
import { useScanReview, type ScanReview } from './useScanReview'
import { useTextFrames } from './useTextFrames'

export interface ScanSession {
  /**
   * The cards this sweep has captured, newest first. The running count the
   * strip shows is this list's length, and how many are still waiting is the
   * captures still reading `pending` — neither is forwarded a second time as a
   * number of its own.
   */
  captured: CapturedCard[]
  /**
   * True when the resolver could not be reached. Nothing is lost while it
   * holds: the captures are on screen and the queue is paused, not dropped.
   */
  isOffline: boolean
  /** Sends the paused queue again after the connection came back. */
  retryPending: () => void
  /**
   * What the ladder could not settle, and what the user says about it. This is
   * where US3 is satisfied: the sweep captures cards, the review is where a
   * card with no set stops being a guess.
   */
  review: ScanReview
  /** Goes straight to `<Camera frameProcessor={...} />`. */
  frameProcessor: ReadonlyFrameProcessor
}

/**
 * The scan loop, wired end to end: camera frames in, captured cards out.
 *
 * It exists so the screen can stay layout. Four pieces have to meet for one
 * sweep to work — the throttled frame processor, the code parser, the
 * stable-read rule and the resolve queue — and every one of them is already
 * tested on its own; what is left is the order they are called in, which is
 * where the one bug that loses a card lives.
 *
 * That bug is the absent read. `useStableRead.observe` has to be called **once
 * per processed frame, with `null` for a frame that carried no code**, because
 * counting the frames a code was missing from is how it tells a card sitting
 * under the lens from a second copy of that card met later in the sweep. So the
 * callback below has no early return: a frame with no text is forwarded as
 * `null` rather than ignored, and `useTextFrames` calls it for those frames
 * precisely so that it can be.
 */
export const useScanSession = (): ScanSession => {
  const reader = useStableRead()
  const queue = useResolveScan()
  const review = useScanReview(queue.resolved, queue.rejected)
  const [codes, setCodes] = useState<string[]>([])

  const frameProcessor = useTextFrames((text: string | null) => {
    const accepted = reader.observe(text === null ? null : parseCardCode(text))
    if (accepted === null) return

    setCodes((captured) => [...captured, accepted])
    queue.resolve(accepted)
    void captureTick()
  })

  return {
    captured: capturedCards(codes, queue.resolved, queue.rejected),
    isOffline: queue.isOffline,
    retryPending: queue.retryQueued,
    review,
    frameProcessor,
  }
}
