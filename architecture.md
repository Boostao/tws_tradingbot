# Project Architecture

This document provides a high-level overview of the TWS Trader Bot project's architectural structure using Mermaid.js.

```mermaid
graph TD
    A[User] --> B[SvelteKit UI]
    B --> C[FastAPI API]
    C --> D[Strategy Services]

    D --> I[Rule Models]
    D --> J[Pine Script Generator]

    C --> K[API Routers]
    K --> L[Strategy Router]
    K --> M[Watchlist Router]
    K --> N[Symbols Router]

    B --> O[UI Routes]
    O --> P[Watchlist Manager]
    O --> Q[Strategy Builder]
    Q --> R[Pine Script Output]

    C --> S[Config Files]
    S --> T[config/watchlist.txt]
    S --> U[config/active_strategy.json]

    M --> V[TradingView Symbol Cache]
    L --> J

    subgraph "External Services"
        V
    end

    subgraph "Configuration"
        S
    end
```