# /spec-plan

Generate an implementation plan for an approved spec.

## Usage

```
/spec-plan <NNN-slug>
```

## What it does

1. Reads `specs/NNN-slug/spec.md`
2. Creates `specs/NNN-slug/plan.md` with the template below
3. Identifies all files that will need to change (backend before frontend)

## plan.md template

```markdown
# NNN — <Title> — Implementation Plan

## Approach
<!-- 2–4 sentences on the chosen design. State key trade-offs. -->

## Layer breakdown

### Backend
| File | Change |
|------|--------|
| `internal/...` | ... |

### Frontend
| File | Change |
|------|--------|
| `apps/...` | ... |

### Database
| Migration | Purpose |
|-----------|---------|
| `db/migrations/NNN_*.sql` | ... |

## Risks / open items
- 

## Sequence
Backend → regen fakes → regen types → frontend hooks → UI → translations → tests → quality gate
```

## Rules

- Read `.claude/memory/decisions.md` and the layer files for the layers the
  plan touches; a constraint recorded there (a library's behavior, a store
  contract) goes into the plan's Risks, not into a task that rediscovers it
- Backend changes always before dependent frontend changes
- Type regen (`npm run generate-types`) after last endpoint change
- Fake regen (`go generate`) after any Store interface change
- Prompt user to run `/spec-tasks NNN-<slug>` when done
