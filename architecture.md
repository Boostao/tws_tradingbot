# Project Architecture

This document reflects the current local TWS trading architecture.

```mermaid
graph TD
    A[User] --> B[SvelteKit Split UI]
    B --> C[FastAPI API]
    B --> D[Sidebar Runtime Controls]

    C --> E[Cockpit Router]
    C --> F[Watchlist Router]
    C --> G[Strategy Router]
    C --> H[Config Router]
    C --> I[State Router]

    E --> J[config/cockpit.json]
    F --> K[config/watchlist.json]
    F --> L[config/watchlist.txt]
    G --> M[config/active_strategy.json]
    H --> N[config/default.yaml]
    I --> O[config/.bot_state.json]

    I --> P[LiveTradingRunner]
    P --> Q[RuleEngine]
    Q --> R[IndicatorFactory]
    P --> S[TWSDataProvider]
    P --> T[Trade Ledger]

    S --> U[TWS / IB Gateway]

    F --> W[TradingView feed import]

    subgraph Runtime
        P
        Q
        R
        S
        T
    end

    subgraph Persistence
        J
        K
        L
        M
        N
        O
    end
```

## Notes

- The cockpit enforces one active strategy slot per workspace.
- The live runner resolves watchlist instruments into `SYMBOL.VENUE` identifiers and reloads ticker/strategy enablement at cycle boundaries.
- The runtime now fetches per-symbol and per-timeframe market-data bundles for rule evaluation.
- The runtime requests market data for the full watchlist feed universe and only executes orders for currently enabled strategy and ticker targets.
- The direct `ibapi` TWS path is the only live execution path in this branch.