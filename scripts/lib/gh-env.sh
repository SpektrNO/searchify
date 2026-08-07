# Load GitHub credentials and project config from repo .env.
# Supports GH_TOKEN or GH_PAT. Never prints secrets.
#
# GH_TOKEN is only set from .env when unset in the environment.
# GITHUB_REPO and GITHUB_PROJECT_* are always refreshed from .env when present.

load_gh_env() {
  local root="${1:-}"
  if [[ -z "$root" ]]; then
    root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  fi
  local env_file="$root/.env"
  [[ -f "$env_file" ]] || return 0

  local has_token_already=false
  [[ -n "${GH_TOKEN:-}" ]] && has_token_already=true

  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ ^(GH_TOKEN|GH_PAT|GITHUB_REPO|GITHUB_PROJECT_[A-Z0-9_]+)= ]] || continue
    key="${line%%=*}"
    val="${line#*=}"
    val="${val%$'\r'}"
    val="${val#\"}"; val="${val%\"}"
    val="${val#\'}"; val="${val%\'}"
    case "$key" in
      GH_TOKEN)
        if ! $has_token_already; then
          export GH_TOKEN="$val"
        fi
        ;;
      GH_PAT)
        export GH_PAT="$val"
        ;;
      GITHUB_REPO) export GITHUB_REPO="$val" ;;
      GITHUB_PROJECT_OWNER) export GITHUB_PROJECT_OWNER="$val" ;;
      GITHUB_PROJECT_NUMBER) export GITHUB_PROJECT_NUMBER="$val" ;;
      GITHUB_PROJECT_STATUS_TODO) export GITHUB_PROJECT_STATUS_TODO="$val" ;;
      GITHUB_PROJECT_STATUS_IN_PROGRESS) export GITHUB_PROJECT_STATUS_IN_PROGRESS="$val" ;;
      GITHUB_PROJECT_STATUS_DONE) export GITHUB_PROJECT_STATUS_DONE="$val" ;;
      GITHUB_PROJECT_STATUS_FIELD) export GITHUB_PROJECT_STATUS_FIELD="$val" ;;
    esac
  done < "$env_file"

  if [[ -z "${GH_TOKEN:-}" && -n "${GH_PAT:-}" ]]; then
    export GH_TOKEN="$GH_PAT"
  fi
}

gh_ready() {
  load_gh_env "${1:-}"
  gh auth status &>/dev/null
}
