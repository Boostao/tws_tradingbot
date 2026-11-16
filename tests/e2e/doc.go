// Package e2e implements the comprehensive, opaque-box, hermetic end-to-end (E2E)
// test suite for tws_traderbot (F1 Cockpit & Cobalt Transformation).
//
// Architectural Principles:
//
// 1. Opaque-Box Boundary Testing:
// Tests validate external behavior, public API contracts, SQLite database persistence,
// Wails v3 IPC service methods, and observable calculation results without relying on
// private implementation details.
//
// 2. Systematic 4-Tier Test Matrix:
//   - Tier 1 (Feature Coverage): Equivalence class partitioning covering all happy
//     paths (>=5 tests per feature across all 55 features = 275 tests).
//   - Tier 2 (Boundary & Corner Cases): Boundary, edge, and error-prone inputs
//     (zeros, negatives, overflows, empty inputs, sparse data, out-of-order bars,
//     and disconnected sockets) (>=5 tests per feature across all 55 features = 275 tests).
//   - Tier 3 (Cross-Feature Interactions): 15 multi-component interaction suites
//     testing end-to-end execution pipelines (normalization -> contract resolution
//     -> session gating -> 9-gate engine -> bracket orders -> dry-run ledger).
//   - Tier 4 (Real-World Scenarios): 6 realistic Cobalt operator workflows simulating
//     authentic trading workflows (Weinstein Phase 2 breakout, circuit breaker lockout & re-arm,
//     Canadian equity multi-venue routing, 1-click workspace swapping, ETH vs RTH session
//     gating, and Screener 2.0 triage to owned positions $$$$).
//
// 3. Hermetic & Offline Execution:
// 100% of Tiers 1-4 execute completely offline without requiring an active TWS UI,
// live IBKR Gateway, or internet connection. Tests utilize pure-Go in-memory SQLite
// (modernc.org/sqlite) and a deterministic MockIBClient.
//
// 4. Zero Markdown Discipline:
// Strictly ZERO .md files are placed under tests/ or any unallowlisted paths, guaranteeing
// 100% compliance with scripts/check-okf-hygiene.py. All architectural documentation and
// test specifications are preserved directly within Go source files and doc comments.
//
// Total Test Count: 571 tests (550 atomic tests + 21 scenario suites).
//
// Invocation:
//
//	go test -v -count=1 ./tests/e2e/...
//	task test:e2e
package e2e
