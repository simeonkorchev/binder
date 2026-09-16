import type { ConfigContext, ExpoConfig } from 'expo/config'

// D2 (specs/001-binder-mvp/spec.md): scanning is react-native-vision-camera +
// ML Kit in a frame processor. Both are native modules that Expo Go does not
// ship, so this app is a **dev build** — `npx expo run:android` / `run:ios`, or
// EAS. `expo start` without `--dev-client` will not load it.
//
// W9 owns the scanner. It adds `react-native-vision-camera` to `plugins` below
// (its config plugin is what writes NSCameraUsageDescription and the Android
// CAMERA permission) together with the ML Kit text-recognition frame processor.
// Nothing here pre-declares those permissions: a permission string the app does
// not yet use is one the store asks about and nobody can justify.
export default ({ config }: ConfigContext): ExpoConfig => ({
  ...config,
  name: 'Binder',
  slug: 'binder',
  scheme: 'binder',
  version: '0.0.0',
  orientation: 'portrait',
  userInterfaceStyle: 'automatic',
  // No over-the-air updates in the MVP: the scanner is a native dev build, so a
  // JS-only OTA channel would ship changes the native side has not agreed to.
  updates: { enabled: false },
  ios: {
    bundleIdentifier: 'com.simeonkorchev.binder',
    supportsTablet: false,
  },
  android: {
    package: 'com.simeonkorchev.binder',
  },
  plugins: [
    'expo-dev-client',
    [
      'expo-build-properties',
      {
        // ML Kit's text-recognition models need a higher floor than Expo's
        // default minSdk, and VisionCamera's frame processors need the same on
        // iOS. Set here, once, rather than in a hand-edited android/ or ios/
        // directory — this project stays CNG (no committed native folders).
        android: { minSdkVersion: 26 },
        ios: { deploymentTarget: '16.0' },
      },
    ],
  ],
})
