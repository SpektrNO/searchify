#!/usr/bin/env bash
# Record a shipped feature in docs/feature-completed.md and mark ✅ in feature-backlog.md.
#
# Usage:
#   ./scripts/record-feature-complete.sh phase2-local-keyword
#   ./scripts/record-feature-complete.sh phase2-local-keyword --issue 42 --note "via PR #7"
#   ./scripts/record-feature-complete.sh --dry-run phase2-local-keyword

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKLOG="$ROOT/docs/feature-backlog.md"
COMPLETED="$ROOT/docs/feature-completed.md"

DRY_RUN=false
FEATURE_ID=""
ISSUE=""
NOTE=""
DATE="$(date +%Y-%m-%d)"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true; shift ;;
    --issue) ISSUE="$2"; shift 2 ;;
    --note) NOTE="$2"; shift 2 ;;
    --date) DATE="$2"; shift 2 ;;
    -*) echo "unknown option: $1" >&2; exit 1 ;;
    *)
      if [[ -z "$FEATURE_ID" ]]; then
        FEATURE_ID="$1"
      else
        echo "unexpected argument: $1" >&2
        exit 1
      fi
      shift
      ;;
  esac
done

if [[ -z "$FEATURE_ID" ]]; then
  echo "usage: $0 <feature-id> [--issue N] [--note text] [--date YYYY-MM-DD] [--dry-run]" >&2
  exit 1
fi

export ROOT BACKLOG COMPLETED FEATURE_ID ISSUE NOTE DATE DRY_RUN
python3 <<'PY'
import os
import re
import sys
from pathlib import Path

backlog = Path(os.environ["BACKLOG"])
completed = Path(os.environ["COMPLETED"])
feature_id = os.environ["FEATURE_ID"]
issue = os.environ.get("ISSUE", "")
note = os.environ.get("NOTE", "")
date = os.environ["DATE"]
dry_run = os.environ.get("DRY_RUN", "false") == "true"

SECTION_HEADERS = {
    "phase1-": "## A. MCP foundation",
    "phase2-": "## B. Local keyword index",
    "phase3-": "## C. Hybrid local search",
    "phase4-": "## D. Web search integration",
    "phase5-": "## E. HTTP and hardening",
}

def section_for(fid: str) -> str:
    for prefix, header in SECTION_HEADERS.items():
        if fid.startswith(prefix):
            return header
    return "## Other"

def parse_backlog(text: str, fid: str) -> dict | None:
    row4 = re.compile(
        rf"^\|\s*`{re.escape(fid)}`\s*\|\s*(.+?)\s*\|\s*([✅🟡⬜])\s*\|\s*(.+?)\s*\|",
        re.MULTILINE,
    )
    m = row4.search(text)
    if m:
        return {"title": m.group(1).strip(), "status": m.group(2), "spec": m.group(3).strip()}
    return None

backlog_text = backlog.read_text(encoding="utf-8")
meta = parse_backlog(backlog_text, feature_id)
if not meta:
    print(f"error: feature id `{feature_id}` not found in {backlog}", file=sys.stderr)
    sys.exit(1)

if meta["status"] == "✅" and f"`{feature_id}`" in completed.read_text(encoding="utf-8"):
    print(f"skip: `{feature_id}` already marked complete in backlog and listed in feature-completed.md")
    sys.exit(0)

github = f"#{issue}" if issue else "—"
notes = note or "Completed via spec→implement pipeline"
section = section_for(feature_id)
completed_text = completed.read_text(encoding="utf-8")

if f"`{feature_id}`" not in completed_text:
    recent_row = f"| {date} | `{feature_id}` | {meta['title']} | {github} | {notes} |"
    marker = "| _—_ | _pipeline completions append here (newest first)_ | | | |"
    if marker in completed_text:
        completed_text = completed_text.replace(marker, f"{recent_row}\n{marker}", 1)
    section_row = f"| `{feature_id}` | {meta['title']} | {date} | {meta['spec']} | {notes} |"
    if section in completed_text:
        pattern = re.compile(
            re.escape(section) + r"\n\n\| ID \| Feature \| Completed \| Spec \| Notes \|\n\|[-| ]+\|\n"
            r"(.*?)(?=\n---|\n## |\Z)",
            re.DOTALL,
        )
        def add_row(m: re.Match) -> str:
            body = m.group(1).rstrip("\n")
            return (
                f"{section}\n\n| ID | Feature | Completed | Spec | Notes |\n|----|---------|-----------|------|-------|\n"
                f"{body}\n{section_row}"
            )
        completed_text, n = pattern.subn(add_row, completed_text, count=1)
        if n == 0:
            completed_text += f"\n\n{section}\n\n| ID | Feature | Completed | Spec | Notes |\n|----|---------|-----------|------|-------|\n{section_row}\n"
    else:
        completed_text = completed_text.rstrip() + (
            f"\n\n{section}\n\n| ID | Feature | Completed | Spec | Notes |\n"
            f"|----|---------|-----------|------|-------|\n{section_row}\n"
        )

new_backlog = re.sub(
    rf"(\|\s*`{re.escape(feature_id)}`\s*\|\s*.+?\s*\|\s*)[✅🟡⬜](\s*\|)",
    r"\1✅\2",
    backlog_text,
    count=1,
)
if new_backlog == backlog_text:
    print(f"warning: could not update status for `{feature_id}` in backlog", file=sys.stderr)
else:
    backlog_text = new_backlog

if dry_run:
    print(f"DRY RUN: would record `{feature_id}` ({meta['title']}) on {date}")
    sys.exit(0)

completed.write_text(completed_text, encoding="utf-8")
backlog.write_text(backlog_text, encoding="utf-8")
print(f"recorded: `{feature_id}` → docs/feature-completed.md ({date})")
print(f"updated: docs/feature-backlog.md status → ✅")
PY
