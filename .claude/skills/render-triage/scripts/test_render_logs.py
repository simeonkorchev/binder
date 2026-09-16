"""Guards for render_logs.py's ledger dedupe.

Run with: python3 .claude/skills/render-triage/scripts/test_render_logs.py

The ledger is the only thing standing between this routine and re-triaging an
error somebody already classified, so the parser and the file format it reads
have to agree. They silently drifted once (rows write the signature in
backticks, the parser demanded bare hex), which made every known signature
report as NEW.
"""

import argparse
import importlib.util
import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stdout

_SCRIPTS = os.path.dirname(os.path.abspath(__file__))
_REPO = os.path.abspath(os.path.join(_SCRIPTS, "..", "..", "..", ".."))

_spec = importlib.util.spec_from_file_location("render_logs", os.path.join(_SCRIPTS, "render_logs.py"))
render_logs = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(render_logs)

LEDGER_ROW_FORMAT = """# Production error triage ledger

| signature | first seen (UTC) | last seen (UTC) | count | status | ref |
|---|---|---|---|---|---|
| `1770044daf` | 2026-09-02T18:33:18Z | 2026-09-02T18:33:22Z | 14 | needs-human | Postgres recovery |
| `d7fbef6c15` | 2026-09-02T13:01:12Z | 2026-09-03T08:20:00Z | 12 | decision | some-finding.md |
"""


class LoadLedgerTest(unittest.TestCase):
    def test_reads_signatures_written_in_backticks(self):
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False) as fh:
            fh.write(LEDGER_ROW_FORMAT)
            path = fh.name
        try:
            known = render_logs.load_ledger(path)
        finally:
            os.unlink(path)

        self.assertEqual({"1770044daf": "needs-human", "d7fbef6c15": "decision"}, known)

    def test_repo_ledger_parses(self):
        """The checked-in ledger must be readable by the parser that consumes it."""
        known = render_logs.load_ledger(os.path.join(_REPO, ".ai", "triage", "ledger.md"))
        self.assertTrue(known, "the repo ledger parsed to zero rows — parser and file format have drifted")
        self.assertTrue(all(len(sig) == 10 for sig in known), f"non-signature keys leaked in: {sorted(known)}")


class ClusterStatusTest(unittest.TestCase):
    def test_a_ledger_signature_is_reported_known_not_new(self):
        entry = {
            "time": "2026-09-04T05:28:05Z",
            "severity": "ERROR",
            "message": "http request to '/api/v1/foods' failed",
            "http_error": {"code": 500, "error": "getting foods: context canceled"},
        }
        line = json.dumps({"timestamp": "2026-09-04T05:28:05Z", "labels": [], "message": json.dumps(entry)})
        sig, _ = render_logs.signature_of(render_logs.parse_entry(line))

        with tempfile.TemporaryDirectory() as d:
            logs = os.path.join(d, "logs.jsonl")
            ledger = os.path.join(d, "ledger.md")
            noise = os.path.join(d, "noise.md")
            with open(logs, "w") as fh:
                fh.write(line + "\n")
            with open(ledger, "w") as fh:
                fh.write("| signature | first seen (UTC) | last seen (UTC) | count | status | ref |\n")
                fh.write("|---|---|---|---|---|---|\n")
                fh.write(f"| `{sig}` | 2026-09-03T00:00:00Z | 2026-09-03T00:00:00Z | 1 | decision | finding.md |\n")
            with open(noise, "w") as fh:
                fh.write("# none\n")

            out = io.StringIO()
            with redirect_stdout(out):
                render_logs.cluster(argparse.Namespace(input=logs, ledger=ledger, noise=noise, json=True))
            clusters = json.loads(out.getvalue())["clusters"]

        self.assertEqual(1, len(clusters))
        self.assertEqual("known:decision", clusters[0]["status"])


if __name__ == "__main__":
    unittest.main()
