#!/usr/bin/env bash
# Prints the first-party Go packages a change touches, as `go test` patterns.
#
# Used by `make test-changed` / `make lint-changed` — the inner loop. It prints
# the packages whose files changed AND every first-party package that imports
# one of them, because a change is most likely to break its callers, not itself.
# It is deliberately not a substitute for the gate: a reflective or generated
# edge it cannot see is caught by `make check` / `make check-changed`, which
# every task still ends with.
#
# Usage: tools/changed-go-pkgs.sh [BASE]      (default: merge-base with origin/main)
# Prints nothing when no Go file changed — callers must treat empty as "skip".
set -uo pipefail
cd "$(dirname "$0")/.."

base="${1:-}"
if [ -z "$base" ]; then
  base="$(git merge-base HEAD origin/main 2>/dev/null || git rev-parse HEAD)"
fi

mod="$(go list -m 2>/dev/null)"
[ -n "$mod" ] || exit 0

# Committed since base, plus staged, plus unstaged, plus untracked — a new
# package is untracked at exactly the moment its author wants to lint it. A
# deleted file's directory may be gone, so every candidate is checked for
# existence below.
changed_dirs="$(
  { git diff --name-only "$base" -- '*.go' 2>/dev/null
    git diff --name-only -- '*.go' 2>/dev/null
    git diff --cached --name-only -- '*.go' 2>/dev/null
    git ls-files --others --exclude-standard -- '*.go' 2>/dev/null
  } | grep -v '^vendor/' | xargs -r -n1 dirname | sort -u
)"
[ -n "$changed_dirs" ] || exit 0

changed_pkgs=""
for d in $changed_dirs; do
  [ -d "$d" ] || continue
  case "$d" in internal/*|pkg/*|cmd/*|tests/*) changed_pkgs="$changed_pkgs $mod/$d" ;; esac
done
changed_pkgs="$(echo "$changed_pkgs" | tr ' ' '\n' | sed '/^$/d' | sort -u)"
[ -n "$changed_pkgs" ] || exit 0

# Importers: one `go list` over the first-party tree, matched against the set.
importers="$(
  go list -f '{{.ImportPath}}{{range .Deps}} {{.}}{{end}}' ./internal/... ./pkg/... ./cmd/... 2>/dev/null \
  | awk -v set="$(echo "$changed_pkgs" | tr '\n' ' ')" '
      BEGIN { n=split(set, a, " "); for (i=1;i<=n;i++) if (a[i] != "") want[a[i]]=1 }
      { for (i=2;i<=NF;i++) if ($i in want) { print $1; break } }'
)"

printf '%s\n%s\n' "$changed_pkgs" "$importers" | sed '/^$/d' | sort -u | sed "s|^$mod/|./|"
