# 001 — Addendum: committing a reviewed scan session

Compact addendum to `spec.md` / `plan.md` / `tasks.md`, written instead of the
full `/spec-feature` ceremony because it changes one endpoint's worth of
surface. It unblocks **T048** and closes finding
`2026-09-17-no-atomic-way-to-commit-a-scanned-session`.

Three things are decided here, in the order they depend on each other.

---

## 1. The scan's *outcome* and the slot's *resolution* are two types

`set_resolution` answers **"how was this card's set determined?"** — it is what
a `binder_slots` row stores, and the Postgres enum of the same name is its
vocabulary. It was also being used to answer a second, unrelated question:
**"what should the client do next with this scan?"**. It cannot answer both,
and the proof is that `internal/card/service/match.go` returns `unresolved`
from two paths that need opposite reactions:

| path | meaning | the user's next move |
|---|---|---|
| `ambiguousPrintings()` | several printings matched; the *card* may be settled | pick one |
| `unresolved(nil)` | nothing matched at all | search by name, or give up |

So a client could not read `resolution` to decide what to show, and
`apps/mobile/.../lib/flaggedScans.ts` classifies on the response's *shape*
instead — a derivation each client re-invents, which is the thing the server is
supposed to have already done.

**Decision.** Add a second, separate concept: `card/model.ScanOutcome`, carried
on `MatchResult` and published as `outcome` on `POST /scans/resolve`.

```
resolved    the card and the set are both settled   (card, printing, no candidates)
card_only   the card is settled, the set is not     (card, no printing, no candidates)
ambiguous   several matched; the user must choose   (candidates non-empty)
no_match    nothing matched                         (no card, no candidates)
```

It is **not** derived from the shape at the edge: every rung of the ladder
states its own outcome where it concludes, so the two `unresolved` sites become
two different answers at the source. `resolution` keeps its old meaning and its
old values — "the set is not determined" is still true of both paths, it is just
not the whole story — so nothing on the wire is removed and no migration is
needed for this part.

The invariant `outcome` ⇄ (card, printing, candidates) is pinned by a table
test over every shape the ladder can produce.

## 2. A user-picked printing is recorded as `manual`, not `exact`

recorded, under protest, that the review sheet has to file a printing the *user*
chose as `exact` because the enum has no rung for "a person decided". That
entry explicitly left the door open for the batch endpoint to do better.

**Decision.** Add `manual` to the `set_resolution` enum
(`db/migrations/005_manual_set_resolution.sql`). It is a printing-bearing
resolution: `RequiresPrinting('manual')` is true, and the `binder_slots` CHECK
keeps its equivalence.

Why a sixth enum value and not an orthogonal `decided_by` column:

- The harm is a *stored* value that is false. `exact` means "the whole printed
  code matched one printing", and a slot written that way is indistinguishable
  from a real code match forever after. A provenance column beside a false
  `set_resolution` would not make the false one true.
- `set_resolution` already answers exactly one question — how the set was
  determined — and "a person read it off the card" is an answer to that
  question, not to a different one. This is the same test applied in §1, and it
  comes out the other way here.
- One value, and the existing equivalence holds unchanged: `manual` joins
  `exact`, `by_prefix_and_number`, `by_number` on the printing-bearing side.

`manual` is only for the printing-bearing case. A user who keeps a card and
leaves its set open is `by_name` — the card is known and the set is not, which
is precisely what `by_name` already says, and nobody is misled by it.

The CHECK is rewritten to name the two rungs that do **not** determine a set
rather than the ones that do, so the migration's second statement never mentions
`manual` (Postgres refuses a new enum value used in the same transaction that
added it) and a later rung does not need the constraint changed again:

```sql
CHECK ((set_resolution NOT IN ('by_name', 'unresolved')) = (card_printing_id IS NOT NULL))
```

`manual` is a resolution a **slot may store**, never one the **ladder can
answer**. Those were one list (`model.SetResolutions()`); they become two:

- `SetResolutions()` — the `set_resolution` enum, verbatim; the vocabulary of a
  slot, and of `POST /binders/{binderId}/slots*`.
- `ScanResolutions()` — what `ResolveScan` can return; the enum published on
  `POST /scans/resolve`.

## 3. `POST /binders/{binderId}/slots/batch` — one request, one transaction

### Request

```http
POST /binders/{binderId}/slots/batch
```

```json
{
  "cards": [
    { "cardId": "…", "cardPrintingId": "…",  "setResolution": "exact"  },
    { "cardId": "…", "cardPrintingId": "…",  "setResolution": "manual" },
    { "cardId": "…", "cardPrintingId": null, "setResolution": "by_name" }
  ]
}
```

Every entry becomes one slot, **appended in order** after the binder's last
card. Deliberately absent:

- **No `position`.** A batch appends; that is what committing a sweep does.
  Explicit positions in a batch would let one request ask for an arrangement
  with a hole in it, which is the invariant this domain exists to keep.
- **No `copies` count.** A decision covering three physical cards is three
  entries. The client already holds `copies` and expanding it is one loop;
  putting it on the wire adds a second way to say the same thing and a second
  thing the bound has to be computed over.

### Response — `201 Created`

```json
{ "slots": [ { …slotBody… } ] }
```

`slotBody` is the existing single-slot DTO, in position order. A client that
sent `n` cards gets `n` slots back in the order it sent them.

### Rules

- **All or nothing.** One `InTx`: the ownership check, the count, and a single
  multi-row `INSERT … RETURNING`. A failure anywhere rolls the whole batch
  back; the binder is never left holding a prefix of the sweep.
- **Positions stay dense from zero.** The batch takes `count … count+n-1`,
  which are free by the density invariant, so no gap has to be opened and no
  row has to be parked (`internal/binder/store/position.go`).
- **Empty is not an error** (`000` §8b). `{"cards": []}` on a binder the actor
  owns is `201` and `{"slots": []}`; ownership is still checked, so an empty
  batch against somebody else's binder is the same `404` as a full one.
- **Bounded**: `model.MaxSlotsPerBatch` = `SlotsPerPage * 50` = **450** — fifty
  binder pages, far more than one sitting at the scanner produces. Past it the
  request is rejected with `422` and nothing is written; the client splits the
  commit. Enforced twice (`000` §10): `maxItems` in the schema, so huma answers
  before a handler runs, and `service.ErrBatchTooLarge` in the service, so the
  guard is not a licence for the layer below to assume.
- **Boundary validation** (`000` §10): each entry's `setResolution` and
  `cardPrintingId` must agree, checked in `Resolve` with the offending index in
  the error location (`body.cards[3].cardPrintingId`), and checked again in the
  service, which returns the same sentinels `AddSlot` does.
- **404, not 403**, for another user's binder, as everywhere else in this domain.

### Layers touched

| File | Change |
|---|---|
| `db/migrations/005_manual_set_resolution.sql` | new — the `manual` rung and the rewritten CHECK |
| `internal/card/model/match.go` | `SetResolutionManual`, `ScanResolutions()`, `ScanOutcome`, `MatchResult.Outcome` |
| `internal/card/service/match.go` | each rung states its outcome; `unresolved()` splits into `noMatch()` and `ambiguousCards()` |
| `internal/card/api/scan.go` | `outcome` on the wire; the scan enum narrows to `ScanResolutions()` |
| `internal/binder/model/input.go` | `SlotCard` (shared by both writes), `AddSlotsInput`, `MaxSlotsPerBatch` |
| `internal/binder/service/service.go` | `InsertSlots` on `SlotStore`; `ErrBatchTooLarge` |
| `internal/binder/service/slot.go` | `AddSlots` |
| `internal/binder/store/slot.go` | `InsertSlots` — one multi-row INSERT, placeholders from loop indices only (`004` permitted exception) |
| `internal/binder/api/slot.go` | the operation, its DTOs and their `Resolve` |
| `internal/binder/api/errors.go` | `ErrBatchTooLarge` → 422 |
| `packages/types/` | regenerated by `make gen-spec check-types` |

### Not done here

`apps/mobile/` is mid-wave from another change and is out of scope for this
addendum. Two follow-ups for whoever picks T048's client half up:

1. `lib/flaggedScans.ts` can classify on `outcome` and stop sniffing the shape.
2. The review sheet sends `manual` for a printing the user picked, not `exact`.
