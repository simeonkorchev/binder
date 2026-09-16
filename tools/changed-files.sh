#!/usr/bin/env bash
# Prints the files a change touches under a path prefix, newline-separated.
#
# Used by the `*-changed` mobile lanes. Committed since BASE, plus staged, plus
# unstaged; deleted files are dropped (a linter cannot read them). Prints
# nothing when nothing matched — callers must treat empty as "skip".
#
# Usage: tools/changed-files.sh <path-prefix> [ext-regex] [BASE]
#   tools/changed-files.sh apps/mobile '\.(ts|tsx)$'
set -uo pipefail
cd "$(dirname "$0")/.."

prefix="${1:?usage: changed-files.sh <path-prefix> [ext-regex] [BASE]}"
ext="${2:-.}"
base="${3:-}"
if [ -z "$base" ]; then
  base="$(git merge-base HEAD origin/main 2>/dev/null || git rev-parse HEAD)"
fi

# `git diff` only knows about tracked files, so a brand-new file would be
# invisible to every scoped lane until it is staged — which is exactly when a
# new feature's first package appears. ls-files --others closes that.
{ git diff --name-only "$base" 2>/dev/null
  git diff --name-only 2>/dev/null
  git diff --cached --name-only 2>/dev/null
  git ls-files --others --exclude-standard 2>/dev/null
} | sort -u \
  | grep -E "^$prefix/" \
  | grep -E "$ext" \
  | while read -r f; do [ -f "$f" ] && echo "$f"; done
