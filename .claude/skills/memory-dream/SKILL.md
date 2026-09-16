---
name: memory-dream
description: "Nightly consolidation of the git-tracked memory in .claude/memory - folds the Routines' inbox entries into topic files, verifies every existing entry against the tree and the PR history, drops what it cannot verify, records declined and reverted PRs as decisions, and proposes rule promotions for a human in one chore(memory) PR. Use for the memory-dream Routine or when asked to consolidate memory."
---

# Memory dream

## Intent

Every session on this repo starts from `.claude/memory/MEMORY.md`. The
Routines add what they learn as one file each under `.claude/memory/inbox/`
because ten concurrent PRs editing one index would conflict on every landing.
Nothing reads the inbox except this routine. Its job is the batch half of
memory — **Verify, Organize, Enrich** — so that every next session starts
truer than the last and nobody re-derives what a previous run already paid
for.

Memory is only worth its context cost when it is true. A wrong entry is
worse than no entry: it is read with the same confidence as a rule. So this
routine is allowed to delete, and is required to verify before it keeps.

## Hard boundaries

Never, in this routine:

- Edit `.claude/rules/`, `CLAUDE.md`, a skill, an agent, or any code —
  `tools/memory-check.sh` included (#581 patched it; a checker false
  positive is a **Proposed rule** entry with the diff, never a fix). A
  rule change is a **proposal** in the PR body (step 4); a human applies it.
- Delete or soften an entry in `decisions.md`. Decisions are reversed by a
  human; you may add, and you may mark one `superseded by #N` with evidence.
- Drop a memory because it is old. Drop it only when verification fails:
  the path is gone, the cited PR says the opposite, or a rule now covers it.
- Let `MEMORY.md` exceed 200 lines / 25 KB, or let `tools/memory-check.sh`
  fail. Both are enforced; the loader silently truncates past the limit.
- Invent an entry. Every bullet you write carries the evidence you read.
- Ask a question, fetch anything but this repo's PRs, or enable auto-merge.
- Open a second PR while a `chore/memory-dream-*` pull request is open
  (`mcp__github__list_pull_requests`, state `open`). Stop instead; a
  leftover branch with no open PR does not count.

## 0. Load

1. `CLAUDE.md`, `.claude/memory/README.md` (the format and the rules).
2. `.claude/memory/MEMORY.md` and **every** topic file it names.
3. `.ai/findings/README.md`, `.ai/triage/ledger.md` (how the ledgers work).
4. `make memory-gen`, then `.claude/memory/generated/repo-layout.md` — the
   current truth for any layout claim a hand-written entry makes.
5. `last=$(git log -1 --format=%cI --grep='chore(memory): dream' origin/main)`;
   empty means this is the first dream — use 14 days.

## 1. Gather

Collect, without judging yet:

1. **Inbox**: every `.claude/memory/inbox/*.md`.
2. **PR outcomes since `$last`** via `mcp__github__search_pull_requests`
   (fallback `git log --since="$last" origin/main`):
   - closed **unmerged** PRs → candidate `decisions.md` entries ("declined");
   - `git log --since="$last" --grep='^Revert' origin/main` → candidate
     decisions ("reverted, do not re-propose");
   - routine PRs whose title says no target / nothing found (#522, #539
     shape) → a waste signal for `routines.md`;
   - fix PRs that name a check red on `main` (#543 shape) → `flaky-tests.md`.
3. **Ledgers**: new rows in `.ai/triage/ledger.md` and the delta of
   `.ai/findings/open/` since `$last`.
4. **Routine run transcripts**, only if `mcp__Claude_Code_Remote__list_sessions`
   is available: the final message of each Routine session since `$last`,
   for stop conditions and rejected targets the run did not write to the
   inbox. Optional; skip silently when the tool is absent.

## 2. Verify

For every unit in every topic file (not `generated/`):

- Every backticked path exists (`tools/memory-check.sh` does this
  mechanically; you do it with judgment for prose paths).
- The cited PR or commit exists and supports the claim: `git show`, or
  `mcp__github__pull_request_read`. A claim its own evidence contradicts is
  dropped.
- No rule in `.claude/rules/` now covers it. If one does, the entry is redundant:
  drop it and cite the rule in the PR body.
- Otherwise keep it, refresh the unit's `verified` date and the file's
  `modified:` front-matter.

Everything dropped is listed in the PR body with the reason. Nothing is
dropped silently.

## 3. Organize

- Each inbox entry is a unit in waiting (README: *Inbox entry format*):
  move its `###` unit under the `section:` of the `file:` it names (create
  the section if new) and add its index line. When an existing unit already
  says the same thing under another slug, merge into that unit instead
  (refresh `verified`, add the evidence) and drop the new slug. An entry
  written in the old free-form shape is folded by the README's *Where a
  memory goes* table. Then `git rm` the inbox file.
- Two units that say the same thing become one; the surviving slug is the
  older one. A unit nobody will act on again is deleted.
- Index: one line per unit, `- [type] <keywords a session would search for> → file.md#slug`,
  under the file's heading; `tools/memory-check.sh` fails on any unit
  missing from the index or any index line pointing at no unit. A topic
  file past ~120 lines is split by section and both halves indexed.
- Nothing here changes `generated/`; it is regenerated by every session.

## 4. Enrich

- **Promotion.** A gotcha or correction confirmed by two independent sources
  (two PRs, a PR and a transcript, an inbox entry and the ledger) belongs in
  a rule, not in memory. Draft the exact `.claude/rules/` or skill edit as a diff
  block under **Proposed rule — needs human** in the PR body. Leave the memory
  entry in place until the rule lands.
- **Rejections.** Every closed-unmerged or reverted PR from step 1 becomes a
  `decisions.md` bullet: what, PR link, date, and "reason not recorded" when
  the PR carries none. This is the channel that stops a Routine from
  re-proposing what the maintainer already declined.
- **Waste.** Stop conditions and no-target runs go into `routines.md` as one
  line each, so the next run of that Routine checks before surveying.
- **Feedback.** A maintainer correction that appears in a transcript or a PR
  review and is not in `feedback.md` is added there.

## 5. Deliver

- `make check-memory` must pass. `wc -l .claude/memory/MEMORY.md` ≤ 200.
- Branch off `origin/main`: `chore/memory-dream-<yyyymmdd>`. One commit per
  concern is fine; the last is `chore(memory): dream <yyyy-mm-dd>`.
- Open the PR with `mcp__github__create_pull_request`, title
  `chore(memory): dream <yyyy-mm-dd>`. Body:

```markdown
## TL;DR
<N inbox entries folded, N entries verified, N dropped, N decisions added, N rule proposals>

## Folded
- <inbox file> → <topic file>: <one line>

## Dropped
- <topic file>: <bullet> — <reason: path gone | contradicted by #N | covered by .claude/rules/00X §Y>

## Decisions added
- <bullet as written>

## Proposed rule — needs human
<for each: which file, why (the two sources), and the diff>

## Not run
<transcripts unavailable, GitHub tools unavailable, …>
```

Nothing to fold, nothing dropped, nothing to add: open no PR and say so in
one line. An empty dream is a fine outcome; a fabricated one is not.
