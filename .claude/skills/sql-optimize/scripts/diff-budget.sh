#!/usr/bin/env bash
# Prints the evidence block the /sql-optimize PR body needs and fails once the
# diff passes the budget. Store-SQL sibling of
# .claude/skills/go-quality-sweep/scripts/diff-budget.sh.
#
# Sections: diff budget vs the base branch, production vs test split, files
# changed, scope check (exactly one internal/<domain>/store package; findings
# allowed), exported Go declarations the diff removes or changes (justify each
# as a pure rename or dead code), forbidden paths (migrations, *.sql, generated
# types, anything above the store, the two always-excluded stores), Ginkgo
# skip/pending markers, and SQL hotspots: changed production lines that touch
# ordering, limits, locking, join shape, NULL semantics or isolation, which a
# reviewer must eyeball because they are where "same rows" silently breaks.
#
# Usage: .claude/skills/sql-optimize/scripts/diff-budget.sh [base-ref]
set -euo pipefail

base="${1:-origin/main}"
budget="${SQL_OPTIMIZE_BUDGET:-1000}"
prod_files_budget="${SQL_OPTIMIZE_PROD_FILES:-4}"

excl=(':(exclude)**/*fakes/**' ':(exclude)packages/types/**' ':(exclude)vendor/**')
store_re='^internal/[^/]+/store/'

sum() { awk '{a+=$1; d+=$2} END {printf "insertions=%d deletions=%d total=%d\n", a, d, a+d}'; }
numstat() { git diff --numstat "${base}" -- . "${excl[@]}"; }
names() { git diff --name-only "${base}" -- . "${excl[@]}"; }

# Untracked files are invisible to git diff; charge them to the budget so an
# uncommitted new spec file still counts.
untracked=$(git ls-files --others --exclude-standard -- . | grep -vE 'fakes/|^packages/types/|^vendor/' || true)
untracked_lines=0
if [ -n "${untracked}" ]; then
  untracked_lines=$(echo "${untracked}" | xargs -r cat | wc -l | tr -d ' ')
  echo "== untracked files (counted as insertions): ${untracked_lines} lines"
  echo "${untracked}" | sed 's/^/  /'
fi

echo "== diff budget vs ${base} (budget ${budget}; fakes, packages/types and vendor excluded)"
numstat | sum
total=$(numstat | awk -v u="${untracked_lines}" '{t+=$1+$2} END {print t+u}')
echo "total including untracked: ${total}"

echo "== of which production Go (non-test; cap ${prod_files_budget} files)"
prod_stat=$(git diff --numstat "${base}" -- '*.go' ':(exclude)*_test.go' "${excl[@]}" || true)
echo "${prod_stat}" | sum
prod_files=$(echo "${prod_stat}" | grep -c . || true)
echo "production files touched: ${prod_files}"

echo "== of which Go tests"
git diff --numstat "${base}" -- '*_test.go' "${excl[@]}" | sum

echo "== files changed"
names | sed 's/^/  /'

echo "== scope: store packages touched (must be exactly one)"
pkgs=$( { names; echo "${untracked}"; } | grep -oE "${store_re}" | sort -u || true)
if [ -n "${pkgs}" ]; then echo "${pkgs}" | sed 's/^/  /'; else echo "  none"; fi

echo "== scope: files outside internal/<domain>/store/ and .ai/findings/open/ (must be empty)"
outside=$( { names; echo "${untracked}"; } | grep -v '^$' | grep -vE "${store_re}|^\.ai/findings/open/" || true)
if [ -n "${outside}" ]; then echo "${outside}" | sed 's/^/  /'; else echo "  none"; fi

echo "== exported Go declarations removed or changed (justify each in the PR, or revert)"
removed=$(git diff -U0 "${base}" -- '*.go' ':(exclude)*_test.go' "${excl[@]}" \
  | grep -E '^-(func|type|var|const) [A-Z]|^-func \([^)]*\) [A-Z]' || true)
if [ -n "${removed}" ]; then echo "${removed}" | sed 's/^/  /'; else echo "  none"; fi

echo "== forbidden paths touched (must be empty)"
forbidden=$( { git diff --name-only "${base}" -- \
    db/migrations '**/*.sql' packages/types \
    internal/nutrition/store/food.go internal/llm/store \
    'internal/*/api' 'internal/*/service' 'internal/*/model' cmd '**/.env*' go.mod go.sum ; \
    echo "${untracked}" | grep -E '^(db/|packages/types/|internal/[^/]+/(api|service|model)/|internal/llm/store/)' ; } | grep -v '^$' || true)
if [ -n "${forbidden}" ]; then echo "${forbidden}" | sed 's/^/  /'; else echo "  none"; fi

echo "== Ginkgo skip/pending markers added (must be empty)"
skips=$(git diff -U0 "${base}" -- '*_test.go' "${excl[@]}" \
  | grep -E '^\+.*\b(Skip\(|XIt\(|PIt\(|XDescribe\(|PDescribe\(|XContext\(|PContext\(|XWhen\(|PWhen\()' || true)
if [ -n "${skips}" ]; then echo "${skips}" | sed 's/^/  /'; else echo "  none"; fi

echo "== SQL hotspots: production lines touching ordering, limits, locking, join shape, NULL semantics, isolation"
echo "   (pure moves - the same line removed and added elsewhere, whitespace ignored - are filtered out)"
hot=$(git diff -U0 "${base}" -- '*.go' ':(exclude)*_test.go' "${excl[@]}" \
  | grep -vE "^(\+\+\+|---) " \
  | grep -iE '^[-+].*(ORDER BY|LIMIT|OFFSET|FOR UPDATE|FOR SHARE|ON CONFLICT|DISTINCT|GROUP BY|HAVING|(LEFT|RIGHT|FULL|CROSS) JOIN|NOT IN|NOT EXISTS|IS (NOT )?NULL|COALESCE|NULLS (FIRST|LAST)|UNION|sql\.Level|Isolation|ErrNoRows|RETURNING)' \
  | awk '{ sign=substr($0,1,1); body=substr($0,2); gsub(/^[ \t]+|[ \t]+$/, "", body);
           if (sign=="-") { rm[body]++; rl[++nr]=body } else { ad[body]++; al[++na]=body } }
         END { for (i=1;i<=nr;i++) if (!(rl[i] in ad)) print "-" rl[i];
               for (i=1;i<=na;i++) if (!(al[i] in rm)) print "+" al[i] }' || true)
if [ -n "${hot}" ]; then echo "${hot}" | head -40 | sed 's/^/  /'; else echo "  none"; fi

status=0
if [ "${prod_files}" -gt "${prod_files_budget}" ]; then
  echo "WARN: ${prod_files} production files > ${prod_files_budget}; the slice is too wide." >&2
fi
if [ "${total}" -gt "${budget}" ]; then
  echo "BUDGET EXCEEDED: ${total} > ${budget} changed lines. Revert the last change until under budget." >&2
  status=1
fi
exit "${status}"
