# 001 — Binder MVP — Implementation Plan

## Approach

A card *printing* — a card as it appears in one set — is the thing a scan
actually identifies, so it gets its own table and is what the match ladder
resolves to. The printed code `LOB-EN005` is stored whole **and** split into
its prefix (`LOB`) and number (`EN005`) as indexed generated columns, because
the fallback lookups key on the halves; deriving them at query time would
make every fallback a sequential scan over ~100k rows.

Binder position is stored as one flat `position` integer per slot, not as
`(page, row, col)`. Page and slot are derived (`position / 9`, `position % 9`).
Reordering is then a single bulk update of integers, and a 3×3 page is
`WHERE position BETWEEN $1 AND $1+8`. The trade-off: reordering rewrites the
positions of the affected range rather than one row. That is the right side of
the trade for binders of a few hundred cards, and it keeps the invariant
"position is unique and dense per binder" enforceable by the database.

Both ingestion pipelines take their HTTP client as an interface so they are
unit-testable against committed fixtures. This is not gold-plating: this
environment cannot reach YGOPRODeck at all, so a pipeline that can only be
tested live could not be verified here.

## Layer breakdown

### Database

| Migration | Purpose |
|-----------|---------|
| `db/migrations/001_cards.sql` | `cards`, `card_sets`, `card_printings` + indexes on `set_code`, `set_prefix`, `set_number`, and a trigram index on `cards.name` for the name fallback |
| `db/migrations/002_users.sql` | `users`, with the opt-in `contact_email` / `contact_phone` kept separate from the auth identity |
| `db/migrations/003_binders.sql` | `binders`, `binder_slots` with `UNIQUE (binder_id, position)` and the `set_resolution` enum |
| `db/migrations/004_listings.sql` | `listings`, with a nullable `cardmarket_listing_id` reserved for phase 2 |

`set_resolution` is an enum, not a bool: `exact`, `by_prefix_and_number`,
`by_number`, `by_name`, `unresolved`. US3 needs to tell the user *how sure*
the match is, which a boolean cannot carry.

### Backend

| File | Change |
|------|--------|
| `internal/card/model/` | `Card`, `CardSet`, `CardPrinting`, `MatchResult`, `SetResolution` |
| `internal/card/store/` | Printing lookups per rung of the ladder; name trigram search |
| `internal/card/service/match.go` | The match ladder. One function, one pass, early return per rung |
| `internal/card/api/` | `POST /scans/resolve`, `GET /cards` |
| `internal/binder/{model,store,service,api}/` | Binder CRUD, paged read, bulk reorder in one tx |
| `internal/listing/{model,store,service,api}/` | List / unlist / browse; seller contact |
| `internal/user/{model,store,service,api}/` | Identity, contact fields |
| `cmd/cardimport/` | One request to `cardinfo.php` for the full dump, then upsert in batches |
| `cmd/cardimages/` | Rate-limited image fetch → object storage; resumable |
| `pkg/ygoprodeck/` | Client + response types + the fetcher interface the tests fake |

### Frontend (`apps/mobile`)

| File | Change |
|------|--------|
| `src/features/scan/` | Camera + frame processor, OCR parse, `useResolveScan` hook, review sheet for flagged matches |
| `src/features/binder/` | 3×3 page grid, swipe between pages, drag-to-reorder, add/remove |
| `src/features/market/` | Listing browse, filters, seller contact sheet |
| `src/features/auth/` | Sign-in flow, session storage in SecureStore |

### Contract

`packages/types` generated from the Go source via the OpenAPI spec — never
hand-edited, per `001-architecture.md`.

## Risks / open items

- D1–D4 in `spec.md` are unanswered; W7 (auth) and W9 (scanner) cannot start
  until D1 and D2 are decided.
- The YGOPRODeck facts this plan assumes are unverified from this environment.
  W2's first task is to verify them and **amend this plan** if they are wrong.
- ML Kit via VisionCamera requires an Expo **dev build**. If you need Expo Go,
  D2 has to change and the scanner becomes a still-capture flow, not continuous.

## Sequence

Toolchain and gate → migrations → (importers ∥ card domain ∥ binder domain ∥ auth)
→ listings → OpenAPI + types regen → (scanner ∥ binder UI ∥ market UI) → gate.
