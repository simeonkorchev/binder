# 001 — Binder MVP: scan, match, organise, sell

## Status: Review

## Problem

Collectors have hundreds of physical trading cards in binders and no practical
way to get them into a digital form. Typing a card list by hand is the reason
most collections are never catalogued. If capturing a binder takes an evening
instead of a weekend, everything downstream — knowing what you own, trading,
selling — becomes possible.

## User stories

- **US1** — As a collector, I want to sweep my phone across a binder page and
  have cards recognised continuously, so I never tap "capture" per card.
- **US2** — As a collector, I want a card whose printed code was misread to
  still resolve to the right card, so OCR noise does not cost me an entry.
- **US3** — As a collector, I want cards whose set could not be determined to
  be flagged rather than silently guessed, so my collection is not quietly wrong.
- **US4** — As a collector, I want my scanned cards laid out as 3×3 binder
  pages that mirror the physical binder.
- **US5** — As a collector, I want to reorder, add and remove cards in a binder.
- **US6** — As a seller, I want to mark cards for sale.
- **US7** — As a buyer, I want to browse cards for sale and contact the seller.

## Scope

### In
- Continuous on-device scanning: card detection + OCR of name and printed code.
- Card database imported from YGOPRODeck, in Postgres.
- Card image pipeline, rate-limited, images stored in our own object storage.
- Match ladder: exact code → number + prefix → number → name; unresolved flagged.
- Binder: 3×3 paged view, reorder, add, remove.
- Listings: mark for sale, browse listings, contact seller (phone / email).
- Auth: sign in, one account owns its binders.

### Out (deliberately, for this MVP)
- **In-app chat.** The brief allows a contact button as the fallback; that is
  what MVP ships. Chat is a product of its own (moderation, push, retention)
  and `000-principles.md` §8 forbids building it speculatively.
- **Cardmarket publishing.** Phase 2. The listing model is shaped so a
  `cardmarket_listing_id` column and a publisher service are additive.
- Price data, deck building, trade matching, multi-game support (Yu-Gi-Oh only).
- Web client.

## API contract

```
POST   /scans/resolve        body: {ocrText, codeText, name} -> {match, confidence, candidates[]}
GET    /cards/{id}
GET    /cards?q=&set=&page=

POST   /binders              -> binder
GET    /binders              -> binder[]
GET    /binders/{id}?page=n  -> {slots[9], pageCount}
PATCH  /binders/{id}         -> rename
POST   /binders/{id}/slots   -> add card at position
PATCH  /binders/{id}/slots   -> reorder (bulk position update)
DELETE /binders/{id}/slots/{slotId}

POST   /listings             -> mark a slot for sale
DELETE /listings/{id}
GET    /listings?q=&set=     -> browse
GET    /sellers/{id}/contact -> {email, phone} (owner-consented fields only)
```

## Data model changes

New tables: `cards`, `card_sets`, `card_printings`, `users`, `binders`,
`binder_slots`, `listings`. Full DDL and the reasoning for each in `plan.md`.

## Open decisions — these need your call before implementation

| # | Decision | Recommendation |
|---|----------|----------------|
| D1 | Auth provider | Apple + Google sign-in via Expo AuthSession, Go-issued session JWT. WorkOS (spotter's choice) is built for B2B orgs; this is a consumer app. |
| D2 | OCR engine | `react-native-vision-camera` frame processor + ML Kit text recognition. Cross-platform and on-device. Apple's `DataScannerViewController` feels closer to the brief's "Apple document-like" but is iOS-only. **Either way this needs an Expo dev build — it will not run in Expo Go.** |
| D3 | Object storage for card images | GCS, matching spotter's stack. S3 if you'd rather not add a second cloud. |
| D4 | Seller contact | Contact button revealing email/phone the seller opted in to share. No chat in MVP (see Out). |

## Risks

- **Network egress**: this build environment blocks `ygoprodeck.com` and
  `images.ygoprodeck.com` (HTTP 403 at the proxy). The importers will be built
  behind a fetcher interface and tested against committed fixtures; the real
  ingest must run somewhere with egress. **No task is marked done on the claim
  that an unrunnable pipeline works.**
- **YGOPRODeck API details are from prior knowledge, not the live docs** (they
  are unreachable from here): the v7 `cardinfo.php` shape, the ~20 req/s limit,
  and the "download images, do not hotlink" rule. W2 verifies all three against
  the live API before the importer is trusted.
- OCR accuracy on foil/damaged cards is the main product risk. The match ladder
  exists precisely because exact reads cannot be assumed.
