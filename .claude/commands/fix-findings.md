---
description: TDD-driven routine that fixes a few backlog findings and opens an MR
---

# Fix backlog findings (test-first, behavior-changing)

You are running an automated remediation routine on the Spotter monorepo.
Goal: pick a few items from the hygiene findings backlog, fix them **test
first**, and open an MR. This is the consumer of what `/cleanup` produces.

Unlike `/cleanup`, these fixes **MAY change behavior** — so every change is
driven by tests (a failing test for behavior changes, pinning tests for
behavior-preserving cleanups — see Test workflow), every touched branch is
covered, and the MR goes through normal human review (never auto-merged).

## Source of work
- Findings live as ONE FILE EACH under `.ai/findings/open/` (format documented
  in `.ai/findings/README.md`: a title line plus Category / Path / Found
  header and a full description). Every file there is a candidate.
- Optional argument `$ARGUMENTS`: if given, narrow selection to that layer
  (`internal` | `trainer-web` | `client-mobile`), category (`bug`, `dup`, …),
  or a path substring. With no argument, you choose (see Selection).
- A `domain` finding (from `/domain-audit`) ends in a **Decision block**. It
  is a candidate only when exactly one box is ticked; an unticked one is
  listed in the MR under **Awaiting decision** and otherwise left alone.

## Step 0 — avoid double-fixing (start of every run)
Before selecting anything, list the repo's OPEN pull requests. If an open PR
already deletes a finding file (someone is fixing it), do NOT pick that
finding this run. There is nothing else to reconcile: a merged fix PR deletes
its finding files itself, and a fix PR closed unmerged leaves them in place.
Also skip any finding whose `Path` sits inside a file or folder that an open
pull request touches (any PR, ≤ 30 days old — `mcp__github__list_pull_requests`,
plus every unmerged `chore/*`, `claude/*`, `fix/*`, `feature/*` branch from
`git ls-remote --heads origin`, since a sibling run pushes before it has a PR;
`git diff --name-only origin/main...FETCH_HEAD` — deepen the shallow cloud clone first (`git fetch --deepen=200 origin main`), or the boundary commit reports every file as touched per branch): fixing under an
open PR makes that PR conflict.

## Selection — pick 1–3 items per run (keep the MR reviewable)

Every open finding is fixed in one of two modes; **both are eligible**:

- **Mode A — behavior-changing** (`bug`, `security`, `i18n`, missing/dropped
  fields): there is a single correct behavior the current code violates. Pin
  it with a classic failing test (RED → GREEN).
- **Mode B — behavior-preserving** (`dup`, `dead-code`, `types`, `simplify`,
  `perf`): observable behavior must NOT change, so a failing-first test is
  impossible **by definition** — never reject an item for that reason. Use
  the PIN → CHANGE → PROVE workflow below instead.

1. A `domain` finding with **A or B ticked** is picked before anything else,
   whatever its severity — a human already spent the decision. Fix it in
   Mode A starting from its *Drafted RED test* (verify the draft fails for
   the stated reason before trusting it; rewrite it if it does not). One
   with **Leave it** ticked is resolved without code: delete the file and
   write `.claude/memory/inbox/<yyyy-mm-dd>-decision-<slug>.md`
   (`type: project`) carrying the decision, the finding's title and who
   decided, for `/memory-dream` to fold into `decisions.md`.
2. Then read every other file under `.ai/findings/open/`, **highest `Severity` first**
   (`high` → `medium` → `low`; a file without the line is `medium`). Within a
   severity, prefer items that:
   - live in **ONE layer** so a single quality gate covers the run;
   - touch few files and no exported/external contract;
   - together keep the diff small (~≤400 lines). Stop adding items once it grows.
3. **DEFER** (leave its file in place, explain in the MR) ONLY items that match
   one of these explicit rules — nothing else is defer-worthy:
   - changes bytes sent to an LLM prompt (e.g. the doubled recipe-label item);
   - changes an HTTP status code, wire format, or other externally observable
     API contract (e.g. the 400-vs-401 item) — internal helper signatures and
     hook/prop shapes with zero external consumers do NOT count;
   - adds or changes an **authorization rule** (`security` items need product
     sign-off; instead of fixing, surface them prominently in the MR body
     under **Security — needs human decision**);
   - purely visual style/colour consolidation with no testable behavior.
   "Feels risky" is not a defer reason — risk is what the pinning tests, the
   gate, and human review are for. i18n string extraction is explicitly
   eligible (Mode A): add the keys to every locale file and test the rendered
   output.
4. **Zero-pick is the exception, not the default.** If your first pass rejects
   everything, re-rank the non-deferred items by risk (fewest files → no
   exported surface → most mechanical) and take at least the top one in
   Mode B. Report "nothing safely fixable this run" ONLY when every open
   finding matches a defer rule above — and then list each item with the exact
   rule that excluded it, so a human can prune the rules.

## Test workflow — per finding

**Mode A (behavior-changing): RED → GREEN → REFACTOR**
1. **RED** — before touching production code, write the test(s) expressing the
   corrected behavior. Run them; confirm they FAIL for the expected reason.
   Paste the red run in the MR.
2. **GREEN** — make the minimal change to pass. No drive-by edits.
3. **REFACTOR** — clean up with tests staying green.

**Mode B (behavior-preserving): PIN → CHANGE → PROVE**
1. **PIN** — before touching production code, ensure the current observable
   behavior of every branch you are about to touch is exercised by a test.
   Add missing characterization tests — they PASS immediately; that is the
   point. Paste this pinning run in the MR as the Mode-B analogue of RED.
2. **CHANGE** — the minimal dedupe / deletion / tightening.
   - Dead code: prove zero references first (`find_referencing_symbols` /
     grep) and paste the proof in the MR.
   - Loose types / dead params: the compiler is the red signal — removing the
     cast or param must be validated by `npm run typecheck` / `go build`.
3. **PROVE** — pinning tests still green, gate green, coverage equal or up.

**Branch coverage (both modes, required)** — enumerate every branch of the
code you touched (each `if`/`else`, `switch` case, error path, guard,
nil/empty case) and ensure a test exercises each. Verify with coverage tooling:
- Go: `go test -cover ./<pkg>/...` (use `-coverprofile` to inspect lines);
- trainer-web / client-mobile: `vitest run --coverage` / jest `--coverage`.
A changed line with an uncovered branch is **not done**.

## Per-layer test conventions (follow `.claude/rules/006-testing.md`)
- **Go (`internal/`)**: Ginkgo/Gomega; table-driven where natural; test
  service/store behavior, not internals. Read `002-go-conventions.md`.
- **trainer-web**: Vitest + RTL + MSW; **test the hook, not the component**;
  cover guard / happy / refetch / error / callback. Read `003-frontend.md`.
- **client-mobile**: Jest + RNTL; same hook-first rule + `005-mobile.md`.
- Test **observable behavior, not implementation**. No `setTimeout` waits, no
  `it.skip`, no snapshot tests for dynamic output.

## Constraints
- Respect all `.claude/rules/` for the layer you touch (always `000-principles.md`,
  `004-security.md`, `006-testing.md` + the layer file). Read them first.
- Read memory first: `.claude/memory/MEMORY.md` (auto-loaded; `cat` it if
  it is not in your context), then `routines.md`, `decisions.md` and
  `flaky-tests.md`. A decision recorded there is binding — a finding whose
  fix was declined there is deferred, not fixed. `.claude/rules/` win over memory.
- Boy-scout only within files you fix; no unrelated reformatting / diff noise.
- One logical fix per commit; conventional-commit messages (`fix:` / `test:`).
- Never weaken or skip a test to make the gate pass.

## Definition of done
Walk `.claude/rules/007-clean-code-checklist.md` over every file you touched (the
layer's rows + Tests) before the gate — the same list interactive sessions
and the sweeps use.

## Verification gate (MUST pass before MR — paste output in description)
Run the affected layer's full gate:
- Backend:        `make lint && make test`. `make test` boots a local Postgres 16
  itself when there is no Docker and no `TEST_DATABASE_URL` (the cloud VM has
  neither), so the store packages are never skipped; "rootless Docker not
  found" means you ran `ginkgo` directly instead of `make test`.
- trainer-web:    `cd apps/trainer-web && npm run lint && npm run typecheck && npm run test:run && npm run knip`
- client-mobile:  `cd apps/client-mobile && npm run lint && npm run typecheck && npm run test:run && npm run knip`
If a fix can't pass the gate within scope, REVERT it and leave the finding's
file in place.

## Resolve the findings (same PR)
For every finding you fixed in THIS PR, **delete its file** under
`.ai/findings/open/` in the same PR. When the PR merges, the finding is gone —
that IS the resolved state. If the PR is closed unmerged, the file simply
stays on `main` and remains open; nothing to reconcile.
- Do NOT touch finding files you did not fix (keeps conflicts impossible).
- Commit with the fix (e.g. `chore: resolve fixed findings`).

## Deliverable: the MR
1. New branch off `main`: `fix/findings-<yyyymmdd>`.
2. Commit in small logical chunks — test commit, then fix commit, is encouraged
   (the history shows the red-before-green order).
3. Push, open PR against `main` with body:
   - **Findings fixed**: each item, its (deleted) finding file, and source PR
   - **TDD evidence**: per finding, the RED run (Mode A) or the PIN run plus
     zero-reference proof (Mode B), then the GREEN run
   - **Branch coverage**: branches touched + the test covering each (coverage output)
   - **Gate output**: pasted command results
   - **Deferred**: findings considered but left open, with the reason
   - **Awaiting decision**: every `domain` finding with no box ticked, one
     line each with its proposed option, so the maintainer sees the queue
4. These changes alter behavior — **request human review; do NOT auto-merge.**
5. Remember: only a fact that changes what the next run does and that the
   tree, `git log` or the PR list cannot show in one command (a finding
   that cannot be fixed as written and why, an environment quirk, a gate
   red on main that `flaky-tests.md` lacks). A stop condition — an open PR
   already fixing the finding, nothing eligible — is never memory; it goes
   in your final message only. Grep `.claude/memory/` first; already there
   means write nothing. Otherwise ONE unit in waiting,
   `.claude/memory/inbox/<yyyy-mm-dd>-<slug>.md` in the inbox format of
   `.claude/memory/README.md` (`file:` and `section:` from *Where a memory
   goes*, the finished `### slug` unit, its Evidence line; one fact, no
   narrative), committed on the same branch. Never edit
   `MEMORY.md` or a topic file in a routine run.
