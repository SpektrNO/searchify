#!/usr/bin/env bash
# Set workflow status on GitHub issues via mutually exclusive labels.
#
# Usage:
#   ./scripts/github-issue-status.sh in-progress 42
#   ./scripts/github-issue-status.sh todo 42 43
#   ./scripts/github-issue-status.sh done 42
#
# Labels: status/todo | status/in-progress | status/done
# Loads GH_TOKEN from .env (see scripts/lib/gh-env.sh).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/gh-env.sh
source "$ROOT/scripts/lib/gh-env.sh"

REPO="${GITHUB_REPO:-spektr/searchify}"
DRY_RUN=false

STATUS_LABELS=(status/todo status/in-progress status/done)

usage() {
  echo "usage: $0 [--dry-run] <todo|in-progress|done> <issue#> [issue#...]" >&2
  exit 1
}

ensure_status_labels() {
  $DRY_RUN && return 0
  gh label create "status/todo" --color "ededed" --description "Not started" --repo "$REPO" 2>/dev/null || true
  gh label create "status/in-progress" --color "fbca04" --description "Agent or dev actively working" --repo "$REPO" 2>/dev/null || true
  gh label create "status/done" --color "0e8a16" --description "Work complete (issue may still be open)" --repo "$REPO" 2>/dev/null || true
}

set_issue_status() {
  local status="$1" number="$2"
  local target_label=""

  case "$status" in
    todo) target_label="status/todo" ;;
    in-progress|inprogress|in_progress) target_label="status/in-progress" ;;
    done) target_label="status/done" ;;
    *) echo "error: unknown status: $status (use todo, in-progress, done)" >&2; exit 1 ;;
  esac

  if $DRY_RUN; then
    echo "DRY RUN: #$number → ${target_label}"
    return 0
  fi

  local remove_args=()
  local lbl
  for lbl in "${STATUS_LABELS[@]}"; do
    [[ "$lbl" != "$target_label" ]] && remove_args+=(--remove-label "$lbl")
  done

  gh issue edit "$number" --repo "$REPO" "${remove_args[@]}" --add-label "$target_label" >/dev/null
  echo "#${number} → ${target_label}"
}

[[ $# -lt 2 ]] && usage

if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=true
  shift
fi

STATUS="${1:-}"
shift
[[ $# -lt 1 ]] && usage

if ! gh_ready "$ROOT"; then
  echo "error: gh not authenticated. Set GH_TOKEN in .env or run: gh auth login" >&2
  exit 1
fi

ensure_status_labels

for num in "$@"; do
  num="${num#\#}"
  set_issue_status "$STATUS" "$num"
done
