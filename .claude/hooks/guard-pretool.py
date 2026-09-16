#!/usr/bin/env python3
"""
PreToolUse guard for Claude Code.
Exit 2 → DENY (block the tool call).
Exit 1 → WARN (log + allow through, shown to Claude as advisory).
Exit 0 → allow silently.
Reads JSON from stdin: {"tool": "...", "input": {...}}
"""
import json
import sys
import os
import re
from pathlib import Path
from datetime import datetime

ROOT = Path(__file__).resolve().parents[2]
WARN_LOG = ROOT / ".claude" / "guard-warnings.log"

# ---------------------------------------------------------------------------
# DENY rules — hard blocks
# ---------------------------------------------------------------------------
DENY_PATHS = [
    "packages/types/src/api.ts",  # generated from the Go source via `make gen-spec`
    "servicefakes/",              # counterfeiter output; regenerate with `go generate`
    "node_modules/",
    "dist/",
    ".gen.go",
]

DENY_CONTENT_PATTERNS = [
    (r"sk_live_[A-Za-z0-9]+", "hardcoded Stripe live key"),
    (r"(CLERK_SECRET_KEY|WORKOS_API_KEY|WORKOS_WEBHOOK_SECRET)\s*=\s*['\"][^'\"]+['\"]", "hardcoded auth secret"),
    (r"dangerouslySetInnerHTML", "dangerouslySetInnerHTML forbidden"),
]

# Protect existing migrations — new files OK, edits to existing files DENY
MIGRATIONS_DIR = "db/migrations/"

# ---------------------------------------------------------------------------
# WARN rules — logged but allowed
# ---------------------------------------------------------------------------
WARN_CONTENT_PATTERNS = [
    (r"fmt\.Sprintf\([^)]*SELECT|INSERT|UPDATE|DELETE", "possible SQL injection via fmt.Sprintf"),
    (r"log\.(Print|Printf|Println|Fatal|Fatalf)\([^)]*email|password|phone", "possible PII in log statement"),
    (r"panic\(", "panic() outside main.go or _test.go"),
]


def log_warn(tool: str, reason: str, path: str = ""):
    try:
        WARN_LOG.parent.mkdir(parents=True, exist_ok=True)
        with WARN_LOG.open("a") as f:
            ts = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
            f.write(f"{ts} WARN tool={tool} path={path!r} reason={reason!r}\n")
    except Exception:
        pass


def check_path_deny(path: str) -> str | None:
    """Return denial reason if path is forbidden, else None."""
    for forbidden in DENY_PATHS:
        if forbidden in path:
            return f"path is off-limits: {forbidden}"

    # Block edits to existing migration files (new files OK)
    if MIGRATIONS_DIR in path:
        full = ROOT / path
        if full.exists():
            return "existing migration file is immutable — create a new migration instead"

    return None


def check_content_deny(content: str, path: str = "") -> str | None:
    # Skip warn-only files
    for pattern, reason in DENY_CONTENT_PATTERNS:
        if re.search(pattern, content):
            return reason
    return None


def check_content_warn(content: str, path: str = "", tool: str = "") -> None:
    is_main_or_test = path.endswith("main.go") or path.endswith("_test.go")
    for pattern, reason in WARN_CONTENT_PATTERNS:
        if pattern.startswith("panic") and is_main_or_test:
            continue
        if re.search(pattern, content, re.IGNORECASE):
            log_warn(tool, reason, path)


def deny(reason: str):
    print(json.dumps({"decision": "deny", "reason": reason}))
    sys.exit(2)


def allow():
    sys.exit(0)


def main():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        allow()

    tool = payload.get("tool", "")
    inp = payload.get("input", {})

    # -----------------------------------------------------------------------
    # Bash tool — check for dangerous commands
    # -----------------------------------------------------------------------
    if tool == "Bash":
        cmd = inp.get("command", "")
        if re.search(r"rm\s+-rf\s+/", cmd):
            deny("rm -rf / is forbidden")
        if re.search(r"DROP\s+TABLE|DROP\s+DATABASE", cmd, re.IGNORECASE):
            deny("destructive SQL DDL in shell command")
        allow()

    # -----------------------------------------------------------------------
    # Write / Edit tools — check paths and content
    # -----------------------------------------------------------------------
    if tool in ("Write", "Edit"):
        path = inp.get("file_path", "")

        # Normalise to relative
        try:
            rel = str(Path(path).resolve().relative_to(ROOT))
        except ValueError:
            rel = path

        reason = check_path_deny(rel)
        if reason:
            deny(reason)

        content = inp.get("content", "") or inp.get("new_string", "")
        reason = check_content_deny(content, rel)
        if reason:
            deny(reason)

        check_content_warn(content, rel, tool)

    allow()


if __name__ == "__main__":
    main()
