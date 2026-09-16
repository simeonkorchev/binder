#!/usr/bin/env python3
"""Fetch and cluster Render error logs for the /render-triage routine.

Subcommands
  fetch    Pull the last N hours of logs for one Render service via the REST API
           (needs RENDER_API_KEY; RENDER_SERVICE_NAME defaults to shape-u-prod-be)
           and write them as JSON lines. Use this when the Render MCP server is
           not attached to the session; with MCP, save list_logs output to the
           same JSONL shape instead ({"timestamp":..., "message":...} per line).
  cluster  Read that JSONL (or raw log lines), keep error-class entries, mask
           volatile tokens, group by signature, cross-check .ai/triage/ledger.md
           and .ai/triage/noise.md, and print a markdown report plus a JSON
           summary (--json) for the ledger.

The script never prints the API key and redacts emails, bearer tokens and
long hex/uuid values from every sample line it emits.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import sys
import urllib.parse
import urllib.request
from collections import defaultdict
from datetime import datetime, timedelta, timezone

API = "https://api.render.com/v1"
ERROR_SEVERITIES = {"ERROR", "CRITICAL", "ALERT", "EMERGENCY", "FATAL", "PANIC"}
PANIC_MARKERS = ("[PANIC RECOVER]", "panic:", "runtime error:", "fatal error:", "http server failed")

MASKS = [
    (re.compile(r"[\w.+-]+@[\w-]+\.[\w.-]+"), "<email>"),
    (re.compile(r"(?i)bearer\s+[A-Za-z0-9._\-]+"), "bearer <token>"),
    (re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"), "<uuid>"),
    (re.compile(r"\b[0-9a-fA-F]{16,}\b"), "<hex>"),
    (re.compile(r"\b\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?"), "<ts>"),
    (re.compile(r"\b\d+(\.\d+)?(ms|s|µs|us|ns)\b"), "<dur>"),
    (re.compile(r"\b\d+\b"), "<n>"),
]


def mask(text: str) -> str:
    for rx, rep in MASKS:
        text = rx.sub(rep, text)
    return text


# ---------------------------------------------------------------- fetch ----

def api_get(path: str, key: str, params: dict) -> dict | list:
    qs = urllib.parse.urlencode({k: v for k, v in params.items() if v is not None}, doseq=True)
    req = urllib.request.Request(f"{API}{path}?{qs}", headers={
        "Authorization": f"Bearer {key}", "Accept": "application/json"})
    with urllib.request.urlopen(req, timeout=60) as resp:  # noqa: S310 - fixed https host
        return json.load(resp)


def fetch(args: argparse.Namespace) -> int:
    key = os.environ.get("RENDER_API_KEY")
    if not key:
        print("RENDER_API_KEY is not set; cannot fetch. Attach the Render MCP connector "
              "or configure the key in the environment.", file=sys.stderr)
        return 2
    services = api_get("/services", key, {"name": args.service, "limit": 5})
    match = [s.get("service", s) for s in services if s.get("service", s).get("name") == args.service]
    if not match:
        print(f"no Render service named {args.service!r} visible to this key", file=sys.stderr)
        return 2
    svc = match[0]
    end = datetime.now(timezone.utc)
    start = end - timedelta(hours=args.hours)
    params = {
        "ownerId": svc["ownerId"], "resource": svc["id"],
        "startTime": start.isoformat().replace("+00:00", "Z"),
        "endTime": end.isoformat().replace("+00:00", "Z"),
        "limit": 100, "direction": "backward",
    }
    if args.level:
        params["level"] = args.level
    total = 0
    with open(args.out, "w", encoding="utf-8") as fh:
        while True:
            page = api_get("/logs", key, params)
            for entry in page.get("logs", []):
                fh.write(json.dumps(entry) + "\n")
                total += 1
            if not page.get("hasMore") or total >= args.max_entries:
                break
            params["startTime"] = page.get("nextStartTime", params["startTime"])
            params["endTime"] = page.get("nextEndTime", params["endTime"])
    print(f"service={svc['id']} window={params['startTime']}..{end.isoformat()} entries={total} -> {args.out}")
    return 0


# -------------------------------------------------------------- cluster ----

def parse_entry(raw: str) -> dict | None:
    raw = raw.strip()
    if not raw:
        return None
    ts, message, labels = None, raw, {}
    try:
        outer = json.loads(raw)
        if isinstance(outer, dict) and "message" in outer:
            ts = outer.get("timestamp")
            message = str(outer.get("message", ""))
            labels = {l.get("name"): l.get("value") for l in outer.get("labels", []) if isinstance(l, dict)}
        elif isinstance(outer, dict):
            message = raw
    except json.JSONDecodeError:
        pass
    inner = None
    try:
        candidate = json.loads(message)
        if isinstance(candidate, dict):
            inner = candidate
    except json.JSONDecodeError:
        pass
    if inner is not None:
        severity = str(inner.get("severity") or inner.get("level") or "").upper()
        msg = str(inner.get("msg") or inner.get("message") or "")
        err = str(inner.get("error") or "")
        ts = ts or inner.get("time")
    else:
        severity = str(labels.get("level") or "").upper()
        msg, err = message, ""
    is_panic = any(marker in message for marker in PANIC_MARKERS)
    is_error = severity in ERROR_SEVERITIES or is_panic
    return {"ts": ts, "severity": severity or ("PANIC" if is_panic else ""), "msg": msg,
            "error": err, "raw": message, "is_error": is_error, "is_panic": is_panic}


def first_line(text: str) -> str:
    """pgx and net errors concatenate one line per attempt; only the first line is stable."""
    return " ".join(text.split("\n", 1)[0].split())


def signature_of(entry: dict) -> tuple[str, str]:
    text = mask(f"{first_line(entry['msg'])} | {first_line(entry['error'])}".strip(" |"))
    return hashlib.sha1(text.encode()).hexdigest()[:10], text


def load_ledger(path: str) -> dict[str, str]:
    known: dict[str, str] = {}
    if not os.path.exists(path):
        return known
    with open(path, encoding="utf-8") as fh:
        rows = fh.readlines()
    for line in rows:
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) < 5:
            continue
        # Ledger rows render the signature as inline code, matching the report
        # this script prints; the id itself is the backtick-free content.
        sig = cells[0].strip("`")
        if re.fullmatch(r"[0-9a-f]{10}", sig):
            known[sig] = cells[4]
    return known


def load_noise(path: str) -> list[re.Pattern]:
    patterns = []
    if not os.path.exists(path):
        return patterns
    for line in open(path, encoding="utf-8"):
        line = line.strip()
        if line.startswith("- `") and line.endswith("`"):
            patterns.append(re.compile(line[3:-1]))
    return patterns


def cluster(args: argparse.Namespace) -> int:
    ledger = load_ledger(args.ledger)
    noise = load_noise(args.noise)
    groups: dict[str, dict] = {}
    total = errors = 0
    for raw in open(args.input, encoding="utf-8"):
        entry = parse_entry(raw)
        if not entry:
            continue
        total += 1
        if not entry["is_error"]:
            continue
        errors += 1
        sig, text = signature_of(entry)
        g = groups.setdefault(sig, {"signature": sig, "text": text, "count": 0, "first": None, "last": None,
                                    "panic": False, "sample": mask(entry["raw"])[:400]})
        g["count"] += 1
        g["panic"] = g["panic"] or entry["is_panic"]
        ts = entry["ts"] or ""
        g["first"] = min(filter(None, [g["first"], ts])) if (g["first"] or ts) else None
        g["last"] = max(filter(None, [g["last"], ts])) if (g["last"] or ts) else None
    for g in groups.values():
        if any(rx.search(g["text"]) for rx in noise):
            g["status"] = "noise"
        elif g["signature"] in ledger:
            g["status"] = f"known:{ledger[g['signature']]}"
        else:
            g["status"] = "NEW"
    ranked = sorted(groups.values(), key=lambda g: (g["status"] != "NEW", not g["panic"], -g["count"]))
    if args.json:
        print(json.dumps({"entries": total, "error_entries": errors, "clusters": ranked}, indent=2))
        return 0
    print(f"entries={total} error_entries={errors} clusters={len(ranked)} "
          f"new={sum(g['status']=='NEW' for g in ranked)}")
    print()
    print("| status | sig | count | panic | first | last | signature text |")
    print("|---|---|---|---|---|---|---|")
    for g in ranked:
        print(f"| {g['status']} | `{g['signature']}` | {g['count']} | {'yes' if g['panic'] else ''} | "
              f"{g['first'] or ''} | {g['last'] or ''} | {g['text'][:120]} |")
    print()
    for g in ranked:
        if g["status"] == "NEW":
            print(f"### {g['signature']}  (count {g['count']}{', PANIC' if g['panic'] else ''})")
            print(f"    {g['sample']}")
    return 0


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    f = sub.add_parser("fetch")
    f.add_argument("--service", default=os.environ.get("RENDER_SERVICE_NAME", "shape-u-prod-be"))
    f.add_argument("--hours", type=int, default=24)
    f.add_argument("--level", default=None, help="Render-side level filter, e.g. error; omitted = all levels")
    f.add_argument("--max-entries", type=int, default=5000)
    f.add_argument("--out", default="render-logs.jsonl")
    f.set_defaults(fn=fetch)
    c = sub.add_parser("cluster")
    c.add_argument("input")
    c.add_argument("--ledger", default=".ai/triage/ledger.md")
    c.add_argument("--noise", default=".ai/triage/noise.md")
    c.add_argument("--json", action="store_true")
    c.set_defaults(fn=cluster)
    args = ap.parse_args()
    return args.fn(args)


if __name__ == "__main__":
    sys.exit(main())
