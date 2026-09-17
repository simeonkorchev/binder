# Hygiene findings

Durable record of issues spotted but **intentionally not fixed** by the
behavior-preserving routines: `/cleanup` (see `.claude/commands/cleanup.md`)
and `/sql-optimize` (see `.claude/skills/sql-optimize/SKILL.md`, which also logs
`perf` items such as a missing index it is not allowed to add).

Those routines may not change behavior, so whenever they find a real bug or a
worthwhile-but-risky refactor they log it here instead of touching it. This is
the to-fix-later backlog; PR descriptions are not (they get buried once
merged). Findings are picked up and fixed by the `/fix-findings` routine (see
`.claude/commands/fix-findings.md`) — test-first, through normal review, since
those changes DO alter behavior.

## Format — one file per finding

Each unresolved finding is **one markdown file** under `open/`, named

```
open/<yyyy-mm-dd>-<short-slug>.md        (date = when it was found)
```

with this structure:

```markdown
# <one-line summary of what is wrong>

- **Category**: bug | security | dup | dead-code | types | i18n | simplify | perf | test | domain
- **Severity**: high | medium | low
- **Path**: `path/to/file-or-folder`
- **Found**: <yyyy-mm-dd> (#<PR that found it>)

<full description: what is wrong, why it was not fixed on the spot, and
anything useful for the fixer — branches, callers, risks, defer reasons.>
```

**Severity** decides the order `/fix-findings` drains the backlog in:
`high` = wrong behavior a user or a coach can hit, a security or data-integrity
gap, or a rule violation that will be copied; `medium` = correctness debt with
no user-visible effect today; `low` = tidiness. A file without the line counts
as `medium` (findings older than 2026-09-09 predate the field).

## `domain` findings — a contradiction that needs a decision

`/domain-audit` (`.claude/skills/domain-audit/SKILL.md`) flags domain logic
that disagrees with its spec, a rule, or its siblings. Such a finding is not
fixable until a human picks the correct behavior, so it carries these extra
sections after the description, in this order: **Contradiction** (one input,
the two outputs, the two sites), **Sources**, **Options** (A, B, leave it),
**Proposed**, **Drafted RED test** (a fenced diff), **Blast radius**, and
this block, verbatim:

```markdown
## Decision (maintainer)
- [ ] A — <one line>
- [ ] B — <one line>
- [ ] Leave it — the current behavior is intended; record it in decisions.md
Decided by: <name>, <yyyy-mm-dd>. Notes:
```

The maintainer ticks exactly one box and commits. `/fix-findings` ignores a
`domain` finding with no box ticked, takes a ticked A or B **first**
(regardless of severity — a human already spent the decision) starting from
the drafted RED test, and on *Leave it* deletes the file,
recording the decision in the finding's own closing note. A sweep's older "product decision" finding on the same
contradiction is upgraded in place by the audit (category → `domain`, block
appended), never duplicated.

**Intake is capped**: a producing routine logs at most 3 findings per run —
the three that matter most; the rest stay in its PR body. The cap exists
because ~10 producer runs a day against one consumer made this ledger
write-only (215 added, 18 resolved in the 60 days to 2026-09-09).

## Lifecycle

- **Open** — the file exists. `/cleanup`, the sweeps and `/domain-audit` create it. **Dedupe first**: grep
  `open/` for the same path + issue before adding; if a file already
  describes it, do not add another.
- **Resolved** — the PR that fixes the finding **deletes its file in the
  same diff**. Fix merged ⇒ file gone. Fix PR closed unmerged ⇒ the file
  simply stays open on `main`; nothing to reconcile.
- There is no separate in-progress state. To avoid double-fixing, check the
  repo's open PRs for one that already deletes the same finding file before
  claiming it (see `/fix-findings` Step 0).

Resolved findings need no archive section — `git log --diff-filter=D --
.ai/findings/open/` is the history.

> Why one file per finding: parallel routine PRs used to append to a single
> `backlog.md` and produced a merge conflict on every landing. Separate
> files with unique date-slug names merge cleanly no matter how many PRs
> add or remove findings concurrently.

If a stale branch created before this switch resurrects the legacy
`backlog.md`, move any of its open entries that lack a file here into
`open/` files by hand and delete `backlog.md` again.
