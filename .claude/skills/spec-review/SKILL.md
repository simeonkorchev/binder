# /spec-review

Audit a spec's three artifacts for accidental complexity — YAGNI, over-engineering, premature abstraction, bloated tasks — before implementation. Complements `/spec-analyze` (which checks consistency, not scope).

## Usage

```
/spec-review <NNN-slug>
```

## What it does

1. Reads all three artifacts: `specs/NNN-slug/spec.md`, `specs/NNN-slug/plan.md`, `specs/NNN-slug/tasks.md`
2. Dispatches the `complexity-gatekeeper` agent to audit them (read-only)
3. Writes the review to `specs/NNN-slug/review.md`
4. Surfaces the verdict + a short summary to the user

## How to run it

Invoke the `complexity-gatekeeper` agent (Agent tool, `subagent_type: complexity-gatekeeper`) with a prompt that:
- names the three artifact paths for `specs/NNN-slug/`
- tells it to ground every finding in the spec's user stories, respect house conventions in `CLAUDE.md` / `.claude/rules/`, audit both over- and under-engineering, and count real call sites before flagging an abstraction
- instructs it to return the full review in its output format (verdict + the five sections)

Write the agent's returned review verbatim to `specs/NNN-slug/review.md`, prefixed with a `# NNN — <Title> — Complexity Review` heading. Then print the one-line **VERDICT** and the section headers with their item counts.

## Output (advisory)

Findings are **advisory** — like `/spec-analyze` warnings, the implementer decides. Severities:

| Severity | Meaning |
|----------|---------|
| `[Must-cut]` | Strong recommendation to cut/defer before implementing |
| `[Consider]` | Worth weighing; safety or necessity uncertain |
| `[Nit]` | Minor |

Verdicts: `APPROVE` (proceed), `TRIM` (cut, but not blocking), `BLOCK` (rework advised — still the human's call in this pipeline).

## Rules

- Report-only: never modify `spec.md`, `plan.md`, `tasks.md`, or any code. The only file written is `review.md`.
- Findings are advisory; they do not hard-block `/spec-implement`.
- Every cut the review proposes must cite the user story / scope item it is not required by — reject uncited cuts.
- Do not let the review add scope, redesign the feature, or expand the task list.
- After writing, prompt: "Review the cuts in `specs/NNN-slug/review.md`. Apply what you accept to `plan.md`/`tasks.md`, then run `/spec-implement NNN-<slug>`."
