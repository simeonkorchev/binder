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
 * Runs ML Kit text recognition over the camera's frames and hands the
 * recognised text to `onText` on the JS thread.
 *
 * The returned value goes straight to `<Camera frameProcessor={...} />`.
 */
export const useTextFrames = (
  onText: (text: string) => void,
): ReadonlyFrameProcessor => {
  const { scanText } = useTextRecognition()

  // The camera tears down and rebuilds its frame-processor context whenever the
  // processor identity changes, so the worklet must not close over a callback
  // that is a new function on every render. It calls the ref instead, and the
  // ref is what changes.
  const latestOnText = useRef(onText)
  useEffect(() => {
    latestOnText.current = onText
  }, [onText])

  const forwardText = useRunOnJS((text: string) => {
    latestOnText.current(text)
  }, [])

  return useFrameProcessor(
    (frame: Frame) => {
      'worklet'
      runAtTargetFps(scanFramesPerSecond, () => {
        'worklet'
        const { resultText } = scanText(frame)
        if (resultText.length === 0) return
        void forwardText(resultText)
      })
    },
    [forwardText, scanText],
  )
}
