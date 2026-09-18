import type { ConfigContext, ExpoConfig } from 'expo/config'

// D2 (specs/001-binder-mvp/spec.md): scanning is react-native-vision-camera +
// ML Kit in a frame processor. Both are native modules that Expo Go does not
// ship, so this app is a **dev build** — `npx expo run:android` / `run:ios`, or
// EAS. `expo start` without `--dev-client` will not load it.
//
// `react-native-vision-camera`'s config plugin (below) is what writes
// NSCameraUsageDescription and the Android CAMERA permission; the ML Kit
// text-recognition models are declared by
// `react-native-vision-camera-text-recognition`'s own AndroidManifest
// (`com.google.mlkit.vision.DEPENDENCIES`), so it needs no plugin entry.
// The camera is the only permission the app asks for.
export default ({ config }: ConfigContext): ExpoConfig => ({
  ...config,
  name: 'Binder',
  slug: 'binder',
  // Two schemes: the app's own, and the application id. Google returns an
  // installed client's authorization code to a redirect built from the app id
  // (`com.simeonkorchev.binder:/oauthredirect`, which is what
  // `features/auth/lib/googleIdentity.ts` asks `makeRedirectUri` for), and the
  // OS only hands that URL back to an app that has registered the scheme.
  scheme: ['binder', 'com.simeonkorchev.binder'],
  version: '0.0.0',
  orientation: 'portrait',
  userInterfaceStyle: 'automatic',
  // No over-the-air updates in the MVP: the scanner is a native dev build, so a
  // JS-only OTA channel would ship changes the native side has not agreed to.
  updates: { enabled: false },
  ios: {
    bundleIdentifier: 'com.simeonkorchev.binder',
    supportsTablet: false,
    // The Sign in with Apple entitlement. Without it the system sheet refuses
    // at runtime, and Apple sign-in is not optional once Google is offered
    // (D1) — so this is what makes the App Store requirement buildable.
    usesAppleSignIn: true,
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
        // 16.4, not 16.0: expo-build-properties on SDK 57 rejects anything lower
        // ("ios.deploymentTarget needs to be at least version 16.4") and no test
        // sees it — the config plugin only runs during prebuild/export/run, so
        // `npx expo export` is what catches it.
        ios: { deploymentTarget: '16.4' },
      },
    ],
    [
      'react-native-vision-camera',
      {
        // Baked into the native manifests at prebuild, so it is the one
        // user-facing string the app's `t()` catalogue cannot own. Localising
        // it means `expo-localization`'s Info.plist strings — a later decision.
        cameraPermissionText:
          'Binder uses the camera to read the code printed on your cards.',
        // The scanner reads text only: no audio, no location, and no QR/barcode
        // model (~2.4 MB) the app would never call.
        enableMicrophonePermission: false,
        enableLocation: false,
        enableCodeScanner: false,
      },
    ],
  ],
})
