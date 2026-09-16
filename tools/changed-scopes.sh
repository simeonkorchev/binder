#!/usr/bin/env bash
# Prints which layers a change affects: go, mobile, packages.
#
# The regexes below are COPIED VERBATIM from .github/workflows/ci.yml (the
# `changes` job). Two classifiers that disagree is how local green becomes CI
# red, so tools/check-ci-parity.sh fails the gate if they ever drift — fix them
# together, never one alone.
#
# Usage: tools/changed-scopes.sh [BASE]   (default: merge-base with origin/main)
# Prints nothing when nothing relevant changed.
set -uo pipefail
cd "$(dirname "$0")/.."

base="${1:-}"
if [ -z "$base" ]; then
  base="$(git merge-base HEAD origin/main 2>/dev/null || git rev-parse HEAD)"
fi

# Untracked files count: a new file is untracked at exactly the moment its
# author wants the gate to cover it.
changed="$(
  { git diff --name-only "$base" 2>/dev/null
    git diff --name-only 2>/dev/null
    git diff --cached --name-only 2>/dev/null
    git ls-files --others --exclude-standard 2>/dev/null
  } | sort -u
)"
[ -n "$changed" ] || exit 0

# --- BEGIN ci-parity block (keep byte-identical to ci.yml) ---
go_re='(^|/)[^/]+\.go$|^go\.(mod|sum)$|^db/|^vendor/|^tools/|^tests/|^\.golangci(\.yml|-version)$|^Makefile$'
shared_re='^packages/|^package\.json$|^package-lock\.json$'
# --- END ci-parity block ---

matches() { echo "$changed" | grep -Eq "$1"; }

# Editing the workflow re-runs everything in CI; mirror that here.
if matches '^\.github/workflows/ci\.yml$'; then
  printf 'go\nmobile\npackages\n'; exit 0
fi

matches "$go_re"                       && echo go
matches "^apps/mobile/|$shared_re"     && echo mobile
matches "$shared_re"                   && echo packages
exit 0
