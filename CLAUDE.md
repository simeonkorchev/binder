# Binder

Digital trading-card binders. Scan a card with the phone, match it against a
Postgres card database, organise it into 3×3 binder pages, list it for sale.

> **Keep this file under 120 lines.** Move detail into layer-specific
> CLAUDE.md files or `.claude/rules/`.

## Status: seeded, not yet built

**No application code exists yet.** This repo currently contains only the
engineering guardrails, copied from `simeonkorchev/spotter` (see
*Provenance* at the bottom). The first implementation session creates the
tree. Until then, treat every path below as *planned*, not present.

Planned layout — confirm or change it in the spec before building:

```
binder/
├── apps/mobile/       # Expo + React Native (the scanner + the binder UI)
├── packages/types/    # Auto-generated OpenAPI TS types (never hand-edit)
├── internal/          # Go backend, clean architecture, one dir per domain
├── cmd/               # Go CLI entry points (incl. the card-import pipeline)
├── pkg/               # Shared Go packages
└── db/                # Migrations & seed data
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

**The rules still cite spotter's tree** — `apps/trainer-web`, `internal/nutrition`,
`packages/intl`. Those are *worked examples of the principle*, not paths here.
Read them for the rule; translate the path. Rewrite a rule's examples only
when this repo has a real equivalent to point at, and say so in the commit.

## Quality gate — does not exist yet

Spotter's four-command gate (`make bootstrap` / `verify` / `check-changed` /
`check`) is **not** in this repo: it is wired to spotter's Makefile, its
change classifier and its CI. Standing up the equivalent here is part of the
first implementation slice, and the shape to copy is documented in
[spotter MR #665](https://github.com/simeonkorchev/spotter/pull/665).

Until it exists, run the underlying tools directly (`go test ./...`,
`golangci-lint run`, the app's own `npm test`) and say in the PR what you ran.
**Do not claim a gate passed that is not there.**

## Memory — use it before re-deriving anything

`.claude/memory/` is a git-tracked index you **grep by symptom**; the
SessionStart hook prints the map, not the contents. It is **empty on purpose** —
spotter's 145 units describe spotter, not this codebase. Write yours as you
learn: what the next session would re-derive goes in before done. Format and
the placement table: `.claude/memory/README.md`. Precedence:
`.claude/rules/` > memory > your own inference; `decisions.md` is binding.

## Boy-scout rule

Leave code cleaner than found. Fix pre-existing lint/TS errors in files you touch.

## Spec pipeline

For features touching ≥2 layers, DB migrations, or new endpoints — use
`/spec-feature` → `/spec-plan` → `/spec-tasks` → `/spec-review` +
`/spec-analyze` → `/spec-implement`. Skip for: bug fixes, translations, dep
bumps, renames, test-only changes. Active specs live in `specs/`.

Given the repo is empty, **the first real feature should go through the full
pipeline** — scanning, card matching and the binder model are all ≥2 layers.

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

Several skills (`/render-triage`, and the Routine cadences in the sweep
skills) assume infrastructure this repo does not have yet — a Render
deployment, a findings backlog, a cron fleet. They are here so the shape
survives; wire them up when the thing they watch exists.

## Provenance

Seeded from `simeonkorchev/spotter` @ MR #665. Verbatim: `.claude/rules/`,
`.claude/skills/`, `.claude/agents/`, `.claude/commands/`. Adapted: this file,
`.claude/settings.json`, `.claude/hooks/`, `.claude/memory/README.md`.
Deliberately excluded: spotter's memory units, its Makefile and `tools/`,
its CI, and all application code.
