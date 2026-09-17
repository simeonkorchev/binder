# A reviewed scan session cannot be committed into a binder atomically

- **Category**: domain
- **Severity**: high
- **Path**: `internal/binder/api/slot.go`, `packages/types/openapi.json`
- **Found**: 2026-09-17 (W9, while finishing T047)

T048 asks for the reviewed scan session to go into the binder in **one call,
one transaction**. The contract publishes no endpoint that can do it.

Every write the binder domain exposes is single-row:

| Endpoint | What it writes |
|---|---|
| `POST /binders` | one binder, name only — `CreateBinderInputBody` has no slots |
| `POST /binders/{binderId}/slots` | exactly one slot (`AddSlotBody` is one `cardId`) |
| `PATCH /binders/{binderId}/slots` | moves one slot |
| `DELETE /binders/{binderId}/slots/{slotId}` | removes one slot |

So committing a 60-card sweep is 1 + 60 requests, and `service.AddSlot` opens
its **own** `InTx` per request. A dropped connection, a 401 on an expired
session or a `binder_slots_printing_matches_resolution` rejection at card 40
leaves a binder holding 39 cards with no record of the other 21 — and by then
the user has put the cards away, which is exactly the outcome the task says is
the worst one. A client-side loop cannot fix this: the transaction boundary is
on the server.

**Not fixed here on purpose.** The fix is a new endpoint — `POST
/binders/{binderId}/slots:batch`, or slots accepted on `POST /binders` — which
is a new operation across `api → service → store`, a regenerated contract and a
regenerated fake. That is the spec pipeline (`/spec-feature`), not a mobile
task, and inventing the loop instead would have shipped the half-write the task
forbids.

**For whoever picks it up.** The pieces the client already has are in
`apps/mobile/src/features/scan/useScanReview.ts`: every flagged card carries a
`ReviewDecision` (`kept` with a card and an optional printing, or `discarded`)
and a `copies` count saying how many slots that one decision is worth. The
`AddSlotBody.setResolution` each decision maps to is decided in
`.claude/memory/decisions.md#2026-09-17-a-user-picked-printing-is-recorded-as-exact`;
note that the enum has no `manual` rung, which the batch endpoint may want to
add rather than inherit that compromise. Positions must stay dense, so the
batch must append in order inside one transaction rather than fan out.
