---
description: Behavior-preserving codebase hygiene pass that opens an MR
---

# Codebase hygiene pass (behavior-preserving)

You are running an automated quality routine on the Spotter monorepo. Goal:
improve code health WITHOUT changing observable behavior, then open an MR.

## Hard constraints
- BEHAVIOR MUST NOT CHANGE. No new features, no bug "fixes", no API/contract
  changes, no DB migrations, no dependency bumps. Pure internal improvement.
- If you find a real bug, DO NOT fix it. Record it as a finding file
  (`.ai/findings/open/`, see Deliverable step 3) and the MR description
  under "Found, not fixed", then move on.
- Respect all `.claude/rules/` for the layer you touch. Read them first:
  - Always: `.claude/rules/000-principles.md`
  - Go: `internal/CLAUDE.md` + `.claude/rules/002-go-conventions.md`
  - trainer-web: `apps/trainer-web/CLAUDE.md` + `.claude/rules/003-frontend.md`
  - client-mobile: `apps/client-mobile/CLAUDE.md` + `.claude/rules/003-frontend.md` + `005-mobile.md`
  - Always: `.claude/rules/004-security.md`, `.claude/rules/006-testing.md`
  it is not in your context), then `routines.md`, `decisions.md` and
  `generated/repo-layout.md`. A decision recorded there is binding — never
  re-propose it. `.claude/rules/` win over memory on any conflict.

## Scope — LARGE (sweep a whole feature/domain per run, MULTIPLE files)
1. Pick ONE target layer this run: `internal/`, `apps/trainer-web/`, or
   `apps/client-mobile/`. Default: the layer with most recent churn
   (`git log --since="14 days ago" --name-only`), and rotate so the same area
   is not picked two runs in a row (check recent `chore/hygiene-*` branches).
   **Exclude** every directory touched by any open pull request whose tip is
   at most 30 days old — sibling routines and human feature branches alike
   (`mcp__github__list_pull_requests`, plus every unmerged `chore/*`, `claude/*`,
   `fix/*`, `feature/*` branch from `git ls-remote --heads origin` — a sibling
   run pushes before it has a PR; `git diff --name-only origin/main...FETCH_HEAD` — deepen the shallow cloud clone first (`git fetch --deepen=200 origin main`), or the boundary commit reports every file as touched
   per branch). Refactoring
   under an open PR makes it conflict (memory `routines.md`: #541 vs #552).
2. Pick a WHOLE feature/domain, not a single file (e.g. the whole nutrition
   vertical api+service+store+model for one concern, or a full web feature
   folder with its pages+hooks+components+utils+tests). Survey the folder
   first, list every file with cleanup potential, then work through them.
3. SIZE TARGET: substantial PR — ~8-20 files, ~300-900 changed lines. A
   one-file / sub-100-line PR is TOO SMALL; keep going to a natural domain
   boundary or ~900 lines. Split only past ~1200 lines. Never invent churn —
   every change a genuine improvement, not reformatting noise.

## Allowed changes (behavior-preserving only)
Exactly the rows of `.claude/rules/007-clean-code-checklist.md` whose Kind is
**preserving** (dead code, duplication, naming, simplification, type
tightening, splits, shared helpers, characterization tests that pin existing
behavior) — the same list every interactive session walks before a PR, so
this routine adds no standard of its own. Rows whose Kind is *changing* are
logged as findings, never fixed here. Boy-scout lint/TS fixes in files you
touch are fine.

## Forbidden
- Changing function behavior, defaults, error handling semantics, ordering,
  rounding, timezones, query results.
- Editing `internal/.../food.go` store (handle empties upstream, not in store).
- Touching generated files (`packages/types`), migrations, env wiring.
- Reformatting files you otherwise didn't change (no diff noise).

## Verification gate (MUST pass before MR — paste output in description)
Run only the affected layer's gate:
- Backend:        `make lint && make test`. `make test` boots a local Postgres 16
  itself when there is no Docker and no `TEST_DATABASE_URL` (the cloud VM has
  neither), so the store packages are never skipped; "rootless Docker not
  found" means you ran `ginkgo` directly instead of `make test`.
- trainer-web:    `cd apps/trainer-web && npm run lint && npm run typecheck && npm run test:run && npm run knip`
- client-mobile:  `cd apps/client-mobile && npm run lint && npm run typecheck && npm run test:run && npm run knip`
If anything fails and you can't fix it within the behavior-preserving rules,
REVERT that change. Never weaken/skip a test to make the gate pass.

## Deliverable: the MR
1. New branch off `main`: `chore/hygiene-<layer>-<yyyymmdd>`.
2. Commit in small logical chunks, conventional-commit messages.
3. Persist findings as files (`.ai/findings/open/`, format documented in
   `.ai/findings/README.md`) in THIS PR:
   - For every "Found, not fixed" item, create ONE NEW FILE
     `open/<yyyy-mm-dd>-<short-slug>.md` with the documented header
     (title, Category, Severity, Path, Found) and a full description.
     **At most 3 per run** — the three that matter most; the rest stay in
     the PR body under "Found, not fixed".
   - DEDUPE first: grep `.ai/findings/open/` for the same file + issue;
     if an existing finding file already describes it, do not add another.
   - Only ADD finding files in this routine — never edit or delete existing
     ones (resolving them changes behavior and belongs to `/fix-findings`).
   - Commit them with the rest (e.g. `chore: log hygiene findings`).
4. Push, open PR against `main` with body:
   - **Scope**: module/feature area + files touched
   - **Changes**: bulleted, each tagged (dead-code | rename | simplify | types | test)
   - **Behavior impact**: "None — verified by <gate output / tests>"
   - **Gate output**: pasted command results
   - **Found, not fixed**: bugs/smells noticed but intentionally skipped
     (these must also be logged as files under `.ai/findings/open/` per step 3)
5. If there is nothing worth changing in the chosen scope, say so and open no MR
   (do not create an empty PR). If you still spotted findings worth logging,
   you may open a findings-only PR that only adds files under `.ai/findings/open/`.
6. Remember: only a fact that changes what the next run does and that the
   tree, `git log` or the PR list cannot show in one command (a scope
   rejected for a reason the code does not show, an environment quirk, a
   gate red on main that `flaky-tests.md` lacks). A stop condition —
   back-pressure, exclusion, no target — is never memory; it goes in your
   from *Where a memory goes*, the finished `### slug` unit, its Evidence
