# Load GitHub credentials from repo .env when GH_TOKEN is unset.
# Supports GH_TOKEN or GH_PAT. Never prints secrets.

load_gh_env() {
  local root="${1:-}"
  if [[ -z "$root" ]]; then
    root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  fi
  local env_file="$root/.env"
  [[ -f "$env_file" ]] || return 0
  [[ -n "${GH_TOKEN:-}" ]] && return 0

  local line key val
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ ^(GH_TOKEN|GH_PAT|GITHUB_REPO)= ]] || continue
    key="${line%%=*}"
    val="${line#*=}"
    val="${val%$'\r'}"
    val="${val#\"}"; val="${val%\"}"
    val="${val#\'}"; val="${val%\'}"
    case "$key" in
      GH_TOKEN) export GH_TOKEN="$val" ;;
      GH_PAT) export GH_PAT="$val" ;;
      GITHUB_REPO) export GITHUB_REPO="$val" ;;
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
