# Clean-code checklist — the one list every change is held to

This is the definition of done for **every** change on this repo: a feature
from `/spec-implement`, a bug fix in an interactive session, and the
Routines' sweeps (`/go-quality-sweep`, `/fe-refactor`, `/cleanup`,
`/fix-findings`) all walk the **same** list. Code that passes it when
written is code no Routine refactors later. Each row cites the rule it
enforces; the rule holds the *why* and the examples, this file only says
*what to check*.

**When**: before marking any task done, walk the rows for each layer you
touched, over every file you touched. The Routines walk them over their
whole target. Interactive sessions fix hits before opening the PR; a
Routine fixes or logs them (`.ai/findings/README.md`).

**Kind** is for the Routines: *preserving* edits are proven by
characterization tests that already pass; *changing* edits need a RED test
first. In interactive work every row is simply the standard.

## Both layers

| Standard | Rule | Kind |
|----------|------|------|
| One responsibility per file, type, hook or component; Go ≤ 400 lines (hard 500), FE ≤ 300 (hard 400) | `000` §7, `002` §2, `003` §7 | preserving |
| The same logic or derivation exists once — one helper, one constant, one rule | `000` §6, `003` §9 | preserving |
| Nothing speculative: no pass-through function, one-field wrapper, boolean flag parameter, parameter every caller passes identically, or interface method nothing calls | `000` §8, `002` §1a, `003` §1c | preserving |
| No work thrown away: a sort repeated, a map built only to be ranged back, a 1:1 mapper between identical shapes | `002` §1a, `003` §9 | preserving |
| Dead code is deleted (with the zero-reference proof), never commented out or kept "in case" | `000` §5, `002` §1a; `knip` on FE | preserving |
| Deep nesting, else-after-return, a branch a type or constructor already rules out, identical switch arms, nested ternaries → early returns and named predicates | `002` §1, `003` §9 | preserving |
| Every suppression (`//nolint`, `eslint-disable`) names the linter and a reason; a suppression that hides a smell is a smell | `000` §8a | preserving |
| Mapping at a boundary is total — no dropped fields; a completeness test proves it | `000` §9, `002` §8a, `006` "Mapper completeness" | changing |
| Invariants are checked at the boundary they enter, once | `000` §10 | changing |
| Siblings agree: N−1 do X, one silently does Y (rounding, a skipped kind, UTC vs local) → make it match what the rule or the siblings' docs say; nothing written down → product decision, log it | `000` §4 | changing |
| Code matches its own name, doc comment, OpenAPI description or i18n key | `000` §2 | changing |
| A test that pins an absurd value is fixed with the code, source of truth cited | `006` "Test behavior" | changing |

## Go (`internal/`, `pkg/`, `cmd/`)

| Standard | Rule | Kind |
|----------|------|------|
| Handlers never choose a status: `handleErr` + the domain's `serviceErrorMappings` table | `002` §4.4 | changing |
| Row→model and request→model shaping is a named pure converter with a completeness test, not inline in a handler or an `InTx` closure | `002` §8a, `000` §9 | preserving |
| Cross-domain dependencies are consumer-side interfaces, never another domain's concrete `*service.Service` | `002` §3, §3a | preserving |
| A `string` whose legal values are enumerable is a named type with constants; `map[string]any` / `any` where keys are known is a struct | `002` §1a | preserving |
| One receiver kind per type | `002` §1.3 | preserving |
| `sql.ErrNoRows` never escapes a store: `dataerror` for a by-id lookup, an empty slice for a collection | `002` §4.3, `000` §8b | changing |
| Every error is wrapped with a lowercase gerund + `%w`; a sentinel joined with its cause uses double-`%w` | `002` §4.2 | changing if a caller could now match, else preserving |
| Sentinel messages lowercase and un-punctuated; `slog` keys snake_case, typed attrs, `eslog.Error(err)` | `002` §4.1, §4a | preserving |
| Expected errors (bad token, 404, `context.Canceled`) are demoted with `eslog.LeveledErr`, never logged at ERROR; a demoted level or attached attrs are honoured to the log line | `002` §4a | changing |
| An `InTx` closure that `storetx.Value` expresses, a date parse `pkg/dateparse` has → use the shared one | `002` §7, §8 | preserving |
| Modern stdlib idioms (`min`/`max`, `slices`, `maps`, range-over-int) over hand-rolled loops, judged by reading | `002` §1a | preserving |
| No globals, no `init()`, no `panic` outside `must*` builders and the documented `pkg/` exceptions | `002` §3, §4.5 | preserving |

## Frontend (`apps/trainer-web`, `apps/client-mobile`)

| Standard | Rule | Kind |
|----------|------|------|
| Data fetching, mutations and business logic live in a `use*` hook next to the feature; components keep layout and local UI state | `003` §1, §1b | preserving |
| A hook has one concern and a small API (≈ ≤ 10 returned values); a bigger one is split and composed by a thin orchestrator | `003` §1c | preserving |
| Data shaping (`parseFloat(x) \|\| 0`, key formatting, array building) is a named pure mapper next to the API hook with a completeness test, not inline in an event handler | `003` §9, `006` "Mapper completeness" | preserving |
| No prop drilled through a component that never reads it, no wrapper forwarding everything to one child, no hook returning another hook unchanged | `003` §1c | preserving |
| `@spotter/types` is imported only in hooks and a feature's `types.ts`; components, pages and screens use the re-export | `003` §5 | preserving |
| No `any`, `as unknown as`, or `Record<string, unknown>` where fields are known; a typed guard for truly dynamic input; `useForm<FormShape>()` always generic; error bodies narrowed with `apiErrorMessage` from `lib/apiError.ts` | `003` §5 | preserving |
| A `string` whose legal values are enumerable is a literal union | `003` §5 | preserving |
| Every `use*` hook declares its return type | `003` §5 | preserving |
| Derive during render — no `useState` + `useEffect` that copies a prop or derives a value; no loading flag beside a query that already exposes one | `003` §1c | preserving |
| No manual `memo` / `useMemo` / `useCallback`; the React Compiler memoizes — after removing one, the compiler probe must still report the function compiled | `003` Anti-patterns, `005` "Component conventions" | preserving |
| No default on a prop every parent passes; a local formatter, debounce or clamp when `lib/` has one is replaced only if identical for every input | `000` §8, `003` §1c | preserving |
| No `window.alert` / `window.confirm` (toast or in-page dialog; native `Alert.alert` on mobile is fine); no `setTimeout` to sequence state and navigation | `003` Anti-patterns | changing |
| Numeric form fields registered with `valueAsNumber`, not parsed strings | `003` §1c | changing |
| Every user-facing string goes through `t('key')`, key present in both locales | `003` Anti-patterns | changing |
| `useEffect` dependency arrays are complete; no `react-hooks` eslint suppression inside a component or hook (it makes the compiler skip the whole function) | `003` §6, Anti-patterns | changing |
| Static styles in `StyleSheet.create` (mobile) / inline with theme tokens (web); no hardcoded colours, no Tailwind utility classes | `003` "Theme tokens", `005` "Component conventions" | preserving |
| Every list or data section has a translated empty state; every interactive control has an accessible name and keyboard reach; no colour-only meaning | `003` §10 | changing |

## Tests (both layers)

| Standard | Rule | Kind |
|----------|------|------|
| Test the hook, not the component; `renderHook` + MSW at the network boundary; stable module-level mocks | `006` | test-only |
| `userEvent` not `fireEvent`; role-first selectors, `getByTestId` last; `findBy*`/`waitFor`, never `setTimeout`; no snapshots of dynamic output | `006` "Selector hierarchy", "No flaky async" | test-only |
| Go: external test package, `JustBeforeEach` invokes once, counterfeiter fakes, `MatchError` on sentinels, assert the record not its length | `002` §9, `006` "Go backend" | test-only |
| Every touched branch (each `if`/`else`, error path, guard, empty case) has a test that exercises it | `006` "Cover these cases" | test-only |
