---
paths:
  - "apps/mobile/**"
---

# Mobile (client-mobile) Rules

## Client user modes

| Mode | Condition | Behaviour |
|------|-----------|-----------|
| **Flex** | No program (`isFlexUser=true`) | No targets. 3 template slots rendered. `slot_order` always set to `mealTarget.mealOrder`. |
| **Daily-target-only** | Has `meal_daily_macro_targets`, no `meal_macro_targets` | Daily goal shown. 3 template slots synthesised by `useDashboard`. `slot_order` stamped on every meal. |
| **Structured** | Has both target types | Named slots from `MealTargets`. `slot_order=mealTarget.mealOrder` stamped on every meal. |

## `slot_order` on meal creation

**`slot_order` is 1-based.** It mirrors `MealTargets.mealOrder`, which the diary uses both to match a meal to its slot and to name it ("Meal 3"). `lib/mealSlotOrder.ts` holds the rule: `FIRST_MEAL_SLOT` and `nextMealSlotOrder(meals)`. Use it wherever a slot has to be derived rather than taken from a target — do not open-code `max + 1` or a `?? 0` default.

All entry points always stamp `slot_order = mealTarget.mealOrder`. There is no flex/null path.

| Entry point | Behaviour |
|-------------|-----------|
| `MealSlotCard` | `mealTarget.mealOrder` |
| `ManualMealBuilder` | `mealTarget.mealOrder` |
| `MealLoggerSheet` | `selectedTarget.mealOrder` |
| Text modal (Dashboard) | `target.mealOrder` |
| Barcode (Dashboard) | `effectiveTarget.mealOrder` |
| Reminder quick-log reply | `nextMealSlotOrder(day's meals)` — no target to read |

Meals with `slot_order=null` (logged before this change) are matched to slots positionally (Pass 2 in `mealForTarget` — no migration needed).

**A derived slot continues after the last logged meal, it does not backfill.** `nextMealSlotOrder` returns `max(taken) + 1`, so a reply on a day whose only meal is dinner gets slot 4 rather than the free slot 1. This is a decision, not an oversight: nothing on the reminder path knows the time of day, and first-free-slot would file a late-evening reply as the client's breakfast. The cost — a card beyond the plan while earlier slots sit empty — is accepted. Revisit only with a rule that can tell which meal of the day a reply belongs to.

**An out-of-range `slot_order` fails silently.** Pass 1 of `mealForTarget` finds no slot, Pass 2 only rescues `null`, so the meal is appended by `allSortedMealTargets` as a synthetic slot numbered by *position*. The user reads that position as the meal's name, so a wrong value surfaces as a plausible-looking but wrong meal number rather than as an error. A quick-logged meal reached production filed as "Meal 4" on an empty day this way. When touching either side, extend `features/reminders/quickLogSlotContract.test.ts` — it runs the write path into the read path, which is where this class of bug lives.

## Day-type assignment

`POST /api/v1/days/{date}/initialize` called for all 7 days on mount/week change.
- Stores `initDayTargetIds` (date → dailyTargetId | null); `null` = flex day, `undefined` = not yet initialised
- `initDayTypes` set only when user explicitly picks via `DayTypeSheet`
- Default day type: **training** when no prior user choice

## Data fetching

- **Scope query keys to the widest useful range, not to the current selection.** Use `weekStart` (not `selectedDate`) for data that covers the whole week (e.g. meals). Per-date keys fire a new request on every tap.
- **Exception — `dailyTargets` must be keyed by `selectedDate`, not `weekStart`.** The backend SQL returns targets for the meal plan *active on* the given date (`WHERE start_date <= $date`). If a program changes mid-week, `weekStart` returns the old plan's targets for all days, showing wrong macros and potentially causing blank screens when `activeDailyTarget.id` points to the wrong plan.
- **Filter week-scope data in memory; never fire a narrower query for data already in cache.** If `useMeals(weekStart, weekEnd)` returns the full week, filter by date in-memory — do not also call a per-day endpoint.
- **Track non-React-Query async ops in the loading gate.** `useEffect`-driven `async` calls (e.g. `initializeBatch`) are invisible to `isLoading`. Gate the UI explicitly: `initDayTargetIds[selectedDate] !== undefined`.
- **Always cancel `useEffect` async calls on cleanup** to prevent StrictMode double-fire writing stale state:
  ```ts
  useEffect(() => {
    let cancelled = false
    someAsyncFn().then(result => { if (!cancelled) applyResult(result) })
    return () => { cancelled = true }
  }, [dep])
  ```

## Expo-specific

- Use `expo-haptics` for physical feedback on key actions
- Use `expo-image` for high-performance image rendering
- Long lists: `FlatList` with `keyExtractor` and `getItemLayout` for fixed-height rows (`FlashList` is not a dependency; adding one is a spec decision)
- Platform-specific UI: `Platform.select()` for simple cases; `.ios.tsx`/`.android.tsx` file extensions when branching becomes complex
- Align native package versions: `npx expo install --fix`
- When changes don't appear after editing: `npx expo start --clear` (Metro cache)

## Component conventions

- **Styles**: `StyleSheet.create()` at bottom of file — not inline objects. Inline objects create new refs per render.
- **Size**: target 300 lines, hard limit 400 (§7 below, same as `003-frontend.md`). Extract subcomponents rather than growing the file.
- **`memo()` / `useMemo` / `useCallback`**: never add them — the React Compiler memoizes (the anti-pattern and its two traps are in `003-frontend.md`).
- **Functional components only**: No class components.
- **Named exports** for shared components; default exports for screens.

```tsx
// ✅ StyleSheet.create — type-checked, optimized
const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: colors.background },
  title: { fontSize: 18, fontWeight: '600' },
})

// ❌ Inline object — new ref every render, no type checking
<View style={{ flex: 1, backgroundColor: colors.background }}>
```

## File length

**Target: 300 lines. Hard limit: 400 lines.** Same rule as `003-frontend.md §7`.

Split by: extracting headless hook → new `useXxx.ts`; extracting tab/section → new screen/component file; extracting business logic → `lib/utils/`.

Exceptions: auto-generated files, test files.

## Navigation

- Route types in `AppNavigator.tsx` → `RootStackParamList`
- Always type navigation props: `NativeStackScreenProps<RootStackParamList, 'ScreenName'>`

## Metro config

Uses `watchFolders` + `nodeModulesPaths` for monorepo resolution. Do not remove these from `metro.config.cjs`.