// `SafeAreaProvider` renders nothing until the native view reports its insets,
// so without this every test of a screen under it sees an empty tree. The
// library ships this mock for exactly that reason; it supplies a 320×640 frame
// with zero insets.
jest.mock('react-native-safe-area-context', () =>
  jest.requireActual<{ default: unknown }>('react-native-safe-area-context/jest/mock').default,
)

// The scan tab is the app's first screen, so **every** test that renders the
// navigator loads the scanner's native modules — and none of them exist under
// jest: VisionCamera throws `system/camera-module-not-found` at import time,
// before any test body runs. These are the four, mocked at their thinnest: a
// camera that has not been asked for permission yet (so a screen test sees the
// explainer, which is what a phone with no answer yet would show), no capture
// device, and a frame processor nobody feeds. A test about the scan loop
// itself overrides them with its own `jest.mock` in the file.
jest.mock('react-native-vision-camera', () => {
  const Camera = (): null => null
  Camera.getCameraPermissionStatus = (): string => 'not-determined'
  Camera.requestCameraPermission = (): Promise<string> => Promise.resolve('denied')
  return {
    Camera,
    useCameraDevice: () => undefined,
    useFrameProcessor: (frameProcessor: unknown) => ({ frameProcessor, type: 'readonly' }),
    runAtTargetFps: (_fps: number, work: () => void) => {
      work()
    },
  }
})

jest.mock('react-native-vision-camera-text-recognition', () => ({
  useTextRecognition: () => ({ scanText: () => ({ resultText: '' }) }),
}))

jest.mock('react-native-worklets-core', () => ({
  useRunOnJS: (callback: unknown) => callback,
}))

jest.mock('expo-haptics', () => ({
  AndroidHaptics: { Confirm: 'confirm' },
  ImpactFeedbackStyle: { Light: 'light' },
  impactAsync: () => Promise.resolve(),
  performAndroidHapticsAsync: () => Promise.resolve(),
}))
