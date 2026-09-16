---
paths:
  - "internal/**"
  - "pkg/**"
  - "cmd/**"
  - "tests/**"
  - "apps/**"
  - "packages/**"
---

# Testing Strategy

## Mental model: Testing Trophy

Heaviest investment in **integration tests** (shared hooks + MSW), not unit tests of individual functions or E2E.

```
         /‾‾‾‾‾‾‾‾\
        / E2E (few) \       Playwright (web) / Maestro (mobile)
       /‾‾‾‾‾‾‾‾‾‾‾‾\
      / Integration   \     Custom hooks + React Query + MSW
     /‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾\
    /    Unit (utils)   \   Pure functions, formatters
   /‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾\
```

## Testing matrix

| Layer | Target | Tooling | Goal |
|-------|--------|---------|------|
| Shared hooks | Custom hooks (`features/<domain>/`; trainer-web `lib/hooks/`) | Vitest + `renderHook` + MSW | Verify business logic + API integration |
| Web UI | Components | Vitest + RTL | Render + user interaction |
| Mobile UI | Screens | Jest + RNTL | Touch targets, native states |
| Web E2E | Happy paths only | Playwright | Login → core flow smoke test |
| Mobile E2E | Happy paths only | Maestro (YAML) | Launch → core flow smoke test |

## Rule: Test the hook, not the component

Components should be thin (layout only). Test the shared hook instead.

```typescript
// ✅ Test the hook
import { renderHook, act } from '@testing-library/react'
import { useInviteClient } from '@/features/coaching/api/useInviteClient'

it('sends invitation and resets on success', async () => {
  const { result } = renderHook(() => useInviteClient({ onSent: vi.fn() }))

  await act(async () => {
    await result.current.sendInvitation('coach@example.com')
  })

  expect(result.current.status).toBe('success')
})

// ❌ Don't test internal state of the component
expect(wrapper.state('isOpen')).toBe(true)
```

## Rule: Hook tests — stable mock at module level

For hooks that call `useApiClient`, mock it at module level with a stable object reference. Placing the mock inside a test body creates a new object reference each render, causing spurious re-renders.

```typescript
// ✅ CORRECT — stable reference, reset between tests
const mockGet = vi.fn()
const mockPost = vi.fn()
const mockApiClient = { GET: mockGet, POST: mockPost }
vi.mock('@/lib/api-client', () => ({ useApiClient: () => mockApiClient }))

describe('useMyHook', () => {
    beforeEach(() => { mockGet.mockReset(); mockPost.mockReset() })

    it('fetches data', async () => {
        mockGet.mockResolvedValue({ data: [...], error: null })
        const { result } = renderHook(() => useMyHook({ userId: 'u1' }))
        await vi.waitFor(() => expect(result.current.isLoading).toBe(false))
        expect(result.current.data).toHaveLength(2)
    })
})
```

## Rule: Use MSW at the network boundary

For component tests, intercept at the network layer. For hook-only tests, prefer the stable mock pattern above — it's faster and avoids MSW setup overhead.

```typescript
// src/test/mocks/handlers.ts
import { http, HttpResponse } from 'msw'

export const handlers = [
  http.post('/api/v1/invitations', () => HttpResponse.json({ id: '123' })),
]
```

## Rule: Test behavior, not implementation

If renaming an internal variable breaks a test, the test is wrong.

```typescript
// ❌ Tests implementation detail
expect(component.state.isOpen).toBe(true)

// ✅ Tests observable behavior
expect(screen.getByRole('dialog')).toBeVisible()
```

## Rule: Selector hierarchy (RTL / RNTL)

1. `getByRole` — always first
2. `getByLabelText`
3. `getByText`
4. `getByPlaceholderText`
5. `getByTestId` — last resort (textless icons, dynamic elements)

## Rule: No flaky async patterns

```typescript
// ❌ Flaky
await new Promise(r => setTimeout(r, 500))

// ✅ Explicit
await waitFor(() => expect(screen.getByText('Success')).toBeVisible())
await findByText('Success')  // equivalent, shorter
```

Flaky test = broken build. Quarantine, fix async logic, rewrite with `waitFor`.

## E2E scope

**There is no E2E harness today.** Neither app has an `e2e/` directory, and
the spec pipeline must not plan E2E tasks until one exists. What does exist is
the manual emulator loop in `apps/client-mobile/CLAUDE.md` (adb + screenshots
in both themes), which is mandatory for UI changes.

When a harness is added: **happy paths only** (the auth flow, one core path per
major feature); edge cases stay in integration tests. Planned tooling is
Playwright for web and Maestro for mobile, under `apps/<app>/e2e/`, never
co-located with component files.

## Rule: Cover these cases for every hook

Each extracted hook must have tests for:

1. **Guard** — hook does not fetch when required params missing
2. **Happy path** — correct endpoint called with correct params; returned data matches
3. **Refetch / mutation** — `refetch()` triggers a second call; `act()` wraps imperative calls
4. **Error path** — API error doesn't throw uncaught; `isLoading` returns to `false`
5. **Callback** — `onSuccess`/`onDeleted`/`onSwapped` called when expected

```typescript
// Pattern: testing refetch (tick-based)
it('re-fetches when refetch called', async () => {
    mockGet.mockResolvedValue({ data: DATA })
    const { result } = renderHook(() => useMyHook({ userId: 'u1' }))
    await vi.waitFor(() => expect(result.current.isLoading).toBe(false))

    mockGet.mockResolvedValue({ data: [] })
    await act(() => { result.current.refetch() })
    await vi.waitFor(() => expect(result.current.data).toHaveLength(0))
    expect(mockGet).toHaveBeenCalledTimes(2)
})

// Pattern: testing mutation with prop changes (rerender)
it('falls back to first item when selection not in new list', async () => {
    let date = '2026-05-15'
    const { result, rerender } = renderHook(() => useMyHook({ clientId: 'c1', date }))
    await vi.waitFor(() => expect(result.current.selectedId).toBe('t1'))

    act(() => { result.current.setSelectedId('old-id') })

    mockGet.mockResolvedValue({ data: [{ id: 't3' }] })
    date = '2026-05-16'
    rerender()     // triggers effect because date changed
    await vi.waitFor(() => expect(result.current.selectedId).toBe('t3'))
})
```

## Go backend: what to test

The matrix above is frontend-shaped. The backend equivalent:

| Layer | Test against | Tooling |
|-------|--------------|---------|
| `api/` handlers | The real huma/echo stack via `httptest` + a counterfeiter fake service | Ginkgo, `newTestAdapter` |
| `service/` | Fake store + fake permission service | Ginkgo, `servicefakes` |
| `store/` | A real Postgres in testcontainers, inside a rolled-back tx | Ginkgo, `dbhelper` |
| Pure helpers, mappers, tables | Plain `go test` in an internal (`package api`) test file | stdlib `testing` |

**Rules that carry the most weight here:**

1. **Every endpoint gets an authorization test.** Assert the unauthorized caller is refused *and* that the store was never touched — `Expect(fakeStore.XCallCount()).To(BeZero())`. A gate that returns the right status after already reading the data is not a gate.
2. **Assert the record, not its length.** `Expect(items).To(HaveLen(1))` passes just as happily on a blank item as a correct one — two tests in this repo did exactly that while a nil-resolution bug wrote empty rows. Assert the fields that prove the item is real.
3. **A guard test must be able to fail.** After writing one, break the thing it guards and confirm it goes red. A guard that cannot fail is worse than no guard, because it reads as coverage.
4. **Derive lists from source, not by hand.** A test that enumerates "all the sentinels" from a hand-written slice stops being true the day someone adds one. Parse the package (`go/ast`) so new entries are picked up automatically — see `nutrition/api/errors_internal_test.go`.
5. **`-race` is not optional.** `make test` runs `ginkgo -race`; concurrent code (errgroup fan-outs in the service layer) is only as safe as the last race run.
6. **Store tests need Docker.** They spin up Postgres via testcontainers and are skipped-by-failure in environments without it. If you cannot run them locally, say so explicitly rather than reporting a green gate.

## Rule: Mapper completeness tests

Any function that copies an entity field-by-field from one representation to
another (DB row → model, model → API DTO, DTO → form state, and the reverse)
must have a test that fills **every** source field with a distinct non-zero
value and asserts **every** destination field. See `000-principles.md` §9 for
why — a forgotten assignment compiles and silently drops data.

The test, not the eyeball, owns coverage: assert the whole struct, not just the
field your current task touches. When you add a field to the entity, this test
fails until you map it — that failure is the point.

```go
// ✅ Go: one fixture, every field asserted
func TestToAdminFoodMapsEveryField(t *testing.T) {
    f := fullyPopulatedFood()       // every field set to a distinct non-zero value
    resp := toAdminFood(f, nil, "https://cdn.test")

    if resp.EnglishBaseName == nil || *resp.EnglishBaseName != "Tuna base EN" { /* ... */ }
    // ...assert ID, Name, BaseName, EnglishName, EnglishBaseName, ImageName,
    //    ImageURL, FoodIcon, Category, macros, ComputedCalories — the full set.
    // A field intentionally not mapped (MicroNutrients) is named in a comment.
}
```

```typescript
// ✅ Web: edit every field, assert the whole save payload
it('sends every edited name field in the update payload', async () => {
    // ...type into name BG, base name BG, name EN, base name EN
    const [, payload] = mockUpdateFood.mock.calls[0]
    expect(payload.name).toBe('...')
    expect(payload.baseName).toBe('...')
    expect(payload.englishName).toBe('...')
    expect(payload.englishBaseName).toBe('...')   // the field that broke
})
```

## Anti-patterns

- ❌ Snapshot tests for dynamic components
- ❌ Mocking `fetch`/`axios` directly
- ❌ Module-level mock inside `describe` block — creates new object refs → re-renders
- ❌ `setTimeout` in tests
- ❌ Asserting on internal state variables
- ❌ `getByTestId` as first selector choice
- ❌ Skipping a flaky test instead of fixing it (`it.skip`)
- ❌ Extracting a hook without writing tests for it — both go in the same PR
