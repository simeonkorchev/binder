---
name: domain-audit
description: "Daily cross-layer audit of ONE domain concept (calories and macros, units and portions, dates and timezones, streaks, targets, sets and reps, reminders, sync, org scoping) - traces it from spec to model, store, API, both apps and translations, flags every contradiction it can show with two sources (spec vs code, or N-1 siblings vs one), writes each as a `domain` finding carrying a proposed fix, a drafted RED test and a one-line Decision block for the maintainer, and opens one findings-only PR. Never changes code; the decided findings are fixed by /fix-findings. Use for the domain-audit Routine or when asked whether a concept behaves consistently across layers."
---

# Domain audit (twice daily, one concept, flag and propose, never fix)

## Intent

The quality sweeps look at one layer and one slice; domain logic goes wrong
*between* layers. The Go service rounds calories one way, the mobile app
another, and the spec in `specs/` that produced both says a third. Nothing
else reads a concept end to end, and nothing checks code against the spec it
came from.

This routine does that for one concept per run, twice a day (05:00 and
17:00 UTC). Its output is decisions, not
code: every contradiction becomes a finding that a maintainer can settle by
ticking one box, and `/fix-findings` implements the ticked ones test-first.
A routine must never decide what the product does — with routine PRs merging
on green CI, a "fixer" here would silently change what coaches and clients
see. "Nothing written down → product decision, log it"
(`.claude/rules/007-clean-code-checklist.md`, *Siblings agree*) is the rule
this routine exists to serve.

## Hard boundaries

Never, in this routine:

- Change application code, a test, a spec, a rule, a skill, a locale value,
  or a memory topic file. The PR contains finding files, at most one
- Tick a Decision box, or word the options so only one is sane. The
  maintainer decides; you propose.
- Flag from taste. A finding needs **two sources**: the spec, a rule, a doc
  comment, an OpenAPI description, a DB constraint or an i18n string on one
  side, and code on the other — or N−1 siblings against one. "This looks
  odd" is not a finding.
  finding already describes (grep `.ai/findings/open/` for the path and the
  rule first). An open finding on the same contradiction that lacks a
  Decision block is **upgraded** — category set to `domain`, the block
  appended — not duplicated.
- Log more than **3** findings per run. The rest go in the PR body under
  *Also seen*, ranked, for the next run of this concept.
- Ask a question, enable auto-merge, or open a second PR while a
  `chore/domain-audit-*` pull request is open
  (`mcp__github__list_pull_requests`, state `open`; a leftover branch with
  no open PR does not count). Stop instead, and say so in the final
  message — a stop is never memory.

## 0. Load

1. `CLAUDE.md`, `.ai/findings/README.md` (the `domain` category and the
   Decision block are defined there — follow it exactly).
2. `.claude/rules/000-principles.md` (§4 siblings, §8b empty, §9 mapping,
   §10 invariants) and `.claude/rules/007-clean-code-checklist.md` (the
   *changing* rows are the kinds of contradiction you are looking for).
   in your context), then `routines.md`, `decisions.md` and
   `generated/repo-layout.md`. A decision recorded there is binding — never
   re-flag it. `.claude/rules/` win over memory on any conflict.
4. `ls specs/` — the specs are the primary source of truth for what a
   concept is supposed to do. Tooling is plain grep/Read; no MCP code tools.

## 1. Stop conditions and concept choice

Any stop means finish without a PR and say why.

- `git fetch origin main` and work from `origin/main`.
- **Back-pressure**: an open `chore/domain-audit-*` pull request. Stop.
- **Rotation**: pick the concept audited least recently. History is the
  merged PR titles (`git log --grep='chore(domain-audit)' --format=%s origin/main`)
  plus any inbox or `routines.md` line saying "<concept> audited <date>,
  clean". Never the same concept two runs in a row.
- **Concepts** (seed list; a concept is something a user has a mental model
  of and a spec, a rule or a sibling defines):
  calories and macros derivation (food, recipe, custom item, barcode);
  units, portions and serving sizes; dates, timezones and "today";
  streaks and habit counting; macro periods and daily targets; satiety
  score; sets, reps, PR detection and prescriptions; exercise history and
  alternatives; reminders and nudges (quiet hours, kill switch); health-app
  sync (sleep, vitals); coach ↔ client org scoping and permissions; recipe
  duplication, naming and translations; food database import/export;
  progress insights and body measurements; questionnaire scoring; photo
  comparison. Add a concept when a spec introduces one.

## 2. Trace

Build the concept's map before judging anything.

1. **Sources of truth**, in precedence order: the spec(s) in `specs/`
   (formula tables, fallback rules, user stories); `.claude/rules/` and
   `decisions.md`; Go doc comments and Huma `Description` strings; the
   OpenAPI descriptions in `packages/types/openapi.json`; DB constraints and
   migrations under `db/`; the locale strings in `packages/intl/locales/`
   (what the UI *promises*). Quote each rule you extract with its location.
2. **Implementation sites**: grep the concept's vocabulary across
   `internal/`, `pkg/`, `apps/trainer-web/src`, `apps/client-mobile/src`,
   `packages/`. For each site record: what rule it applies (rounding,
   clamping, default, unit, inclusion, ordering, timezone, fallback, zero
   handling) and which source defines that rule — or "none".
3. Lay it out as a table in your notes: rule → source → each site's
   behavior. A row where two cells disagree is a candidate.

## 3. Flag

A candidate becomes a finding only when you can write **one concrete input
and the two outputs it produces** (site A says X, site B or the spec says
Y). Prove it by reading the code paths, and by running an existing test or
a one-off script when the arithmetic is not obvious. No proof, no finding.

Rank candidates by what a user can hit (`high`), correctness debt with no
user-visible effect (`medium`), tidiness (`low`), and take the top three.
Then, for each, write the finding file per `.ai/findings/README.md` with
every section the `domain` category requires:

- **Contradiction** — the input, the two outputs, the two sites.
- **Sources** — what defines the correct behavior, quoted, or "nothing
  written down".
- **Options** — A and B (and *leave it*), each one line, each a behavior a
  user could observe. If a source settles it, say which option it supports;
  still offer both.
- **Proposed** — your recommendation in one sentence, with the reason.
- **Drafted RED test** — a fenced diff against the real test file for
  option A (and B if it differs materially): Ginkgo for Go, Vitest for
  trainer-web, Jest for client-mobile, per `.claude/rules/006-testing.md`.
  It must fail on today's code for the stated reason. It is a draft:
  `/fix-findings` verifies it before trusting it.
- **Blast radius** — every caller, screen and consumer that changes.
- **Decision block** — verbatim from the README, no box ticked.

An open finding that already describes the contradiction (usually a sweep's
"product decision" entry) is edited in place instead: category → `domain`,
the missing sections and the Decision block appended, `Found` line extended
with today's date and this routine.

## 4. Deliver

- Branch off `origin/main`: `chore/domain-audit-<yyyymmdd>-<concept-slug>`.
- One commit: `chore(domain-audit): <concept> — <N> contradictions flagged`.
  Zero flagged: no findings PR; write the inbox entry from step 5 and open
  it as `chore/memory-<yyyymmdd-HHMM>` so the rotation records the concept
  as clean.
- Open the PR with `mcp__github__create_pull_request`, same title. Never
  auto-merge. Body:

```markdown
## TL;DR
<concept>: <N> contradictions flagged, <M> existing findings upgraded, <K> also seen. Decisions needed: <N>.

## Sources read
- <spec / rule / comment, one line each>

## Flagged — one decision each
| Finding | Severity | Contradiction (input → A vs B) | Proposed |
|---|---|---|---|

## Upgraded
- <existing finding file>: <what was added>

## Also seen (not logged — cap of 3)
- <site, one line each, ranked>

## Not run
<anything the environment could not execute>
```

The maintainer settles a finding by ticking one box in its file and
committing; `/fix-findings` picks decided `domain` findings first.

## 5. Remember

Before the final message: one file
`section:`, the `### slug` unit, its Evidence line), committed on the same branch as
`chore(memory): remember <slug>`, when the run learned something the next
audit of this concept would otherwise re-derive: the concept's source map
(which spec sections and files define it — that is the expensive part), a
candidate rejected because a source settled it, a concept found clean.
first; a fact already there is not written again.
folds the inbox in nightly.

Work end to end without asking questions. The findings and their decision
blocks are the only output that matters.
