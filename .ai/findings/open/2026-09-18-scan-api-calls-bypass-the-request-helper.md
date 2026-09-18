# The scan feature's two API calls bypass `lib/apiRequest`, so they send no session and act on no 401

- **Category**: dup
- **Severity**: medium
- **Path**: `apps/mobile/src/features/scan/api/useCardSearch.ts`, `apps/mobile/src/features/scan/api/useResolveScan.ts`
- **Found**: 2026-09-18 (T073 clean-code walk)

`lib/apiRequest.ts` is the one place a request is built: it attaches the bearer
token, throws an `ApiError` carrying the status, and drops the session on a 401
so the navigator swaps the stack (T082). Three of the four features go through
it — `useBinders`, `useBinderPage`, `useBinderSlots`, `useListings`,
`useSellerContact`, `useSlotListing`, `exchangeSession`. The scan feature does
not: `useCardSearch` and `useResolveScan` were written in W9, before T082
existed, and each builds its own `fetch(apiUrl(...))`.

This is the "siblings agree" row of `007-clean-code-checklist.md`: N−1 do X and
one silently does Y, with the rule written down — `apiRequest.ts`'s own doc says
the token is attached there and nowhere else, and named the scanner's queue as
one of its callers until this finding corrected it.

**It is not a live bug today.** `cmd/binderd/routes.go` registers
`cardapi.RegisterEndpoints(api, svc.cards)` with no `ActorFunc`, so `GET /cards`
and `POST /scans/resolve` accept a request with no credential; the scanner works
signed out. It becomes a bug the moment the card domain wants an actor — for a
per-user scan quota, a rate limit, or anything else that needs to know who is
sweeping — and it will fail as "the card search could not be reached", because
nothing in the scan feature can tell a 401 from a dropped connection.

Not fixed on the spot because it is behaviour-changing: the requests would start
carrying an `Authorization` header, and a 401 would begin ending the session
from the scan tab.

## Proposed fix

- `useCardSearch`: `readJson<SearchCardsBody>(\`/cards?q=${encodeURIComponent(query)}\`)`
  in place of the `fetch`; the existing `catch` already turns a throw into
  `hasFailed`.
- `useResolveScan.requestScan`: `writeJson<ScanMatchBody>('POST', '/scans/resolve', { code })`,
  with the 4xx/5xx split kept off `ApiError.status` — a status `>= 500` rethrows
  (the trip can be made again), anything else is `{ kind: 'rejected' }` (this
  code will be refused every time). The distinction is the queue's contract and
  must survive the move.
- Delete the now-unused `apiUrl` import from both; `apiUrl` stays the one URL
  builder, called from `apiRequest.ts`.
- Correct `apiRequest.ts`'s doc back to "every feature calls it" and drop the
  reference to this file.

## Drafted RED test

```diff
--- a/apps/mobile/src/features/scan/api/useCardSearch.test.ts
+++ b/apps/mobile/src/features/scan/api/useCardSearch.test.ts
+  it('attaches the session to the card search', async () => {
+    await rememberSession(session)
+    const search = await renderSearch()
+
+    await searchFor(search.current, 'Dark Magician')
+
+    expect(mockFetch.mock.calls[0]?.[1]?.headers).toMatchObject({
+      Authorization: `Bearer ${session.token}`,
+    })
+  })
```

Today `mockFetch.mock.calls[0]?.[1]` is `undefined` for the search — the hook
passes no init at all — so the assertion fails on the missing header. The same
test against `useResolveScan` (which does pass an init, with only
`Content-Type`) fails on the absent `Authorization`. `MarketScreen.test.tsx` and
`apiRequest.test.ts` already hold the `rememberSession(session)` pattern to copy.

## Blast radius

Two hooks, their two suites, and `ScanScreen.test.tsx` / `ReviewSheet.test.tsx`
which assert on the URLs but not on the headers. No screen changes, so no
screenshots are needed.
