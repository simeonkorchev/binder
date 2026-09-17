# 001 — Binder MVP — Tasks

Grouped into **waves**. Everything inside a wave is independent and dispatches
in parallel to one Opus 5 agent per work package; a wave starts when the
previous one is green.

---

## Wave 0 — foundations (serial; blocks everything)

**W0 — toolchain, skeleton and the quality gate.** Nothing can be verified
until the gate exists, so this is one agent, alone.

- [x] T001 `go.mod`, module layout, `internal/`/`pkg/`/`cmd/` skeleton
- [x] T002 `.golangci.yml` pinned, matching the linters `002-go-conventions.md` assumes
- [x] T003 `Makefile` — port the four-command gate (`bootstrap`/`verify`/`check-changed`/`check`) from spotter MR #665, with its change classifier
- [x] T004 `docker-compose.yml` for Postgres + `tools/test-db-local.sh`
- [x] T005 Expo app skeleton at `apps/mobile`, TS strict, lint + typecheck + test scripts
- [x] T006 `.github/workflows/ci.yml` running the same gate as `make check`
- [x] T007 `make check-ci-parity` so local green means CI green

## Wave 1 — data and identity

- [x] T010 **W1** Migrations 001–004 per `plan.md` — `db/migrations/`
- [x] T011 **W1** Store integration test harness on real Postgres — `internal/testdb/`
- [x] T012 [P] **W7** Auth: sign-in, session JWT, middleware — `internal/user/**`. Provider tokens verified against the provider's JWKS in `pkg/oidc` (fetching behind an interface, TTL cache, refetch on an unknown key id); session JWT in `internal/user/session` with the secret validated at startup and **no fallback**; `api.Middleware` + `api.ActorFromContext` is the `ActorFunc` the binder and listing domains take. **No live Apple or Google sign-in has been performed** — `appleid.apple.com/auth/keys` is 403 at this environment's proxy; Google's JWKS endpoint was fetched and parsed successfully with the real `HTTPFetcher`

## Wave 2 — ingestion and core domains (all parallel)

- [ ] T020 [P] **W2** Verify the YGOPRODeck contract live (shape, rate limit, image policy); amend `plan.md` if it differs — `pkg/ygoprodeck/`. **Still open: needs a machine with egress** — the exact curl commands and what to change per outcome are in `VERIFY-YGOPRODECK.md`
- [x] T021 [P] **W2** Client + fixtures + full-dump importer — `cmd/cardimport/`
- [x] T022 [P] **W3** Rate-limited, resumable image pipeline → object storage — `cmd/cardimages/`
- [x] T023 [P] **W4** [US2][US3] Card model + store lookups per ladder rung — `internal/card/{model,store}/`
- [x] T024 [P] **W4** [US2][US3] The match ladder + `SetResolution` — `internal/card/service/match.go`
- [x] T025 [P] **W4** `POST /scans/resolve`, `GET /cards` — `internal/card/api/`
- [x] T026 [P] **W5** [US4][US5] Binder model, store, paged read, bulk reorder in one tx — `internal/binder/**`. The reorder is one bulk statement over the affected window; positions get past the non-deferrable `UNIQUE (binder_id, position)` by parking above `max(position)` and returning (`internal/binder/store/position.go`)

## Wave 3 — listings and the contract

- [x] T030 **W6** [US6][US7] Listings domain + seller contact — `internal/listing/**`. Four endpoints; the seller is reached by joining `binder_slots → binders`, and their contact details through a consumer-side `SellerContacts` interface the user domain satisfies at wiring time. Someone else's slot or listing is 404, never 403; a double-list is the UNIQUE constraint translated to 409, proven against the real schema, as is the cascade that takes a listing away with its card
- [x] T033 **W12** HTTP server bootstrap — `cmd/binderd/main.go`. **Nothing serves any endpoint today**: `binder`, `card`, `listing` and `user` all expose `RegisterEndpoints`, and only handler specs ever call them. This blocks T031, because `make gen-spec` generates the OpenAPI document from a registered `huma.API`, and it blocks every Wave 4 screen. Includes:
  - one `humago` API with all four domains registered, config from env validated at startup (exit if missing, per `004-security.md`)
  - **the contact seam adapter.** W6 declared `listing/service.SellerContacts.ContactFor(ctx, uuid) (listing/model.SellerContact, error)`; W7 landed `user/service.Service.SellerContact(ctx, uuid) (user/model.Contact, error)` — same two `*string` fields and the same documented contract, different name and type. Adapt here, at the wiring layer. **Do not change either domain to force a match** — that is what the consumer-side interface exists to avoid (`002` §3a).
  - W7's session middleware supplying the `ActorFunc` the binder and listing APIs take. Concretely: `session.LoadConfig()` → `session.New(cfg, nil)` (fails on an unset `SESSION_JWT_SECRET` and on one under 32 bytes — there is deliberately no fallback), `identity.LoadConfig()` → `identity.NewFromConfig(cfg, nil)` (needs `APPLE_CLIENT_IDS` and `GOOGLE_CLIENT_IDS`, comma-separated), then `userapi.Middleware(tokens)` wrapped around the mux and `userapi.ActorFromContext` passed to all three `RegisterEndpoints`. **Do not declare a context key anywhere else** — `internal/user/api/actor_test.go` already asserts at compile time that `ActorFromContext` satisfies both other domains' `ActorFunc`
  - a smoke test that the process starts, serves `/health`, and that every registered route answers (the first time anything is exercised over a real socket)
- [x] T031 [REGEN] **W8** OpenAPI spec generated from the Go source — `make gen-spec`
- [x] T032 [REGEN] **W8** `packages/types` regenerated; `make check-contract` green

## Wave 4 — mobile (all parallel)

### W9 — the scan experience [US1][US2][US3]

The product differentiator. A continuous sweep: the user moves the phone across
a binder page and cards land in the session without a single tap. **The scan
loop never blocks on the network** — resolution is fired async and its result
catches up.

- [x] T040 VisionCamera setup, camera permission flow, Expo config plugin for the **dev build** — **code complete, screenshots not taken.** `useCameraAccess` classifies a refusal *after* asking, because Android reports `denied` both for a permission never requested and for one blocked forever; `denied` gets another ask, `blocked` gets the settings app, a phone with no capture device gets a translated line, and the permission is re-read on resume. The config plugin landed with the app shell (`app.config.ts`). Unticked for the same reason as T038 — see T072
- [x] T041 Card-shaped guide-frame overlay, with the code line's expected position hinted (the printed code sits *below* the art) — **code complete, screenshots not taken.** `components/ScanGuideFrame.tsx`: a 59 × 86 frame and a dashed strip on the bottom-right where the code is printed. Unticked for the same reason as T038 — see T072
- [x] T042 ML Kit text-recognition frame processor, **throttled** — running every frame burns battery for no extra reads — `useTextFrames.ts`
- [x] T043 `{CODE}-{ID}` parser: pure function, OCR-noise tolerant (`0`/`O`, `1`/`I`/`l`, stray punctuation, case). **Completeness test per `000-principles.md` §9** — `lib/parseCardCode.ts`
- [x] T044 Stable-read debounce + in-session dedupe: accept only after N consecutive identical reads; a re-read while the code stays in frame is the *same* card, a re-read after it left is a second copy. Pure and unit-tested — `lib/useStableRead.ts`
- [x] T045 `useResolveScan` — async, queued, non-blocking; survives a dropped connection and resolves later — `api/useResolveScan.ts`
- [x] T046 Capture feedback: haptic tick, running count, captured-card strip — **code complete, screenshots not taken.** No Reanimated: the strip is a `FlatList` of fixed-width tiles and adding a native animation library to a dev build for it is a spec decision (`.claude/memory/decisions.md#2026-09-17-capture-feedback-is-a-tick-a-count-and-a-strip-with-no-reanimated`). `useScanSession` wires the loop, including the absent reads `useStableRead` needs to see a second copy. Unticked for the same reason as T038 — see T072
- [x] T047 Review sheet for flagged matches — `unresolved` and `by_name` slots surfaced for confirm/correct. **US3 is only satisfied here**, not by the scan loop — **code complete, screenshots not taken.** Four flags, read off the answer's *shape* because the ladder answers `unresolved` both for "several matched" and for "nothing matched" (`lib/flaggedScans.ts`): a card with no set, a code that matched nothing, a scan with candidates to choose between, and a scan the resolver refused. Every row ends in a press — a candidate, keeping the named card with its set open, a name search through `GET /cards?q=`, or leaving the card out — and every reason is a sentence, never a colour. A `Modal` over the scan tab rather than a route, because the session lives in the screen's hooks. Screenshots outstanding for the same reason as T038 — see T072
- [ ] T048 Commit the reviewed session into the binder — one call, one transaction — **blocked: the contract publishes no endpoint that can do it.** Every binder write is single-row (`POST /binders` takes a name only, `POST /binders/{binderId}/slots` takes one `cardId`) and `service.AddSlot` opens its own `InTx` per call, so a 60-card sweep is 61 requests and a failure at card 40 leaves a binder holding 39 — after the user has put the cards away. Deliberately **not** built as a client-side loop. Needs a batch endpoint across `api → service → store`, which is `/spec-feature` work: see finding `2026-09-17-no-atomic-way-to-commit-a-scanned-session` and `.claude/memory/decisions.md#2026-09-17-a-user-picked-printing-is-recorded-as-exact` for the `set_resolution` each decision maps to

### W13 — mobile foundations (blocks every screen below)

Same class of gap as T033: the rules require things no task owned. `apps/mobile`
has no i18n, no theme tokens and no navigation, but `003-frontend.md` and
`005-mobile.md` require every user-facing string through `t('key')` present in
**both** locales, static styles in `StyleSheet.create` using **theme tokens with
no hardcoded colours**, and the app has five screens to move between.

- [x] T036 i18n setup + `locales/{en,bg}.json`, and a test that fails when a key exists in one locale but not the other — i18next, language from the device preference list; `src/i18n/locales.test.ts` compares the flattened key sets both ways, and `i18next.d.ts` types `t()` against `en.json` so an unknown key does not compile
- [x] T037 Theme tokens, light **and** dark — `src/theme/`, nine tokens named for the surface they sit on, `useTheme()` following the device appearance. `tokens.test.ts` is the gate: WCAG AA for every text pairing plus a separation floor between adjacent fills, in both palettes
- [ ] T038 Navigation — **code complete, screenshots not taken.** React Navigation (not expo-router; reasons in `.claude/memory/decisions.md#2026-09-17-navigation-is-react-navigation-not-expo-router`), three tabs (Scan, Binders, Market) and two pushed routes (`BinderPage {binderId}`, `SellerContact {sellerId}`), titles and tab names through `t()`, chrome mapped from the theme tokens. Every route renders `PlaceholderScreen` until its wave lands. Left unticked because `CLAUDE.md` requires both-theme screenshots before a UI change is done and **this container has no emulator** (no `adb`, no Android SDK) — see T072

### W10 — the binder [US4][US5]

- [ ] T050 3×3 page grid, swipe paging, page indicator — `apps/mobile/src/features/binder/`
- [ ] T051 Drag-to-reorder across slots and across page boundaries; bulk position update in one call
- [ ] T052 Add and remove cards; translated empty state for an empty binder

### W11 — the marketplace [US6][US7]

- [ ] T060 Mark a slot for sale / unlist — `apps/mobile/src/features/market/`
- [ ] T061 Browse listings, filters, translated empty state
- [ ] T062 Seller contact sheet — reveals only opted-in fields, and renders the "no contact shared" case

## Wave 5 — done means done

- [ ] T070 Quality gate — `make check`
- [ ] T071 Quality gate — `cd apps/mobile && npm run lint && npm run typecheck && npm run test:run && npm run knip`
- [ ] T072 UI screenshots of every new screen in BOTH themes (`CLAUDE.md` requires this before any UI change is done) — **still blocked in this container**: no emulator, no `adb`, no Android SDK, and the scanner is a dev build, so neither Expo Go nor a web preview can render it. Blocks ticking T038, T040, T041, T046 and T047
- [ ] T073 Clean-code checklist — `.claude/rules/007-clean-code-checklist.md` walked over every touched file
- [ ] T074 Memory written: what the next session would re-derive — `.claude/memory/`

---

## Dispatch map

| Wave | Agents in parallel | Blocked on |
|------|--------------------|------------|
| 0 | W0 | — |
| 1 | W1, W7 | W0 |
| 2 | W2, W3, W4, W5 | W1 |
| 3 | W6, W8 | W4, W5 |
| 4 | W9, W10, W11 | W8 |
| 5 | single | all |

D1–D4 are decided (see `spec.md`). Every wave is unblocked; each starts when
the previous one is green.
