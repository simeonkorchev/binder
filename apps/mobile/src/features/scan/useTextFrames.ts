import { useEffect, useRef } from 'react'
import {
  runAtTargetFps,
  useFrameProcessor,
  type Frame,
  type ReadonlyFrameProcessor,
} from 'react-native-vision-camera'
import { useTextRecognition } from 'react-native-vision-camera-text-recognition'
import { useRunOnJS } from 'react-native-worklets-core'

/**
 * How often ML Kit is allowed to read a frame.
 *
 * The camera delivers 30-60 frames a second and a text recognition pass costs
 * far more than a frame interval, so an unthrottled processor drops frames and
 * drains the battery without reading a single extra code — a card held in the
 * guide frame is there for a second or more. Four passes a second still gives
 * `useStableRead` its consecutive reads well inside that second.
 */
export const scanFramesPerSecond = 4

/**
 * Runs ML Kit text recognition over the camera's frames and hands the result of
 * every processed frame to `onFrameText` on the JS thread.
 *
 * `onFrameText` fires **once per frame the recognition pass actually ran on**,
 * with `null` when ML Kit found no text in it. Reporting the empty frames is
 * not a detail: `lib/useStableRead.ts` decides that a card has left the guide
 * frame by counting the reads that did *not* carry its code, so a card, a blank
 * wall, and then the same card again is one capture instead of two — the
 * collector's second copy — unless the blank frames are reported too. The
 * throttle is what makes this hook the only place that can report them: a frame
 * `runAtTargetFps` skipped was never read and must not count as an absent read,
 * and nothing downstream can tell the two apart.
 *
 * The returned value goes straight to `<Camera frameProcessor={...} />`.
 */
export const useTextFrames = (
  onFrameText: (text: string | null) => void,
): ReadonlyFrameProcessor => {
  const { scanText } = useTextRecognition()

  // The camera tears down and rebuilds its frame-processor context whenever the
  // processor identity changes, so the worklet must not close over a callback
  // that is a new function on every render. It calls the ref instead, and the
  // ref is what changes.
  const latestOnFrameText = useRef(onFrameText)
  useEffect(() => {
    latestOnFrameText.current = onFrameText
  }, [onFrameText])

  const forwardText = useRunOnJS((text: string | null) => {
    latestOnFrameText.current(text)
  }, [])

  return useFrameProcessor(
    (frame: Frame) => {
      'worklet'
      runAtTargetFps(scanFramesPerSecond, () => {
        'worklet'
        const { resultText } = scanText(frame)
        void forwardText(resultText.length === 0 ? null : resultText)
      })
    },
    [forwardText, scanText],
  )
}
