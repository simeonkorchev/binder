---
paths:
  - "apps/**"
  - "packages/**"
---

# Frontend Conventions

## Shared patterns (web + mobile)

- **Inline styles everywhere** — no CSS-in-JS libraries. `className` only for classes the app's own `index.css` defines. trainer-web imports Tailwind (`@import "tailwindcss"`) for its preflight reset only: do not add utility classes; the handful still in the tree are debt, not a pattern
- **Theme via `useTheme()`** — never hardcode hex values; use `colors.*` from theme
- **React Query for all data** — custom hook per API resource, next to its feature (`features/<domain>/`; trainer-web cross-feature hooks in `lib/hooks/`)
- **i18next for text** — all user-facing strings in `packages/intl/locales/{bg,en}.json`
- **Packages export raw `.ts`** — no build step; Vite/Metro consume directly

## Theme tokens — a contract, not a palette

Both apps read colors from `packages/ds/src/tokens.ts` (dark "indigo + gold", light "cream — white hero"). **Each token names the surface it sits on; using it on a different surface class is a bug even when it compiles and looks fine in one theme.**

| Token | Sits on / used for |
|-------|--------------------|
| `bgPrimary` | Screen background |
| `bgSecondary` | White "hero" cards (daily goals), sheets, default surfaces |
| `bgCard` | Content cards (calendar, meal slots) — soft cream in light |
| `bgChip` | Small raised controls on a card (input-mode buttons) |
| `bgRaised` | Tinted tiles on a white hero card (daily macro tiles) |
| `bgTertiary` | Inset boxes: icon tiles, search fields, steppers |
| `progressTrack` | Unfilled portion of progress bars |
| `text*` | Text on any `bg*` surface (primary / secondary / tertiary tiers) |
| `accent`, `accentBright`/`accentDeep`, `action` | Gold **fills/borders**; bright/deep are the CTA gradient ends |
| `accentText` | Gold used as **text** — darker in light mode so small labels clear AA |
| `onAccent*` | **Only** on gold accent surfaces (dark ink) — never over photos |
| `onStatus` | Dark ink on filled status surfaces (error/success/warning) — white fails on all three |
| `calories`/`protein`/`carbs`/`fat`, `success`/`warning`/`error` | Status/macro accents |
| `cardBorder` | Card and tile borders (pair with `CARD_CHROME` from `lib/cardShadow`) |

Rules:
- Never hardcode hex values in components — read from `colors.*`.
- **Text/icons over photos, video thumbs, or scrims** use the `ON_PHOTO*` / `SCRIM_*` constants from `client-mobile/src/lib/color.ts`, never theme tokens — photos are not themed surfaces (the `onAccent` ink flip once turned the workout hero dark-on-dark this way).
- **Changing a token's *semantics*** (not just its value) requires auditing every consumer (`grep -rn "tokenName"` across both apps) and updating the contrast matrix.
- The contrast matrix in `packages/ds/src/tokens.test.ts` is the gate: every declared fg/bg pairing has a minimum WCAG ratio, **and** adjacent fills (`SURFACE_PAIRS`) have a separation floor so an inset box can't dissolve into the page. Adding a token = adding its pairings there; a palette tweak that breaks a pairing must be resolved in the palette, not by deleting the assertion.
- **Component tests must mock the theme with the shared test palette, never a hand-written literal** — client-mobile: `apps/client-mobile/src/test/theme.ts` (`testColors`, `testColorsDark`); trainer-web: render through `apps/trainer-web/src/test/utils/render.tsx`, which mounts the real providers. Hand-rolled palettes in test doubles silently keep rendering the old colors after a token change, and a renamed token resolves to `undefined` instead of failing.

## 1. Headless Component Pattern (enforced)

Components own: layout, styles, local UI state (`isOpen`, `isFocused`).
Custom hooks own: data fetching, mutations, business logic, derived state.

**Never call `useQuery`, `useMutation`, or `useApiClient` directly inside a component.**

```tsx
// ❌ BAD — mixes UI with API logic
const InviteClient = () => {
  const apiClient = useApiClient()
  const handleSubmit = async () => {
    await apiClient.POST('/api/v1/invitations', { body: { email } })
  }
  return <form onSubmit={handleSubmit}>...</form>
}

// ✅ GOOD — component is pure UI
// features/coaching/api/useInviteClient.ts
export const useInviteClient = ({ onSent }) => {
  const apiClient = useApiClient()
  const [status, setStatus] = useState(null)
  const sendInvitation = async (email) => { /* ... */ }
  return { sendInvitation, status, isLoading }
}

// components/InviteClient.jsx
const InviteClient = ({ onInvitationSent }) => {
  const { sendInvitation, status, isLoading } = useInviteClient({ onSent: onInvitationSent })
  return <form>...</form>
}
```

All pages and components have been migrated, and this is now enforced rather than
asserted: UI files (`components/`, `pages/`, `screens/`, `navigation/`,
`features/*/components/`) may not import `useApiClient` or `@tanstack/react-query`.
`Support.tsx` was the last page holding out — its request moved to
`lib/hooks/useSupportRequest.ts`.

## 1b. Hook implementation conventions

Hooks live next to their feature (`src/features/<domain>/…/use*.ts`); trainer-web also keeps cross-feature hooks in `src/lib/hooks/use*.ts`, client-mobile has no `src/lib/hooks/`. All data fetching uses **React Query** (`useQuery` / `useMutation`). Never use `useEffect + useState` to fetch data — it fires twice in React 18 Strict Mode and doesn't deduplicate.

```ts
// ❌ BAD — fires twice in React 18 Strict Mode; multiple mounts → N×N requests
const useClients = () => {
  const [clients, setClients] = useState([])
  const apiClient = useApiClient()
  const load = useCallback(async () => { ... }, [apiClient])
  useEffect(() => { load() }, [load])
  return { clients }
}

// ✅ GOOD — React Query deduplicates by key; Strict Mode safe
const useClients = () => {
  const apiClient = useApiClient()
  return useQuery({
    queryKey: ['clients'],
    queryFn: () => apiClient.GET('/api/v1/coaches/clients').then(r => r.data?.clients || []),
  })
}
```

**Why this matters:** React 18 Strict Mode mounts → unmounts → remounts every component in development. `useEffect` fires twice per mount. Multiple components loading the same endpoint × Strict Mode = 4× duplicate requests visible in the Network tab.

### Synchronous return from async hooks

When a component must branch _immediately_ after `await`, the hook must return the decision synchronously from the async fn — not via React state (which updates asynchronously):

```js
// ❌ BAD — caller checks state that hasn't updated yet
const { error } = useCreateClient()
await createClient(data)
if (!error) { ... }   // error is still null

// ✅ GOOD — return result directly
const createClient = async (data) => {
    // ...
    return { clientId, hasError }   // caller reads this, not state
}
```

### No apiClient prop-drilling

Sub-components must call their own hook internally — never receive `apiClient` as a prop.

```jsx
// ❌ BAD — apiClient prop leaks implementation
<RecipesPanel clientId={id} apiClient={apiClient} />

// ✅ GOOD — hook called inside the sub-component
const RecipesPanel = ({ clientId }) => {
    const { recipes } = useClientRecipes({ clientId, activeSubtab: 'recipes' })
    // ...
}
```

### useApiClient scope — hooks only

`useApiClient` may only be imported in hook files: `src/lib/hooks/` or `features/<domain>/api/`. Pages and components call custom hooks — never `useApiClient` directly.

## 1c. Hook and component shape

The checklist in `007-clean-code-checklist.md` holds every session to these; `/fe-refactor` enforces the same list.

- **One concern per hook, a small API.** A hook returning more than ~10 values, or owning a form *and* a recorder *and* an uploader, is split into focused hooks composed by a thin orchestrator.
- **Derive during render.** No `useState` + `useEffect` pair that copies a prop or derives a value; compute it in the body. No loading flag kept beside a query that already exposes `isPending`/`isLoading`.
- **No layers that add nothing**: a prop drilled through a component that never reads it, a wrapper that forwards everything to one child, a hook that returns another hook unchanged — remove the layer.
- **No default on a prop every parent passes** (`000-principles.md` §8).
- **Shared helpers first**: `lib/` already has the date formatter, debounce and clamp; a local copy is replaced only when it is identical for every input.
- **Numeric form fields** register with `valueAsNumber`; the handler never parses strings.
- **Data shaping is a mapper**, not handler code: `parseFloat(x) || 0`, key formatting, array building live in a named pure function next to the API hook, with a completeness test (§9, `006-testing.md`).

## 2. Query key factory

Never hardcode query keys as inline strings. All keys live in `lib/queryKeys.ts`.

```typescript
// lib/queryKeys.ts
export const mealKeys = {
  all: ['meals'] as const,
  list: (from: string, to: string, userID?: string) =>
    [...mealKeys.all, from, to, userID] as const,
}

export const periodKeys = {
  all: ['periods'] as const,
  byUser: (userID: string) => [...periodKeys.all, userID] as const,
}

// Usage in hook
useQuery({ queryKey: mealKeys.list(from, to, userID), queryFn: ... })

// Invalidation
queryClient.invalidateQueries({ queryKey: mealKeys.all })
```

## 3. Directory structure: feature-driven (new code)

New features go into `features/<domain>/` — not top-level `components/` or `hooks/`.

```
src/
└── features/
    └── nutrition/
        ├── api/         # hooks (useMeals.ts, usePeriods.ts)
        ├── components/  # UI components exclusive to this feature
        └── utils/       # domain-specific helpers
```

Existing `components/`, `hooks/`, `pages/` don't need migration — apply to new work only.

## 4. Naming conventions

| Artifact | Convention | Example |
|----------|-----------|---------|
| Components, Contexts | `PascalCase` | `ProfileCard`, `ThemeContext` |
| Hooks | `camelCase`, prefix `use` | `useAuth`, `useMeals` |
| Functions, utilities | `camelCase` | `formatDate`, `calcCalories` |
| Files with JSX | `PascalCase.tsx/.jsx` | `Button.tsx`, `DashboardScreen.tsx` |
| Files with pure logic | `camelCase.ts/.js` | `apiClient.ts`, `useMeals.ts` |
| Global constants | `UPPER_SNAKE_CASE` | `MAX_RETRY_COUNT`, `API_TIMEOUT` |

## 5. TypeScript

- No `any` — use `unknown` + type guard when type is truly dynamic
- `interface` for component props and objects (better extension, compiler perf)
- `type` for unions, intersections, aliases: `type Status = 'idle' | 'loading' | 'success' | 'error'`
- **Explicit return types on all custom hooks and shared API functions** — inference hides breaking changes

```typescript
// ❌ Inferred return type — breaks silently
export const useMeals = (from: string, to: string) => {
  return useQuery({ ... })
}

// ✅ Explicit — caller contract is visible
export const useMeals = (from: string, to: string): UseQueryResult<Meal[]> => {
  return useQuery({ ... })
}
```

- Discriminated unions for state: `status: 'idle' | 'loading' | 'success' | 'error'`
- Use `Pick`, `Omit`, `Partial` to keep types DRY
- A `string` whose legal values are enumerable is a literal union, never a comment listing them
- **`useForm<FormShape>()` is always generic** with a declared interface; `UseFormRegister<Record<string, unknown>>` in props is the smell
- **Error bodies are narrowed with `apiErrorMessage` from `lib/apiError.ts`** (both apps), never `(err as { detail?: string })`
- **`@spotter/types` is imported only in hooks and a feature's `types.ts`**; components, pages and screens use the re-export, so a generated-type change has one blast radius per feature

## 6. ESLint hooks rules

`react-hooks/rules-of-hooks` and `react-hooks/exhaustive-deps` **must be `error`, not `warn`**.

Broken `exhaustive-deps` in React Native → infinite render loops → device overheating.

Both are set in `packages/eslint-config/react.js`, so the two apps cannot drift
apart. They used to be duplicated per-app overrides.

## 6b. Where these rules live

Most of this document is enforced by `packages/eslint-config/conventions.js`,
which cites the section each block implements. Before that existed, the only
enforced rules were `exhaustive-deps` and `no-nested-ternary`.

Some rules in this document are **not yet enforced** because the codebase has
pre-existing debt that a config change can't clear — the line cap, hardcoded hex,
explicit hook return types, `fireEvent`, `getByTestId`, and `.ts` filename casing.
`packages/eslint-config/README.md` lists each one with its exact debt count and
what unblocks it. Treat those as review items until they land.

## API client

```ts
import { createApiClient } from '@spotter/api'
// Never use env vars inside packages — inject baseUrl + getToken from app
```

## Auth

- WorkOS AuthKit (hosted sign-in) through `@spotter/auth` — never a vendor SDK directly
- Web: `import { useAuth } from '@spotter/auth/web'`
- Mobile: `import { AuthProvider, useAuth } from '@spotter/auth/native'`
- Web env: `import.meta.env.VITE_*`
- Mobile env: `process.env.EXPO_PUBLIC_*`

## Testing

See `.claude/rules/006-testing.md` for full testing strategy.

Quick rules:
- Use `userEvent` not `fireEvent`
- Use MSW at network boundary
- Test hooks via `renderHook`, not the components that use them
- `findBy*` / `waitFor` for async — never `setTimeout`

## 7. File length

**Target: 300 lines. Hard limit: 400 lines.**

A file above 300 lines almost always mixes responsibilities. Split it:

| What to split out | Where it goes |
|---|---|
| Data / mutations | New `useXxx` hook next to the feature (`features/<domain>/`; trainer-web cross-feature: `lib/hooks/`) |
| Sub-section UI | New component file in `components/` |
| Business logic | Utility in `lib/utils/` |

**Exceptions** (do not split): auto-generated files (`api.ts`), test files, API client wrappers (`*-api-client.ts`).

```tsx
// ❌ ClientDetail.tsx at 2500 lines — unmaintainable God component
// ✅ Split into:
//    ClientDetail.tsx          (orchestration, <300 lines)
//    ClientMetricsTab.tsx      (metrics sub-section)
//    ClientCheckInsTab.tsx     (check-ins sub-section)
//    ClientNutritionTab.tsx    (nutrition sub-section)
```

## 8. State wiring integrity — DDD refactor rule

When moving logic out of a component into a hook, **the wiring back to the component must be complete**. Two silent failure modes to watch for:

### Frozen state — destructured without setter

```tsx
// ❌ BAD — state can never update; UI is permanently frozen
const [periods] = useState<Period[]>([])

// ✅ GOOD
const [periods, setPeriods] = useState<Period[]>([])
```

### Silent callback — no-op discards data from child

```tsx
// ❌ BAD — useMacroPeriods fires onPeriodsChange(data) but data is thrown away
<MacroPeriods onPeriodsChange={() => {}} />

// ✅ GOOD — parent receives and stores the data
<MacroPeriods onPeriodsChange={(p) => setPeriods(p as Period[])} />
```

### Detection

Both failure modes are now lint rules (`no-restricted-syntax` in
`packages/eslint-config/conventions.js`), so `npm run lint` catches them — the
manual greps this section used to prescribe are gone.

A lazy initializer (`useState(() => …)`) is excluded from the frozen-state check:
dropping the setter there is the deliberate compute-once idiom. Note the initializer
still runs *during* render — it is not a way to move impure work out of render — but
its result is kept in state instead of being recomputed on every pass, which is what
the React Compiler's purity rule objects to.

Known-good exceptions the selectors can't detect, because they depend on whether
the view is interactive: `onPress={() => {}}` on modal backdrops (touch stoppers);
action callbacks (`onDelete`, `onEdit`) on read-only or `pointerEvents="none"`
(skeleton) views. Those need an inline disable citing this section — see
`MealIngredientList.tsx` for the pattern.

## 9. Derive once, name it, reuse — no nested ternaries

Compute a derived value or predicate **once**, give it a name, and reuse it.
Don't re-spell a condition you already have, don't copy the same derivation into
two call sites, and don't nest ternaries — extract a named helper with early
returns.

```ts
// ❌ BAD
const serving = label.basisPer100g ? 100 : (label.servingSizeG || 100)                         // in the effect
const basisG  = label.basisPer100g ? 100 : (label.servingSizeG > 0 ? label.servingSizeG : 100) // dup'd + nested
const isValid = servingG > 0 && hasName
if (!label || servingG <= 0 || !hasName) return null                                            // re-spells isValid

// ✅ GOOD
function labelBasisGrams(label: ParsedLabel): number {   // one named source of truth, early returns
  if (label.basisPer100g) return 100
  return label.servingSizeG > 0 ? label.servingSizeG : 100
}
const isValid = servingG > 0 && hasName
if (!label || !isValid) return null                      // reuse the predicate
const factor = 100 / labelBasisGrams(label)              // reuse the derivation
```

Rules of thumb:
- **Conditional inside a conditional** → extract a named function with early returns (cap: one ternary per expression).
- **A boolean you already computed** (`isValid`, `hasName`) → reference it; never re-spell its terms.
- **The same derivation in two places** → one helper, not a copy (never `useMemo` — the compiler memoizes; see the anti-patterns below).

**Enforced** in logic files: `no-nested-ternary` is `error` for `**/*.ts` via
`packages/eslint-config` (both apps and all `packages/*`). JSX conditional
rendering in `.tsx` is intentionally exempt (it's idiomatic there); the
outstanding `.tsx` nesting is tracked as a `simplify` finding for the
`/fix-findings` routine to chip away at.

## 10. UI quality, accessibility, empty states

These were previously written down only in the trainer-web agent; they are
standards for every FE change, interactive or Routine (`007` rows cite them).

- **Every list or data section has an empty state** — themed text through
  `t('…')`, never a blank area (`items.length === 0 ? <EmptyState/> : …`).
- **Accessibility, WCAG 2.1 AA as the bar**: 4.5:1 contrast for body text
  (3:1 for ≥ 18px bold); every interactive element reachable by keyboard with a
  visible focus indicator; an accessible name (`aria-label`, `accessibilityLabel`)
  on icon-only controls; `role`/`aria-*` on custom controls; `<img>` always has
  `alt` (`alt=""` when decorative); never colour alone to convey meaning.
  `packages/eslint-config/a11y.js` enforces the mechanical part on web.
- **No generic "AI-slop" UI**: no gradient heroes, three identical icon cards,
  shadow on everything, emoji headings, "Get Started / Learn More" CTA pairs.
  Specific verbs, one CTA, left-aligned body text, deliberate asymmetry. The
  trainer-web agent carries the detection table and the fix checklist.
- **Never `dangerouslySetInnerHTML` with user content** (`004-security.md`; the
  pre-tool guard denies it).

## 11. Error states — unknown is never rendered as known

`LoadStateView` (client-mobile) cites this section; it is the rule every
remote read and write is held to, on both apps.

**Three states, never two.** A remote read is `loading | error | success`, and
`success` splits into `empty | data`. An empty list renders **only** when the
server said empty. The failure this codebase exhibits is collapsing `error`
into `empty`, which tells the user a confident lie: a diary week that failed
to load is indistinguishable from a week with no meals.

- **Never launder a failure into a neutral value.** `catch { return null }`,
  `catch { return [] }` and `r.data ?? []` across a *failed* query all turn
  "I don't know" into "I know: nothing". (`?? []` inside `select` is fine —
  `select` only runs on success, so it is narrowing a documented `null` body,
  not hiding an error.)
- **A hook that can fail exposes that it failed.** Any hook returning
  aggregated data from queries returns `isError` alongside it. A consumer that
  cannot see the failure cannot handle it — `useMealsForDates` returned
  `{ meals, isPending }`, so every caller was structurally incapable of
  telling an outage from an empty week.
- **Reads that decide a write must refuse on failure.** When a value derived
  from a query picks a slot, an id or a merge target, a failed read is not a
  degraded write, it is a destructive one — the backend's `FindMealBySlot`
  merges into whatever is at the slot you guessed. Block the write and say so.
  `features/reminders/replyQueue.ts` is the reference: it rejects rather than
  guessing, and explains why.
- **`catch` makes a decision.** Rethrow, surface state the UI renders, or
  carry a comment saying why this failure is genuinely uninteresting.
  A bare `catch {}` is none of the three.
- **A domain value is not an error value.** `initDayTargetIds[date] === null`
  means "flex day, the server said so". Writing `null` on a *failed* request
  makes an outage indistinguishable from a real answer — and because the
  retry loop skips days already recorded, it never self-corrects. A failure
  needs its own state.
- **Success is confirmed, never asserted.** Success UI (toast, haptic,
  navigation) fires from `onSuccess` / after a resolved `await`, never after a
  fire-and-forget `mutate()`. Feedback belongs with the mutation, not copied
  into each call site.

**Rendering the ladder**: on mobile use `LoadStateView` (loader → full-space
`ErrorRetryView` when nothing is cached → children + non-blocking
`StaleDataBanner` when a refetch is failing). Do not hand-roll a fourth
variant of this ladder.

## Anti-patterns

- ❌ Nested ternaries / re-spelling a predicate you already computed / copy-pasting a derivation — extract a named helper (`no-nested-ternary`, enforced on `.ts`) and reuse
- ❌ `useQuery`/`useMutation`/`useApiClient` directly inside UI components or pages
- ❌ `apiClient` passed as prop to sub-components — use hooks inside the sub-component
- ❌ Reading React state immediately after `await` to branch — return result value instead
- ❌ Hardcoded query key strings outside `queryKeys.ts`
- ❌ `any` — use `unknown` + narrow
- ❌ Missing explicit return type on custom hooks
- ❌ `fireEvent` over `userEvent`
- ❌ `getByTestId` as first resort
- ❌ Hardcoded colors outside theme
- ❌ Snapshot tests for dynamic components
- ❌ Files exceeding 400 lines — split into focused hooks / sub-components / utilities
- ❌ **`useEffect + useState` for data fetching** — causes duplicate requests in React 18 Strict Mode (2× per component per mount). Use `useQuery` instead. Multiple components subscribing to the same query key share one in-flight request and the cached result.
- ❌ `useCallback(async () => { fetch() }, [apiClient])` + `useEffect(() => { fn() }, [fn])` — the "stable callback effect" pattern looks safe but Strict Mode still fires it twice and prevents React Query deduplication.
- ❌ **`window.alert()` / `window.confirm()` for feedback** — they block the main thread and cannot be themed or translated consistently. Use `useToast()` from `lib/toast-context` (both apps have the provider) for success/error feedback; a confirm becomes an in-page dialog. Native `Alert.alert` on mobile is fine.
- ❌ **`setTimeout(fn, N)` to sequence a state update and a navigation/callback** — run the callback from the mutation's `onSuccess` or an effect on the settled state, never on a magic delay.
- ❌ **Manual `memo()` / `useMemo` / `useCallback` "for performance"** — both apps compile with the React Compiler (`babel-plugin-react-compiler`; trainer-web via `vite.config.ts`, client-mobile via `app.json` `experiments.reactCompiler` → Metro → `babel-preset-expo`), which memoizes automatically; hand-written memoization is noise and hides real dependencies. **One trap**: the compiler's default `infer` mode only compiles components (PascalCase, creates JSX) and hooks that call another hook. A `use*` function with no call to a `use*`-named identifier in its body is skipped silently and its closures are new every render; a hook imported under an alias that does not start with `use` (`useGetRecipes as sdkUseGetRecipes`) does not count. Give such a hook `'use memo'` as its first statement. **Second trap**: a `// eslint-disable-next-line react-hooks/...` anywhere inside a component or hook makes the compiler skip that whole function. Verify with `node ../../.claude/skills/fe-refactor/scripts/compiler-probe.mjs <file>` from the app directory before removing memoization around either.
- ❌ **Hardcoded user-facing strings** — every label, button, placeholder, tooltip, aria-label, and error message must use `useTranslation` + `t('key')`. Add keys to both `packages/intl/locales/en.json` and `packages/intl/locales/bg.json`. No Bulgarian or English text may appear as a literal in component source code.
