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
`renderHook` is async for the same reason — `const { result } = await
renderHook(() => useX())`, or `result` is undefined and the failure reads
`Cannot read properties of undefined (reading 'current')`.
Evidence: `apps/mobile/src/App.test.tsx`, `src/theme/useTheme.test.ts` · since 2026-09-16 · verified 2026-09-17

### knip-infers-expo-dependencies-from-app-config
knip's Expo plugin reads `app.config.ts` and demands a dependency for certain
keys: **any** config makes it require `expo-updates` unless `updates.enabled` is
`false`, and `userInterfaceStyle` (or `ios.backgroundColor`) makes it require
`expo-system-ui`. Both surface as `Unlisted dependencies`. Fix the cause — turn
the feature off in the config, or install the package it genuinely needs —
rather than adding the name to `ignoreDependencies`.
Evidence: `apps/mobile/app.config.ts`, `apps/mobile/knip.json` · since 2026-09-16 · verified 2026-09-16

### rendering-anything-under-the-navigator-in-jest
Three things bite in order when a test renders `App` or any screen below it.

`SafeAreaProvider` renders **nothing** until the native view reports its insets,
so the tree is just `<RNCSafeAreaProvider />` and every query fails. `jest.setup.ts`
(wired through `setupFiles`) installs the mock the library ships —
`react-native-safe-area-context/jest/mock` — which supplies a 320×640 frame with
zero insets.

React Navigation's header renders `role="heading"`, which RNTL's
`getByRole('header')` does **not** match; assert the screen another way.

A bottom tab is `role="button"` with the accessible name the library composes
from the title: `Scan, tab, 1 of 3`. That name plus `toBeSelected()` is the
role-first way to assert which destination is open — and because the name is
built from `t('nav.scan')`, it doubles as proof the translation reached the
navigator.
Evidence: `apps/mobile/jest.setup.ts`, `apps/mobile/src/App.test.tsx` (commit 956ef3b) · since 2026-09-17 · verified 2026-09-17

### i18next-use-collides-with-react-19-use
`import { use } from 'i18next'` then `use(initReactI18next)` at module scope
fails lint with *React Hook "use" cannot be called at the top level*
(`react-hooks/rules-of-hooks`): React 19 has a hook named `use`, and the rule
matches on the name. Importing the default instance instead trips
`import/no-named-as-default-member` on `i18n.use(...)`. Alias the named export —
`import { init, use as registerPlugin } from 'i18next'` — rather than suppress
either rule.
Evidence: `apps/mobile/src/i18n/i18n.ts` (commit daaf34c) · since 2026-09-17 · verified 2026-09-17

### knip-reports-an-exported-type-no-other-file-imports
knip's default issue types include unused exported **types**, so a
`export type RootStackParamList` that only its own file uses fails
`npm run knip` even though the code is correct — which is what a type written
for screens that do not exist yet looks like. Give it a real importer: a test
that pins the contract (`src/navigation/AppNavigator.test.ts` builds each
route's params) is one, and it fails to compile when a param is renamed.
Do not reach for `ignoreExportsUsedInFile` — it would silence the check for
every type in the app.
Evidence: `apps/mobile/src/navigation/AppNavigator.test.ts` (commit 956ef3b) · since 2026-09-17 · verified 2026-09-17

### jest-has-a-real-fetch-and-runtime-expo-public-env
Two facts that decide how a mobile API hook is tested here, both the opposite
of what the Expo setup suggests.

`globalThis.fetch` and `Response` are real under `jest-expo`, so a hook that
calls `fetch` is mocked with `const mockFetch: jest.MockedFunction<typeof fetch>
= jest.fn()` plus `globalThis.fetch = mockFetch`, and the answers are real
`new Response(JSON.stringify(body), { status })` objects. A `Response` body
reads **once**: `mockResolvedValue(oneResponse)` passes the first call and fails
the second with a body-already-read error that surfaces as a network failure.
Use `mockImplementation` and build a fresh response per call.

`process.env.EXPO_PUBLIC_*` is **not** inlined at transform time under jest, as
it is in a Metro build — it can be set in `beforeEach` and read at runtime. A
module that reads it at import time cannot be configured by a test; read it
inside the request instead.
Evidence: `apps/mobile/src/features/scan/api/useResolveScan.test.ts` (commit 87833f4) · since 2026-09-17 · verified 2026-09-17

## Navigation, i18n and theme

### where-the-app-shell-lives
Three seams every screen consumes, all under `apps/mobile/src`:

`navigation/AppNavigator.tsx` — `RootTabParamList` (Scan, Binders, Market) and
`RootStackParamList` (`Tabs`, `BinderPage {binderId}`, `SellerContact
{sellerId}`), plus the `ReactNavigation.RootParamList` augmentation that types
`useNavigation()` app-wide. The param list must stay a **type alias**: React
Navigation's `ParamListBase` is an index-signature type and an interface has no
implicit index signature to satisfy it, which is why the augmentation carries an
explained `no-empty-object-type` suppression.

`i18n/` — i18next, `en`/`bg`, language from the device preference list.
`t()` is typed against `en.json` (`i18next.d.ts`), so an unknown key does not
compile, and `locales.test.ts` fails when a key exists in one locale only.

`theme/` — `useTheme(): { name, colors }` following the device appearance, nine
tokens named for the surface they sit on, and `tokens.test.ts` holding the WCAG
matrix and a separation floor between adjacent fills. A new token is added
together with its pairings there.

Every route is registered with `navigation/PlaceholderScreen.tsx`; a screen wave
replaces one registration. Tab icons are deliberately unset — the labels carry
the accessible name, and no icon font is a dependency yet.
Evidence: `apps/mobile/src/navigation/AppNavigator.tsx` (commits e7eb2dd, daaf34c, 956ef3b) · since 2026-09-17 · verified 2026-09-17

## The scan loop

### a-re-read-is-a-second-copy-only-after-the-code-left-the-frame
The scanner reads the same card four times a second, so `lib/useStableRead.ts`
owns two decisions no frame can make. A code is accepted after **three
consecutive identical reads** (ML Kit misreads a glyph on one frame far more
often than on three). Whether seeing it again is the same card or the next copy
of it is decided by whether it **left the frame**, which is measured in reads,
never wall-clock: three consecutive reads that did not carry the code — another
card, or no code at all — and it is forgotten, so the next stable read of it is
a second copy and lands in the session. Under that threshold it is one card
sitting still and every read after the first is ignored. Collectors own
duplicates, so both halves are product requirements, not tuning.

The threshold is in reads because the scanner only learns anything when the
frame processor hands it a result; a stalled or backgrounded camera must not age
a card out on a timer nobody watched. The contract that follows: `observe` must
be called **once per processed frame, with `null` when the frame carried no
code**. `useTextFrames` only reports frames ML Kit found text in, so a screen
that forwards nothing else can never forget a card across a blank view — the
wiring (T046) has to supply the absent reads.
Evidence: `apps/mobile/src/features/scan/lib/useStableRead.ts` (commit b33f820) · since 2026-09-17 · verified 2026-09-17

### the-generated-scan-match-claims-a-card-that-cannot-be-null
huma renders a Go pointer-to-struct field as a plain `$ref` with no null in it,
so `components['schemas']['ScanMatch']` says `card` and `printing` are always
there while `internal/card/api/scan.go` documents the opposite — the card is
null when nothing matched, the printing is null for every rung that resolves no
set. The wire is the source of truth; `features/scan/types.ts` restores the
nullability with `Omit<…> & { card: … | null }`, which keeps the link to the
generated type. Any pointer-to-struct field on any endpoint has the same gap.

`@binder/types` is **not** in `apps/mobile/package.json` and resolves through
the workspace root symlink. `import type` is erased, so nothing is bundled and
knip stays quiet; a **value** import from that package would need the dependency
declared before Metro could resolve it.
Evidence: `apps/mobile/src/features/scan/types.ts` (commit 8559d4c) · since 2026-09-17 · verified 2026-09-17
