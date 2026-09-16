# Binder

Digital trading-card binders. Scan a card with the phone, match it against a
Postgres card database, organise it into 3×3 binder pages, list it for sale.

> **Keep this file under 120 lines.** Move detail into layer-specific
> CLAUDE.md files or `.claude/rules/`.

## Status: toolchain and schema in, domains not yet

The gate, the Expo skeleton and `db/migrations/` exist. `internal/`, `pkg/`,
`cmd/` and `packages/` are still empty — the Go lanes of the gate say so and
skip rather than failing, and start doing real work the moment a package lands.

```
binder/
├── apps/mobile/       # Expo + React Native dev build (scanner + binder UI)
├── packages/          # shared workspaces — empty until the OpenAPI types land
├── internal/          # Go backend, clean architecture, one dir per domain
├── cmd/               # Go CLI entry points (incl. the card-import pipeline)
├── pkg/               # Shared Go packages
└── db/migrations/     # Forward-only SQL, applied by tools/migrate.sh
```

Go domain layout, per domain: `api/` → `service/` → `store/` → `model/`.
Data flows one direction. Never import upward. (`.claude/rules/001-architecture.md`)

## Rules by layer

Rules live in `.claude/rules/` and load automatically — `000`, `001`, `004`,
`007` in every session, the layer files when you read a matching path
(`paths:` frontmatter). The table is the map, not a to-do.

| Working in | Read |
|------------|------|
| **All layers** | `.claude/rules/000-principles.md` (engineering principles — always read first) + `.claude/rules/007-clean-code-checklist.md` (definition of done, walked before every PR) |
| `apps/mobile/` | `.claude/rules/003-frontend.md` + `.claude/rules/005-mobile.md` + `.claude/rules/004-security.md` |
| `internal/` | `.claude/rules/002-go-conventions.md` + `.claude/rules/004-security.md` |
| Cross-cutting | `.claude/rules/001-architecture.md` |
| Testing (all layers) | `.claude/rules/006-testing.md` |

**The rules still cite spotter's tree** (`apps/trainer-web`, `internal/nutrition`):
worked examples of the principle, not paths here. Read them for the rule and
translate the path; rewrite a rule's examples only when this repo has a real
equivalent, and say so in the commit.

## Quality gate

`make` on its own prints the map. Four commands:

```bash
make bootstrap       # once, fresh container: npm deps, pinned linters, `make warm`
make verify          # while you work: scoped lint+test for what changed, in parallel
make check-changed   # before done: the FULL gate, only for the layers you touched
make check           # everything, serial (CI equivalent)
```

`verify` and `check-changed` scope by layer — `go`, `mobile`, `packages` —
so a mobile task never pays for the Go suite; **anything under `packages/`
counts as the mobile app too.** `BASE=<ref>` compares against another ref.
The classifier is copied verbatim from CI's `changes` job and
`make check-ci-parity` fails the gate if the two ever drift.

After `bootstrap`, put the Go bin dir **first** on PATH in the same shell as
the gate — `export PATH="$(go env GOPATH)/bin:$PATH"` — or an older
`golangci-lint` from the image shadows the pinned one. The store suites get
their Postgres from `tools/test-db-local.sh` (Docker image pulls are blocked
here); every test target calls it when `TEST_DATABASE_URL` is unset, and
`make migrate-test` applies `db/migrations/`. The egress traps behind both are
in memory (`deploy.md`, `go.md`). **Do not claim a gate passed that you did
not run.**

## Memory — use it before re-deriving anything

`.claude/memory/` is a git-tracked index you **grep by symptom**; the
SessionStart hook prints the map, not the contents. It carries only what this
repo has taught a session, never spotter's units. Keep writing yours: what the
next session would re-derive goes in before done. Format and placement table:
`.claude/memory/README.md`. Precedence: `.claude/rules/` > memory > your own
inference; `decisions.md` is binding.

## Boy-scout rule

Leave code cleaner than found. Fix pre-existing lint/TS errors in files you touch.

## Spec pipeline

For features touching ≥2 layers, DB migrations, or new endpoints — use
`/spec-feature` → `/spec-plan` → `/spec-tasks` → `/spec-review` +
`/spec-analyze` → `/spec-implement`. Skip for: bug fixes, translations, dep
bumps, renames, test-only changes. Active specs live in `specs/`.

`specs/001-binder-mvp/` is the live one: scanning, card matching and the binder
model are all ≥2 layers, so every slice of it goes through the pipeline.

## Skills

Project skills live in `.claude/skills/`. Prefer them over generic approaches.

| Situation | Use |
|-----------|-----|
| New feature spec | `/spec-feature` |
| Implementation plan from spec | `/spec-plan` |
| Break plan into tasks | `/spec-tasks` |
| Audit spec/plan/tasks for over-engineering (YAGNI) | `/spec-review` |
| Execute tasks | `/spec-implement` |
| Senior-engineer quality pass over one slice: Go / mobile | `/go-quality-sweep` / `/fe-refactor` |
| Behavior-preserving store-SQL query pass, proven on Postgres | `/sql-optimize` |
| Cross-layer audit of one domain concept | `/domain-audit` |
| Behavior-preserving hygiene sweep, findings logged not fixed | `/cleanup` |
| Test-first fix of 1–3 backlog findings | `/fix-findings` |
| Nightly memory consolidation | `/memory-dream` |
| Go technique reference | the `golang-*` skills — see `.claude/skills/VENDOR.md` |
| Reanimated / Gesture Handler / SVG technique | `react-native-best-practices` |
| Open-ended design exploration | `superpowers:brainstorming` → feed back into `/spec-feature` |

Several skills (`/render-triage`, the Routine cadences in the sweeps,
`/memory-dream`'s `tools/memory-check.sh`) assume infrastructure this repo does
not have yet. They are here so the shape survives; wire them up when the thing
they watch exists.

## Provenance

Seeded from `simeonkorchev/spotter` @ MR #665. Verbatim: `.claude/rules/`,
`.claude/skills/`, `.claude/agents/`, `.claude/commands/`. Adapted: this file,
`.claude/settings.json`, `.claude/hooks/`, `.claude/memory/README.md`.
Deliberately excluded: spotter's memory units and all application code.
The Makefile, `tools/` and CI were ported in W0, adapted to this repo's
layers (there is no trainer-web; the app is `apps/mobile`).
