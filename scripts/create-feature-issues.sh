#!/usr/bin/env bash
# Create GitHub section epics + feature issues + task sub-issues.
#
# Each section → 1 epic issue.
# Each feature → sub-issue under its section epic.
# Each task → sub-issue under its feature issue.
# Requires: gh auth login  OR  GH_TOKEN with repo scope.
#
# Usage:
#   ./scripts/create-feature-issues.sh              # create epics + features + tasks
#   ./scripts/create-feature-issues.sh --dry-run    # preview only
#   ./scripts/create-feature-issues.sh --parents-only  # skip task sub-issues
#   ./scripts/create-feature-issues.sh --only drama-event-log  # one new backlog row
#   ./scripts/create-feature-issues.sh --only id1 --only id2 --quiet  # incremental, less noise

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/gh-env.sh
source "$ROOT/scripts/lib/gh-env.sh"
# shellcheck source=lib/gh-issues.sh
source "$ROOT/scripts/lib/gh-issues.sh"

REPO="${GITHUB_REPO:-SpektrNO/searchify}"
DRY_RUN=false
PARENTS_ONLY=false
QUIET=false
ONLY_IDS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=true ;;
    --parents-only) PARENTS_ONLY=true ;;
    --quiet) QUIET=true ;;
    --only)
      shift
      [[ $# -gt 0 ]] || { echo "error: --only requires a feature id" >&2; exit 1; }
      ONLY_IDS+=("$1")
      ;;
    --only=*)
      ONLY_IDS+=("${1#--only=}")
      ;;
    -h|--help)
      sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1 (try --help)" >&2
      exit 1
      ;;
  esac
  shift
done

log_skip() {
  $QUIET || echo "$@"
}

REPO_OWNER="${REPO%%/*}"
REPO_NAME="${REPO#*/}"

if ! $DRY_RUN; then
  if ! gh_ready "$ROOT"; then
    echo "error: gh not authenticated. Set GH_TOKEN in .env or run: gh auth login" >&2
    exit 1
  fi
fi

ensure_label() {
  local name="$1" color="$2" description="$3"
  $DRY_RUN && return 0
  gh label create "$name" --color "$color" --description "$description" --repo "$REPO" 2>/dev/null || true
}

ensure_label "feature" "1d76db" "Feature epic — docs/feature-backlog.md"
ensure_label "task" "ededed" "Implementation sub-task of a feature"
ensure_label "epic" "8250df" "Section epic grouping multiple features"
ensure_label "partial" "fbca04" "Partially implemented"
ensure_label "status/todo" "ededed" "Not started"
ensure_label "status/in-progress" "fbca04" "Agent or dev actively working"
ensure_label "status/done" "0e8a16" "Work complete"

# Standard task pipeline (slug|title|description)
TASK_SPEC="spec|Product handoff|Write acceptance criteria + MCP/CLI/REST contracts for the feature (spec task)."
TASK_AUDIT="audit|Audit partial work|Compare existing code to spec; list gaps in parent issue before continuing."
TASK_ENGINE="engine|Engine|Implement Go changes under \`cmd/searchify\` and \`internal/*\` (MCP tools, index, HTTP/REST as needed)."
TASK_VERIFY="verify|Verify|Run \`go test ./...\` (and rebuild binary when relevant); confirm acceptance criteria."
TASK_DOCS="docs|Docs|Update \`README.md\` / \`docs/architecture.md\`, backlog status, append \`docs/feature-completed.md\` via \`record-feature-complete.sh\`, archive handoff."

BACKLOG_FILE="$ROOT/docs/feature-backlog.md"

# Args: status, section_slug, optional feature_id
task_list_for() {
  local status="$1" section="$2" feature_id="${3:-}"
  local -a tasks=()

  if [[ "$status" == "🟡" ]]; then
    tasks+=("$TASK_AUDIT")
  else
    tasks+=("$TASK_SPEC")
  fi

  case "$section" in
    *optional*|*scale*|*ops*)
      # Docs-only / ops writeups
      if [[ "$feature_id" == "opt-tls-reverse-proxy" ]]; then
        tasks+=("$TASK_DOCS" "$TASK_VERIFY")
      else
        tasks+=("$TASK_ENGINE" "$TASK_VERIFY" "$TASK_DOCS")
      fi
      ;;
    *)
      tasks+=("$TASK_ENGINE" "$TASK_VERIFY" "$TASK_DOCS")
      ;;
  esac

  printf '%s\n' "${tasks[@]}"
}

extract_backlog_features() {
  python3 - "$BACKLOG_FILE" <<'PY'
import re
import sys

path = sys.argv[1]
lines = open(path, encoding="utf-8").read().splitlines()

section_title = None
section_slug = None
headers = None

def clean_section(raw: str) -> str:
  return re.sub(r"^[A-Z]\.\s*", "", raw).strip()

def section_to_slug(title: str) -> str:
  s = title.lower()
  s = s.replace("&", " and ")
  s = re.sub(r"\([^)]*\)", " ", s)
  s = re.sub(r"[^a-z0-9]+", "-", s).strip("-")
  return s

def looks_separator(cells):
  flat = "".join(c.replace(":", "").replace("-", "").strip() for c in cells)
  return flat == ""

for line in lines:
  if line.startswith("## "):
    section_title = clean_section(line[3:].strip())
    section_slug = section_to_slug(section_title)
    headers = None
    continue

  if not section_title:
    continue

  if not line.startswith("|"):
    continue

  cells = [c.strip() for c in line.strip().strip("|").split("|")]
  if not cells:
    continue

  if looks_separator(cells):
    continue

  if "ID" in cells and "Status" in cells:
    headers = cells
    continue

  if not headers:
    continue

  if len(cells) != len(headers):
    # Strict mode: malformed table rows are ignored.
    continue

  row = dict(zip(headers, cells))
  id_cell = row.get("ID", "")
  status = row.get("Status", "").strip()
  id_match = re.fullmatch(r"`([^`]+)`", id_cell)
  if not id_match or not status:
    # Strict mode: require backticked ID + explicit status.
    continue

  if status.startswith("✅"):
    continue

  feature_id = id_match.group(1)
  feature_title = row.get("Feature", "").strip()
  if not feature_title:
    # Fallback for compact subtables that omit "Feature".
    feature_title = feature_id.replace("-", " ")

  spec = row.get("Spec", "").strip() or "docs/feature-backlog.md"
  spec = spec.strip("`")
  print("|".join([section_slug, section_title, feature_id, feature_title, status, spec]))
PY
}

set_issue_type() {
  local issue_number="$1" issue_type="$2"
  gh api --method PATCH "/repos/${REPO}/issues/${issue_number}" -f type="$issue_type" >/dev/null
}

issue_node_id() {
  local number="$1"
  gh api graphql -f query='
    query($owner: String!, $repo: String!, $number: Int!) {
      repository(owner: $owner, name: $repo) {
        issue(number: $number) { id }
      }
    }' -f owner="$REPO_OWNER" -f repo="$REPO_NAME" -F number="$number" \
    --jq '.data.repository.issue.id'
}

link_sub_issue() {
  local parent_id="$1" child_id="$2"
  gh api graphql -f query='
    mutation($issueId: ID!, $subIssueId: ID!) {
      addSubIssue(input: { issueId: $issueId, subIssueId: $subIssueId }) {
        issue { number }
      }
    }' -f issueId="$parent_id" -f subIssueId="$child_id" >/dev/null
}

# Echo the parent issue number of a sub-issue, or empty if it has none.
issue_parent_number() {
  local number="$1"
  gh api graphql -f query='
    query($owner: String!, $repo: String!, $number: Int!) {
      repository(owner: $owner, name: $repo) {
        issue(number: $number) { parent { number } }
      }
    }' -f owner="$REPO_OWNER" -f repo="$REPO_NAME" -F number="$number" \
    --jq '.data.repository.issue.parent.number // empty' 2>/dev/null || true
}

# Link child under parent only if the child has no parent yet (idempotent).
ensure_sub_issue() {
  local parent_num="$1" child_num="$2" label="$3"
  local existing
  existing=$(issue_parent_number "$child_num")
  if [[ -n "$existing" ]]; then
    if [[ "$existing" != "$parent_num" ]]; then
      echo "warn: ${label} #${child_num} already under #${existing}, expected #${parent_num}" >&2
    fi
    return 0
  fi
  local parent_nid child_nid
  parent_nid=$(issue_node_id "$parent_num")
  child_nid=$(issue_node_id "$child_num")
  link_sub_issue "$parent_nid" "$child_nid" \
    || echo "warn: could not link ${label} #${child_num} under #${parent_num}" >&2
}

parent_issue_number() {
  gh_feature_issue_number "$1"
}

epic_issue_number() {
  gh_epic_issue_number "$1"
}

task_issue_exists() {
  gh_task_issue_exists "$1" "$2"
}

build_parent_body() {
  local id="$1" title="$2" status="$3" spec="$4" parent_num="${5:-}"
  local task_section=""

  while IFS='|' read -r slug tname _; do
  task_section+="- [ ] \`${slug}\` — ${tname}"$'\n'
  done < <(task_list_for "$status" "${6:-foundation}" "$id")

  cat <<EOF
## Feature

**ID:** \`${id}\`
**Status:** ${status}
**Spec:** [\`${spec}\`](https://github.com/${REPO}/blob/main/${spec})

## Tasks

${task_section}
_Sub-issues are linked below when created by \`create-feature-issues.sh\`._

## Implementation

Use branch \`feature/${id}\`. Open a PR with \`Closes #<parent>\` when the feature is done.

## References

- [Feature backlog](https://github.com/${REPO}/blob/main/docs/feature-backlog.md)
- \`docs/architecture.md\`
- \`docs/github-workflow.md\`
EOF
}

build_epic_body() {
  local section="$1" title="$2"
  cat <<EOF
## Epic

**Section:** \`${section}\`
**Backlog group:** ${title}

## Scope

This epic tracks all feature issues in the \`${section}\` section.
Feature sub-issues are linked below when created by \`create-feature-issues.sh\`.

## References

- [Feature backlog](https://github.com/${REPO}/blob/main/docs/feature-backlog.md)
EOF
}

epics_created=0
epics_skipped=0
parents_created=0
parents_skipped=0
tasks_created=0
tasks_skipped=0

declare -A SECTION_EPICS
declare -A SECTION_TITLES

FEATURE_ROWS_FILE="$(mktemp)"
trap 'rm -f "$FEATURE_ROWS_FILE"' EXIT
extract_backlog_features > "$FEATURE_ROWS_FILE"

if [[ ${#ONLY_IDS[@]} -gt 0 ]]; then
  FILTERED_FILE="$(mktemp)"
  while IFS='|' read -r section section_title id title status spec; do
    for want in "${ONLY_IDS[@]}"; do
      if [[ "$id" == "$want" ]]; then
        printf '%s\n' "${section}|${section_title}|${id}|${title}|${status}|${spec}" >> "$FILTERED_FILE"
        break
      fi
    done
  done < "$FEATURE_ROWS_FILE"
  mv "$FILTERED_FILE" "$FEATURE_ROWS_FILE"

  if [[ ! -s "$FEATURE_ROWS_FILE" ]]; then
    echo "error: no eligible backlog rows for --only: ${ONLY_IDS[*]}" >&2
    echo "hint: id must be backticked in $BACKLOG_FILE with Status not ✅" >&2
    exit 1
  fi

  echo "filter: ${#ONLY_IDS[@]} id(s) → $(wc -l < "$FEATURE_ROWS_FILE" | tr -d ' ') row(s)"
fi

if [[ ! -s "$FEATURE_ROWS_FILE" ]]; then
  echo "no eligible features found in $BACKLOG_FILE (strict parser)." >&2
  exit 1
fi

while IFS='|' read -r section_slug section_title _ _ _ _; do
  [[ -n "$section_slug" ]] || continue
  SECTION_TITLES["$section_slug"]="$section_title"
  ensure_label "$section_slug" "6e7781" "Feature backlog section: ${section_title}"
done < "$FEATURE_ROWS_FILE"

while IFS= read -r section; do
  section_title="${SECTION_TITLES[$section]}"
  epic_title="[epic/${section}] ${section_title}"
  epic_num=""
  if ! $DRY_RUN; then
    epic_num=$(epic_issue_number "$section")
  fi

  if [[ -n "$epic_num" && "$epic_num" != "null" ]]; then
    log_skip "epic exists: #${epic_num} ${section}"
    ((epics_skipped++)) || true
    SECTION_EPICS["$section"]="$epic_num"
    continue
  fi

  epic_body="$(build_epic_body "$section" "$section_title")"
  if $DRY_RUN; then
    echo "DRY epic: ${epic_title}"
    ((epics_created++)) || true
    SECTION_EPICS["$section"]="DRY"
  else
    epic_url=$(gh issue create --repo "$REPO" --title "$epic_title" --body "$epic_body" --label "epic,${section},status/todo")
    epic_num=$(echo "$epic_url" | grep -oE '[0-9]+$')
    set_issue_type "$epic_num" "Epic" || echo "warn: could not set issue type Epic on #${epic_num}" >&2
    echo "epic: ${epic_url}"
    ((epics_created++)) || true
    SECTION_EPICS["$section"]="$epic_num"
    sleep 0.3
  fi
done < <(printf '%s\n' "${!SECTION_TITLES[@]}" | sort)

while IFS='|' read -r section section_title id title status spec; do
  issue_title="[${id}] ${title}"
  labels="feature,${section},status/todo"
  [[ "$status" == "🟡" ]] && labels="${labels},partial"

  parent_num=""
  if ! $DRY_RUN; then
    parent_num=$(parent_issue_number "$id")
  fi

  if [[ -n "$parent_num" && "$parent_num" != "null" ]]; then
    log_skip "parent exists: #${parent_num} ${id}"
    ((parents_skipped++)) || true
  else
    body=$(build_parent_body "$id" "$title" "$status" "$spec" "$parent_num" "$section")
    if $DRY_RUN; then
      echo "DRY parent: ${issue_title}"
      ((parents_created++)) || true
    else
      url=$(gh issue create --repo "$REPO" --title "$issue_title" --body "$body" --label "$labels")
      parent_num=$(echo "$url" | grep -oE '[0-9]+$')
      set_issue_type "$parent_num" "Feature" || echo "warn: could not set issue type Feature on #${parent_num}" >&2
      echo "parent: ${url}"
      ((parents_created++)) || true
      sleep 0.3
    fi
  fi

  if ! $DRY_RUN; then
    epic_num="${SECTION_EPICS[$section]:-}"
    if [[ -n "$epic_num" && "$epic_num" != "null" && "$epic_num" != "DRY" ]]; then
      ensure_sub_issue "$epic_num" "$parent_num" "feature"
    fi
  fi

  $PARENTS_ONLY && continue

  while IFS='|' read -r slug tname tdesc; do
    task_title="[${id}/${slug}] ${tname}"
    task_labels="task,${section},status/todo"

    if $DRY_RUN; then
      echo "  DRY task: ${task_title}"
      ((tasks_created++)) || true
      continue
    fi

    if task_issue_exists "$id" "$slug"; then
      log_skip "  skip task (exists): ${id}/${slug}"
      ((tasks_skipped++)) || true
      continue
    fi

    task_body="$(cat <<EOF
## Task

**Feature:** \`${id}\` — ${title}
**Step:** \`${slug}\`

${tdesc}

## Spec

[\`${spec}\`](https://github.com/${REPO}/blob/main/${spec})

Parent feature: #${parent_num}
EOF
)"

    task_url=$(gh issue create --repo "$REPO" --title "$task_title" --body "$task_body" --label "$task_labels")
    task_num=$(echo "$task_url" | grep -oE '[0-9]+$')
    set_issue_type "$task_num" "Task" || echo "warn: could not set issue type Task on #${task_num}" >&2
    echo "  task: ${task_url}"

    ensure_sub_issue "$parent_num" "$task_num" "task"
    ((tasks_created++)) || true
    sleep 0.3
  done < <(task_list_for "$status" "$section" "$id")
done < "$FEATURE_ROWS_FILE"

echo "---"
echo "epics:   created=${epics_created} skipped=${epics_skipped}"
echo "parents: created=${parents_created} skipped=${parents_skipped}"
echo "tasks:   created=${tasks_created} skipped=${tasks_skipped} dry_run=${DRY_RUN}"
