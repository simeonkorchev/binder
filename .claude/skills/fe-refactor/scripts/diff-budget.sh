#!/usr/bin/env bash
# Prints the evidence block the /fe-refactor PR body needs and fails once the
# diff passes the budget. Frontend sibling of
# .claude/skills/go-quality-sweep/scripts/diff-budget.sh.
#
# Budget model: the floor (500) counts every changed line so a PR is worth a
# review cycle; the hard cap (2000) counts PRODUCTION lines only, so pin tests
# are never the reason a routine drops scope or trims a test. A total past
# 3000 only warns (reviewability hint).
#
# Sections: diff budget vs the base branch (generated types/clients and the
# lockfile excluded), production vs test split, files changed, scope check (one
# app only, findings allowed), exported TS declarations the diff removes or
# changes (justify each as a pure rename or dead code), forbidden paths, test
# skip/only markers, and review hotspots (production lines touching query keys,
# effect deps, styles) a reviewer should eyeball.
#
# Usage: .claude/skills/fe-refactor/scripts/diff-budget.sh [base-ref]
set -euo pipefail

base="${1:-origin/main}"
budget="${FE_REFACTOR_BUDGET:-2000}"            # hard cap on production lines
floor="${FE_REFACTOR_FLOOR:-500}"               # floor on total lines (tests included)
total_warn="${FE_REFACTOR_TOTAL_WARN:-3000}"     # reviewability warning on total lines
prod_budget="${FE_REFACTOR_PROD_BUDGET:-1000}"   # warning threshold on production lines
prod_files_budget="${FE_REFACTOR_PROD_FILES:-15}"

excl=(':(exclude)packages/types/**' ':(exclude)packages/api/src/generated/**' ':(exclude)package-lock.json')
is_test='(\.test\.tsx?$|/__tests__/|/src/test/)'
apps_re='^apps/(trainer-web|client-mobile)/'

sum() { awk '{a+=$1; d+=$2} END {printf "insertions=%d deletions=%d total=%d\n", a, d, a+d}'; }
numstat() { git diff --numstat "${base}" -- . "${excl[@]}"; }
names() { git diff --name-only "${base}" -- . "${excl[@]}"; }

# Untracked files are invisible to git diff; charge them to the budget so an
# uncommitted new test file still counts.
untracked=$(git ls-files --others --exclude-standard -- . | grep -vE '^packages/types/|^packages/api/src/generated/|^package-lock\.json$' || true)
untracked_lines=0
if [ -n "${untracked}" ]; then
  untracked_lines=$(echo "${untracked}" | xargs -r cat | wc -l | tr -d ' ')
  echo "== untracked files (counted as insertions): ${untracked_lines} lines"
  echo "${untracked}" | sed 's/^/  /'
fi

echo "== diff budget vs ${base} (floor ${floor} total, cap ${budget} production; generated types/clients and lockfile excluded)"
numstat | sum
total=$(numstat | awk -v u="${untracked_lines}" '{t+=$1+$2} END {print t+u}')
echo "total including untracked: ${total}"

echo "== of which production code (apps/*/src, non-test; hard cap ${budget} lines, warn past ${prod_budget} lines / ${prod_files_budget} files)"
prod_stat=$(numstat | awk -v re="${apps_re}src/" '$3 ~ re' | grep -vE "${is_test}" || true)
echo "${prod_stat}" | sum
untracked_prod_lines=0
if [ -n "${untracked}" ]; then
  untracked_prod=$(echo "${untracked}" | grep -E "${apps_re}src/" | grep -vE "${is_test}" || true)
  [ -n "${untracked_prod}" ] && untracked_prod_lines=$(echo "${untracked_prod}" | xargs -r cat | wc -l | tr -d ' ')
fi
prod_total=$(echo "${prod_stat}" | awk -v u="${untracked_prod_lines}" '{t+=$1+$2} END {print t+u}')
prod_files=$(echo "${prod_stat}" | grep -c . || true)
echo "production files touched: ${prod_files}"

echo "== of which tests"
{ numstat | grep -E "${is_test}" || true; } | sum

echo "== files changed"
names | sed 's/^/  /'

echo "== scope: apps touched (must be exactly one)"
apps=$( { names; echo "${untracked}"; } | grep -oE "${apps_re}" | sort -u | sed 's#^apps/##; s#/$##' || true)
if [ -n "${apps}" ]; then echo "${apps}" | sed 's/^/  /'; else echo "  none"; fi

echo "== scope: files outside apps/<app>/, packages/intl/locales/ (additive only), .ai/findings/open/ and .claude/memory/inbox/ (must be empty)"
outside=$( { names; echo "${untracked}"; } | grep -v '^$' | grep -vE "${apps_re}|^\.ai/findings/open/|^\.claude/memory/inbox/|^packages/intl/locales/" || true)
if [ -n "${outside}" ]; then echo "${outside}" | sed 's/^/  /'; else echo "  none"; fi

echo "== exported TS declarations removed or changed in production code (justify each in the PR, or revert)"
removed=$(git diff -U0 "${base}" -- 'apps/*/src/**/*.ts' 'apps/*/src/**/*.tsx' "${excl[@]}" \
  | grep -E '^-\s*export\s+(default\b|const\b|let\b|function\b|async function\b|class\b|interface\b|type\b|enum\b|\{)' || true)
if [ -n "${removed}" ]; then echo "${removed}" | sed 's/^/  /'; else echo "  none"; fi

echo "== forbidden paths touched (must be empty)"
forbidden=$( { git diff --name-only "${base}" -- \
    packages/types packages/api/src/generated db \
    'apps/client-mobile/metro.config.cjs' 'apps/*/src/navigation/AppNavigator.tsx' \
    '**/package.json' package-lock.json '**/.env*' ; echo "${untracked}" | grep -E '^(packages/(types|api/src/generated)/|db/)' ; } | grep -v '^$' || true)
if [ -n "${forbidden}" ]; then echo "${forbidden}" | sed 's/^/  /'; else echo "  none"; fi

echo "== locale edits (changing edits may only ADD keys; any removed line must be justified or reverted)"
locale_removed=$(git diff -U0 "${base}" -- 'packages/intl/locales/*.json' | grep -E '^-[^-]' | grep -vE '^-\s*[\]}],?\s*$' || true)
if [ -n "${locale_removed}" ]; then echo "${locale_removed}" | sed 's/^/  /'; else echo "  none"; fi

echo "== test skip/only markers added (must be empty)"
skips=$(git diff -U0 "${base}" -- 'apps/*/src/**' "${excl[@]}" \
  | grep -E '^\+.*\b((it|test|describe)\.(skip|only)|xit|xdescribe|xtest)\s*\(' || true)
if [ -n "${skips}" ]; then echo "${skips}" | sed 's/^/  /'; else echo "  none"; fi

echo "== review hotspots: production lines touching query keys, enabled/staleTime, effect deps, styles, i18n keys"
echo "   (pure moves - the same line removed and added elsewhere, whitespace ignored - are filtered out)"
hot=$(git diff -U0 "${base}" -- 'apps/*/src/**/*.ts' 'apps/*/src/**/*.tsx' "${excl[@]}" \
  | grep -vE "^(\+\+\+|---) " \
  | grep -E '^[-+].*(queryKey|invalidateQueries|enabled:|staleTime|useEffect\(|\], \[|StyleSheet\.create|style=\{|t\(\x27|t\("|var\(--)' \
  | awk '{ sign=substr($0,1,1); body=substr($0,2); gsub(/^[ \t]+|[ \t]+$/, "", body);
           if (sign=="-") { rm[body]++; rl[++nr]=body } else { ad[body]++; al[++na]=body } }
         END { for (i=1;i<=nr;i++) if (!(rl[i] in ad)) print "-" rl[i];
               for (i=1;i<=na;i++) if (!(al[i] in rm)) print "+" al[i] }' || true)
if [ -n "${hot}" ]; then echo "${hot}" | head -40 | sed 's/^/  /'; else echo "  none"; fi

echo "== changing edits since ${base} (each fix( commit must follow its RED test commit)"
git log --format='  %h %s' "${base}..HEAD" 2>/dev/null | grep -E '^  [0-9a-f]+ (fix|test)(\(|:)' || echo "  none"

status=0
if [ "${prod_total}" -gt "${prod_budget}" ]; then
  echo "WARN: production code ${prod_total} > ${prod_budget} changed lines; stop taking on files." >&2
fi
if [ "${prod_files}" -gt "${prod_files_budget}" ]; then
  echo "WARN: ${prod_files} production files > ${prod_files_budget}; the slice is too wide." >&2
fi
if [ "${prod_total}" -gt "${budget}" ]; then
  echo "BUDGET EXCEEDED: ${prod_total} > ${budget} production lines. Revert the last production step until under budget; never trim tests." >&2
  status=1
fi
if [ "${total}" -gt "${total_warn}" ]; then
  echo "WARN: ${total} total changed lines > ${total_warn}; the PR is large to review. Tests are not the reason to cut scope." >&2
fi
if [ "${total}" -lt "${floor}" ]; then
  echo "BELOW FLOOR: ${total} < ${floor} changed lines. Add another pinned slice from the same app; do not pad. Set FE_REFACTOR_FLOOR=0 only for a findings-only PR." >&2
  status=1
fi
exit "${status}"
