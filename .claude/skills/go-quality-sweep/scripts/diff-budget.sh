#!/usr/bin/env bash
# Prints the evidence block the /go-quality-sweep PR body needs: the diff
# budget against the base branch (generated fakes and the generated OpenAPI
# types excluded), the exported Go declarations the diff removes or changes
# (each must be justified as a behavior-preserving rename or dead code), and
# any forbidden path the diff touches.
#
# Budget model: the floor (500) counts every changed line so a PR is worth a
# review cycle; the hard cap (2000) counts PRODUCTION Go lines only, so pin
# specs are never the reason a routine drops scope or trims a spec. A total
# past 3000 only warns (reviewability hint).
#
# Usage: .claude/skills/go-quality-sweep/scripts/diff-budget.sh [base-ref]
set -euo pipefail

base="${1:-origin/main}"
budget="${GO_SWEEP_BUDGET:-2000}"            # hard cap on production Go lines
floor="${GO_SWEEP_FLOOR:-500}"               # floor on total lines (specs included)
total_warn="${GO_SWEEP_TOTAL_WARN:-3000}"     # reviewability warning on total lines

excl=(':(exclude)**/*fakes/**' ':(exclude)packages/types/**' ':(exclude)vendor/**')

sum() { awk '{a+=$1; d+=$2} END {printf "insertions=%d deletions=%d total=%d\n", a, d, a+d}'; }

# Untracked files are invisible to git diff; count them so an uncommitted new
# test file is still charged to the budget.
untracked=$(git ls-files --others --exclude-standard -- . | grep -vE 'fakes/|^packages/types/|^vendor/' || true)
untracked_lines=0
if [ -n "${untracked}" ]; then
  untracked_lines=$(echo "${untracked}" | xargs -r cat | wc -l | tr -d ' ')
  echo "== untracked files (counted as insertions): ${untracked_lines} lines"
  echo "${untracked}" | sed 's/^/  /'
fi

echo "== diff budget vs ${base} (floor ${floor} total, cap ${budget} production; fakes, packages/types and vendor excluded)"
git diff --numstat "${base}" -- . "${excl[@]}" | sum
total=$(git diff --numstat "${base}" -- . "${excl[@]}" | awk -v u="${untracked_lines}" '{t+=$1+$2} END {print t+u}')
echo "total including untracked: ${total}"

echo "== of which production Go (non-test; hard cap ${budget})"
git diff --numstat "${base}" -- '*.go' ':(exclude)*_test.go' "${excl[@]}" | sum
untracked_prod_lines=0
if [ -n "${untracked}" ]; then
  untracked_prod=$(echo "${untracked}" | grep -E '\.go$' | grep -vE '_test\.go$' || true)
  [ -n "${untracked_prod}" ] && untracked_prod_lines=$(echo "${untracked_prod}" | xargs -r cat | wc -l | tr -d ' ')
fi
prod_total=$(git diff --numstat "${base}" -- '*.go' ':(exclude)*_test.go' "${excl[@]}" | awk -v u="${untracked_prod_lines}" '{t+=$1+$2} END {print t+u}')
echo "production total including untracked: ${prod_total}"

echo "== of which Go tests"
git diff --numstat "${base}" -- '*_test.go' "${excl[@]}" | sum

echo "== files changed"
git diff --name-only "${base}" -- . "${excl[@]}" | sed 's/^/  /'

echo "== exported Go declarations removed or changed (justify each in the PR, or revert)"
removed=$(git diff -U0 "${base}" -- '*.go' ':(exclude)*_test.go' "${excl[@]}" \
  | grep -E '^-(func|type|var|const) [A-Z]|^-func \([^)]*\) [A-Z]' || true)
if [ -n "${removed}" ]; then echo "${removed}" | sed 's/^/  /'; else echo "  none"; fi

echo "== forbidden paths touched (must be empty)"
forbidden=$(git diff --name-only "${base}" -- db/migrations packages/types internal/nutrition/store/food.go cmd/spotter/main.go '**/*.sql' || true)
if [ -n "${forbidden}" ]; then echo "${forbidden}" | sed 's/^/  /'; else echo "  none"; fi

echo "== changing edits since ${base} (each fix( commit must follow its RED test commit)"
git log --format='  %h %s' "${base}..HEAD" 2>/dev/null | grep -E '^  [0-9a-f]+ (fix|test)(\(|:)' || echo "  none"

status=0
if [ "${prod_total}" -gt "${budget}" ]; then
  echo "BUDGET EXCEEDED: ${prod_total} > ${budget} production lines. Revert the last production step until under budget; never trim specs." >&2
  status=1
fi
if [ "${total}" -gt "${total_warn}" ]; then
  echo "WARN: ${total} total changed lines > ${total_warn}; the PR is large to review. Specs are not the reason to cut scope." >&2
fi
if [ "${total}" -lt "${floor}" ]; then
  echo "BELOW FLOOR: ${total} < ${floor} changed lines. Add another pinned target from the same or an adjacent package; do not pad. Set GO_SWEEP_FLOOR=0 only for a findings-only PR." >&2
  status=1
fi
exit "${status}"
