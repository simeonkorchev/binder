import { renderHook } from '@testing-library/react-native'
import type { Frame } from 'react-native-vision-camera'

import { testFrame } from './testFrame'
import { scanFramesPerSecond, useTextFrames } from './useTextFrames'

/**
 * What the native plugin can actually hand back, which is not what its own
 * `.d.ts` promises. `scanText` is declared `(frame: Frame) => Text` with a
 * required `resultText: string`; the Kotlin returns an empty map when ML Kit
 * found no text, and null when the frame carried no image.
 */
type ScanResult = { resultText?: string } | null

const mockScanText = jest.fn<ScanResult, [Frame]>()
const mockRunAtTargetFps = jest.fn<void, [number, () => void]>()

jest.mock('react-native-vision-camera', () => ({
  useFrameProcessor: (frameProcessor: (frame: Frame) => void) => ({
    frameProcessor,
    type: 'readonly',
  }),
  runAtTargetFps: (fps: number, work: () => void) => mockRunAtTargetFps(fps, work),
}))

jest.mock('react-native-vision-camera-text-recognition', () => ({
  useTextRecognition: () => ({ scanText: mockScanText }),
}))

jest.mock('react-native-worklets-core', () => ({
  useRunOnJS: (callback: (text: string | null) => void) => callback,
}))

describe('useTextFrames', () => {
  beforeEach(() => {
    mockScanText.mockReset()
    mockRunAtTargetFps.mockReset()
    mockRunAtTargetFps.mockImplementation((_fps, work) => {
      work()
    })
    mockScanText.mockReturnValue({ resultText: 'Blue-Eyes White Dragon\nLOB-EN001' })
  })

  // Both of these are the ordinary case, not an edge case: the camera spends
  // most of its frames pointed at a desk, a hand or the gap between two cards.
  // Reading `.length` off the absent `resultText` threw once per frame on a
  // real device — sixty times a second of caught exceptions — and no test could
  // see it, because the mock was typed to return a string it always had.
  it('reports no text when ML Kit found none, rather than throwing on the empty result', async () => {
    mockScanText.mockReturnValue({})
    const onFrameText = jest.fn<void, [string | null]>()
    const { result } = await renderHook(() => useTextFrames(onFrameText))

    result.current.frameProcessor(testFrame())

    expect(onFrameText).toHaveBeenCalledWith(null)
  })

  it('reports no text when the frame carried no image', async () => {
    mockScanText.mockReturnValue(null)
    const onFrameText = jest.fn<void, [string | null]>()
    const { result } = await renderHook(() => useTextFrames(onFrameText))

    result.current.frameProcessor(testFrame())

    expect(onFrameText).toHaveBeenCalledWith(null)
  })

  it('forwards the text ML Kit recognised in a frame', async () => {
    const onFrameText = jest.fn<void, [string | null]>()
    const { result } = await renderHook(() => useTextFrames(onFrameText))

    result.current.frameProcessor(testFrame())

    expect(onFrameText).toHaveBeenCalledWith('Blue-Eyes White Dragon\nLOB-EN001')
  })

  it('asks the camera to run the recognition pass at the throttled rate', async () => {
    const { result } = await renderHook(() => useTextFrames(jest.fn()))

    result.current.frameProcessor(testFrame())

    expect(mockRunAtTargetFps).toHaveBeenCalledWith(scanFramesPerSecond, expect.any(Function))
  })

  it('reports nothing for a frame the throttle rejected: it was never read', async () => {
    mockRunAtTargetFps.mockImplementation(() => undefined)
    const onFrameText = jest.fn<void, [string | null]>()
    const { result } = await renderHook(() => useTextFrames(onFrameText))

    result.current.frameProcessor(testFrame())

    expect(mockScanText).not.toHaveBeenCalled()
    expect(onFrameText).not.toHaveBeenCalled()
  })

  it('reports a processed frame ML Kit found no text in as an absent read', async () => {
    mockScanText.mockReturnValue({ resultText: '' })
    const onFrameText = jest.fn<void, [string | null]>()
    const { result } = await renderHook(() => useTextFrames(onFrameText))

    result.current.frameProcessor(testFrame())

    expect(onFrameText).toHaveBeenCalledWith(null)
  })

  it('reports exactly once per processed frame', async () => {
    const onFrameText = jest.fn<void, [string | null]>()
    const { result } = await renderHook(() => useTextFrames(onFrameText))

    result.current.frameProcessor(testFrame())
    mockScanText.mockReturnValue({ resultText: '' })
    result.current.frameProcessor(testFrame())

    expect(onFrameText.mock.calls).toEqual([['Blue-Eyes White Dragon\nLOB-EN001'], [null]])
  })

  it('forwards to the newest callback without the camera rebuilding the processor', async () => {
    const firstCallback = jest.fn<void, [string | null]>()
    const secondCallback = jest.fn<void, [string | null]>()
    const { result, rerender } = await renderHook(
      ({ onText }: { onText: (text: string | null) => void }) => useTextFrames(onText),
      { initialProps: { onText: firstCallback } },
    )
    const processorFromFirstRender = result.current.frameProcessor

    await rerender({ onText: secondCallback })
    processorFromFirstRender(testFrame())

    expect(secondCallback).toHaveBeenCalledWith('Blue-Eyes White Dragon\nLOB-EN001')
    expect(firstCallback).not.toHaveBeenCalled()
  })
})
