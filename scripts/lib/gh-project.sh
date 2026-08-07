# GitHub Projects V2 Status sync helpers.
# Requires scopes: read:project, project (gh auth refresh -s read:project,project).
# Soft-fails (returns non-zero, prints warning) when env/scopes missing.
#
# Env:
#   GITHUB_PROJECT_OWNER   org or user login (e.g. SpektrNO)
#   GITHUB_PROJECT_NUMBER  project number from URL
#   GITHUB_PROJECT_STATUS_FIELD   field name (default: Status)
#   GITHUB_PROJECT_STATUS_TODO / _IN_PROGRESS / _DONE  option names
#
# shellcheck source=gh-env.sh
# Callers should source gh-env.sh first and load_gh_env.

# In-process cache
_GH_PROJECT_ID=""
_GH_PROJECT_STATUS_FIELD_ID=""
# option name → id (newline-separated "name|id")
_GH_PROJECT_STATUS_OPTIONS=""

gh_project_configured() {
  [[ -n "${GITHUB_PROJECT_OWNER:-}" && -n "${GITHUB_PROJECT_NUMBER:-}" ]]
}

gh_project_status_option_name() {
  local workflow_status="$1"
  case "$workflow_status" in
    todo) echo "${GITHUB_PROJECT_STATUS_TODO:-Todo}" ;;
    in-progress|inprogress|in_progress) echo "${GITHUB_PROJECT_STATUS_IN_PROGRESS:-In Progress}" ;;
    done) echo "${GITHUB_PROJECT_STATUS_DONE:-Done}" ;;
    *)
      echo "error: unknown workflow status for project: $workflow_status" >&2
      return 1
      ;;
  esac
}

# GraphQL helper; prints JSON to stdout. Returns gh exit code.
_gh_project_graphql() {
  local query="$1"
  shift
  gh api graphql -f query="$query" "$@"
}

# Resolve project id + Status field/options into cache. Soft-fail on error.
gh_project_resolve() {
  if [[ -n "$_GH_PROJECT_ID" && -n "$_GH_PROJECT_STATUS_FIELD_ID" ]]; then
    return 0
  fi

  if ! gh_project_configured; then
    echo "warn: GitHub Project Status sync skipped (set GITHUB_PROJECT_OWNER + GITHUB_PROJECT_NUMBER)" >&2
    return 1
  fi

  local owner="$GITHUB_PROJECT_OWNER"
  local number="$GITHUB_PROJECT_NUMBER"
  local field_name="${GITHUB_PROJECT_STATUS_FIELD:-Status}"
  local query result

  # Try organization first, then user.
  query='
    query($owner: String!, $number: Int!, $field: String!) {
      organization(login: $owner) {
        projectV2(number: $number) {
          id
          field(name: $field) {
            ... on ProjectV2SingleSelectField {
              id
              name
              options { id name }
            }
          }
        }
      }
    }'

  if ! result="$(_gh_project_graphql "$query" -F owner="$owner" -F number="$number" -F field="$field_name" 2>/dev/null)"; then
    echo "warn: GitHub Project Status sync skipped (need scopes read:project,project — run: gh auth refresh -s read:project,project)" >&2
    return 1
  fi

  _GH_PROJECT_ID="$(echo "$result" | jq -r '.data.organization.projectV2.id // empty')"
  _GH_PROJECT_STATUS_FIELD_ID="$(echo "$result" | jq -r '.data.organization.projectV2.field.id // empty')"

  if [[ -z "$_GH_PROJECT_ID" ]]; then
    query='
      query($owner: String!, $number: Int!, $field: String!) {
        user(login: $owner) {
          projectV2(number: $number) {
            id
            field(name: $field) {
              ... on ProjectV2SingleSelectField {
                id
                name
                options { id name }
              }
            }
          }
        }
      }'
    if ! result="$(_gh_project_graphql "$query" -F owner="$owner" -F number="$number" -F field="$field_name" 2>/dev/null)"; then
      echo "warn: GitHub Project Status sync skipped (need scopes read:project,project — run: gh auth refresh -s read:project,project)" >&2
      return 1
    fi
    _GH_PROJECT_ID="$(echo "$result" | jq -r '.data.user.projectV2.id // empty')"
    _GH_PROJECT_STATUS_FIELD_ID="$(echo "$result" | jq -r '.data.user.projectV2.field.id // empty')"
    _GH_PROJECT_STATUS_OPTIONS="$(echo "$result" | jq -r '
      (.data.user.projectV2.field.options // [])
      | map("\(.name)|\(.id)") | .[]
    ' 2>/dev/null || true)"
  else
    _GH_PROJECT_STATUS_OPTIONS="$(echo "$result" | jq -r '
      (.data.organization.projectV2.field.options // [])
      | map("\(.name)|\(.id)") | .[]
    ' 2>/dev/null || true)"
  fi

  if [[ -z "$_GH_PROJECT_ID" ]]; then
    echo "warn: GitHub Project Status sync skipped (project #$number not found for owner $owner)" >&2
    return 1
  fi
  if [[ -z "$_GH_PROJECT_STATUS_FIELD_ID" ]]; then
    echo "warn: GitHub Project Status sync skipped (single-select field \"${field_name}\" not found)" >&2
    return 1
  fi
  return 0
}

_gh_project_option_id() {
  local want_name="$1"
  local line name id
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    name="${line%%|*}"
    id="${line#*|}"
    # Case-insensitive match (Projects platform is case-insensitive).
    if [[ "${name,,}" == "${want_name,,}" ]]; then
      echo "$id"
      return 0
    fi
  done <<< "$_GH_PROJECT_STATUS_OPTIONS"
  return 1
}

_gh_issue_node_id() {
  local repo="$1" number="$2"
  gh api "repos/${repo}/issues/${number}" --jq .node_id
}

# Find project item id for an issue already on the board (empty if absent).
_gh_project_item_id_for_issue() {
  local issue_node_id="$1"
  local query result
  query='
    query($id: ID!) {
      node(id: $id) {
        ... on Issue {
          projectItems(first: 50) {
            nodes {
              id
              project { id }
            }
          }
        }
      }
    }'
  result="$(_gh_project_graphql "$query" -f id="$issue_node_id")" || return 1
  echo "$result" | jq -r --arg pid "$_GH_PROJECT_ID" '
    (.data.node.projectItems.nodes // [])
    | map(select(.project.id == $pid))
    | .[0].id // empty
  '
}

_gh_project_add_issue() {
  local issue_node_id="$1"
  local query result
  query='
    mutation($projectId: ID!, $contentId: ID!) {
      addProjectV2ItemById(input: { projectId: $projectId, contentId: $contentId }) {
        item { id }
      }
    }'
  result="$(_gh_project_graphql "$query" -f projectId="$_GH_PROJECT_ID" -f contentId="$issue_node_id")" || return 1
  echo "$result" | jq -r '.data.addProjectV2ItemById.item.id // empty'
}

_gh_project_set_status() {
  local item_id="$1" option_id="$2"
  local query
  query='
    mutation($projectId: ID!, $itemId: ID!, $fieldId: ID!, $optionId: String!) {
      updateProjectV2ItemFieldValue(
        input: {
          projectId: $projectId
          itemId: $itemId
          fieldId: $fieldId
          value: { singleSelectOptionId: $optionId }
        }
      ) {
        projectV2Item { id }
      }
    }'
  _gh_project_graphql "$query" \
    -f projectId="$_GH_PROJECT_ID" \
    -f itemId="$item_id" \
    -f fieldId="$_GH_PROJECT_STATUS_FIELD_ID" \
    -f optionId="$option_id" >/dev/null
}

# Sync Project Status for one issue. Soft-fail (warn + return 1) without aborting caller.
# Args: <workflow_status> <repo> <issue_number>
gh_project_sync_issue_status() {
  local workflow_status="$1" repo="$2" number="$3"
  local option_name option_id issue_node item_id

  if ! gh_project_resolve; then
    return 1
  fi

  option_name="$(gh_project_status_option_name "$workflow_status")" || return 1
  if ! option_id="$(_gh_project_option_id "$option_name")"; then
    echo "warn: GitHub Project Status option \"${option_name}\" not found on board (check GITHUB_PROJECT_STATUS_* )" >&2
    return 1
  fi

  if ! issue_node="$(_gh_issue_node_id "$repo" "$number" 2>/dev/null)" || [[ -z "$issue_node" ]]; then
    echo "warn: GitHub Project Status sync skipped (could not resolve issue #${number})" >&2
    return 1
  fi

  item_id="$(_gh_project_item_id_for_issue "$issue_node" 2>/dev/null || true)"
  if [[ -z "$item_id" ]]; then
    if ! item_id="$(_gh_project_add_issue "$issue_node" 2>/dev/null)" || [[ -z "$item_id" ]]; then
      echo "warn: GitHub Project Status sync skipped (could not add #${number} to project)" >&2
      return 1
    fi
  fi

  if ! _gh_project_set_status "$item_id" "$option_id" 2>/dev/null; then
    echo "warn: GitHub Project Status sync failed for #${number}" >&2
    return 1
  fi

  echo "#${number} → project Status: ${option_name}"
  return 0
}
