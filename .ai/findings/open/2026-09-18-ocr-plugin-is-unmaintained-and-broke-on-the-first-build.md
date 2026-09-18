# The scanner's OCR plugin is unmaintained and already needs a patch to compile — the first Android build is the evidence

- **Category**: simplify
- **Severity**: high
- **Path**: `apps/mobile/patches/react-native-vision-camera-text-recognition+3.0.5.patch`, `apps/mobile/src/features/scan/`
- **Found**: 2026-09-18 (first EAS Android build, `0a22853`)

The first Android build ever attempted on this project failed compiling
`react-native-vision-camera-text-recognition`:

```
VisionCameraTextRecognitionModule.kt:63 Return type mismatch:
  expected 'HashMap<String, Any>?', actual 'HashMap<String, Any?>'
```

`ReadableMap.toHashMap()` was Java, returning `Map<String, Object>` — a platform
type Kotlin accepted as `HashMap<String, Any>`. React Native 0.86 rewrote it in
Kotlin as `HashMap<String, Any?>`, so the plugin's declared return type stopped
matching what it returns. Patched in `0a22853`; the build gets past it.

**The finding is not that line — it is that there was nowhere to upgrade to.**
`3.1.1` is the latest release and carries the identical signature. npm records
the last publish as **2024-08-12**, two years before the React Native it now has
to compile against. The package has one maintainer and no activity since.

This is not a hypothetical risk that a dependency *might* rot. It rotted, the
first time anyone built it, and the only reason the app compiles is a patch this
repo now owns. D2 in `spec.md` chose VisionCamera + ML Kit; that half is healthy
(`react-native-vision-camera` 4.7.3 is current and its API matched the plugin
exactly). It is only the thin community wrapper between them that is abandoned.

**Nothing in the test suite can catch the next one.** `jest.setup.ts` mocks
`react-native-vision-camera-text-recognition` wholesale, which is correct — it is
a native module — but it means `make check` is green against a package that does
not compile. The signal only exists on a builder, and today that is a 4-minute
EAS round trip.

Not fixed on the spot because option B below is a native-code change that needs
a device to verify, and this container has neither.

## Options

- **A — keep patching.** Cheap per incident (this one was one line), unbounded in
  count, and each SDK bump is a blind 4-minute build to find out. The patch is
  pinned to `+3.0.5`, so any version bump silently stops applying — `patch-package`
  warns, it does not fail, unless `--error-on-fail` is set.
- **B — own the frame processor plugin.** VisionCamera documents writing one
  directly: a Kotlin class extending `FrameProcessorPlugin` registered through
  `FrameProcessorPluginRegistry`, plus the Swift equivalent, calling ML Kit with
  no wrapper in between. Roughly 100 lines per platform, most of it the
  `Text` → map conversion this package already does and which can be lifted from
  it. Costs a local Expo module (the project is CNG, so it would be a config
  plugin, not a committed `android/`). Removes the dependency entirely.
- **Leave it** — accept the patch as permanent and revisit only when it breaks.

## Proposed

**A now, B before the next Expo SDK bump.** The app has never run on a device;
adding native code we have also never run would stack two unverified things.
Once the scanner is proven working on real hardware, B is a contained swap with
a known-good baseline to compare against — and that baseline is exactly what
makes it safe.

Two things to do under A immediately, both cheap:

- Add `--error-on-fail` to the postinstall so a bumped version turns into a
  failed install rather than a build that compiles the unpatched file.
- Pin the dependency exactly (`3.0.5`, not `~3.0.5`), so the patch and the
  package cannot drift apart without someone deciding to.

## Blast radius

Under A: two characters in `package.json` and one flag in the root postinstall;
no source changes, no screenshots. Under B: a new local Expo module, the
scanner's JS binding in `apps/mobile/src/features/scan/`, and `jest.setup.ts`'s
mock retargeted at the new module name. B cannot be verified without a device,
so it is not a container-side task either way.

## Decision (maintainer)
- [ ] A — patch it, pin it exactly, fail the install on a drifted patch
- [ ] B — write our own frame processor plugin and drop the dependency
- [ ] Leave it — the patch is permanent; revisit only when it breaks again
Decided by: <name>, <yyyy-mm-dd>. Notes:
