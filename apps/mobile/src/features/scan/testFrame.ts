import type { Frame } from 'react-native-vision-camera'

/**
 * One camera frame, as the frame processor receives it.
 *
 * A test fixture, shared by the scan tests so that "a frame" means one thing:
 * the scanner reads nothing off the frame itself — the recognition pass is what
 * looks at it — so the values only have to be a coherent portrait frame.
 */
export const testFrame = (): Frame => ({
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
