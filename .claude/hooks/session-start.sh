#!/usr/bin/env bash
# SessionStart hook. Three jobs, none of which may fail the session:
#
# 1. Point Claude Code's auto memory for this repo at the git-tracked
#    .claude/memory/. Auto memory lives machine-locally under
#    ~/.claude/projects/<repo>/memory/, which a cloud Routine (fresh VM) never
#    sees; linking it here makes every session read and write the same memory
#    and lets a Routine's learnings ride along in its PR.
# 2. Regenerate the derived memory (layout, commands) from the tree.
# 3. Print the index and the live state into the session's context.
set -u

root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
mem="$root/.claude/memory"
[ -d "$mem" ] || exit 0

# ── 1. link auto memory ──────────────────────────────────────────────────────
if [ -z "${CLAUDE_CODE_DISABLE_AUTO_MEMORY:-}" ]; then
  cfg="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
  # Claude Code names the per-project dir after the repo path with every
  # non-alphanumeric character replaced by "-": /home/user/binder →
  # -home-user-binder. If the convention ever changes this is a no-op and
  # memory simply stays machine-local.
  slug="$(printf '%s' "$root" | sed 's/[^A-Za-z0-9]/-/g')"
  proj="$cfg/projects/$slug"
  mkdir -p "$proj" 2>/dev/null
  if [ -L "$proj/memory" ]; then
    :  # already linked
  elif [ -e "$proj/memory" ]; then
    echo "memory: $proj/memory exists as a real directory; it is machine-local." \
         "Move its topic files into .claude/memory/ and delete it to share memory through git."
  else
    ln -s "$mem" "$proj/memory" 2>/dev/null && echo "memory: linked $proj/memory → .claude/memory"
  fi
fi

# ── 2. derived memory ────────────────────────────────────────────────────────
if [ -f "$root/tools/memory-gen.sh" ]; then
  bash "$root/tools/memory-gen.sh" > /dev/null 2>&1 || echo "memory: tools/memory-gen.sh failed; run it by hand"
fi

# ── 3. context ───────────────────────────────────────────────────────────────
# The index is a grep target, not a context load. Pasting it cost ~17 KB
# (~4.4k tokens) of every session to deliver the two lines a task actually
# needed; MEMORY.md's own header says to grep it by symptom. Print the map and
# the live state, and let the session pull the unit it needs.
echo
echo "## Memory (git-tracked at .claude/memory/; .claude/rules/ win on any conflict)"
echo "Before deriving anything — an error string, a flag, a command, a gotcha —"
echo "grep the index by the symptom in front of you, then read the unit it names:"
echo
echo "    grep -i '<symptom>' .claude/memory/MEMORY.md     # -> topic.md#slug"
echo
echo "Writing memory is a row of the 007 checklist: what the next session would"
echo "re-derive goes in before done (.claude/memory/README.md says where)."
echo "Topic files ($(grep -cE '→ [A-Za-z0-9_./-]+\.md' "$mem/MEMORY.md") indexed units):"
for f in "$mem"/*.md; do
  b=$(basename "$f")
  case "$b" in MEMORY.md|README.md) continue ;; esac
  printf '  %-18s %s units\n' "$b" "$(grep -cE '^### ' "$f" 2>/dev/null || echo 0)"
done
echo "  generated/       layout + commands, regenerated each session"
echo
echo "## Live state"
echo "- branch: $(git -C "$root" branch --show-current 2>/dev/null || echo '?')"
echo "- open findings: $(ls "$root"/.ai/findings/open/*.md 2>/dev/null | wc -l | tr -d ' ') (.ai/findings/open/)"
inbox=$(ls "$mem"/inbox/*.md 2>/dev/null | wc -l | tr -d ' ')
echo "- memory inbox entries awaiting /memory-dream: $inbox"
exit 0
