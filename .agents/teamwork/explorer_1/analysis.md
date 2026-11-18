# Analysis: Phase B Integration (Phase 5) Decomposition

## 1. Objective
E2E integration of the components built in M4 (Headless CLI, Autopilot, Replay, Paper-Probe) into the `tws_traderbot` system, enabling in-app observation, telemetry, and operational curation.

## 2. Milestone Decomposition

| Milestone | Objective | Focus |
| :--- | :--- | :--- |
| **B.1: Autopilot Backend & IPC** | Enable Wails/Go communication for Autopilot state. | `app_autopilot.go`, `bindings` generation. |
| **B.2: UI Observation Dashboard** | Implement `/monitoring` Autopilot panel. | Svelte frontend, HUD status, safety controls. |
| **B.3: Telemetry & Digest Pipeline** | E2E event stream and reporting. | Throttled events, PDF/JSON digests, OKF sync. |

## 3. Dependency Map

- **Dependency: Autopilot Engine (M4)** — Must be operational (Gates 1-9, chokepoint `PlaceAutopilotOrder`).
- **Dependency: Headless CLI (M4)** — Must be verified via `autopilot:replay` and `autopilot:probe`.
- **Dependency: OKF Curation** — Requires current `roadmap.md` and `capability_matrix.md` updates.

## 4. Feature Inventory Mapping

| Feature (Phase 5) | Components | M4 Foundation |
| :--- | :--- | :--- |
| Autopilot Status UI | Cockpit HUD, Panel | Autopilot Engine |
| Event Stream | Wails IPC Events | Async Journal Writer |
| Autopilot Controls | Start/Stop, Safety Inputs | `PlaceAutopilotOrder` / `StopAutopilot` |
| Session Digests | PDF/JSON Report | Flight Recorder |
| OKF Hygiene | Documentation | OKF Curating |

## 5. Conclusion
Phase B Integration transforms M4's technical building blocks into an operational reality for the Cobalt trader. The primary risks involve IPC overhead and UI state consistency. Focus on keeping the `backend/` engine pure and the `frontend/` UI reactive.
