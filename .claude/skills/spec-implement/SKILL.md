# /spec-implement

Execute tasks from a spec task list, marking each complete as it's done.

## Usage

```
/spec-implement <NNN-slug>
```

## What it does

1. Reads `specs/NNN-slug/tasks.md`
2. Finds the first unchecked task (`- [ ]`)
3. Implements it
4. Marks it `- [x]` in tasks.md
5. Repeats until all tasks are checked or user interrupts

## Behaviour rules

- **Apply the layer rules** from the `CLAUDE.md` "Rules by layer" table for every file you touch (`.claude/rules/002-go-conventions.md` for `internal/`, `003-frontend.md` for the apps, `006-testing.md` for tests). The task list says *what*; the rules say *how*.
- **Stop and ask** before any task marked `[REGEN]` — confirm the right moment to regenerate
- **Stop and ask** before any DB migration task — confirm schema intent
- **Never skip** quality gate tasks — run linters and tests, fix all failures before marking done
- **Walk `.claude/rules/007-clean-code-checklist.md`** over every file a task touched before marking it `[x]`; the Routines hold the code to the same list later
- **Mark completed immediately** — do not batch; mark `[x]` after each task succeeds
- If a task fails, stop and explain; do not proceed to the next task

## When done

Write memory (the 007 checklist's memory row): anything this implementation
learned that the next session would re-derive goes into the layer file
`.claude/memory/README.md` names as a `###` unit with its evidence, indexed
in `MEMORY.md`; a choice taken into `decisions.md`. Nothing the tree already
says. `make check-memory` must pass.

All tasks checked → report summary:
- Files changed
- Tests passing
- Any follow-up items (PRs, migrations to apply, feature flags)

Prompt user to open a PR.
