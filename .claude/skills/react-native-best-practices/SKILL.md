---
name: react-native-best-practices
description: House guidance (forked from Software Mansion, edited to .claude/rules) for the React Native libraries client-mobile uses - Reanimated 4 (animations), Gesture Handler v2 (gestures) and react-native-svg. Use when writing, refactoring or reviewing apps/client-mobile code that imports react-native-reanimated, react-native-gesture-handler or react-native-svg.
---

# React Native best practices (ours)

Three references forked from Software Mansion's `react-native-best-practices`
(MIT; attribution in `.claude/skills/VENDOR.md`) and edited to this repo's
rules. Never refreshed from upstream — this text is ours, and it agrees with
`.claude/rules/`.

Read at most one topic per task, then at most one reference file inside it.

| Topic | Read when the slice touches | Entry point |
|-------|-----------------------------|-------------|
| Animations | `react-native-reanimated` (shared values, `useAnimatedStyle`, layout animations, `scheduleOnRN`) | `references/animations/SKILL.md` |
| Gestures | `react-native-gesture-handler` (`Gesture.Pan()`, `GestureDetector`, composition, gesture tests) | `references/gestures/SKILL.md` |
| SVG | `react-native-svg` | `references/svg/SKILL.md` |

Project versions these were written against: reanimated `~4.1`, gesture
handler `~2.28` (v2 builder API, so ignore the v3 hook-API column), svg `15.12`.

## Conventions the references follow

They are advisory on **library usage**; `.claude/rules/003-frontend.md`,
`.claude/rules/005-mobile.md` and `apps/client-mobile/CLAUDE.md` define every
project convention (headless hooks, `StyleSheet.create`, theme tokens, file
size, testing). Points where the original upstream text differed, already
edited into the references:

- **No `useMemo` around gesture builders or animation builders** — the React
  Compiler memoizes; the app's reference gesture
  (`apps/client-mobile/src/features/progress/components/PhotoCompare.tsx`) has
  none. If a recognizer visibly re-attaches, run the compiler probe first.
- **Static styles in `StyleSheet.create()`**; an animated style (`useAnimatedStyle`
  result or shared value) is the one thing passed inline.
- **TypeScript strict, no `any`.**
- **Migrations** (v2 → v3 gesture API, `PanResponder` → Gesture Handler,
  `runOnJS` → `scheduleOnRN`) change behavior: a spec or a `/fix-findings`
  item, never `/fe-refactor`. The v3 column in the gestures reference is for
  reading only; the app is on `~2.28`.
- **No fetching upstream docs**; the references are the documentation.
