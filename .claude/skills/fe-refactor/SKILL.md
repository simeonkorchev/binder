---
name: fe-refactor
description: Every-90-minutes senior-engineer quality pass over one slice of one frontend app (trainer-web or client-mobile) - pins the slice with tests, then walks a closed checklist of design, type, complexity, UX and domain smells and fixes every one it can prove, behavior-preserving or test-first, inside a 500-2000-line budget, and opens one PR that shows the checklist result
---

# Frontend quality pass

## Intent

You are a senior frontend engineer doing a quality pass over one slice of one
app. When you are done, the slice is in the state a senior engineer would
ship: no design smells, no loose types, no accidental complexity, no
blocking browser dialogs, no domain logic that contradicts the house rules
or its own siblings. You are not here to move code between files. You are
here to leave the slice clean, with every change proven.

The bar is `.claude/rules/000-principles.md` and `.claude/rules/003-frontend.md`.
Clean code is what those files say it is. Where they are silent, apply
standard clean-code practice (small units with one responsibility, precise
types, no duplication, no cleverness, data shaped at the boundary).

Two kinds of change exist, and every edit is one of them. The test is
binary: **would a user, a caller, or an existing test observe any
difference?**

| Kind | Proof | Commit |
|------|-------|--------|
| **Preserving** - no observable difference | Characterization tests that existed before the edit still pass after it. | `refactor(<app>): …` |
| **Changing** - an observable difference, on purpose | A test that fails on the current code is committed first; the fix second; the PR names the rule, doc comment, or sibling that defines the correct behavior. | `test(<app>): RED …` then `fix(<app>): …` |

There is no cap on either kind. The line budget (step 4) bounds the PR. A
changing edit is allowed when its correct behavior is **already written
down**: a rule in `.claude/rules/`, the code's own name or comment, the i18n key
it renders, or the way its N-1 siblings behave. When nothing written down
says what is correct, it is a product decision: log it, do not guess.

## Hard boundaries

Never, in this routine:

- Change style values, layout, token semantics, or the wording of any
  string a user reads. Screenshots in both themes are required for visual
  changes and this environment cannot take them. Moving a literal into
  `t()` with identical text is fine; adding a Bulgarian translation is a
  changing edit with a rendered-text test.
- Change query keys, invalidation targets, `enabled`/`staleTime`, or
  navigation param types. These are cache and routing contracts; log them.
- Add `memo`, `useMemo`, `useCallback`. Both apps compile with the React
  Compiler; manual memoization is noise. Removing one is a preserving edit
  only when `scripts/compiler-probe.mjs` shows the function is still
  compiled afterwards (see the checklist row).
- Edit `packages/types`, `packages/api/src/generated`, `db/`, env wiring,
  `metro.config.cjs`, any `package.json` or the lockfile. Locale files may
  only gain keys.
- Weaken, skip, or delete a test, or edit a test to make it pass anywhere
  but a RED commit. Never trim pin tests for budget.
- Touch files outside the chosen slice, or reformat files you did not
  otherwise change. `.ai/findings/open/` and `.claude/memory/inbox/` are the
  only exceptions.
- Bootstrap an e2e harness, fetch or clone anything, or ask a question.
- Run a command that needs a human's approval. The run is unattended; a
  blocked prompt stalls it until someone notices. The commands this skill
  names are pre-approved in `.claude/settings.json`. `gh` is not installed:
  never search the filesystem for it or any other binary. Count PRs with
  `git ls-remote`, open the PR with the GitHub MCP tools.
- Delegate the run to a subagent or an isolated worktree. Do the work in
  this session, in this checkout, on the branch you will push.

## 0. Load

1. `CLAUDE.md`, `.claude/rules/000-principles.md`, `.claude/rules/003-frontend.md`,
   `.claude/rules/004-security.md`, `.claude/rules/006-testing.md`
2. `apps/trainer-web/CLAUDE.md`, or `apps/client-mobile/CLAUDE.md` +
   `.claude/rules/005-mobile.md`
3. `packages/eslint-config/README.md` "Staged rules": the measured debt
   classes. Clearing a class in your slice is the highest-value outcome.
4. client-mobile slices importing `react-native-reanimated`,
   `react-native-gesture-handler` or `react-native-svg`:
   `.claude/skills/react-native-best-practices/` for that topic only — forked
   and edited to the house rules, so it agrees with `.claude/rules/`. A library migration
   (PanResponder to Gesture Handler, an animation curve) is a product
   decision: log it.

5. Memory: `.claude/memory/MEMORY.md` (auto-loaded; `cat` it if it is not
   in your context), then `routines.md`, `decisions.md`, `flaky-tests.md`
   and `generated/repo-layout.md` from the same directory. A decision
   recorded there is binding — never re-propose it; a target it lists as
   rejected is not re-surveyed. `.claude/rules/` win over memory on any
   conflict. Tooling is plain grep/Read/Edit; no MCP code tools are needed.

## 1. Stop conditions and slice choice

Any stop means finish without a PR and say why.

- `git fetch origin main` and work from `origin/main`.
- **Back-pressure**: 3 or more **open pull requests** whose head starts with
  `chore/fe-refactor-` (`mcp__github__list_pull_requests`, state `open`; only
  without the GitHub tools fall back to `git ls-remote --heads origin 'chore/fe-refactor-*'`).
  Stop, and say so in the final message — a stop is not memory. A branch
  with no open PR (a closed PR's leftover, an abandoned run) does not count
  and is not yours to delete; the reviewer's queue is the throttle.
- **Exclusion set**: every directory touched by **any** open pull request whose
  tip is at most 30 days old — sibling routines *and* human feature
  branches. Names: every open PR (`mcp__github__list_pull_requests`, state
  `open`) **plus** every unmerged `claude/*`, `feature/*`, `chore/*`, `fix/*`
  branch `git ls-remote --heads origin` reports — a sibling run pushes its
  branch before it has a PR, and two runs of one Routine can overlap
  (2026-09-09: #570 and #571 both took trainer-web nutrition six minutes
  apart and collided on the barrel and the locale files). Files:
  `git fetch origin <branch> && git diff --name-only origin/main...FETCH_HEAD`;
  age: `git log -1 --format=%cd FETCH_HEAD`. Plus every **file** a merged
  run touched in the last 7 days
  (`git log --since='7 days ago' --grep='refactor(<app>)' --name-only`;
  `--grep='fe-refactor'` matches every squash body and repo-wide chores and
  leaves no target). Merged-run rotation is file-level; the open-PR half
  stays directory-level, it is the one that causes `dirty`. A feature
  The cloud clone is shallow: `git rev-parse --is-shallow-repository` →
  `git fetch --deepen=200 origin main` before any `git log` here, or the
  boundary commit reports every file as touched (memory `routines.md`).
  branch is excluded for the same reason a sibling is: refactoring under an
  open PR makes it conflict (memory `routines.md`: #541 vs #552).
- **Alternate apps**: target the app the most recent `chore/fe-refactor-*`
  PR did not. No prior PR: trainer-web.
- **Baseline gate on untouched main** for that app (`npm ci` at the root if
  needed): `cd apps/<app> && npm run lint && npm run typecheck && npm run test:run && npm run knip`.
  Red on main: stop, report the failing check.
- **Slice**: one feature folder, hook cluster, or component group, at most
  ~12 production files. Rank candidates by hits on the checklist in step 3;
  take the highest outside the exclusion set. Estimate the production diff;
  over ~1600 lines, take a smaller part of it.

## 2. Pin

Before any production edit, for every production file you will touch:
enumerate its observable behavior (exported symbols, every branch of every
exported function or hook, rendered roles and text, callbacks fired) and
make sure a test exercises each. Add the missing characterization tests per
`.claude/rules/006-testing.md`: hooks via `renderHook` with the module-level
stable mock, components via role-first queries with `userEvent` and
`findBy*`, mappers via completeness tests. They pass immediately; that is
the point.

A behavior that cannot be pinned in jsdom/Jest (time, randomness,
native-only APIs, Reanimated worklets, gesture callbacks) is not edited this
run; log it.

Run the tests on the untouched code, keep the output and SHA, and commit
them alone: `test(<app>): pin <slice> behavior before refactor`.

Push the branch as soon as the pin commit exists (`git push -u origin
<branch>`): a sibling run that starts later builds its exclusion set from
the remote branches, and only sees your slice once it is there.

## 3. The checklist

The checklist is **`.claude/rules/007-clean-code-checklist.md`** — the *Both
layers*, *Frontend* and *Tests* tables. It is the same list every interactive
session walks before opening a PR, so this routine never introduces a
standard of its own: walk every row against every file in the slice. Each
hit is **fixed** or **logged**; there is no third outcome. The row's Kind is
the default classification; the binary test in the Intent section decides
when the default does not fit. The rule each row cites holds the *why* and
the correct shape; read it before fixing.

Technique specific to this routine:

- **Memoization removal must be proven**: after removing `useCallback` /
  `useMemo` / `memo`, run `node ../../.claude/skills/fe-refactor/scripts/compiler-probe.mjs <file>`
  from the app directory. The compiler's default mode skips a `use*` function
  that no longer calls a `use*`-named identifier (an alias such as
  `useGetRecipes as sdkUseGetRecipes` does not count) — add `'use memo'` as
  its first statement and re-run. Leave `React.memo` on a component whose
  props come from a hook the probe reports as not compiled. Exit 1 means the
  removal is reverted, not shipped. A `react-hooks` eslint suppression shows
  as `BAILOUT`: fix the underlying violation, delete the suppression.
- **Splits**: a hook or component owning several concerns becomes focused
  hooks and components, one responsibility each, each with its own tests,
  composed by a thin orchestrator.
- **Test migrations** (`fireEvent` → `userEvent`, role-first selectors) are
  test-only commits, after the production steps.

## 4. Fix

Write the plan first: every checklist hit in the slice, its kind, and
whether it is fixed or logged. Then work it in this order, one logical step
per commit:

1. Preserving edits, largest structural change first (splits, then moves,
   then types, then simplifications).
2. Changing edits, each as its own `test(<app>): RED …` commit (one failing
   assertion per site the rule touches) followed by its `fix(<app>): …`
   commit. The RED commit is the only place an existing assertion may
   change.
3. Test-only migrations.

After each step: `npm run typecheck` plus the slice's tests. A red step that
is not a deliberate RED commit is **reverted, not debugged forward**.

After each commit: `.claude/skills/fe-refactor/scripts/diff-budget.sh`. It
fails below **500 changed lines total** (tests included, generated code and
the lockfile excluded) and above **2000 production lines**; it warns past
1000 production lines, 15 production files or 3000 total lines. Tests never
force a cut. Over the cap: revert the last production step and stop. Under
the floor: add an adjacent slice from the same app, pinned and worked
exactly like the first; never pad.

A hit may be **logged instead of fixed** for exactly these reasons, and the
finding names which one:

1. **Product decision** - nothing written down says what is correct, or the
   fix touches copy, layout, a contract, an auth rule, or anything in
   `004-security.md`.
2. **Outside the slice** - the fix needs files you may not touch.
3. **Cannot pin** - the behavior is untestable in this environment.
4. **Budget** - the cap is reached; name the fix so the next run takes it.

"Too risky", "not sure", or "a reviewer might object" are not reasons. Risk
is what the pin tests, the gate, and human review are for.

## 5. Prove

Everything here goes into the PR body.

1. Pin tests: the run on the untouched SHA and the run on the final SHA,
   both green, both pasted.
2. Per changing edit: the source of truth, the RED SHA with its failing
   assertion, the GREEN SHA with its passing run, before/after in one line.
3. The full `diff-budget.sh origin/main` output: budget within limits, one
   app, nothing outside `apps/<app>/`, `packages/intl/locales/` (additive)
   and `.ai/findings/open/`, every removed or changed exported declaration
   justified, forbidden paths none, skip markers none, each review-hotspot
   line explained.
4. Coverage of touched files not lower: trainer-web
   `npx vitest run --coverage <slice dir>`; client-mobile
   `npx jest --coverage --collectCoverageFrom='<slice glob>'`.
5. Full app gate green (the step 1 commands), pasted. Anything that cannot
   pass within the boundaries: revert that step, never the test.
6. Self-review of the whole diff against the Hard boundaries: "none found"
   or what you reverted.

## 6. Findings

Every logged hit becomes one file `.ai/findings/open/<yyyy-mm-dd>-<slug>.md`
in the format of `.ai/findings/README.md`, with the reason from step 4 and
every site listed, so `/fix-findings` can drain it. Dedupe against existing
files first; only add. **At most 3 findings per run** — the three that matter most, each with a
`Severity` (`.ai/findings/README.md`); everything else stays in the PR body's
"Found, not fixed". Ten producer runs a day against one consumer made the
ledger write-only (215 added vs 18 resolved in 60 days).
A slice with `alert()` calls, untyped forms, or a 300-line hook and a PR
saying "nothing found" is a failed survey.

## 7. PR

- Branch off `origin/main`: `chore/fe-refactor-<app>-<yyyymmdd-HHMM>` (UTC).
- Commits: pin → preserving steps → RED/fix pairs → test migrations →
  `chore: log fe-refactor findings`.
- Title `refactor(<app>): <what>`, with ` (+N intended changes)` when fix
  commits exist. Open the PR with the GitHub MCP tools
  (`mcp__github__create_pull_request`); if they are unavailable, push the
  branch and put the full PR body in your final message so a human can open
  it. Never enable auto-merge.

```markdown
## TL;DR
- <what the slice looks like now vs before, one line>
- <N intended behavior changes, or "no intended behavior changes">
- <size: N files, +A/-D lines, of which tests +T>

## Review scope
Produced by the `/fe-refactor` routine. Preserving edits are pinned by tests that predate them; changing edits each cite the rule that defines the correct behavior and carry a RED commit. Style, layout, query keys, memoization and contracts are outside this routine by design - please file those as findings, not change requests.

## Scope
- App / slice: `apps/<app>/src/<...>`
- Files: <list>
- Budget: <production> / 2000 production lines, <total> total (floor 500)

## Checklist result
| 007 row | Sites | Outcome |
|---|---|---|
| <standard, as worded in `.claude/rules/007-clean-code-checklist.md`> | <n> | fixed in `<sha>` / logged: <reason> |
(every checklist hit in the slice appears here; "no hits" rows may be omitted)

## Intended behavior changes
**1. <title>** - source of truth: <rule § / comment / siblings>; before → after: <one line>; RED `<sha>` (<assertion>), GREEN `<sha>`; blast radius: <callers>
(or "None")

## Evidence
1. Pin tests before (`<sha>`): <summary line> - after (`<sha>`): <summary line>
2. diff-budget.sh: <pasted>
3. Coverage: <rows>
4. Gate: <pasted>
5. Boundaries self-review: none found / <reverted>

## Found, not fixed
- <finding file> - <reason>

## Not run
- Screenshots in both themes: not possible here; no style or layout lines changed
```

Below the floor after three slices, or everything reverted: open no PR and
report what was found. A findings-only PR is fine.

## 8. Remember

Before the final message: if this run learned something the next run would
otherwise re-derive, write it as one unit in waiting,
`.claude/memory/inbox/<yyyy-mm-dd>-<slug>.md` in the inbox format of
`.claude/memory/README.md`: front-matter naming the topic `file:` and
`section:` from *Where a memory goes*, then the finished `### slug` unit with
its Evidence line, so the dream only moves it. One fact per entry, stated as
"X happens when Y", never a narrative of the run. Committed on the same
branch as `chore(memory): remember <slug>`.

Worth remembering — only a fact that **changes what the next run does** and
that it could not get from the tree, `git log` or the PR list in one
command: a target rejected for a reason the code does not show, a package
whose store suite cannot run here, a tool or environment quirk, a gate red
on `main` that `flaky-tests.md` does not list yet, a pin that was
unexpectedly hard. **A stop condition is never memory**: back-pressure,
an exclusion set, "no target this run", a stalled CI queue — the next run
recomputes each of those in seconds, so they go in your final message
and nowhere else (2026-09-10: six memory-only PRs in twelve hours said
"back-pressure stop"; every one was noise). Before writing, grep
`.claude/memory/` for the fact; already there means write nothing.

A run that opens no code PR may open a memory-only PR from
`chore/memory-<yyyymmdd-HHMM>` **only** for an entry that passes the test
above. Never edit `MEMORY.md` or a topic file in a routine run —
`/memory-dream` folds the inbox in nightly. Nothing learned: write
nothing, open nothing.

Work end to end without asking questions. The PR is the only output that
matters.
