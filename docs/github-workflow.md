# GitHub workflow

**Repo:** `spektr/searchify` (set `GITHUB_REPO` in `.env` to match your remote)

## Issue hierarchy

Section epic (Epic) → Feature (`[feature-id] Title`) → Task (`[feature-id/slug] Title`)

Example tasks for Searchify: `audit`, `spec`, `engine`, `verify`, `docs`

## Scripts (minimal scaffold)

| Script | Purpose |
|--------|---------|
| `github-issue-status.sh` | Set `status/todo`, `status/in-progress`, or `status/done` on issues |
| `record-feature-complete.sh` | Mark backlog ✅ and append to feature-completed.md |

Deferred until backlog is ready for issue creation:

| Script | Purpose |
|--------|---------|
| `create-feature-issues.sh` | Parse backlog → GitHub epic/feature/task issues |
| `load-feature-issue.sh` | Resolve feature + list open tasks |

Copy those from the Relativity reference repo when needed. See personal skill `setup-project-scaffolding`.

## During work

```bash
./scripts/github-issue-status.sh in-progress <task#>
# ... implement ...
gh issue close <task#> --comment "Done in ..."
./scripts/github-issue-status.sh done <task#>
```

## Feature done + PR

1. Close all task issues for the feature
2. Close parent feature issue
3. `./scripts/record-feature-complete.sh <feature-id> --issue <n> --note "PR #…"`
4. Move `docs/handoffs/current.md` → `docs/handoffs/archive/YYYY-MM-DD-<feature-id>.md`

Bootstrap this scaffold on a new repo: personal Cursor skill `setup-project-scaffolding` (`/setup-project-scaffolding`).
