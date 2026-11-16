# `okf/` — Self-Curating Operational Knowledge Framework Protocol

**Read root [`../AGENTS.md`](../AGENTS.md) and [`./00_index.md`](./00_index.md) before editing any specification.** Every engineering change that alters architecture, Wails IPC bindings, SQLite schema, strategy evaluation, or settled product policies must leave `okf/` synchronized in the same commit.

## 1. Zero Stray Markdown Rule (Enforced Allowlist)

- **Never create ad-hoc `.md` notes** (`*_NOTES.md`, `*_AUDIT.md`, `*_DESIGN.md`, `TODO.md`) inside `backend/`, `frontend/`, `build/`, or `scripts/`.
- Only allowlisted markdown paths may exist in the repository, mechanically enforced by [`../scripts/check-okf-hygiene.py`](../scripts/check-okf-hygiene.py):
  - Root docs: `README.md`, `AGENTS.md`
  - Scoped agent rules: `okf/AGENTS.md`
  - Automation & skills: `.github/**/*.md`, `.agents/**/*.md`
  - Knowledge & archives: `okf/**/*.md`, `archive/**/*.md`

## 2. Directory Lifecycle & In-Place Truth Rules

| Directory | Lifecycle Role | Editing Rule |
|---|---|---|
| **`01_history_and_evolution/` & `../archive/`** *(optional)* | Frozen historical provenance. | **Immutable reference only.** Never treat as active product scope or feature gating. |
| **`02_present_state/` & `05_engineering_and_agents/`** | Living, in-place truth of current `HEAD` (system architecture, SQLite schema, TWS integration, strategy engine, Go/Wails/Svelte playbook). | **Overwrite stale facts in place** (symbol names, file paths, `Status:` banners). **Never append dated changelogs.** |
| **`03_future_architecture/`** | Unbuilt proposals and roadmap items. | **Graduate to `02_present_state/` when shipped** and update links in [`00_index.md`](./00_index.md). |
| **`04_evidence_and_sources/`** | Canonical ADR & ambiguity register [`decisions_and_gaps.md`](./04_evidence_and_sources/decisions_and_gaps.md). | Append numbered **`D-XX`** entries only for settled architecture/trading-policy decisions or open ambiguities. |

## 3. Same-Commit Synchronization Trigger & Hygiene Gate

Update `okf/` **in the same commit** whenever a change modifies:
1. **Wails app services (`main.go`, `app*.go`), `backend/api/`, or `frontend/` views** $\rightarrow$ update [`00_index.md`](./00_index.md), [`02_present_state/system_architecture.md`](./02_present_state/system_architecture.md), and [`05_engineering_and_agents/engineering_playbook.md`](./05_engineering_and_agents/engineering_playbook.md).
2. **SQLite schema (`backend/db/`), domain models (`backend/models/`), or strategy/bot execution (`backend/strategy/`, `backend/bot/`, `backend/wrapper/`)** $\rightarrow$ update [`02_present_state/system_architecture.md`](./02_present_state/system_architecture.md) and log a **`D-XX`** entry in [`04_evidence_and_sources/decisions_and_gaps.md`](./04_evidence_and_sources/decisions_and_gaps.md) for any settled policy change.
3. **Any new `.md` file under `okf/01_*`..`okf/05_*`** $\rightarrow$ register it in [`00_index.md`](./00_index.md).

Before committing, verify zero markdown bloat, zero broken relative links, and rule byte budgets (`< 24,000` bytes):

```bash
python3 scripts/check-okf-hygiene.py
```
