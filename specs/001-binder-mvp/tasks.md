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
- [ ] T012 [P] **W7** Auth: sign-in, session JWT, middleware — `internal/user/**`

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
- [ ] T031 [REGEN] **W8** OpenAPI spec generated from the Go source — `make gen-spec`
- [ ] T032 [REGEN] **W8** `packages/types` regenerated; `make check-contract` green

## Wave 4 — mobile (all parallel)

### W9 — the scan experience [US1][US2][US3]

The product differentiator. A continuous sweep: the user moves the phone across
a binder page and cards land in the session without a single tap. **The scan
loop never blocks on the network** — resolution is fired async and its result
catches up.

- [ ] T040 VisionCamera setup, camera permission flow, Expo config plugin for the **dev build** — `apps/mobile/src/features/scan/`
- [ ] T041 Card-shaped guide-frame overlay, with the code line's expected position hinted (the printed code sits *below* the art) — `components/ScanGuideFrame.tsx`
- [ ] T042 ML Kit text-recognition frame processor, **throttled** — running every frame burns battery for no extra reads — `useTextFrames.ts`
- [ ] T043 `{CODE}-{ID}` parser: pure function, OCR-noise tolerant (`0`/`O`, `1`/`I`/`l`, stray punctuation, case). **Completeness test per `000-principles.md` §9** — `lib/parseCardCode.ts`
- [ ] T044 Stable-read debounce + in-session dedupe: accept only after N consecutive identical reads; a re-read while the code stays in frame is the *same* card, a re-read after it left is a second copy. Pure and unit-tested — `lib/useStableRead.ts`
- [ ] T045 `useResolveScan` — async, queued, non-blocking; survives a dropped connection and resolves later — `api/useResolveScan.ts`
- [ ] T046 Capture feedback: haptic tick, running count, captured-card strip. `react-native-best-practices` skill governs any Reanimated used here
- [ ] T047 Review sheet for flagged matches — `unresolved` and `by_name` slots surfaced for confirm/correct. **US3 is only satisfied here**, not by the scan loop
- [ ] T048 Commit the reviewed session into the binder — one call, one transaction

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
- [ ] T072 UI screenshots of every new screen in BOTH themes (`CLAUDE.md` requires this before any UI change is done)
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
