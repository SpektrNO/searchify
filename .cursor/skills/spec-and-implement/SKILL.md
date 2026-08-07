---
name: spec-and-implement
description: >-
  Spec→implement pipeline with two modes: full multi-agent (Task workers) or lean
  (supervisor thin handoff + implement in-chat). Use for /spec-only, /implement-handoff,
  /spec-and-implement, /lean-implement, or --lean. Accepts feature issue # or feature id.
disable-model-invocation: true
---

# Spec → Implement pipeline

**Modes (manual — do not auto-pick):**

| Mode | Trigger | Who does the work |
|------|---------|-------------------|
| **full** (default) | `/spec-and-implement`, `/spec-only`, `/implement-handoff` without `--lean` | Supervisor Phase 0 + **Task** workers + Phase 3 PR |
| **lean** | `/lean-implement <feature>` **or** `/spec-and-implement <feature> --lean` / `— lean` | Supervisor Phase 0 + **thin handoff + implement in this chat** + Phase 3 PR |

Shared in **both** modes: Phase 0-pre archive, `feature/<id>` branch, parent issue link, GitHub task status (close **tasks** only), commits on the feature branch, Phase 3 PR with `Closes #<parent>`.

---

## Full mode (multi-agent)

**Supervisor (this chat):** Phase 0 (issue + branch) → Task launches → summaries → Phase 3 (PR).  
**Workers:** Cursor Task subagents — never "read the agent file and act as persona" in the supervisor chat.

| Phase | Who | Mechanism |
|-------|-----|-----------|
| 0 — Clear previous handoff | Supervisor | Verify prior feature complete → archive `current.md` |
| 0 — Resolve feature + branch | Supervisor | `load-feature-issue.sh` + git checkout/create `feature/<id>` |
| 1 — Product specify | `specifier` | **Task** using `.cursor/agents/specifier.md` |
| 2 — Developer implement | `developer-implementer` | **Task** using `.cursor/agents/developer-implementer.md` |
| 3 — Pull request | Supervisor | ensure pushed → `gh pr create` linking parent issue |

**Hard rules for the supervisor (full mode)**

1. Do **not** write `docs/handoffs/current.md` yourself when Phase 1 is required.
2. Do **not** edit production code / close implementation issues yourself when Phase 2 is required.
3. Do **not** merge Phase 1 and Phase 2 into one Task.
4. Pass a **self-contained prompt** into each Task (workers do not see this chat's history).
5. Shared memory between agents is `docs/handoffs/current.md` (+ GitHub sub-issues).
6. **One branch per feature:** `feature/<feature-id>`. Never create task branches.
7. Do **not** launch Task workers until the correct feature branch is checked out.
8. **Workers commit (+ push) per closed task**. Supervisor does not squash unless the user asks.

---

## Lean mode (hybrid)

**When to use:** Small infra tweaks, one-layer fixes, or when architecture already defines the contract. Prefer **full** for new product surfaces or unclear UX.

**Supervisor does the work** — no Task launches unless you escalate explicitly.

| Phase | Who | Mechanism |
|-------|-----|-----------|
| 0-pre / 0a / 0b | Supervisor | Same as full |
| 1 — Thin handoff | Supervisor | Short `docs/handoffs/current.md`: Intent, acceptance, MCP/CLI/REST contract, GitHub tracking |
| 2 — Implement | Supervisor | Follow `.cursor/agents/developer-implementer.md` as a **checklist** in this chat (not a Task). Close task issues; commit+push per closed task. |
| 3 — PR | Supervisor | Same as full (`Closes #<parent>`) |

**Lean rules**

1. Still run Phase 0-pre when starting a **new** feature.
2. Still use `feature/<feature-id>`; never close the parent feature issue.
3. Do **not** Task-launch specifier/implementer in lean mode.
4. Thin handoff is enough — do not over-specify.
5. Announce once: *"Lean mode: implementing in-supervisor."*

---

Run phases **in order**. Do not skip Phase 0 when the user gives an issue # or feature name.

### GitHub CLI + sandbox (required)

`gh` talks to `api.github.com`. Cursor's default shell sandbox often returns **Forbidden**.

Supervisor and Task workers must use Shell `required_permissions: ["full_network"]` (or `["all"]`) for:

- `./scripts/load-feature-issue.sh`
- `./scripts/github-issue-status.sh`
- `gh issue close` / `gh issue view` / `gh pr create`
- `git push` / remote fetch

Do **not** treat sandbox Forbidden as "no GitHub issues."

Full-mode Task prompt reminder:

```text
Shell: for any gh / scripts/load-feature-issue.sh / github-issue-status.sh call,
use required_permissions: ["full_network"] (retry with ["all"] if still Forbidden).
Do not conclude issues are missing from a sandbox network error.
Stay on branch feature/<feature_id> — do not create other branches.
After each closed task: git commit (HEREDOC, why-focused) then git push. No empty commits. No PR.
Never close the parent feature issue.
```

---

## Phase 0 — Resolve feature + git branch (supervisor)

| Input | Example |
|-------|---------|
| Issue number | `42`, `#42` |
| Feature id | `scaffold-monorepo` |
| Title fragment | `monorepo` |
| Current branch | `feature/scaffold-monorepo` → id `scaffold-monorepo` |

Detect mode first: if the user message contains `/lean-implement`, `--lean`, or `— lean` → **lean**; otherwise **full**.

### 0-pre — Clear previous handoff (before any new feature)

Run at the start of `/spec-only`, `/spec-and-implement`, `/lean-implement`, or when starting a **different** feature than `docs/handoffs/current.md`. Skip when continuing the **same** feature (`/implement-handoff` on Status `spec` / `implementing`).

1. Read `docs/handoffs/current.md`.
2. If empty / placeholder only → proceed to 0a.
3. Otherwise verify completion: Status `done`; backlog **✅**; all **task** sub-issues closed (parent may stay open for PR merge); prefer entry in `feature-completed.md`.
4. If any check fails → **stop**. Ask user. Do not overwrite `current.md`.
5. If checks pass → archive:
   ```bash
   mv docs/handoffs/current.md docs/handoffs/archive/YYYY-MM-DD-<prior-feature-id>.md
   ```
   Prefer restore slot from `docs/handoffs/_template.md`.
6. Commit + push archive when appropriate.
7. Continue with 0a for the **new** feature.

See `docs/handoffs/archive/README.md`.

### 0a — Feature issue

1. `./scripts/load-feature-issue.sh <input>` with `full_network`.
2. Retry network/`Forbidden` with `full_network` or `all` before giving up. Auth-only failure → backlog fallback.
3. Keep: parent `#`, URL, `feature_id`, open tasks, next slug, `feature/<feature_id>`, **mode** full|lean.

### 0b — Feature branch (before work)

Canonical: **`feature/<feature-id>`**.

1. `git branch --show-current` / `git status -sb`.
2. Already on target → continue.
3. Else checkout/create (ask if unrelated dirty tree); fetch if needed.
4. Link parent issue to branch (`gh issue develop` or comment `Working branch: feature/<id>`).
5. Confirm: *"On `feature/<id>` for #<parent> (mode: full|lean)."*

### Task slug map

| Slug | Phase | Full worker | Lean |
|------|-------|-------------|------|
| `audit` / `spec` | 1 | `specifier` Task | Supervisor thin handoff |
| `engine`…`docs` | 2 | `developer-implementer` Task | Supervisor in-chat |

**Never** `gh issue close` the **parent** feature. Close **task** issues only.

---

## Phase 1 — Product specify

### Full → Task `specifier`

1. Launch Task; wait; show handoff summary.
2. Stop unless `— full` / confirmed. Ask: *"Proceed to implementation?"*

### Lean → supervisor thin handoff

1. Short `docs/handoffs/current.md` (Intent, acceptance, MCP/CLI/REST contracts, GitHub tracking).
2. Close `spec`/`audit` tasks; commit+push.
3. Continue to lean Phase 2 without asking.

### Phase 1 Task prompt template (full only)

```text
You are running as the specifier subagent for Searchify.
Follow .cursor/agents/specifier.md. Do not write production code.

Feature id: <feature_id>
Parent issue: #<n> — <url>
Git branch: feature/<feature_id>
Open Phase-1 tasks: <audit/spec>
Spec path: <from issue or backlog>

1. load-feature-issue.sh if needed.
2. audit/spec only → docs/handoffs/current.md from _template.md.
3. GitHub tracking; Status spec; close task issues (never parent).
4. Commit+push per closed Phase-1 task.
5. End with 3–5 bullet handoff summary + commit subjects.

Shell: gh/issue scripts need required_permissions: ["full_network"] (retry ["all"]).
Stay on feature/<feature_id>. No other branches. No PR. Never close parent.
```

---

## Phase 2 — Developer implement

### Full → Task `developer-implementer`

1. Launch Task; wait; summarize; do not re-implement.
2. Phase 3 when Status `done`.

### Lean → supervisor in-chat

1. Status `implementing`.
2. Implement engine→docs via developer-implementer checklist.
3. Commit+push per closed task; never close parent.
4. record-feature-complete.sh; Status `done`; Phase 3.

### Phase 2 Task prompt template (full only)

```text
You are running as the developer-implementer subagent for Searchify.
Implement only from docs/handoffs/current.md. Follow .cursor/agents/developer-implementer.md.

Feature id: <feature_id>
Parent: #<n>
Branch: feature/<feature_id>

1. load-feature-issue.sh for open engine…docs tasks.
2. Status implementing; implement in order; close tasks only.
3. Commit+push per task with diffs; skip empty commits.
4. Implementation result; Status done|blocked; record-feature-complete.sh.
5. Report files, verification, deviations, closed issues, commit subjects.
6. No PR — supervisor Phase 3.

Shell: full_network for gh. Stay on feature branch. Never close parent.
```

---

## Phase 3 — Pull request (supervisor)

After Status `done` (full, lean, or `/implement-handoff`):

1. Confirm branch `feature/<feature_id>`.
2. Commit leftovers if dirty; do not squash unless asked.
3. `git push -u origin HEAD`.
4. `gh pr create` with body including **`Closes #<parent>`**.
5. Return PR URL; optional comment on parent issue.

`/spec-only` (full): skip Phase 3 unless asked for a draft PR.

---

## User shortcuts

| User says | Mode | Behavior |
|-----------|------|----------|
| `/spec-and-implement <feature>` | full | 0-pre → 0 → Task Phase 1 → ask → Task Phase 2 → PR |
| `/spec-and-implement <feature> — full` | full | No pause between 1 and 2 |
| `/spec-and-implement <feature> --lean` / `— lean` | lean | 0-pre → 0 → thin handoff → in-chat implement → PR |
| `/lean-implement <feature>` | lean | Same as `--lean` |
| `/spec-only <feature>` | full | Task Phase 1 only |
| `/implement-handoff` | full | Skip 0-pre → Task Phase 2 → PR |
| `/implement-handoff --lean` | lean | Skip 0-pre → in-chat implement → PR |

---

## How to verify mode

- **Full:** separate Task runs; then PR URL.
- **Lean:** no specifier/implementer Task; announce lean mode; then PR URL.
