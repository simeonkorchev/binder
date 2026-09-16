---
name: sql-optimize
description: "Six-hourly, behavior-preserving SQL query quality pass over ONE internal/<domain>/store package: pins each query's contract with store integration tests on a real Postgres first, rewrites within a 1000-changed-line budget, proves identical rows/order/errors plus EXPLAIN plan shape, and opens one PR with a TL;DR and the evidence. Use for the sql-optimize Routine or when asked to optimize store SQL without changing behavior."
---

# SQL query quality pass (behavior-preserving, budgeted, evidence-first)

You are running an automated pass over the **hand-written SQL in the Go store
layer only** (`internal/<domain>/store/`, sqlx + pgx, query text in constants).
The aim is efficiency without any change in correctness: fewer round trips
and better plans, with identical results. The deliverable is one small pull
request that a reviewer can approve in minutes because the PR itself proves
every rewritten query returns exactly what it returned before.

Sibling routines, not to be duplicated:

| Routine | Layers | Cadence | Size | Behavior |
|---------|--------|---------|------|----------|
| `/cleanup` | Go + web + mobile | daily | 300-900 lines, whole domain | preserved |
| `/fix-findings` | all | every 3 h + daily | 1-3 backlog items | MAY change (test-first) |
| `/go-quality-sweep` | Go only, **never query text** | every 90 min | 500-2000 lines, one package | preserved, or test-first |
| `/fe-refactor` | web + mobile only | every 90 min | 500-2000 lines, one slice | preserved, or test-first |
| **`/sql-optimize`** | **`internal/*/store` SQL only** | **every 6 h** | **<= 1000 lines, one package** | **preserved, proven on Postgres** |

`/go-quality-sweep` forbids changing query text or query results; this
routine is the one place that may rewrite a query, and in exchange it never
touches anything above the store (`api/`, `service/`, `model/`). Real bugs you
spot are **logged, never fixed** here (step 6).

## 0. Load context (in this order, before touching code)

1. `CLAUDE.md`, `internal/CLAUDE.md`
2. `.claude/rules/000-principles.md`, `.claude/rules/002-go-conventions.md` (§7
   transactions, §8 bulk queries, §8a row→model mapping), `.claude/rules/004-security.md`,
   `.claude/rules/006-testing.md` (Go section: store suites run on a real Postgres
   inside a rolled-back tx)
3. Technique skills (forked and edited to the house rules; `.claude/skills/VENDOR.md`):
   `golang-refactoring/references/safety-net.md` (characterization tests
   first) and `golang-safety/references/slice-map-safety.md` (the regrouping
   code a batched query needs). `.claude/rules/` win on any overlap.
4. `tests/dbhelper/dbsetup.go`: how the suites get their database
   (testcontainers, or `TEST_DATABASE_URL` when set).
5. Memory: `.claude/memory/MEMORY.md` (auto-loaded; `cat` it if it is not
   in your context), then `routines.md`, `decisions.md`, `flaky-tests.md`
   and `generated/repo-layout.md` from the same directory. A decision
   recorded there is binding — never re-propose it; a target it lists as
   rejected is not re-surveyed. `.claude/rules/` win over memory on any
   conflict. Tooling is plain grep/Read/Edit; no MCP code tools are needed.

## 1. Backpressure, exclusions, environment (stop conditions)

Run these first; any hit means **finish without a PR** and say why.

- `git fetch origin main` and work from `origin/main`.
- **Back-pressure**: 3 or more **open pull requests** whose head starts with
  `chore/sql-optimize-` (`mcp__github__list_pull_requests`, state `open`; only
  without the GitHub tools fall back to `git ls-remote --heads origin 'chore/sql-optimize-*'`).
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
  age: `git log -1 --format=%cd FETCH_HEAD`. Plus every store package a merged
  run touched in the last 7 days
  (`git log --since='7 days ago' --grep='sql-optimize' --name-only`, and the
  same for `--grep='chore(sql):'` — a merged run's title carries that, not
  the routine's name).
  The cloud clone is shallow: `git rev-parse --is-shallow-repository` →
  `git fetch --deepen=200 origin main` before any `git log` here, or the
  boundary commit reports every file as touched (memory `routines.md`).
  A feature
  branch is excluded for the same reason a sibling is: refactoring under an
  open PR makes it conflict (memory `routines.md`: #541 vs #552). Never
  pick a query inside it; that is how runs avoid conflicting with each
  other, with the Go sweep, and with the maintainer's feature work. (This routine fires every 6 hours; SQL
  changes slowly, so four runs a day keep the queue reviewable.)
- **Always excluded**: `internal/nutrition/store/food.go` (empties are handled
  upstream by design) and `internal/llm/store` (its suite commits through the
  shared pool).
- **A database is mandatory.** The evidence in this routine is store suites
  executing on Postgres; there is no store-less fallback scope. Check
  `docker info` or `TEST_DATABASE_URL`; when neither works (remote sandboxes
  have no Docker) run the bootstrap:

  ```bash
  export PATH="$(go env GOPATH)/bin:$PATH"
  eval "$(tools/test-db-local.sh)" && [ -n "$TEST_DATABASE_URL" ]   # local Postgres 16 + pgvector
  go install github.com/onsi/ginkgo/v2/ginkgo@"$(grep 'github.com/onsi/ginkgo/v2 ' go.mod | awk '{print $2}')"
  make install-golangci-lint || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@"$(cat .golangci-version)"
  ```

  The script also enables `auto_explain` (every statement's
  `EXPLAIN (ANALYZE, BUFFERS)` plan goes to `$TEST_DATABASE_LOG`) and
  `pg_stat_statements` (calls per distinct statement); step 2b uses both.
  Then prove it: `ginkgo run -race ./internal/support/store/` must execute
  specs (not compile-only). If the suite still cannot run, stop; a green gate
  that skipped the store tests is not evidence.
- **Baseline gate on untouched main**: `make lint && make test`. Red on
  `origin/main`: stop, report the failing check, fix nothing. A green run
  doubles as the "before" baseline for the PR body.

## 2. Pick ONE store package and at most 4 queries

Read every non-test file of the chosen package. Rank candidate queries by
expected impact with this checklist; every row is behavior-preserving only
when the "watch out" column holds:

| Smell | Rewrite | Watch out |
|-------|---------|-----------|
| Query inside a `for` loop (N+1) | one batched query with `sqlx.In` + `tx.Rebind` or `= ANY($n)`, regrouped in Go | per-item ordering, duplicate ids, unknown ids and the empty list must behave exactly as the loop did |
| Several round trips assembling one result | one statement / CTE | the same rows must come back when one piece is empty |
| `SELECT *` | explicit column list matching the row struct | do not drop a column the struct scans |
| Correlated subquery re-evaluated per row | `JOIN` / CTE with identical semantics | a join can multiply rows where a subquery did not |
| `NOT IN (subquery)` | `NOT EXISTS` | only when the compared column is `NOT NULL` in `db/migrations` — prove it |
| `DISTINCT` / `ORDER BY` inside a subquery the outer query discards | remove | keep any that feeds `LIMIT` or an aggregate |
| Repeated query fragments across methods | shared constant fragments (as `localizedName(...)` does) | no runtime assembly from inputs |
| Query text built with `fmt.Sprintf` | fixed constants, one per variant | never interpolate identifiers from callers |
| Query with no comment naming the index that backs it | add the one-line comment (existing convention) | comments only; no index changes |
| `OFFSET` pagination, unbounded scans, missing `ORDER BY` | **do not change** (behavior). Log a finding | |

Rules of choice:

- One package per PR, never two. Rotate: prefer a package no sql-optimize PR
  touched in the last 7 days; with no history, start with the packages that
  have the most queries inside loops.
- Estimate the diff before starting. If the honest estimate including the pin
  specs is over ~800 lines, take fewer queries.
- Skip anything in the exclusion set and anything **Forbidden** below.
- Write down the intended rewrite per query before editing.
- If the package yields nothing worth changing, survey one more package; if
  that is clean too, finish with "nothing worth changing this run" and no PR.

## 2b. Plan triage: let Postgres rank the candidates

Reading SQL finds smells; the planner finds bottlenecks. Do this before the
final pick, and keep the output as the "before" half of PROVE step 2.

1. **Round trips.** Reset the counters, run the package suite, read the
   counts:
   ```bash
   psql "$TEST_DATABASE_URL" -c 'SELECT pg_stat_statements_reset()'
   ginkgo run -race ./internal/<domain>/store/
   psql "$TEST_DATABASE_URL" -c "SELECT calls, rows, left(regexp_replace(query, '\s+', ' ', 'g'), 120) AS q
     FROM pg_stat_statements WHERE query ILIKE 'select%' ORDER BY calls DESC LIMIT 25"
   ```
   A store statement whose `calls` track a fixture count (20 calls from a
   spec that seeded 20 items) is an N+1: batch candidate. Ignore the
   `FOR KEY SHARE` foreign-key checks and migration bookkeeping.
2. **Plans at volume.** The suites seed a handful of rows, so every plan in
   the log is a seq scan and proves nothing. For each candidate, in a
   throwaway spec (deleted before committing): seed the tables the query
   touches with `generate_series` inside the spec's transaction (50k+ rows in
   the largest table, spread across many clients/orgs so per-tenant
   predicates are selective), run `ANALYZE <table>` (allowed inside a
   transaction and sees the uncommitted rows), then
   `tx.Queryx("EXPLAIN (ANALYZE, BUFFERS) "+query, args...)` printed to
   `GinkgoWriter`. The rollback removes everything.
3. **Read the plan** and file each signal by outcome. Timings on a tiny
   database are noise; compare node types, `loops`, `Buffers`, and
   `Rows Removed by Filter`:

   | Signal | Meaning | Outcome |
   |--------|---------|---------|
   | `Seq Scan` with `Rows Removed by Filter` far above rows returned | no usable index for the predicate | rewrite if an existing index is blocked by a cast/function on the column; otherwise a `perf` finding with the plan (no migrations here) |
   | `Nested Loop` whose inner `loops=N` tracks the outer row count, or a `SubPlan` | correlated subquery re-evaluated per row | JOIN / CTE candidate |
   | `Index Scan` with a `Filter:` line removing most rows | index covers the wrong column | finding, with the plan |
   | `rows=` estimate off by >10x from `actual rows` after ANALYZE | planner misestimate | note in the finding |
   | `Sort Method: external merge` or `Batches: >1` | spills to disk at volume | note only; not a query rewrite |
   | many statements per store method (step 1) | round trips | batch candidate |

4. Rank the candidates by these findings, not by how the SQL reads, and only
   then choose the queries for step 3.

## 3. PIN: store specs before any production edit

A behavior-preserving rewrite cannot have a failing-first test by definition.
The evidence is a pinning spec that is green **before and after**, in the
package's Ginkgo suite (real Postgres via `tests/dbhelper`, each spec inside a
rolled-back tx, conventions per `002-go-conventions.md` §9).

For every query you intend to rewrite, a spec must cover:

1. the happy path with **at least three rows**, asserting exact rows, fields
   and order (assert the record, not its length);
2. the empty result: empty slice + nil error, or the documented not-found
   error, whichever the current code does;
3. every predicate you will touch: a fixture on each side of it, including a
   **NULL** fixture for any nullable column in it;
4. tenant/owner scoping: rows of another client/org/coach must NOT appear;
5. for loop → batch rewrites: duplicate ids, unknown ids, the empty id list.

Reuse an existing spec only if it already covers all five; otherwise add the
missing specs. They pass immediately; that is the point. If a query's
contract cannot be pinned (depends on `now()`, randomness, concurrent
sessions), do not rewrite it this run; log it as a finding.

Run the specs on the untouched code, capture the run (command, summary line,
commit SHA) as the PIN evidence, and commit the tests **on their own**:
`test(<domain>/store): pin <query> contract before rewrite`. History showing
the test commit before the change commit is the reviewer's proof.

## 4. CHANGE: one query per commit

Allowed (all behavior-preserving): every rewrite in the step 2 table, plus
the Go that regroups a batched result, boy-scout lint fixes in files you
already touch, and comments that name the backing index.

Keep the method signature, the row struct, the error wrapping text, the
`sqlxtx.MustGetTx(ctx)` pattern, and the isolation level exactly as they are.

Working rhythm:

- One query per commit: `perf(<domain>/store): batch <method> lookups` or
  `refactor(<domain>/store): <what>`. After each commit run
  `go build ./... && go vet ./... && ginkgo run -race ./internal/<domain>/store/`.
  A red step is **reverted, not debugged forward**.
- After every commit run `.claude/skills/sql-optimize/scripts/diff-budget.sh`.
  It fails once the diff passes **1000 changed lines** and warns past 4
  production files; it also lists scope violations, forbidden paths, skip
  markers and the SQL hotspots a reviewer must eyeball. On a failure: revert
  the last step and stop adding scope.

Forbidden (each one is a behavior change or a review risk; revert):

- Changing rows, columns, order, `LIMIT`/`OFFSET`, `FOR UPDATE`, `ON CONFLICT`
  targets, error values or wrapping text, isolation level, or where a
  transaction begins and ends.
- `NOT IN` → `NOT EXISTS` on a nullable column; a join that can multiply
  rows; dropping a `DISTINCT` the outer query relied on.
- Schema, index, or extension changes (`db/migrations/**`, `*.sql`); a
  missing index is a `perf` finding with its `EXPLAIN`, never a migration.
- Touching `api/`, `service/`, `model/`, fakes, `packages/types`, env or
  wiring; anything outside `internal/<domain>/store/` and
  `.ai/findings/open/`.
- Removing or renaming an exported store method or row struct field. The
  budget script lists every exported declaration the diff removes or changes;
  each must be a rename with every reference updated or dead (proof pasted).
- Building query text from request data (`fmt.Sprintf` with caller input).
- Weakening, skipping or deleting any spec (`Skip(`, `XIt`, `PIt`).
- Fixing a bug you notice. Log it under `.ai/findings/open/` instead.
- Anything in the exclusion set from step 1.

## 5. PROVE: the gate and the evidence

Before the gate, walk `.claude/rules/007-clean-code-checklist.md` (Both layers + Go rows) over every Go file you touched — the same list every session uses; a query rewrite that leaves a smell behind is not done.

Every item below goes into the PR body. Missing evidence means the PR is not
ready.

1. **Pin specs, before and after.** The step 3 run on the untouched commit,
   then the same command on the final commit. Both green, both pasted with
   their SHAs.
2. **Plan shape.** The step 2b before-plan next to the same volume-seeded
   `EXPLAIN (ANALYZE, BUFFERS)` of the new SQL, plus the `pg_stat_statements`
   call count of the store method before and after. Timings mean nothing;
   the evidence is node types (seq vs index scan, join strategy), `loops`,
   `Rows Removed by Filter`, and the round-trip count (N queries → 1).
3. **Differential check** for structural rewrites (join/subquery/`NOT IN`,
   loop → batch): same fixtures, old SQL executed raw vs the new store method,
   `Expect(new).To(Equal(old))`. Keep it in the suite only if it reads as a
   real spec; otherwise run it, paste it, delete it.
4. **Budget script.** The full output of
   `.claude/skills/sql-optimize/scripts/diff-budget.sh origin/main`: budget
   within cap, exactly one store package touched, nothing outside it and
   `.ai/findings/open/`, exported declarations justified, forbidden paths
   none, skip markers none, and each remaining SQL hotspot line explained.
5. **Coverage not lower** for the package:
   `go test -cover ./internal/<domain>/store/...` before and after, plus the
   branch → spec list for the Go you touched (regrouping, empty-list guard).
6. **Full backend gate green**: `make lint && make test` (every suite with
   `-race`). Anything you cannot make pass within the allowed changes:
   **revert that change**, never the test.
7. **Self-review against the Forbidden list.** Re-read the whole diff once with
   only that list in mind and state "none found" or what you reverted.

## 6. Findings: log, do not fix

A real bug, a missing index (with its `EXPLAIN`), `OFFSET` pagination, a
missing `ORDER BY`, a scoping doubt, or a query you could not pin becomes one
file `.ai/findings/open/<yyyy-mm-dd>-<slug>.md` in the format of
`.ai/findings/README.md` (title, Category `perf` | `bug`, Severity, Path, Found).
Dedupe against the existing files first. Only add; never edit or delete
existing findings here. **At most 3 findings per run** — the three that matter most, each with a
`Severity` (`.ai/findings/README.md`); everything else stays in the PR body's
"Found, not fixed". Ten producer runs a day against one consumer made the
ledger write-only (215 added vs 18 resolved in 60 days).

## 7. Deliver: branch, PR, body

- Branch off `origin/main`: `chore/sql-optimize-<domain>-<yyyymmdd-HHMM>` (UTC).
- Commits in order: `test(<domain>/store): pin …` → one `perf(…)`/`refactor(…)`
  per query → `chore: log sql-optimize findings` (if any).
- Push and open the PR against `main` (`gh pr create`; fall back to the
  GitHub tools if `gh` is absent). Request human review. Never enable
  auto-merge. If no PR tooling is available at all, still push the branch and
  end the run with the full PR title and body in your final message so a
  human can open it; never discard the work silently. Title:
  `chore(sql): <domain> — <what>`. Body, in this order:

```markdown
## TL;DR
- <what changed, one line: which queries, N round trips → 1, seq scan → index scan>
- <why it is safe: pinned by N store specs on Postgres, identical rows/order/errors>
- <size: N files, +A/-D lines, of which tests +T>

## Scope
- Package: `internal/<domain>/store`
- Files: <list>
- Budget: <total> / 1000 changed lines (production <n>, tests <n>) — from diff-budget.sh

## Queries changed
| File | Method | Smell | Rewrite |
|------|--------|-------|---------|

## Behavior-preservation evidence
1. **Pin specs first** — commit `<sha>` adds <n> specs covering <happy/empty/NULL/scoping/batch cases>; they pass on the untouched code:
   <PIN run, trimmed to the summary lines>
2. **Same specs after the change** — commit `<sha>`:
   <PROVE run>
3. **Plan shape** — per query, EXPLAIN before / after (node types, round trips)
4. **Differential check** — <output, or "not a structural rewrite">
5. **Budget script** — exported declarations: none (or each + why); scope: one package; forbidden paths: none; skip markers: none; hotspots: <each explained>
6. **Coverage** — <before / after line for the package>
7. **Self-review against Forbidden** — none found (or what was reverted)

## Backwards compatibility
Same exported signatures, same row structs, same rows/order/errors, no migrations, no caller changed. Callers checked: <list from find_referencing_symbols / grep>.

## Gate output
<make lint && make test, pasted>

## Found, not fixed
- <each finding file added, one line>

## Not run
- <anything skipped, e.g. "EXPLAIN on production-sized data: not available in this environment">
```

- If every candidate was reverted or nothing fit the budget, open no PR and
  report "nothing safely improvable this run" with the candidates you
  rejected and why. A findings-only PR that adds files under
  `.ai/findings/open/` is fine.

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
matters; a run that ends with local edits and no PR has produced nothing.
