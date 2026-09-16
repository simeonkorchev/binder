#!/usr/bin/env bash
# Fails when the local gate and the CI workflow drift apart.
#
# Two classifiers that disagree, or a CI step with no local equivalent, is how
# "green on my machine" becomes a red PR and a wasted push/wait/fix round trip.
# This is the check that makes the parity self-enforcing rather than remembered.
set -uo pipefail
cd "$(dirname "$0")/.."

wf=.github/workflows/ci.yml
scopes=tools/changed-scopes.sh
fail=0

# 1. The change classifier must be byte-identical on both sides.
for re in go_re shared_re; do
  a=$(grep -oE "^ *$re='[^']*'" "$wf"     | head -1 | sed "s/^ *//")
  b=$(grep -oE "^ *$re='[^']*'" "$scopes" | head -1 | sed "s/^ *//")
  if [ -z "$a" ] || [ -z "$b" ]; then
    echo "ERROR: could not read $re from both $wf and $scopes"; fail=1
  elif [ "$a" != "$b" ]; then
    echo "ERROR: $re has drifted between the CI workflow and $scopes."
    echo "  CI     : $a"
    echo "  local  : $b"
    echo "  Fix both together — they are one rule with two readers."
    fail=1
  fi
done

# 2. Every scope the classifier can emit must have a CI job that consumes it,
#    and every job output the classifier feeds must be a scope it can emit.
#    Without this, adding a layer locally leaves CI silently not running it.
emitted=$(grep -oE '^\s*(matches "[^"]*"\s*&&\s*)?echo (go|mobile|packages)$' "$scopes" \
          | grep -oE '(go|mobile|packages)$' | sort -u)
consumed=$(grep -oE "needs\.changes\.outputs\.[a-z_]+" "$wf" | cut -d. -f4 | sort -u \
           | sed 's/^backend$/go/')
if [ "$emitted" != "$consumed" ]; then
  echo "ERROR: the scopes $scopes emits and the scopes $wf gates on have drifted."
  echo "  emitted by the classifier : $(echo "$emitted" | tr '\n' ' ')"
  echo "  gated on in CI            : $(echo "$consumed" | tr '\n' ' ')"
  fail=1
fi

# 3. Every command CI runs must have a local equivalent. The map is the record
#    of that decision; a CI step that is not in it fails this check loudly
#    rather than silently becoming a red PR nobody could reproduce locally.
known="
npm ci|bootstrap
npm run lint|lint-mobile
npm run typecheck|typecheck-mobile
npm run test:run|test-mobile
npm run knip|dead-code-mobile
make install-go-tools|install-go-tools (called by bootstrap)
make lint|lint
make migrate-test|migrate-test
make test|test
make gate-packages|gate-packages
"
while read -r cmd; do
  [ -n "$cmd" ] || continue
  case "$cmd" in 'echo '*) continue ;; esac
  echo "$known" | grep -qF "$cmd|" || {
    echo "ERROR: CI runs a step the local gate has no recorded equivalent for:"
    echo "  $cmd"
    echo "  Add it to a make target, then record it in the map in $0."
    fail=1
  }
done < <(grep -hoE '^\s+(- )?run: [^|].*' "$wf" | sed -E 's/^\s+(- )?run: //' | sort -u)

if [ "$fail" -eq 0 ]; then
  echo "check-ci-parity: classifier identical on both sides; every scope is gated in CI; every CI step has a local equivalent."
fi
exit "$fail"
