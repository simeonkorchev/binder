import { useRef } from 'react'

/**
 * How many consecutive reads of the same code accept it.
 *
 * ML Kit misreads a glyph on a single frame far more often than on three in a
 * row, so requiring a repeat is what keeps a hallucinated code out of the
 * session. At `scanFramesPerSecond` (4) three reads cost 750 ms — well inside
 * the time a card spends under the lens during a sweep.
 */
export const stableReadsRequired = 3

/**
 * How many consecutive reads without a code before that code counts as having
 * left the guide frame.
 *
 * "Left the frame" has to be defined in **reads, not wall-clock**: the scanner
 * only learns anything when the frame processor hands it a result, and a
 * backgrounded or stalled camera must not age a card out on a timer nobody
 * watched. One read is one processed frame — a frame whose text carried a
 * different code, or none at all, is evidence that this code is no longer under
 * the lens. Three of them in a row (750 ms at 4 fps) is the threshold: a card
 * actually sitting in the frame is read on every pass, so three consecutive
 * misses means the phone moved on.
 */
export const absentReadsToLeaveFrame = 3

interface StableReadState {
  /** The code the current streak is counting. */
  candidate: string | null
  /** How many consecutive reads have now produced `candidate`. */
  streak: number
  /**
   * Accepted codes that have not yet left the frame, each with the number of
   * consecutive reads since it was last seen. A code in here is suppressed:
   * it is the same physical card, still under the lens.
   */
  heldInFrame: ReadonlyMap<string, number>
}

const emptyState = (): StableReadState => ({
  candidate: null,
  streak: 0,
  heldInFrame: new Map<string, number>(),
})

/**
 * Ages every held code by one read and drops the ones that have now left the
 * frame. The code this read carried is the one code that did *not* go unseen,
 * so it is reset rather than aged.
 */
const afterOneRead = (
  heldInFrame: ReadonlyMap<string, number>,
  read: string | null,
): Map<string, number> => {
  const aged = new Map<string, number>()
  for (const [code, readsSinceSeen] of heldInFrame) {
    if (code === read) {
      aged.set(code, 0)
      continue
    }
    const age = readsSinceSeen + 1
    if (age < absentReadsToLeaveFrame) aged.set(code, age)
  }
  return aged
}

interface ReadOutcome {
  state: StableReadState
  /** The code to add to the session, or null — the common case. */
  accepted: string | null
}

/**
 * The whole rule, as a pure function of the previous state and one read.
 *
 * A card is accepted when it has been read `stableReadsRequired` times in a row
 * **and** it is not already held. Holding is what separates the two cases that
 * look identical to the camera:
 *
 * - the same card sitting under the lens, read over and over — one card, and
 *   every read after the first is ignored, or a binder page would land in the
 *   session twenty times;
 * - a second copy of the same card, met later in the sweep — a genuine second
 *   entry, and refusing it would quietly lose a collector's duplicate.
 *
 * The only difference between them is whether the code went away in between,
 * which is what `absentReadsToLeaveFrame` measures.
 */
const observeRead = (state: StableReadState, code: string | null): ReadOutcome => {
  const heldInFrame = afterOneRead(state.heldInFrame, code)

  if (code === null) {
    return { state: { candidate: null, streak: 0, heldInFrame }, accepted: null }
  }

  const streak = code === state.candidate ? state.streak + 1 : 1
  const stillInFrame = state.heldInFrame.has(code)

  const next: StableReadState = { candidate: code, streak, heldInFrame }
  if (stillInFrame || streak < stableReadsRequired) return { state: next, accepted: null }

  heldInFrame.set(code, 0)
  return { state: next, accepted: code }
}

export interface StableReader {
  /**
   * Records one processed frame and returns the code to add to the session, or
   * null when this read changes nothing.
   *
   * Call it **once per frame the processor handled**, passing the parsed code
   * or `null` when the frame carried none: an absent read is half the rule, so
   * a caller that only reports successful parses can never tell a card that
   * left the frame from one that stayed.
   */
  observe: (code: string | null) => string | null
}

/**
 * Turns the noisy per-frame stream of parsed codes into the handful of cards
 * the user actually swept past.
 *
 * The read stream is not render state — it arrives several times a second and
 * nothing on screen draws it — so it lives in a ref and the accepted code is
 * returned to the caller synchronously. Re-rendering the camera four times a
 * second to carry a value the caller already has would be the expensive way to
 * say the same thing.
 */
export const useStableRead = (): StableReader => {
  const state = useRef<StableReadState>(emptyState())

  return {
    observe: (code: string | null): string | null => {
      const outcome = observeRead(state.current, code)
      state.current = outcome.state
      return outcome.accepted
    },
  }
}
