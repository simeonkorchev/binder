import { renderHook } from '@testing-library/react-native'
import type { Frame } from 'react-native-vision-camera'

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
  useRunOnJS: (callback: (text: string) => void) => callback,
}))

const testFrame = (): Frame => ({
  isValid: true,
  width: 1920,
  height: 1080,
  bytesPerRow: 1920,
  planesCount: 1,
  isMirrored: false,
  timestamp: 1_000,
  orientation: 'portrait',
  pixelFormat: 'yuv',
  toArrayBuffer: () => new ArrayBuffer(0),
  toString: () => '1920 x 1080 Frame',
  getNativeBuffer: () => {
    throw new Error('the scanner never reads the native buffer')
  },
})

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
    const onText = jest.fn<void, [string]>()
    const { result } = await renderHook(() => useTextFrames(onText))

    result.current.frameProcessor(testFrame())

    expect(onText).toHaveBeenCalledWith('Blue-Eyes White Dragon\nLOB-EN001')
  })

  it('asks the camera to run the recognition pass at the throttled rate', async () => {
    const { result } = await renderHook(() => useTextFrames(jest.fn()))

    result.current.frameProcessor(testFrame())

    expect(mockRunAtTargetFps).toHaveBeenCalledWith(scanFramesPerSecond, expect.any(Function))
  })

  it('does not run the recognition pass on a frame the throttle rejected', async () => {
    mockRunAtTargetFps.mockImplementation(() => undefined)
    const onText = jest.fn<void, [string]>()
    const { result } = await renderHook(() => useTextFrames(onText))

    result.current.frameProcessor(testFrame())

    expect(mockScanText).not.toHaveBeenCalled()
    expect(onText).not.toHaveBeenCalled()
  })

  it('ignores a frame ML Kit found no text in', async () => {
    mockScanText.mockReturnValue({ resultText: '' })
    const onText = jest.fn<void, [string]>()
    const { result } = await renderHook(() => useTextFrames(onText))

    result.current.frameProcessor(testFrame())

    expect(onText).not.toHaveBeenCalled()
  })

  it('forwards to the newest callback without the camera rebuilding the processor', async () => {
    const firstCallback = jest.fn<void, [string]>()
    const secondCallback = jest.fn<void, [string]>()
    const { result, rerender } = await renderHook(
      ({ onText }: { onText: (text: string) => void }) => useTextFrames(onText),
      { initialProps: { onText: firstCallback } },
    )
    const processorFromFirstRender = result.current.frameProcessor

    await rerender({ onText: secondCallback })
    processorFromFirstRender(testFrame())

    expect(secondCallback).toHaveBeenCalledWith('Blue-Eyes White Dragon\nLOB-EN001')
    expect(firstCallback).not.toHaveBeenCalled()
  })
})
