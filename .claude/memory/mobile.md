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

### every-navigator-test-loads-the-scanners-native-modules
A test that renders `App` fails at **import** with `system/camera-module-not-found:
Failed to initialize VisionCamera`, before any test body runs: the Scan tab is
the app's first screen, so the whole scanner — VisionCamera, its ML Kit text
recognition, worklets-core and expo-haptics — is loaded by every test that
mounts the navigator, and none of those native modules exist under jest.

The four mocks live in `apps/mobile/jest.setup.ts` next to the SafeAreaProvider
one, at their thinnest: a permission nobody has answered yet
(`not-determined`), no capture device, a frame processor nobody feeds, and
haptics that resolve. That default is deliberate — a screen test sees the
permission explainer, which is what a fresh phone shows. A test about the scan
loop itself overrides them with its own `jest.mock` in the file, which wins
over the setup mock.
Evidence: `apps/mobile/jest.setup.ts`, `apps/mobile/src/App.test.tsx` (commit a315c50) · since 2026-09-17 · verified 2026-09-17

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

### binder-types-resolves-through-the-workspace-symlink
`@binder/types` is **not** in `apps/mobile/package.json` and resolves through
the workspace root symlink. `import type` is erased, so nothing is bundled and
knip stays quiet; a **value** import from that package would need the dependency
declared before Metro could resolve it.

The generated types tell the truth about null since 2026-09-17, so a feature's
`types.ts` aliases `components['schemas'][…]` directly. **Never re-add
nullability on the client** with `Omit<generated, …> & { … }`: the scan feature
carried exactly that correction for `card`, `printing` and `candidates`, and it
was a second source of truth for a fact the first one already stated. A
generated type that looks wrong is a bug in `pkg/humaschema`, fixed there and
regenerated (go.md#huma-documents-three-shapes-as-the-wrong-nullability).
Evidence: `apps/mobile/src/features/scan/types.ts` (commit 1de05ff) · since 2026-09-17 · verified 2026-09-17

### the-absent-reads-are-reported-by-usetextframes-not-by-the-screen
`useStableRead.observe` must be called once per processed frame with `null`
when the frame carried no code, and the place that can honour it is
`features/scan/useTextFrames.ts` — it reports **every** frame the recognition
pass ran on, empty ones as `null`. It is the only place that can: a frame
`runAtTargetFps` skipped was never read and must not age a card out, a frame
that was read and held no text must, and nothing downstream can tell those two
apart. `useScanSession` therefore has no early return on an empty frame.

The failure this prevents is silent — a card, a blank wall, then the same card
again captures **once** instead of twice, and the collector's second copy is
gone with no error anywhere. The test that pins it drives real frames through
the real chain (`useScanSession.test.ts`, "captures a second copy met after the
first left the frame"); putting the old `if (resultText.length === 0) return`
back turns it red at 1 instead of 2.
Evidence: `apps/mobile/src/features/scan/useTextFrames.ts` (commit 1ce85b1), `apps/mobile/src/features/scan/useScanSession.test.ts` (commit 6af3612) · since 2026-09-17 · verified 2026-09-17

### a-camera-refusal-has-two-ways-out-and-the-status-only-tells-them-apart-after-asking
A denied camera permission can be asked for again; a blocked one can only be
changed in the settings app. VisionCamera reports `not-determined` exactly
while the OS is still willing to show the dialog — on Android
`CameraViewModule.getPermission` maps a denied permission back to
`not-determined` for as long as `shouldShowRequestPermissionRationale` holds —
so the status read **after** a refusal is what separates them.

The status read **before** asking cannot, and this is the trap: Android
answers `denied` both for a permission that was never requested (the rationale
flag is false until the first ask) and for one blocked forever. Reading it as
"blocked" sends a first-run user to the settings app for a dialog they were
never shown. `features/scan/useCameraAccess.ts` therefore trusts only
`granted` at mount, sends every other start to the explainer, and classifies
the refusal afterwards. The permission also changes while the app is
suspended, so the status is re-read on resume — and only a grant is acted on
there, because whether the dialog will open again is something the request
tells us and the status does not.
Evidence: `apps/mobile/src/features/scan/useCameraAccess.ts` (commit a48869b) · since 2026-09-17 · verified 2026-09-17

### the-capture-tick-uses-androids-view-haptics-to-stay-at-one-permission
`expo-haptics`' `impactAsync` drives Android's `Vibrator`, which needs the
`VIBRATE` permission in the manifest; `performAndroidHapticsAsync` uses the
view's haptic feedback and needs none. `app.config.ts` states that the camera
is the only permission this app asks for, so `lib/captureTick.ts` branches on
`Platform.OS` to keep that true. A failure is swallowed — a phone with no
haptic engine must not take a captured card down with it.
Evidence: `apps/mobile/src/features/scan/lib/captureTick.ts` (commit 6af3612) · since 2026-09-17 · verified 2026-09-17

### where-the-scan-screen-layer-lives
`features/scan/ScanScreen.tsx` (a default export, registered on the `Scan` tab
in `navigation/AppNavigator.tsx`) is layout only and branches three ways:
`CameraAccessNotice` when access is not granted, a translated line when
`useCameraDevice('back')` finds nothing, and the viewfinder otherwise.
`useScanSession` is the whole loop behind it — frame processor, parser, stable
read, resolve queue — and `useCameraAccess` owns the permission.

Two things in there are easy to lose. `isActive={useIsFocused()}`: a bottom tab
keeps its screens mounted, so without it the camera keeps reading frames while
the user is in a binder. And `lib/capturedCards.ts` joins captures to answers
**by code, first answer per code**, keyed `${index}:${code}` — the code is not
unique, because capturing it twice is the collector's second copy.
Evidence: `apps/mobile/src/features/scan/ScanScreen.tsx` (commit a315c50) · since 2026-09-17 · verified 2026-09-17

## The review sheet

### the-ladders-unresolved-means-two-different-things
`ScanMatch.resolution` cannot be read as "why this scan is flagged", and a
review UI that switches on it silently turns a one-tap choice into a search.
`internal/card/service/match.go` answers `unresolved` for **two** unrelated
outcomes: a code rung that matched *several* printings (`ambiguousPrintings` —
candidates, plus the card itself when every candidate is a printing of one
card), and a ladder that matched *nothing at all* (`unresolved(nil)`). The name
rung's tie is the first kind too.

`lib/flaggedScans.ts` therefore classifies on the answer's **shape**, in this
order: non-empty `candidates` → the user picks; no `card` → nothing matched; no
`printing` → the `by_name` case, a card whose set is unknown. Only after those
three is a scan settled. A refused scan from `useResolveScan`'s `rejected` list
is the fourth flag, and it is a row like any other — the queue keeps refusals
precisely so a swept card is never silently gone, and a list nobody shows is
the same as dropping it.

The five shapes the ladder can actually produce are the fixtures at the top of
`lib/flaggedScans.test.ts`; start there before changing the classifier.
Evidence: `apps/mobile/src/features/scan/lib/flaggedScans.ts` (commit 4e45e80) · since 2026-09-17 · verified 2026-09-17

### the-review-sheet-is-a-modal-over-the-scan-tab-not-a-route
`components/ReviewSheet.tsx` is a React Native `Modal` the scan screen renders,
not a `RootStackParamList` route. The session it reviews — the queue's answers
and the user's decisions — lives in the hooks `ScanScreen` holds, so a pushed
route would have to carry that state through navigation params and a live sweep
would become a snapshot taken when the route opened. The screen owns only
`isReviewOpen`; the sheet owns only which row is being searched, so exactly one
`useCardSearch` is ever mounted.

`useScanReview(resolved, rejected)` holds **only** the decisions and derives the
flagged rows every render, because the queue can still be draining while the
sheet is open. Decisions are keyed by **code**, matching the one-question-per-
code rule `flaggedScans` and `capturedCards` both group on: one decision settles
every copy of that card the sweep captured, and `copies` says how many.

`GET /cards?q=` answers with cards and never printings (`SearchCardsBody.cards`
is `ScanCard[]`), so a card corrected through the name search is filed with its
set still unknown. That is the truthful outcome, and the one a binder slot with
a null `card_printing_id` records.
Evidence: `apps/mobile/src/features/scan/components/ReviewSheet.tsx` (commits f82fc88, 96d495c) · since 2026-09-17 · verified 2026-09-17

### committing-a-scanned-session-has-no-atomic-endpoint
Do not write a client-side loop over `POST /binders/{binderId}/slots` to put a
sweep into a binder. Every binder write the contract publishes is single-row and
`service.AddSlot` opens its own `InTx` per call, so a 60-card sweep is 61
requests and a failure at card 40 leaves a binder holding 39 cards — after the
user has put the cards away. T048 is blocked on a batch endpoint, which is a new
operation across `api → service → store` and so a spec-pipeline change; the
details and the decision mapping are in finding
2026-09-17-no-atomic-way-to-commit-a-scanned-session.
Evidence: `packages/types/openapi.json`, `internal/binder/service/slot.go` · since 2026-09-17 · verified 2026-09-17
