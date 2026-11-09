# Handoff: Default-branch watch

**Status:** spec  
**Created:** 2026-11-09  
**Specifier:** spec complete  
**Developer:** pending

## GitHub tracking

| Field | Value |
|-------|-------|
| Feature id | `opt-watch-default-branch` |
| Parent issue | #104 — https://github.com/SpektrNO/searchify/issues/104 |
| Open tasks | `engine`, `verify`, `docs` |

Task order: `audit` → `spec` → `engine` → `verify` → `docs`

## Intent

While `SEARCHIFY_WATCH_PATHS` is on, index and recovery-scan a git working tree only when its checked-out branch is the remote default branch, so a checkout cannot overflow the watch queue into a recovery scan of every watch root.

## Technical contract

Follow [architecture.md](../architecture.md#default-branch-watch-opt-watch-default-branch). No new MCP tools, REST routes, or env vars. Manual `index` / `index_paths` / `POST /v1/index` stay ungated.

| Area | Requirement |
|------|-------------|
| MCP tools | Unchanged |
| Search mode | Unchanged |
| Performance | HEAD checks read the HEAD file only. Resolve the default branch once per repo per process; `git remote set-head <remote> --auto` at most once when `refs/remotes/<remote>/HEAD` is missing. A failed or unreachable remote leaves that repo on today's ungated behavior. |
| Acceptance | See below |

Acceptance:

- Non-git watch paths keep today's queue, overflow recovery, and rescan behavior.
- A repo with no remote is not gated.
- Remote choice is the checked-out branch's upstream remote, else the sole remote, else `origin`. Do not guess `main` or `master`.
- Default branch comes from `refs/remotes/<remote>/HEAD`. Cache it. Do not refresh it on later checkouts.
- On the default branch, events and recovery include that repo.
- On any other branch, or detached HEAD, drop that repo's pending paths and do not recovery-scan it. A checkout flood must not recovery-scan other watch roots.
- When HEAD moves onto the default branch, recovery-scan that repo only.
- `.git` stays skipped except reading HEAD (work tree `.git` directory or `gitdir:` file).

## Touchpoints

- `internal/local/watch.go` — admit events, drop pending, scope recovery
- New helper beside the watcher for work-tree HEAD and cached default branch
- `README.md` auto-index section and the bulk-ingestion note in `docs/architecture.md`
- Must not contradict [architecture.md](../architecture.md)

## Out of scope

- Gating CLI/MCP/REST index calls
- Guessing `main`/`master`, polling remotes, or refreshing a stale remote HEAD
- Branch-aware `index_status` fields
- Detaching watches or deleting indexed rows solely because HEAD left the default branch (leave the existing index until a later on-default rescan)

---

## Implementation result

*(Developer agent fills this section.)*

### Changes

- 

### Verification

- [ ] How tested
- [ ] What remains manual

### Deviations from spec

- None / list with rationale

### Follow-ups

- 
