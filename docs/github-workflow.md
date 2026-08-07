# GitHub workflow

How issues and docs stay in sync for feature work in `SpektrNO/searchify`.

**Related:** [feature-backlog.md](./feature-backlog.md) · [feature-completed.md](./feature-completed.md) · [architecture.md](./architecture.md)

## Issue hierarchy

```
Section epic (type: Epic)
└── Feature issue (type: Feature)     title: [feature-id] …
    └── Task sub-issues (type: Task)  title: [feature-id/slug] …
```

Task slugs (in order): `audit` → `spec` → `engine` → `verify` → `docs`

Section-specific omissions (see `create-feature-issues.sh`):

| Section | Tasks after `spec`/`audit` |
|---------|----------------------------|
| Optional scale and ops | default: `engine`, `verify`, `docs`; `opt-tls-reverse-proxy`: `docs`, `verify` |
| All other sections | `engine`, `verify`, `docs` |

**Workflow labels** (mutually exclusive): `status/todo` → `status/in-progress` → `status/done`  
Labels track progress; **closing** an issue is separate (see below).

Optional: sync the same status to a GitHub Project board via `GITHUB_PROJECT_*` in `.env`.

## Prerequisites

```bash
# .env (see .env.example)
GITHUB_REPO=SpektrNO/searchify
GH_TOKEN=...

# or
gh auth login
```

## Bootstrap issues from backlog

```bash
./scripts/create-feature-issues.sh --dry-run
./scripts/create-feature-issues.sh --only opt-hnsw-vectors
# or all open backlog rows:
./scripts/create-feature-issues.sh
```

## Scripts

| Script | Purpose |
|--------|---------|
| `create-feature-issues.sh` | Create section epics + feature issues + task sub-issues from `feature-backlog.md` |
| `load-feature-issue.sh` | Resolve a feature by issue #, id, or title; list open/closed sub-tasks |
| `github-issue-status.sh` | Set `status/todo`, `status/in-progress`, or `status/done` on issue(s) (+ optional Project Status) |
| `record-feature-complete.sh` | Mark feature ✅ in backlog; append to `feature-completed.md` |

`record-feature-complete.sh` updates **docs only** — it does not close GitHub issues.

## Agent shortcuts

| Command | Behavior |
|---------|----------|
| `/spec-only <feature>` | Specifier Task → `docs/handoffs/current.md` |
| `/implement-handoff` | Implementer Task from current handoff |
| `/spec-and-implement <feature>` | Full pipeline (ask between phases) |
| `/lean-implement <feature>` | Thin handoff + implement in-supervisor |

See `.cursor/skills/spec-and-implement/SKILL.md`.

## During work

1. Branch: `feature/<feature-id>` (one branch per feature).
2. Load tracking: `./scripts/load-feature-issue.sh <feature-id>`
3. Start a task: `./scripts/github-issue-status.sh in-progress <task#> <parent#>`
4. Implement that task’s scope only; commit on the feature branch.
5. Close the **task** when done (`gh issue close <task#>`). Never close the parent feature until the PR merges with `Closes #<parent>`.

## Feature done + PR

```bash
FEATURE=opt-hnsw-vectors
REPO=SpektrNO/searchify
PR=123

./scripts/load-feature-issue.sh "$FEATURE"
# Close any remaining task sub-issues (not the parent)
gh issue close <task#> --repo "$REPO" --comment "Completed in PR #$PR"

./scripts/record-feature-complete.sh "$FEATURE" --issue <parent#> --note "PR #$PR"
# PR body should include: Closes #<parent>
```

Archive the handoff: move `docs/handoffs/current.md` → `docs/handoffs/archive/YYYY-MM-DD-<feature-id>.md`.
