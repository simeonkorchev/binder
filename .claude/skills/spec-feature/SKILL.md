# /spec-feature

Create a feature spec for a new user story or capability.

## Usage

```
/spec-feature <slug> "<one-line description>"
```

## What it does

1. Determines the next spec number by listing `specs/` directories
2. Creates `specs/NNN-<slug>/spec.md` with the template below

## spec.md template

```markdown
# NNN — <Title>

## Status: Draft | Review | Approved | Done

## Problem
<!-- What user pain or gap does this solve? -->

## User stories
- US1: As a <role>, I want <action> so that <outcome>.

## Scope
### In
- 

### Out
- 

## API contract
<!-- List new/changed endpoints. Use OpenAPI-style signatures. -->

## Data model changes
<!-- New tables, columns, or migrations required. -->

## Open questions
- 
```

## Rules

- Read `.claude/memory/decisions.md` first: a declined item is not re-specified, a
  settled one is cited in the spec, not re-derived. Read the layer files
  (`go.md`, `frontend.md`, the app's) for constraints the rules do not say.
- Slug must be kebab-case, all lowercase
- Do not create spec.md for bug fixes, translations, dep bumps, or renames
- After creating spec, prompt the user to run `/spec-plan NNN-<slug>`
