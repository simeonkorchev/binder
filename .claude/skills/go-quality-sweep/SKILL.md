---
name: go-quality-sweep
description: "Every-90-minutes senior-engineer quality pass over one package or concern of the Go backend (internal/, pkg/, cmd/) - pins the target with specs, then walks a closed checklist of design, type, error-handling, complexity and domain smells and fixes every one it can prove, behavior-preserving or spec-first, inside a 500-2000-line budget, and opens one PR that shows the checklist result. Use for the go-sweep Routine or when asked to run a Go-only hygiene pass."
---

# Go quality pass

## Intent

You are a senior Go engineer doing a quality pass over one package or one
concern of one domain. When you are done, the target is in the state a
senior engineer would ship: no god types, no loose types, no error paths
that hide their cause or pick their own status, no accidental complexity,
no domain logic that contradicts the house rules or its own siblings. You
are not here to move methods between files. You are here to leave the
target clean, with every change proven.

The bar is `.claude/rules/000-principles.md` and `.claude/rules/002-go-conventions.md`,
with the `golang-*` skills for technique (forked and edited to those rules; `.claude/skills/VENDOR.md`).
The rules win over the skills wherever they overlap. Where both are silent,
apply standard clean-code practice (small units with one responsibility,
precise types, no duplication, no cleverness, data shaped at the boundary).

Two kinds of change exist, and every edit is one of them. The test is
binary: **would a caller, a client, a log reader, or an existing spec
observe any difference?**

| Kind | Proof | Commit |
|------|-------|--------|
| **Preserving** - no observable difference | Characterization specs that existed before the edit still pass after it; `packages/types/openapi.json` byte-identical when `api/` is touched. | `refactor(<pkg>): …` |
| **Changing** - an observable difference, on purpose | A spec that fails on the current code is committed first; the fix second; the PR names the rule, doc comment, or sibling that defines the correct behavior. | `test(<pkg>): RED …` then `fix(<pkg>): …` |

There is no cap on either kind. The line budget (step 4) bounds the PR. A
changing edit is allowed when its correct behavior is **already written
down**: a section of `002-go-conventions.md`, the domain's
`serviceErrorMappings` table, the code's own name or doc comment, or the
way its N-1 siblings behave. When nothing written down says what is
correct, it is a product decision: log it, do not guess.

## Hard boundaries

Never, in this routine:

- Change a wire field, response body, or `packages/types/openapi.json`.
  A status code may change only to the one the domain's mapping table
  already prescribes for that sentinel.
- Change query text or query results. `/sql-optimize` owns store SQL.
- Touch `db/migrations/**`, `*.sql`, `packages/types/**`,
  `internal/nutrition/store/food.go`, `cmd/spotter/main.go` wiring,
  env/config, dependencies, or LLM prompt bytes.
- Edit generated fakes by hand; regenerate with counterfeiter.
- Weaken, skip, or delete a spec (`Skip(`, `XIt`), or edit a spec to make it
  pass anywhere but a RED commit. Never trim pin specs for budget.
- Touch files outside the chosen target, or reformat files you did not
  only exceptions.
- Fetch or clone anything, or ask a question.
- Run a command that needs a human's approval. The run is unattended; a
  blocked prompt stalls it until someone notices. The commands this skill
  names are pre-approved in `.claude/settings.json`. `gh` is not installed:
  never search the filesystem for it or any other binary. Count PRs with
  `git ls-remote`, open the PR with the GitHub MCP tools.
- Delegate the run to a subagent or an isolated worktree. Do the work in
  this session, in this checkout, on the branch you will push.

## 0. Load

1. `CLAUDE.md`, `internal/CLAUDE.md`
2. `.claude/rules/000-principles.md`, `.claude/rules/002-go-conventions.md`,
   `.claude/rules/004-security.md`, `.claude/rules/006-testing.md` (Go section)
3. Technique: `golang-refactoring/SKILL.md` +
   `references/safety-net.md`, `golang-code-style`, `golang-modernize`,
   `golang-safety`; `golang-naming`, `golang-error-handling` and the
   refactoring `catalog.md` when the target calls for them.
   in your context), then `routines.md`, `decisions.md`, `flaky-tests.md`
   and `generated/repo-layout.md` from the same directory. A decision
   recorded there is binding — never re-propose it; a target it lists as
   rejected is not re-surveyed. `.claude/rules/` win over memory on any
   conflict. Tooling is plain grep/Read/Edit; no MCP code tools are needed.

## 1. Stop conditions and target choice

Any stop means finish without a PR and say why.

- `git fetch origin main` and work from `origin/main`. `make lint` or
  `make test` red on untouched main: stop, report the failing check.
- **Back-pressure**: 3 or more **open pull requests** whose head starts with
  `chore/go-sweep-` (`mcp__github__list_pull_requests`, state `open`; only
  without the GitHub tools fall back to `git ls-remote --heads origin 'chore/go-sweep-*'`).
  Stop, and say so in the final message — a stop is not memory. A branch
  with no open PR (a closed PR's leftover, an abandoned run) does not count
  and is not yours to delete; the reviewer's queue is the throttle.
- **Exclusion set**: every file touched by **any** open pull request whose
  tip is at most 30 days old — sibling routines *and* human feature
  branches. Names: every open PR (`mcp__github__list_pull_requests`, state
  `open`) **plus** every unmerged `claude/*`, `feature/*`, `chore/*`, `fix/*`
  branch `git ls-remote --heads origin` reports — a sibling run pushes its
  branch before it has a PR, and two runs of one Routine can overlap
  (2026-09-09: #570 and #571 both took trainer-web nutrition six minutes
  apart and collided on the barrel and the locale files). Files:
  `git fetch origin <branch> && git diff --name-only origin/main...FETCH_HEAD`;
  age: `git log -1 --format=%cd FETCH_HEAD`. Plus every package a merged
  run touched in the last 7 days: squashed merges carry the sweep's own
  subject, `chore(<pkg>): …`, not the routine's name, so list them with
  `git log --since='7 days ago' --format='%h|%cd|%s' --date=short origin/main | grep -E 'chore\('`
  (`--grep='go-sweep'` found 4 of 16 merged sweeps on 2026-09-10).
  The cloud clone is shallow: `git rev-parse --is-shallow-repository` →
  `git fetch --deepen=200 origin main` before any `git log` here, or the
  boundary commit reports every file as touched (memory `routines.md`).
  A feature
  branch is excluded for the same reason a sibling is: refactoring under an
  open PR makes it conflict (memory `routines.md`: #541 vs #552).
- **Store specs**: `make test` boots a local Postgres 16 by itself when there
  is no Docker and no `TEST_DATABASE_URL` (`tools/test-db-local.sh`; scope a
  run with `make test PKGS=./internal/<domain>/...`). If even that fails, `internal/*/store` is
  out of scope this run and the PR says so verbatim.
- **Target**: one package, or one concern spanning `api/` + `service/` of
  one domain. Rank candidates by hits on the checklist in step 3, weighting
  `nolint` suppressions and convention violations highest; rotate away from
  domains swept in the last 7 days. Estimate the production diff; over ~1600
  lines, take a smaller part of it.

## 2. Pin

Before any production edit, for every function or method you will touch:
enumerate its branches (each `if`/`else`, `switch` case, error path, guard,
nil/empty case) and make sure a spec exercises each. Add the missing
characterization specs: Ginkgo/Gomega, external test package,
`JustBeforeEach` invokes once, counterfeiter fakes, `MatchError` on
sentinels, assert the record not its length. Pure helpers may use plain
`testing` in an `_internal_test.go` with the `white-box:` reason. They pass
immediately; that is the point.

If `api/` will change: `make gen-spec` now and keep the result to diff
against later.

A behavior that cannot be pinned (hidden I/O, time, randomness) is not
edited this run; log it.

Run the specs on the untouched code, keep the output and SHA, and commit
them alone: `test(<pkg>): pin <behavior> before refactor`.

Push the branch as soon as the pin commit exists (`git push -u origin
<branch>`): a sibling run that starts later builds its exclusion set from
the remote branches, and only sees your slice once it is there.

## 3. The checklist

The checklist is **`.claude/rules/007-clean-code-checklist.md`** — the *Both
layers*, *Go* and *Tests* tables. It is the same list every interactive
session walks before opening a PR, so this routine never introduces a
standard of its own: walk every row against every file in the target. Each
hit is **fixed** or **logged**; there is no third outcome. The row's Kind is
the default classification; the binary test in the Intent section decides
when the default does not fit. The rule each row cites holds the *why* and
the correct shape; read it before fixing.

Technique specific to this routine:

- **Splits** go behind the consumer-side interfaces callers already declare
  (`002` §3), each new type with its own spec file, and the `nolint:funlen/
  cyclop/gocognit/nestif/dupl` suppression goes away with the split.
- **Modernize by reading**: judge pre-Go-1.21 idioms against the
  `golang-modernize` skill; no analyzer is downloaded, and nothing gated on
  Go 1.26+ applies (`go.mod` is 1.25.6).
- **Dead code** needs the zero-reference proof pasted in the PR.

## 4. Fix

Write the plan first: every checklist hit in the target, its kind, and
whether it is fixed or logged. Then work it in this order, one logical step
per commit:

1. Preserving edits, largest structural change first (splits, then
   helpers, then types, then simplifications and modernize).
2. Changing edits, each as its own `test(<pkg>): RED …` commit (one failing
   assertion per site the rule touches) followed by its `fix(<pkg>): …`
   commit. The RED commit is the only place an existing assertion may
   change.

After each step: `go build ./... && go vet ./... && ginkgo run -race ./internal/<pkg>/...`.
A red step that is not a deliberate RED commit is **reverted, not debugged
forward**.

After each commit: `.claude/skills/go-quality-sweep/scripts/diff-budget.sh`.
It fails below **500 changed lines total** (specs included; fakes,
`packages/types` and vendor excluded) and above **2000 production lines**;
it warns past 3000 total lines. Specs never force a cut. Over the cap:
revert the last production step and stop. Under the floor: add an adjacent
target from the same domain, pinned and worked exactly like the first;
never pad.

A hit may be **logged instead of fixed** for exactly these reasons, and the
finding names which one:

1. **Product decision** - nothing written down says what is correct, or the
   fix touches a wire contract, a status no table prescribes, an auth or
   scoping rule, LLM prompt bytes, or anything in `004-security.md`.
2. **Outside the target** - the fix needs files you may not touch.
3. **Cannot pin** - the behavior is untestable in this environment (store
   specs without a database included).
4. **Budget** - the cap is reached; name the fix so the next run takes it.

"Too risky", "not sure", or "a reviewer might object" are not reasons. Risk
is what the pin specs, the gate, and human review are for.

## 5. Prove

Everything here goes into the PR body.

1. Pin specs: the run on the untouched SHA and the run on the final SHA,
   both green, both pasted.
2. Per changing edit: the source of truth, the RED SHA with its failing
   assertion, the GREEN SHA with its passing run, before/after in one line.
3. The full `diff-budget.sh origin/main` output: budget within limits,
   every removed or changed exported declaration justified as dead, a
   rename, or a type narrowing with all callers updated, forbidden paths
   none.
4. `make check-spec` when `api/` changed: openapi.json unchanged.
5. `make lint && make test` green, pasted. Anything that cannot pass within
   the boundaries: revert that step, never the spec.
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
A target with suppressions, a handler picking status codes, or a 500-line
service and a PR saying "nothing found" is a failed survey.

## 7. PR

- Branch off `origin/main`: `chore/go-sweep-<yyyymmdd>-<hhmm>-<pkg>`.
- Commits: pin → preserving steps → RED/fix pairs →
  `chore: log sweep findings`.
- Title `chore(<pkg>): <what>`, with ` (+N intended changes)` when fix
  commits exist. Open the PR with the GitHub MCP tools
  (`mcp__github__create_pull_request`); if they are unavailable, push the
  branch and put the full PR body in your final message so a human can open
  it. Never enable auto-merge.

```markdown
## TL;DR
<2-3 sentences: what the target looks like now vs before, and "N intended behavior changes" or "no intended behavior changes">

## Review scope
Produced by the `/go-quality-sweep` routine. Preserving edits are pinned by specs that predate them; changing edits each cite the rule that defines the correct behavior and carry a RED commit. Query text, wire contracts, migrations and dependencies are outside this routine by design - please file those as findings, not change requests.

## Scope
- Package/concern: `internal/<...>`
- Files: <list>
- Budget: <production> / 2000 production lines, <total> total (floor 500)

## Checklist result
| 007 row | Sites | Outcome |
|---|---|---|
| <standard, as worded in `.claude/rules/007-clean-code-checklist.md`> | <n> | fixed in `<sha>` / logged: <reason> |

## Intended behavior changes
**1. <title>** - source of truth: <rule § / table row / comment / siblings>; before → after: <one line>; RED `<sha>` (<assertion>), GREEN `<sha>`; blast radius: <callers / routes>
(or "None")

## Evidence
1. Pin specs before (`<sha>`): <summary> - after (`<sha>`): <summary>
2. diff-budget.sh: <pasted>
3. Contract: `make check-spec` unchanged / no api/ files touched
4. Gate: <pasted>
5. Boundaries self-review: none found / <reverted>

## Found, not fixed
- <finding file> - <reason>

## Not run
- <e.g. store suites: no Docker / TEST_DATABASE_URL>
```

Below the floor after three targets, or everything reverted: open no PR and
report what was found. A findings-only PR is fine.

## 8. Remember

Before the final message: if this run learned something the next run would
otherwise re-derive, write it as one unit in waiting,
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


Work end to end without asking questions. The PR is the only output that
matters.
