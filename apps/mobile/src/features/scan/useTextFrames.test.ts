import { renderHook } from '@testing-library/react-native'
import type { Frame } from 'react-native-vision-camera'

import { testFrame } from './testFrame'
import { scanFramesPerSecond, useTextFrames } from './useTextFrames'

const mockScanText = jest.fn<{ resultText: string }, [Frame]>()
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
