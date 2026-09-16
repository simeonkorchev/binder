---
name: gestures
description: "Software Mansion's best practices for gestures in React Native apps using React Native Gesture Handler. Use when implementing tap, pan, pinch, rotation, swipe, long press, fling, hover, drag, or any touch interaction. Trigger on: 'gesture handler', 'GestureDetector', 'tap gesture', 'pan gesture', 'pinch gesture', 'rotation gesture', 'long press', 'fling', 'hover gesture', 'swipe', 'pinch to zoom', 'drag', 'touch handling', 'Pressable', 'RectButton', 'Swipeable', 'Touchable', 'DrawerLayout', 'VirtualGestureDetector', or any request to handle user touch input in a React Native app."
---

# React Native Gesture Handler

Software Mansion's production gesture patterns for React Native using Gesture Handler. Never suggest `PanResponder` when RNGH is available -- it runs on the JS thread and is effectively deprecated.

## Version Decision Tree

```
Check package.json - "react-native-gesture-handler" version
   │
   ├── user asks to migrate v2 -> v3 → a spec decision (`/spec-feature`), never a routine change
   ├── starts with "2." → use builder API (Gesture.Pan(), Gesture.Simultaneous()) — this repo: `~2.28`
   └── starts with "3." → use hook API (usePanGesture(), useSimultaneousGestures())
```

### Key API Differences (v2 vs v3)

| Concept | v2 Builder API | v3 Hook API |
|---------|---------------|-------------|
| Create gesture | `Gesture.Pan().onUpdate(...)` | `usePanGesture({ onUpdate: ... })` |
| Compose (simultaneous) | `Gesture.Simultaneous(a, b)` | `useSimultaneousGestures(a, b)` |
| Compose (race/competing) | `Gesture.Race(a, b)` | `useCompetingGestures(a, b)` |
| Compose (exclusive) | `Gesture.Exclusive(a, b)` | `useExclusiveGestures(a, b)` |
| Activation callback | `.onStart(...)` | `onActivate: ...` |
| Deactivation callback | `.onEnd(...)` | `onDeactivate: ...` |
| Change data | `.onChange(...)` | merged into `onUpdate` (use `changeX`, `changeY`) |
| Cross-component | `.simultaneousWithExternalGesture()` | `.simultaneousWith()` |
| Cross-component | `.requireExternalGestureToFail()` | `.requireToFail()` |
| Cross-component | `.blocksExternalGesture()` | `.block()` |
| Memoization | none — the React Compiler memoizes the builder chain; never `useMemo` (`.claude/rules/003-frontend.md`) | hook input needs memoization |
| SVG / broken hierarchy | `GestureDetector` (may break hierarchy) | `InterceptingGestureDetector` + `VirtualGestureDetector` |
| State manager | callback param `stateManager` | global `GestureStateManager` |
| Buttons | `RectButton`, `BorderlessButton` | `Touchable` |

## Critical Rules

**`GestureHandlerRootView` is mandatory** -- `GestureDetector` will crash at runtime without it as an ancestor. Place it as close to the app root as possible. With Expo Router, wrap `<Stack />` in the root `_layout.tsx`:

```tsx
// app/_layout.tsx
import { Stack } from 'expo-router';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

export default function RootLayout() {
  return (
    <GestureHandlerRootView>
      <Stack />
    </GestureHandlerRootView>
  );
}
```

With React Navigation (no Expo Router), wrap the `<NavigationContainer>` children. With bare React Native, wrap the app root component. Nested `GestureHandlerRootView`s are ignored -- only the topmost instance is used. Default style is `{ flex: 1 }`.

**Gesture objects are plain values** -- both apps compile with the React Compiler, which memoizes the builder chain, so recognizers keep their identity across renders without `useMemo` (the app's reference gesture, `features/progress/components/PhotoCompare.tsx`, has none). Never add one (`.claude/rules/003-frontend.md` anti-patterns). If a recognizer visibly re-attaches, run the compiler probe first: the usual cause is a `react-hooks` eslint suppression making the compiler skip the component.

```tsx
const pan = Gesture.Pan().onBegin(...).onUpdate(...).onEnd(...);
```

**Never call JS-thread functions directly from gesture callbacks** -- when Reanimated is installed, gesture callbacks run on the UI thread (workletized). Calling any non-worklet function (state setters, navigation, audio APIs, native module methods, `useCallback` handlers) directly from a gesture callback crashes with "Tried to synchronously call a non-worklet function on the UI thread". Wrap every JS-thread call in `scheduleOnRN` from `react-native-worklets`:

```tsx
import { scheduleOnRN } from 'react-native-worklets';

// WRONG -- crashes: calling JS function directly from UI thread
const gesture = Gesture.Pan().onUpdate((e) => {
    handleTouch(e.absoluteX, e.absoluteY); // non-worklet function
  });

// CORRECT -- schedules JS function on RN thread
const gesture = Gesture.Pan().onUpdate((e) => {
    scheduleOnRN(handleTouch, e.absoluteX, e.absoluteY);
  });
```

This applies to all gesture callback types including `onTouchesDown`, `onTouchesMove`, `onTouchesUp`, `onStart`, `onUpdate`, `onEnd`, etc. The only code safe to run directly is worklet-compatible code (shared value mutations, other worklet functions).

**Scroll containers** -- import `ScrollView`/`FlatList` from `react-native-gesture-handler`, not `react-native`. Use `RectButton` (v2) or `Touchable` (v3) for tappable items inside scroll containers:

```tsx
import { ScrollView, FlatList, RectButton, Touchable } from 'react-native-gesture-handler';
```

**Never mix React Native touch handlers with RNGH** in the same component tree -- causes double-tap bugs and gesture conflicts. Pick one system per app.

**Callbacks are auto-workletized** -- do not add `'worklet';` to callbacks passed directly (inline) to gesture hooks/builders. The Babel plugin handles this. Only add `'worklet';` to standalone functions assigned to variables before being passed as callbacks.

## References

Load at most one reference file per question. These files are the documentation; do not fetch upstream docs.

| File | When to read |
|------|-------------|
| `gestures.md` | Choosing which gesture type or component to use; callback lifecycle; threading model; `GestureStateManager` for manual activation; SharedValue in gesture config |
| `tap-handling.md` | `RectButton`, `Pressable`, `Touchable`, tappable items in scroll containers, tap gestures, double-tap, hit slop |
| `continuous-gestures.md` | Pan (drag), Pinch (zoom), Rotation, Long press, Fling (swipe), Hover; Reanimated integration patterns; offset accumulation; velocity and decay |
| `gesture-composition.md` | Combining gestures on one component (`Simultaneous`/`Race`/`Exclusive`); cross-component relations; `VirtualGestureDetector` for SVG and Text; Pan inside ScrollView |
| `swipeable-and-drawer.md` | `ReanimatedSwipeable` for list item actions; `ReanimatedDrawerLayout` for side menus; custom swipeable with Pan gesture; web scroll compatibility |
| `testing.md` | Jest setup and mocking; `fireGestureHandler` for testing gestures; common troubleshooting (multiple instances, gesture conflicts, `enabled` timing) |
| `v2-to-v3-migration.md` | Migration guide from v2 to v3; API changes; hook usage; memoization; gesture callbacks |
