# 001 — Binder MVP — Tasks

Grouped into **waves**. Everything inside a wave is independent and dispatches
in parallel to one Opus 5 agent per work package; a wave starts when the
previous one is green.

---

## Wave 0 — foundations (serial; blocks everything)

**W0 — toolchain, skeleton and the quality gate.** Nothing can be verified
until the gate exists, so this is one agent, alone.

- [ ] T001 `go.mod`, module layout, `internal/`/`pkg/`/`cmd/` skeleton
- [ ] T002 `.golangci.yml` pinned, matching the linters `002-go-conventions.md` assumes
- [ ] T003 `Makefile` — port the four-command gate (`bootstrap`/`verify`/`check-changed`/`check`) from spotter MR #665, with its change classifier
- [ ] T004 `docker-compose.yml` for Postgres + `tools/test-db-local.sh`
- [ ] T005 Expo app skeleton at `apps/mobile`, TS strict, lint + typecheck + test scripts
- [ ] T006 `.github/workflows/ci.yml` running the same gate as `make check`
- [ ] T007 `make check-ci-parity` so local green means CI green

## Wave 1 — data and identity

- [ ] T010 **W1** Migrations 001–004 per `plan.md` — `db/migrations/`
- [ ] T011 **W1** Store integration test harness on real Postgres — `internal/testdb/`
- [ ] T012 [P] **W7** Auth: sign-in, session JWT, middleware — `internal/user/**` *(blocked on D1)*

## Wave 2 — ingestion and core domains (all parallel)

- [ ] T020 [P] **W2** Verify the YGOPRODeck contract live (shape, rate limit, image policy); amend `plan.md` if it differs — `pkg/ygoprodeck/`
- [ ] T021 [P] **W2** Client + fixtures + full-dump importer — `cmd/cardimport/`
- [ ] T022 [P] **W3** Rate-limited, resumable image pipeline → object storage — `cmd/cardimages/` *(blocked on D3)*
- [ ] T023 [P] **W4** [US2][US3] Card model + store lookups per ladder rung — `internal/card/{model,store}/`
- [ ] T024 [P] **W4** [US2][US3] The match ladder + `SetResolution` — `internal/card/service/match.go`
- [ ] T025 [P] **W4** `POST /scans/resolve`, `GET /cards` — `internal/card/api/`
- [ ] T026 [P] **W5** [US4][US5] Binder model, store, paged read, bulk reorder in one tx — `internal/binder/**`

## Wave 3 — listings and the contract

- [ ] T030 **W6** [US6][US7] Listings domain + seller contact — `internal/listing/**` *(blocked on D4)*
- [ ] T031 [REGEN] **W8** OpenAPI spec generated from the Go source — `make gen-spec`
- [ ] T032 [REGEN] **W8** `packages/types` regenerated; `make check-contract` green

## Wave 4 — mobile (all parallel)

- [ ] T040 [P] **W9** [US1] Camera + continuous frame-processor OCR — `apps/mobile/src/features/scan/` *(blocked on D2)*
- [ ] T041 [P] **W9** [US2][US3] Code parsing, `useResolveScan`, review sheet for flagged matches — same feature
- [ ] T042 [P] **W10** [US4][US5] 3×3 page grid, paging, drag-to-reorder, add/remove — `apps/mobile/src/features/binder/`
- [ ] T043 [P] **W11** [US6][US7] Listing browse + seller contact sheet — `apps/mobile/src/features/market/`

## Wave 5 — done means done

- [ ] T050 Quality gate — `make check`
- [ ] T051 Quality gate — `cd apps/mobile && npm run lint && npm run typecheck && npm run test:run && npm run knip`
- [ ] T052 UI screenshots of every new screen in BOTH themes
- [ ] T053 Clean-code checklist — `.claude/rules/007-clean-code-checklist.md` walked over every touched file
- [ ] T054 Memory written: what the next session would re-derive — `.claude/memory/`

---

## Dispatch map

| Wave | Agents in parallel | Blocked on |
|------|--------------------|------------|
| 0 | W0 | — |
| 1 | W1, W7 | W0; W7 also on **D1** |
| 2 | W2, W3, W4, W5 | W1; W3 also on **D3** |
| 3 | W6, W8 | W4, W5; W6 also on **D4** |
| 4 | W9, W10, W11 | W8; W9 also on **D2** |
| 5 | single | all |

**D1–D4 are in `spec.md` and are unanswered.** W7, W3, W6 and W9 do not start
until their decision lands. Waves 0–2 minus W3 are unblocked today.
