# Shared GitHub issue title lookups.
# Requires REPO to be set. Source after scripts/lib/gh-env.sh.

gh_extract_feature_id() {
  local title="$1"
  if [[ "$title" =~ \[([a-z][a-z0-9-]+)/([a-z]+)\] ]]; then
    echo "${BASH_REMATCH[1]}"
    return
  fi
  if [[ "$title" =~ \[([a-z][a-z0-9-]+)\] ]]; then
    echo "${BASH_REMATCH[1]}"
  fi
}

gh_extract_task_slug() {
  local title="$1"
  if [[ "$title" =~ \[([a-z][a-z0-9-]+)/([a-z]+)\] ]]; then
    echo "${BASH_REMATCH[2]}"
  fi
}

gh_is_feature_title() {
  local feature_id="$1" title="$2"
  [[ "$title" == "[${feature_id}] "* ]]
}

gh_is_task_title() {
  local feature_id="$1" slug="$2" title="$3"
  [[ "$title" == "[${feature_id}/${slug}] "* ]]
}

# Feature titles: [feature-id] Title — not [feature-id/slug] tasks.
gh_feature_issue_number() {
  local feature_id="$1"
  gh issue list --repo "$REPO" --state all --limit 20 \
    --search "in:title \"[${feature_id}\"" --json number,title \
    --jq "map(select(.title | startswith(\"[${feature_id}] \"))) | .[0].number // empty" 2>/dev/null || true
}

gh_epic_issue_number() {
  local section="$1"
  gh issue list --repo "$REPO" --search "in:title \"[epic/${section}\"" --json number,title \
    --jq "map(select(.title | startswith(\"[epic/${section}] \"))) | .[0].number // empty" 2>/dev/null || true
}

gh_task_issue_exists() {
  local id="$1" slug="$2"
  local count
  count=$(gh issue list --repo "$REPO" --search "in:title \"[${id}/${slug}\"" --json title \
    --jq "map(select(.title | startswith(\"[${id}/${slug}] \"))) | length" 2>/dev/null || echo 0)
  [[ "$count" -gt 0 ]]
}

# If issue # points at a task, return the feature parent #; otherwise echo input.
gh_resolve_to_feature_parent() {
  local num="$1"
  local title feature_id parent_num

  title=$(gh issue view "$num" --repo "$REPO" --json title --jq '.title')
  feature_id=$(gh_extract_feature_id "$title")
  if [[ -z "$feature_id" ]]; then
    echo "error: issue #${num} has no recognizable feature id in title: ${title}" >&2
    return 1
  fi
  if gh_is_feature_title "$feature_id" "$title"; then
    echo "$num"
    return 0
  fi
  parent_num=$(gh_feature_issue_number "$feature_id")
  if [[ -z "$parent_num" ]]; then
    echo "error: task #${num} belongs to \`${feature_id}\` but no feature issue titled \`[${feature_id}] ...\` found" >&2
    return 1
  fi
  echo "$parent_num"
}
