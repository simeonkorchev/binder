# Mobile (apps/mobile) memory

## Toolchain

### expo-install-cannot-resolve-versions-here
`npx expo install` and `npx expo install --check` call `api.expo.dev`, which
this project's containers cannot reach (`HTTP Proxy Network Error: Forbidden`).
So the usual "let Expo pick the version" loop is unavailable: read the SDK's own
table instead — `node_modules/expo/bundledNativeModules.json` maps every Expo
package to the range this SDK expects — and write that range into
`apps/mobile/package.json` by hand.

`make expo-check` exists for a machine that does have egress; it is deliberately
**not** in `make check`, because a gate that cannot pass where the work happens
is not a gate.
Evidence: `Makefile` (expo-check) · since 2026-09-16 · verified 2026-09-16

## Testing

### rntl-14-render-is-async-and-needs-test-renderer
`@testing-library/react-native` v14 dropped `react-test-renderer` for the
`test-renderer` package (a peer dependency) and made `render` **async**. A test
written the v13 way fails with `render function has not been called` from the
`screen` proxy — `render` returned a promise, so nothing was ever mounted.
Write `await render(<X />)` and keep `react-test-renderer` out of the tree.
Evidence: `apps/mobile/src/App.test.tsx` · since 2026-09-16 · verified 2026-09-16

### knip-infers-expo-dependencies-from-app-config
knip's Expo plugin reads `app.config.ts` and demands a dependency for certain
keys: **any** config makes it require `expo-updates` unless `updates.enabled` is
`false`, and `userInterfaceStyle` (or `ios.backgroundColor`) makes it require
`expo-system-ui`. Both surface as `Unlisted dependencies`. Fix the cause — turn
the feature off in the config, or install the package it genuinely needs —
rather than adding the name to `ignoreDependencies`.
Evidence: `apps/mobile/app.config.ts`, `apps/mobile/knip.json` · since 2026-09-16 · verified 2026-09-16
