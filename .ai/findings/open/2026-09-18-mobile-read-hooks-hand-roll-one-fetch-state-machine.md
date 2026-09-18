# Six mobile hooks hand-roll data fetching that `003-frontend.md` §1b says must be React Query — and four of them hand-roll the *same* state machine

- **Category**: dup
- **Severity**: high
- **Path**: `apps/mobile/src/features/binder/api/useBinders.ts`, `apps/mobile/src/features/binder/api/useBinderPage.ts`, `apps/mobile/src/features/market/api/useListings.ts`, `apps/mobile/src/features/market/api/useSellerContact.ts`, `apps/mobile/src/features/scan/api/useCardSearch.ts`, `apps/mobile/src/features/scan/api/useResolveScan.ts`
- **Found**: 2026-09-18 (T073 clean-code walk)

`003-frontend.md` §1b is explicit: "All data fetching uses **React Query**
(`useQuery` / `useMutation`). Never use `useEffect + useState` to fetch data —
it fires twice in React 18 Strict Mode and doesn't deduplicate."
`@tanstack/react-query` is not a dependency of `apps/mobile` (see its
`package.json`), and nothing in `specs/001-binder-mvp/` records a decision to
deviate — D1–D5 cover auth, OCR, images, contact and card detection, not data
fetching. Four features were built in parallel by four agents, and all four
reached for `useEffect` + `useState`.

Two rows of `007-clean-code-checklist.md` are hit at once:

1. **Rules precedence.** `.claude/rules/` outranks inference (`CLAUDE.md`
   Memory), so an undocumented deviation from §1b is a rule violation — and per
   `.ai/findings/README.md` a rule violation that *will be copied* is `high`.
   It has already been copied four times.

2. **"The same logic or derivation exists once"** (`000` §6). `useBinders`,
   `useBinderPage`, `useListings` and `useSellerContact` each carry the same
   eighteen lines, with four different spellings of the same idea:
   - a request identity in state (`reloadCount`, `` `${binderId}|${page}|${reloadCount}` ``, `{ filter, attempt }`, `attempt`),
   - `answered: { request, state } | null` beside it,
   - an effect with a `cancelled` flag that writes `answered` in both branches,
   - `state` derived as `answered?.request === request ? answered.state : { status: 'loading' }`,
   - `reload` incrementing the counter.

   The comments explaining *why* loading is derived rather than stored are
   themselves duplicated in three of the four files.

Not fixed on the spot because the fix is not behaviour-preserving in either
direction: adopting React Query adds a dependency and a provider and changes when
requests fire (dedupe, cache, refetch); extracting a bespoke `useRead` helper
would consolidate the duplication into an abstraction the rules do not have, and
would be thrown away by the React Query adoption it delays.

## Proposed fix

Adopt React Query in `apps/mobile` (one `QueryClientProvider` in `App.tsx`; each
read becomes a `useQuery` keyed on its request, each write a `useMutation`
invalidating that key), **or** — if the deviation is deliberate for an offline-
first scanner — record it as a decision in `specs/001-binder-mvp/spec.md` and
extract the one shared `useRead` hook the four copies are asking for. Either way
the outcome is one mechanism, not four. A maintainer picks; this is a dependency
decision, not a refactor.

## Drafted RED test

The behavioural difference React Query makes is deduplication, which is what
§1b's warning is about. This fails today (two mounts, two requests) and passes
after adoption:

```diff
--- a/apps/mobile/src/features/binder/api/useBinders.test.ts
+++ b/apps/mobile/src/features/binder/api/useBinders.test.ts
+  it('reads the binder list once when two screens ask for it', async () => {
+    mockFetch.mockImplementation(() => Promise.resolve(listed(binder('Blue-Eyes'))))
+
+    const { result: first } = renderHook(() => useBinders())
+    const { result: second } = renderHook(() => useBinders())
+    await waitFor(() => expect(first.current.state.status).toBe('ready'))
+    await waitFor(() => expect(second.current.state.status).toBe('ready'))
+
+    expect(mockFetch).toHaveBeenCalledTimes(1)
+  })
```

## Blast radius

Six hooks, their six suites, `App.tsx` (the provider) and every screen test that
mounts one of them — `App.test.tsx`, `MarketScreen.test.tsx`,
`SellerContactScreen.test.tsx`, `ScanScreen.test.tsx`. No rendered output
changes, so no screenshots are needed. This is the largest single piece of
outstanding cleanup in `apps/mobile`.
