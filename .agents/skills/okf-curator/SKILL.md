---
name: okf-curator
description: Synchronizes the Operational Knowledge Framework (`okf/`), master index (`okf/00_index.md`), and decision ledger (`okf/04_evidence_and_sources/decisions_and_gaps.md`) after architectural, Wails IPC, SQLite schema, or strategy/execution changes, and enforces zero stray markdown sprawl.
---

# OKF Curator — Self-Curating Knowledge Sync Workflow

Follow [`okf/AGENTS.md`](../../../okf/AGENTS.md) whenever code, Wails bindings, SQLite schemas, strategy rules, or architectural specs change so `okf/` remains the single live source of truth at `HEAD`.

## When to Trigger

Run this curation pass in the same commit whenever you:
- Add, remove, or change Wails-exposed methods/models (`main.go`, `app*.go`, `backend/api/`, `backend/models/`) or active frontend views (`frontend/src/`).
- Modify SQLite schema (`backend/db/`), TWS wrapper behavior (`backend/wrapper/`), or strategy/execution engine rules (`backend/strategy/`, `backend/bot/`).
- Graduate a design out of `okf/03_future_architecture/` into `okf/02_present_state/` or resolve a `D-XX` decision item.
- Prepare a release or merge commit.

## Step-by-Step Curation Protocol

1. **Prevent stray markdown sprawl:**
   - Do not leave ad-hoc `*.md` notes in `backend/`, `frontend/`, `build/`, or `scripts/`.
   - Consolidate active domain/engineering facts into `okf/02_present_state/` or `okf/05_engineering_and_agents/`.
2. **Update `02_present_state/` & `05_engineering_and_agents/` in place:**
   - Overwrite stale symbol names, file paths, and `Status:` banners directly in place.
   - Never append dated "update logs" to present-state specifications.
   - Graduate any newly implemented feature out of `okf/03_future_architecture/` into `okf/02_present_state/` and update links accordingly.
3. **Record settled decisions in `decisions_and_gaps.md`:**
   - Log or update numbered **`D-XX`** ADR entries in `okf/04_evidence_and_sources/decisions_and_gaps.md` with status, file references, and exact invariants.
4. **Refresh `okf/00_index.md`:**
   - Ensure the `**Audit Baseline Date:**` header and directory manifest table match `okf/` on disk, and verify every `.md` file under `okf/01_*`..`okf/05_*` is indexed.
5. **Run mechanical hygiene verification:**
   ```bash
   python3 scripts/check-okf-hygiene.py
   python3 -m unittest discover -s scripts/tests -p 'test_*.py'
   ```
   Fix any reported allowlist violation, rule byte-budget breach (`>= 24,000` bytes), missing index reference, or broken relative link before committing.
