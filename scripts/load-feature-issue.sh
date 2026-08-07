#!/usr/bin/env bash
# Resolve a feature by GitHub issue #, feature id, or title fragment.
# Print parent issue + ordered sub-tasks (open/closed) for agent consumption.
#
# Usage:
#   ./scripts/load-feature-issue.sh 42
#   ./scripts/load-feature-issue.sh opt-hnsw-vectors
#   ./scripts/load-feature-issue.sh "hnsw vectors"
#   ./scripts/load-feature-issue.sh --json opt-hnsw-vectors
#
# Requires: gh auth login  OR  GH_TOKEN with repo scope.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=lib/gh-env.sh
source "$ROOT/scripts/lib/gh-env.sh"
# shellcheck source=lib/gh-issues.sh
source "$ROOT/scripts/lib/gh-issues.sh"

REPO="${GITHUB_REPO:-SpektrNO/searchify}"
OWNER="${REPO%%/*}"
NAME="${REPO#*/}"
JSON=false

if [[ "${1:-}" == "--json" ]]; then
  JSON=true
  shift
fi

INPUT="${1:-}"
if [[ -z "$INPUT" ]]; then
  echo "usage: $0 [--json] <issue#|feature-id|title-fragment>" >&2
  exit 1
fi

if ! gh_ready "$ROOT"; then
  echo "error: gh not authenticated. Set GH_TOKEN in .env or run: gh auth login" >&2
  exit 1
fi

# Canonical task order (matches create-feature-issues.sh)
TASK_ORDER=(audit spec engine verify docs)

task_rank() {
  local slug="$1"
  local i rank=99
  for i in "${!TASK_ORDER[@]}"; do
    if [[ "${TASK_ORDER[$i]}" == "$slug" ]]; then
      echo "$i"
      return
    fi
  done
  echo "$rank"
}

extract_feature_id() {
  gh_extract_feature_id "$1"
}

extract_task_slug() {
  gh_extract_task_slug "$1"
}

is_feature_title() {
  gh_is_feature_title "$1" "$2"
}

feature_issue_number() {
  gh_feature_issue_number "$1"
}

resolve_issue_number() {
  local raw="$1"
  local num=""

  if [[ "$raw" =~ ^#?([0-9]+)$ ]]; then
    gh_resolve_to_feature_parent "${BASH_REMATCH[1]}"
    return
  fi

  if [[ "$raw" =~ ^[a-z][a-z0-9-]*$ ]]; then
    num=$(gh_feature_issue_number "$raw")
    if [[ -n "$num" ]]; then
      echo "$num"
      return
    fi
  fi

  num=$(gh issue list --repo "$REPO" --state all --limit 10 \
    --search "in:title ${raw}" --label feature --json number,title \
    --jq 'map(select(.title | test("^\\[[a-z][a-z0-9-]+\\] "))) | .[0].number // empty' 2>/dev/null || true)
  if [[ -n "$num" ]]; then
    echo "$num"
    return
  fi

  echo "error: could not resolve feature from: ${raw}" >&2
  echo "hint: use issue number, feature id (e.g. opt-hnsw-vectors), or distinctive title words" >&2
  exit 1
}

fetch_parent() {
  local number="$1"
  gh issue view "$number" --repo "$REPO" --json number,title,body,state,labels,url
}

fetch_sub_issues_graphql() {
  local number="$1"
  gh api graphql -f query='
    query($owner: String!, $repo: String!, $number: Int!) {
      repository(owner: $owner, name: $repo) {
        issue(number: $number) {
          subIssues(first: 20) {
            nodes {
              number
              title
              body
              state
              labels(first: 10) { nodes { name } }
            }
          }
        }
      }
    }' -f owner="$OWNER" -f repo="$NAME" -F number="$number" \
    --jq '.data.repository.issue.subIssues.nodes // []'
}

fetch_sub_issues_search() {
  local feature_id="$1"
  gh issue list --repo "$REPO" --state all --limit 30 \
    --search "in:title \"[${feature_id}/\"" --json number,title,body,state,labels \
    --jq "map(select(.title | test(\"^\\\\[${feature_id}/[a-z]+\\\\] \"))) // []"
}

sort_tasks_json() {
  python3 -c "
import json, sys, re

tasks = json.load(sys.stdin)
order = ['audit', 'spec', 'engine', 'verify', 'docs']
order_map = {s: i for i, s in enumerate(order)}

def slug(title):
    m = re.search(r'\[([a-z][a-z0-9-]+)/([a-z]+)\]', title)
    return m.group(2) if m else 'zzz'

for t in tasks:
    t['slug'] = slug(t.get('title', ''))
    t['rank'] = order_map.get(t['slug'], 99)

tasks.sort(key=lambda t: (t['rank'], t.get('number', 0)))
print(json.dumps(tasks, indent=2))
"
}

PARENT_NUM=$(resolve_issue_number "$INPUT")
PARENT_JSON=$(fetch_parent "$PARENT_NUM")
PARENT_TITLE=$(echo "$PARENT_JSON" | jq -r '.title')
FEATURE_ID=$(extract_feature_id "$PARENT_TITLE")
PARENT_STATE=$(echo "$PARENT_JSON" | jq -r '.state')
PARENT_URL=$(echo "$PARENT_JSON" | jq -r '.url')
PARENT_BODY=$(echo "$PARENT_JSON" | jq -r '.body')

if [[ -z "$FEATURE_ID" ]] || ! is_feature_title "$FEATURE_ID" "$PARENT_TITLE"; then
  echo "error: resolved issue #${PARENT_NUM} is not a feature parent: ${PARENT_TITLE}" >&2
  exit 1
fi

SUB_RAW=$(fetch_sub_issues_graphql "$PARENT_NUM" 2>/dev/null || echo '[]')
if [[ "$(echo "$SUB_RAW" | jq 'length')" -eq 0 ]]; then
  SUB_RAW=$(fetch_sub_issues_search "$FEATURE_ID")
fi

# Normalize GraphQL vs REST label shapes
TASKS_JSON=$(echo "$SUB_RAW" | jq '
  if type == "array" then
    map({
      number: .number,
      title: .title,
      body: (.body // ""),
      state: .state,
      labels: (if .labels.nodes then [.labels.nodes[].name] else [.labels[].name] end)
    })
  else []
  end
' | jq 'map(. + {
  workflow_status: (
    [.labels[]? | select(startswith("status/"))] | first // "status/todo"
  )
})' | sort_tasks_json)

OPEN_TASKS=$(echo "$TASKS_JSON" | jq '[.[] | select(.state == "OPEN")]')
NEXT_TASK=$(echo "$OPEN_TASKS" | jq -r '.[0].slug // empty')
OPEN_COUNT=$(echo "$OPEN_TASKS" | jq 'length')

if $JSON; then
  jq -n \
    --argjson parent "$PARENT_JSON" \
    --arg feature_id "$FEATURE_ID" \
    --argjson tasks "$TASKS_JSON" \
    --argjson open_tasks "$OPEN_TASKS" \
    --arg next_task "$NEXT_TASK" \
    '{
      feature_id: $feature_id,
      parent: $parent,
      tasks: $tasks,
      open_tasks: $open_tasks,
      next_task: (if $next_task == "" then null else $next_task end),
      task_order: ["audit","spec","engine","verify","docs"]
    }'
  exit 0
fi

PARENT_WORKFLOW=$(echo "$PARENT_JSON" | jq -r '[.labels[].name | select(startswith("status/"))] | first // "status/todo"')

cat <<EOF
# Feature: ${FEATURE_ID}

**Parent:** #${PARENT_NUM} — ${PARENT_TITLE} (${PARENT_STATE}, ${PARENT_WORKFLOW})
**URL:** ${PARENT_URL}

## Task order

Implement open tasks in this order: `audit` → `spec` → `engine` → `verify` → `docs`
(docs-only features may omit `engine` — skip missing tasks)

## Sub-tasks

EOF

echo "$TASKS_JSON" | jq -r '.[] | "- [" + (if .state == "CLOSED" then "x" else " " end) + "] #\(.number) `\(.slug)` — \(.title) (\(.workflow_status))"'

cat <<EOF

## Next open task

$(if [[ -n "$NEXT_TASK" ]]; then echo "**\`${NEXT_TASK}\`** (${OPEN_COUNT} open total)"; else echo "_All sub-tasks closed._"; fi)

## Parent issue body

${PARENT_BODY}
EOF
