---
name: render-triage
description: Daily production triage of the Go backend on Render - pulls the last 24h of error logs (Render MCP or REST), clusters them into signatures, skips known noise and already-handled errors, root-causes the new ones in the code, fixes what is safely fixable test-first (RED to GREEN) and opens one PR with the log evidence; everything else becomes a finding or a needs-human ledger row. Never mutates Render.
---

# Render production triage (daily, evidence-first, read-only towards Render)

You are running the daily production-error triage for the Spotter Go backend,
deployed on Render as the web service **`shape-u-prod-be`** (see `render.yaml`).
The deliverable is one of: a fix PR that a reviewer can approve because the
PR carries the log signal, the failing test that reproduces it, and the fix;
or a ledger row plus a finding when a fix is not yours to make.

Relation to the other routines: `/fix-findings` consumes the backlog,
`/go-quality-sweep` refactors without changing behavior. This routine is the
only one that starts from **production signal** and the only one allowed to
change backend behavior on its own initiative, which is why its guardrails
are the strictest.

## 0. Load the guidance

1. `CLAUDE.md`, `internal/CLAUDE.md`
2. `.claude/rules/000-principles.md`, `.claude/rules/002-go-conventions.md`,
   `.claude/rules/004-security.md`, `.claude/rules/006-testing.md`
3. `.ai/triage/ledger.md` (what was already handled) and
   `.ai/triage/noise.md` (what is expected)
   The signature ignores everything after the first line of the message and
   of the error: pgx reports one line per connection attempt, so a single
   database restart would otherwise fan out into a cluster per attempt mix.
4. `pkg/telemetry/` and `pkg/eslog/` to remember the log shape: JSON lines
   with `severity` (`ERROR`, `CRITICAL`, …), `msg`, `error`, and a `@type`
   marker on every ERROR; echo's recover middleware writes `[PANIC RECOVER]`
   lines through its own logger.
5. Memory: `.claude/memory/MEMORY.md` (auto-loaded; `cat` it if it is not
   in your context), then `routines.md`, `decisions.md`, `deploy.md`, `flaky-tests.md`
   and `generated/repo-layout.md` from the same directory. A decision
   recorded there is binding — never re-propose it; a target it lists as
   rejected is not re-surveyed. `.claude/rules/` win over memory on any
   conflict. Tooling is plain grep/Read/Edit; no MCP code tools are needed.

## 1. Connect to Render (read-only) and stop conditions

Fixed identifiers (from `list_workspaces` / `list_services`; the account has
one workspace, so nothing is chosen at run time):

| What | Id |
|---|---|
| Workspace | `tea-d76h4b5m5p6s73bmn570` |
| Service `shape-u-prod-be` | `srv-d7p589d7vvec73bug760` |

Pick the first available source, in this order:

1. **Render MCP server** attached to the session (tools named `mcp__Render__*`).
   Pass `workspaceId` on every call; never call `select_workspace`. Fetch
   **error-class logs only**:
   `list_logs` with `resource: ["srv-d7p589d7vvec73bug760"]`,
   `level: ["error", "critical", "alert", "emergency"]`, `type: ["app"]`,
   `startTime` = now minus 24h, `limit: 100`, `direction: "backward"`.
   The result is paginated: while `hasMore` is true, call again with
   `startTime`/`endTime` set to the returned `nextStartTime`/`nextEndTime`,
   up to 20 pages. Append every `logs[]` entry as one JSON line to
   `$TMPDIR/render-logs.jsonl` (the entry as returned: `timestamp`,
   `message`, `labels`). A 100-entry page is ~100 KB, so write it to the file
   rather than reasoning over it inline. If the workspace or service id ever
   stops resolving, re-derive them with `list_workspaces` / `list_services`
   and log a finding that the skill's table is stale.
2. **REST** via `RENDER_API_KEY` in the environment:
   `python3 .claude/skills/render-triage/scripts/render_logs.py fetch --service shape-u-prod-be --hours 24 --level error --out "$TMPDIR/render-logs.jsonl"`.
3. **Neither** → stop and report exactly what is missing: attach the Render
   connector to the Routine, or set `RENDER_API_KEY` in the environment and
   allow `api.render.com` in its network policy. Open no PR, edit no file.

**Allowed Render calls**: workspace/service listing, `list_logs`,
`get_metrics`, `list_deploys`/`get_deploy`. **Forbidden, without exception**:
`trigger_deploy`, `update_environment_variables`, any `create_*`/`delete_*`,
`query_render_postgres` (production data, PII), and printing or storing the
API key. This routine reads production; it never touches it.

Other stop conditions:

- **Back-pressure**: 2 or more **open pull requests** whose head starts with
  `fix/triage-` (`mcp__github__list_pull_requests`, state `open`; only
  without the GitHub tools fall back to `git ls-remote --heads origin 'fix/triage-*'`).
  Stop, and say so in the final message — a stop is not memory. A branch
  with no open PR (a closed PR's leftover, an abandoned run) does not count
  and is not yours to delete; the reviewer's queue is the throttle.
- **Red main.** `make lint && make test` on untouched `origin/main` must be
  green before you change anything; otherwise stop and report the failing
  check.
- **Open-PR files.** Before editing a file for a fix, check whether any open
  pull request (`mcp__github__list_pull_requests`, or `git ls-remote --heads
  origin` + `git diff --name-only origin/main...FETCH_HEAD`) already touches
  it. If one does, do not fix there this run: record the cluster as a
  `needs-human` ledger row naming that PR, so the fix lands once.

## 2. Cluster and classify

```bash
python3 .claude/skills/render-triage/scripts/render_logs.py cluster "$TMPDIR/render-logs.jsonl"
python3 .claude/skills/render-triage/scripts/render_logs.py cluster "$TMPDIR/render-logs.jsonl" --json > "$TMPDIR/clusters.json"
```

The report lists every error-class cluster (severity ERROR or above, or a
panic marker) with a 10-hex **signature**, count, first/last seen, and a
status: `noise` (matches `noise.md`), `known:<status>` (already in the
ledger) or `NEW`. Only `NEW` clusters are candidates. Rank: panics first,
then by count.

A `NEW` cluster is **suspicious** when any of these hold:

- it is a panic or `runtime error`;
- it is a 5xx-producing path (an `ERROR` from a handler or service with a
  request in context);
- it repeats (count ≥ 3) or spans more than one hour;
- it names our own code (`internal/`, `pkg/`) rather than only a third party.

A single occurrence of a third-party failure (WorkOS, Gemini, a network
timeout) is not suspicious on its own; give it a ledger row `needs-human`
only if it repeats.

Investigate **at most two** suspicious clusters per run.

## 3. Root-cause in the code, then classify the fix

For each chosen cluster:

1. Locate the log call (`grep -rn "<msg text>" internal pkg cmd`), walk the
   call path (grep the function's callers), and read the surrounding
   branches until you can state the trigger as a sentence: "when X arrives
   with Y, Z happens".
2. Check `git log -S'<msg text>' --since='14 days ago'` and the deploy list
   (`list_deploys`): a signature that starts right after a deploy points at
   that diff.
3. Classify:
   - **Fixable here**: a code defect in `internal/`, `pkg/` or `cmd/` whose
     correct behavior is unambiguous (nil dereference, missing guard,
     unmapped sentinel reaching a 500, wrong field mapping, unchecked empty
     result, wrong error wrapping). → step 4.
   - **Needs a decision**: the fix would change an HTTP status, wire field,
     auth rule, LLM prompt bytes, or a product rule. → finding under
     `.ai/findings/open/` (Category `bug` or `security`), ledger row
     `decision`. Do not fix.
   - **Not code**: config, env vars, Render plan limits (the `starter` plan
     has fixed memory: an OOM restart looks like a crash), database
     availability, a third party outage. → ledger row `needs-human` with a
     one-line diagnosis; a finding only if code could degrade more
     gracefully. Do not fix.
   - **Cannot reproduce or pin**: → finding with what you ruled out, ledger
     row `finding`.

## 4. Fix test-first (Mode A, as in `/fix-findings`)

Per `.claude/rules/006-testing.md` and `internal/CLAUDE.md`:

1. **RED** - write the test that reproduces the production input and asserts
   the correct behavior. Ginkgo/Gomega, counterfeiter fakes, external test
   package; `make test` boots a local Postgres 16 for the store tests when
   there is no Docker and no `TEST_DATABASE_URL` (say so if even that fails). Run it; it must **fail for the reason the logs show**.
   Keep the output.
2. **GREEN** - the minimal production change. No drive-by refactors, no
   changes outside the failing path.
3. **COVER** - every branch you touched has a test
   (`go test -cover ./internal/<pkg>/...`).
4. **Total mapping / total error mapping** checks from
   `000-principles.md` sections 8c and 9 when the fix touches a mapper or a
   sentinel.
5. Gate: `make lint && make test`, plus `make check-spec` if anything under
   `api/` changed (`packages/types/openapi.json` must be unchanged: this
   routine never changes the contract).

Budget: one cluster's fix ≤ 300 changed lines including tests; two fixes in
one PR only if they share a root cause, otherwise one PR per cluster.

**Forbidden**: `db/migrations/**`, `packages/types/**`,
`internal/nutrition/store/food.go`, `cmd/spotter/main.go` wiring, env/config,
dependency bumps, weakening or skipping any test, pasting raw log lines that
contain emails, tokens, ids or request bodies (use the masked signature and
sample from the cluster report), committing anything from `$TMPDIR`.

## 4b. Definition of done

Walk `.claude/rules/007-clean-code-checklist.md` (Both layers + Go + Tests) over every file the fix touched, then the gate — a fix that lands a smell is one the next sweep rewrites.

## 5. Record: ledger and findings

- Append one row to `.ai/triage/ledger.md` for **every** cluster you
  classified this run (fixed, decision, needs-human, finding). Never edit or
  delete existing rows. Noise proposals go in the PR body, not in `noise.md`.
- Findings: one file each under `.ai/findings/open/`, format in
  `.ai/findings/README.md` (with `Severity`), dedupe first, add only, at most
  3 per run — the rest go in the PR body.
- If no PR is opened this run but rows or findings were added, open a
  ledger-only PR (`chore/triage-<yyyymmdd>`), so the record lands.

## 6. Deliver: branch, PR, body

- Branch off `origin/main`: `fix/triage-<yyyymmdd>-<slug>`.
- Commits: `test(<pkg>): reproduce <signature> from production logs` (RED)
  → `fix(<pkg>): …` (GREEN) → `chore(triage): record <signature> in ledger`.
- Push, `gh pr create` against `main` (fall back to the GitHub tools).
  Request human review. Never enable auto-merge; this changes production
  behavior.

```markdown
## TL;DR
- <what broke in production, in one line>
- <root cause, in one line>
- <fix + size: N files, +A/-D lines, of which tests +T>

## Signal
- Signature `<sig>` — <count> occurrences, first <ts>, last <ts>, panic: <yes/no>
- Masked sample: `<sample line from the cluster report>`
- Correlated deploy: <deploy id / commit, or "none">
- Window: last 24h of `shape-u-prod-be` via <MCP list_logs | REST>

## Root cause
<the sentence from step 3.1, then the code path with file:line references>

## Fix
- [guard|mapping|sentinel|nil|empty-result|wrapping] <one line>

## TDD evidence
1. **RED** — commit `<sha>`: <test name> fails with: <trimmed output showing the same failure the logs show>
2. **GREEN** — commit `<sha>`: <trimmed passing output>
3. **Coverage** — <rows for touched files>

## Gate output
<make lint && make test, and make check-spec if api/ changed>

## Ledger
- `<sig>` → pr (this PR)
- <other clusters classified this run and their status>

## Suggested noise
- <regex proposals for clusters that are expected, for a human to move into noise.md, or "none">

## Not run
- <store suites without Docker, anything the environment could not execute>
```

- Nothing suspicious after clustering: report "no new suspicious clusters in
  the last 24h" with the cluster table, open no PR (a ledger-only PR is fine
  if you classified anything).
## 7. Remember

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

