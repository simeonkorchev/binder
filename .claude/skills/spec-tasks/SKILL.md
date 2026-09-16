# /spec-tasks

Break an implementation plan into atomic, trackable tasks.

## Usage

```
/spec-tasks <NNN-slug>
```

## What it does

1. Reads `specs/NNN-slug/plan.md`
2. Creates `specs/NNN-slug/tasks.md` with atomic task list

## tasks.md format

```markdown
# NNN — <Title> — Tasks

- [ ] T001 Description — path/to/file.go
- [ ] T002 [P] Description — path/to/other.go
- [ ] T003 [US1] Description — path/to/file.tsx
- [ ] T004 Quality gate — make lint && make test
- [ ] T005 Quality gate — cd apps/trainer-web && npm run lint && npm run typecheck && npm run test:run && npm run knip
- [ ] T006 Quality gate — cd apps/client-mobile && npm run lint && npm run typecheck && npm run test:run && npm run knip
- [ ] T007 Clean-code checklist — .claude/rules/007-clean-code-checklist.md walked over every touched file
```

## Tags

| Tag | Meaning |
|-----|---------|
| `[P]` | Parallelisable with adjacent tasks |
| `[US1]` | Implements user story 1 from spec |
| `[REGEN]` | Must run code generation step |

## Ordering rules

1. DB migrations first
2. Backend (model → store → service → api) before frontend
3. `[REGEN]` tasks immediately after the last interface change they depend on
4. Quality gate tasks always last — one per layer touched, the commands verbatim from `CLAUDE.md` (never a shortened gate)

## Rules

- One task = one file change (or one regen/migration step)
- Never merge "add X and Y to file Z" into one task
- Prompt user to run `/spec-implement NNN-<slug>` when done
