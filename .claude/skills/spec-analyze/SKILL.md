# /spec-analyze

Cross-check a spec's three artifacts for consistency before implementation begins.

## Usage

```
/spec-analyze <NNN-slug>
```

## What it does

1. Reads all three artifacts: `specs/NNN-slug/spec.md`, `specs/NNN-slug/plan.md`, `specs/NNN-slug/tasks.md`
2. Runs the checks below
3. Outputs a structured report — issues block implementation; warnings are advisory

## Checks

### Spec → Plan
- Every user story in `spec.md` is addressed by at least one file in `plan.md`
- Every in-scope item is reflected in the layer breakdown (Backend / Frontend / DB)
- No out-of-scope items appear in `plan.md` (scope creep)
- Risks in `plan.md` cover anything flagged in the spec's constraints or open questions

### Plan → Tasks
- Every file listed in `plan.md` layer breakdown has at least one task in `tasks.md`
- DB migrations exist in tasks before dependent backend changes
- `[REGEN]` tasks appear immediately after the last interface change they depend on
- Quality gate tasks are present and last
- No task references a file not mentioned in `plan.md` (unknown scope)

### Spec → Tasks
- Every user story `US1`…`USN` from `spec.md` is tagged on at least one task
- No user story is orphaned (has no implementing task)

### Internal consistency
- Task count is reasonable for the plan complexity (flag if <3 or >40)
- No duplicate task descriptions
- Sequence ordering: DB → model → store → service → api → regen → frontend hooks → UI → tests → quality gate

## Output format

```
## Spec Analysis — NNN — <Title>

### Issues (must fix before /spec-implement)
- [ ] US2 has no implementing task
- [ ] Plan references `internal/nutrition/store/food.go` but no task covers it
- [ ] No DB migration task but plan.md lists a schema change

### Warnings (review before implementing)
- Task count is 47 — consider splitting into phases
- T012 and T019 have identical descriptions

### Checks passed
- All user stories covered
- REGEN tasks correctly positioned
- Quality gate tasks present
```

If no issues and no warnings: print `✅ Spec NNN is consistent — ready to implement.`

## Rules

- Issues = hard blockers; do not proceed to `/spec-implement` until resolved
- Warnings = advisory; implementer decides
- Do not modify any artifact — report only
- After reporting, prompt: "Fix issues in the relevant artifact, then re-run `/spec-analyze NNN-<slug>`." or "Run `/spec-implement NNN-<slug>` if all issues resolved."
