# Memory

Git-tracked memory for every session that works on Binder — a laptop
session, a cloud session, a subagent, or an unattended Routine. It exists so
a session uses what an earlier one learned instead of re-deriving it, and
it grows from feature work as much as from the Routines.

Claude Code's auto memory normally lives machine-locally under
`~/.claude/projects/<repo>/memory/`, which a Routine (fresh VM) never sees.
The `SessionStart` hook (`.claude/hooks/session-start.sh`) links that
location here, so auto memory reads and writes this directory and the result
travels with the branch.

## Layout

| Path | What | Who writes | Loaded |
|------|------|-----------|--------|
| `MEMORY.md` | Index: **one line per memory** → `file.md#slug`. ≤ 200 lines / 25 KB; the loader drops everything past that | Interactive sessions, `/memory-dream` | Every session |
| `go.md`, `mobile.md` | **Layer files**: what the rules do not say about a layer | Interactive sessions, subagents, `/memory-dream` | On demand, by layer |
| `decisions.md` | Taken or declined, binding | Interactive sessions, `/memory-dream` | On demand |
| `routines.md`, `deploy.md`, `flaky-tests.md` | Fleet, infrastructure, red gates | Same | On demand |
| `feedback.md` | Maintainer corrections not yet promoted to a rule | Interactive sessions, `/memory-dream` | On demand |
| `inbox/<yyyy-mm-dd>-<slug>.md` | Write-once entries from Routine runs | Routines | By `/memory-dream` |
| `generated/` | Derived from the tree by `tools/memory-gen.sh`; never hand-edited, gitignored | The hook / `make memory-gen` | On demand |

## One memory = one unit

Every memory is one `###` unit in a topic file, under a `##` section that
groups it with its neighbours:

```markdown
### <slug-a-session-would-grep-for>
<the fact, as "X happens when Y", and what to do about it — one to three
short paragraphs, never a narrative of how it was found>
Evidence: #<PR> or <commit> or `<path>` · since <yyyy-mm-dd> · verified <yyyy-mm-dd>
```

- The **slug** is the unit's identity: unique across all topic files, stable
  once written, lowercase kebab-case, specific enough to grep (`huma-drops-the-output-when-a-handler-returns-an-error`,
  not `huma-note`). Decisions start with the date.
- The **Evidence** line is mandatory: a PR, a commit, a stable path or a
  dated run, so `/memory-dream` can verify the unit instead of trusting it.
  Never a file under `.ai/findings/`: findings are resolved by deletion, so
  that path dies with the fix and `tools/memory-check.sh` fails the unit.
  Name the finding in plain text (`finding 2026-09-10-<slug>`) and cite the
  PR. `verified` is refreshed by whoever re-checks it — the dream refreshes
  it when the cited evidence still exists and still says the same, which is
  not a re-test of the fact; `since` never changes.
- The **index line** is `- [type] <keywords a session would search for> → file.md#slug`,
  grouped under the file's heading. The keywords carry the **symptom**: the
  error text, the command or the observable a session has in front of it at
  the moment it needs the unit — `"Cannot find package 'vite'" → npm ci at
  the root`, not only the slug's words. The slug makes a unit reachable; the
  symptom makes it findable, because the index is the only part of memory a
  session greps before it knows which topic file to open.
  `tools/memory-check.sh` fails when a unit is missing from the index or an
  index line points at no unit.
- A topic file past ~120 lines is split by section into two files, and both
  are indexed. A unit nobody will act on again is deleted, not archived;
  `git log` is the archive.

## Where a memory goes

| The fact is about | File | Section examples |
|-------------------|------|------------------|
| `internal/`, `pkg/`, `cmd/` — a library's behavior, a store contract, a domain rule settled in code, the Go gate | `go.md` | API layer (Huma), Store and data, Domains, Gate and toolchain |
| Both apps or `packages/` — the compiler, shared hooks, the FE gate | `frontend.md` | React Compiler, Gate |
| One app only | `mobile.md` | Testing, Domain rules settled in code, Navigation |
| A choice taken or declined, with its PR | `decisions.md` | Workflow and tooling, Product and code |
| A Routine's behavior, the fleet, a run's environment | `routines.md` | How the fleet behaves, Environment of a run |
| Render, EAS, CI runners, deploy | `deploy.md` | Backend on Render, CI, Mobile |
| A gate red on untouched `main` | `flaky-tests.md` | Instances, Standing facts |
| Something the maintainer said or corrected | `feedback.md` | — |
| Layout, commands, domain lists | nowhere — `generated/` already has it | |

An inbox entry names its `file:` and `section:` from this table, so the dream
only moves it.

## Rules

1. **Precedence: `.claude/rules/` > memory > your own inference.** Memory never
   overrides a rule; when it contradicts one, the rule is right and the memory
   unit is stale — fix the unit.
2. **Do not remember what the tree can tell you.** Layout, commands, domain
   lists are in `generated/`. Auto memory skips derivable facts on its own;
   humans and Routines should too. **A stop condition is never memory** —
   back-pressure, an exclusion set, "no target", a stalled CI queue are
   recomputed by the next run in one command; a Routine that stops says so in
   its final message and writes nothing.
3. **A decision in `decisions.md` is binding** until a human reverses it.
   Never re-propose a declined change.
4. **Every unit carries evidence** and is verified, not trusted: the dream
   drops a unit whose evidence is gone or contradicted.
5. **Routines write to `inbox/` only** — one file per entry, never the index
   or a topic file (ten Routine PRs a day editing one index would conflict on
   every landing). Interactive sessions and subagents edit topic files and
   the index directly; they are single-writer per branch.
6. **Writing memory is part of done.** The 007 checklist's memory row: before
   the final message, anything the next session would re-derive goes into
   the file the table above names; a maintainer correction into
   `feedback.md`; a choice taken into `decisions.md`. Grep the slug first —
   a fact already here is refreshed (`verified`), not duplicated.
7. **`tools/memory-check.sh`** (CI: Quality Gate → memory) fails a PR whose
   memory names a path that no longer exists, whose index is over the load
   limit, or whose index and units disagree. Backticked repo paths are
   checked, so write them from the repo root (`internal/card/store`,
   not `card/store`) and do not backtick a package name or a symbol.
   Mark a line `<!-- no-check -->` only for a path that is deliberately
   historical.
8. **A red memory gate on untouched `main` is fixed in its own PR**, never
   inside a feature or Routine PR, and never by editing another run's inbox
   entry or a topic file from a Routine. On 2026-09-11 five PRs cut from one
   red base each patched the same inbox file, and every one conflicted once
   the first merged. Open `fix/memory-check-<yyyymmdd>` from `origin/main`
   with the one-line fix (skip it when such a PR is already open — the same
   Step 0 check as a finding), let it merge on green, then merge `main` into
   your branch. Your own PR says so in one line and stays otherwise untouched.

## Inbox entry format (Routines): a unit in waiting

A Routine cannot edit a topic file or the index (rule 5), so it writes the
**finished unit** to `inbox/` and the dream only moves it. The entry names
its destination and is otherwise exactly what will land in the topic file:

```markdown
---
file: go.md                      # from the table above
section: Gate and toolchain      # an existing ## section of that file, or a new one
type: reference | project | feedback
evidence: #552 | 3e0188d | 2026-09-09 run of /go-quality-sweep
---
### <slug-a-session-would-grep-for>
<the fact, as "X happens when Y", and what to do about it — never a narrative
of the run that found it, never more than one fact per entry>
Evidence: #552 · since 2026-09-09 · verified 2026-09-09
```

Before writing one: `grep -ril '<two or three keywords>' .claude/memory/`
(topic files *and* `inbox/`). A hit means the fact is known — name the unit
in the final message instead of writing again. `tools/memory-check.sh`
rejects an inbox entry with no `file:`, a `file:` that is not a topic file, no
`### slug`, a slug that already exists, or no `Evidence:` line.

`/memory-dream` (nightly Routine) moves each entry under the section it
names, adds its index line, verifies every existing unit, prunes what it
cannot verify, and proposes promotions into `.claude/rules/` for a human to
approve.
