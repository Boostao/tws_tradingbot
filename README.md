# TWS TraderBot (Go / Wails Edition)

TWS TraderBot Go is a fast, high-performance, single-binary trading application for Interactive Brokers (IBKR/TWS). It replaces the legacy Python FastAPI backend with a fully native Golang implementation using [Wails](https://wails.io/), while preserving the snappy Svelte SvelteKit frontend.

This architecture fundamentally redesigns the local execution workflow for production readiness and seamless deployments.

---

## 🏗️ Python to Go Translation Notes & Architecture

The previous stack relied on a split FastAPI server and a separate React/Svelte Web UI server communicating via HTTP REST. The Go re-write bundles everything into a single desktop execution window.

### 1. Communication (REST ➔ Wails IPC)
- **Legacy**: `GET /api/v1/cockpit`, `PUT /api/v1/watchlist` over `localhost:8000`.
- **Go**: Uses native Wails inter-process communication (IPC). Functions like `App.UpdateCockpitState()` are bound directly to the frontend's window object. No local web server required. Data types natively map from Go structs to TypeScript interfaces via `wails generate module`.

### 2. State & Database (JSON Files ➔ SQLite)
- **Legacy**: Loaded memory dictionaries parsed from static JSON files (`active_strategy.json`, `cockpit.json`).
- **Go**: Embeds `modernc.org/sqlite` database locally. Cockpit states, global toggles, and UI user preferences are strictly typed in Go and saved transparently to the `system_config` SQL table. 

### 3. Rules Engine (`eval()` ➔ Native Go Structs)
- **Legacy**: Strategy calculations and pine-script generation ran dynamically via Python's `eval()` logic.
- **Go**: PineScript AST concepts have been dropped entirely. Strategies map securely to typed `Rule`, `Condition`, `ActionType`, and `Indicator` structs. Array math like SMA, EMA, Slope, and Crossover logic are natively processed across floating-point slices concurrently in the `backend/strategy` module.

### 4. IBKR Integration (`ib_insync`/`ibapi` ➔ `scmhub/ibapi`)
- **Legacy**: Python's `ibapi` event loops and custom wrappers.
- **Go**: Uses github.com/scmhub/ibapi acting via TCP sockets on `127.0.0.1:7497`. A specialized `IBWrapper` feeds tick and historical blocks cleanly into the `Engine.EvaluateTick()` channel.

---

## 📚 User Manual

### Prerequisites (Production / Running)
**Running the bot requires NO development tools.** Karena the Go app compiles down to a single standalone static binary, you only need:
- **The Executable**: The `.exe`, `.app`, or binary file for your OS.
- **TWS / IB Gateway**: Running locally and configured to accept Socket connections on `127.0.0.1:7497`.

### Prerequisites (Development / Compiling)
If you wish to build the app from source or modify the code, you will need:
- **Go**: 1.25+ (`go version`)
- **Node.js**: 18+ (For compiling the Svelte frontend during builds)
- **Wails v2**: Installed correctly globally (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Dynamic Reloading & Execution Logic
Because the app is a single static binary, dynamic execution is actually **faster and safer** than the old Python script. 
In Python, the bot had to re-read the `watchlist.json` and `active_strategy.json` from the hard drive at the top of every loop. 
In Go, the UI and the Bot Engine share the same memory space:
- **Instant State Toggles**: When you toggle a ticker on/off in the Svelte UI, Wails instantly updates the Go backend. Go writes this to the local SQLite database and immediately updates the `Engine`'s active memory pointer.
- **No Restarts Required**: The `EvaluateTick` loop dynamically checks the current in-memory configurations for active tickers and strategy rules on every single tick, without needing to boot up or restart the internal IBKR socket.

### Development Mode
To run the application with hot-reloading for both the Go backend and Svelte frontend:
```bash
wails dev
```
*This spins up the Svelte dev server and the Go application hook simultaneously. If you edit Go files, the bot logic recompiles. If you edit Svelte files, the UI refreshes instantly.*

### Testing the Bot Engine
The mathematical trading evaluator is unit-tested without needing TWS connections:
```bash
go test -v ./backend/bot ./backend/strategy ./backend/db
```

### Connecting to TWS and Executing
When you boot the app, the Bot Runner initializes automatically.
1. Ensure your IBKR Paper Trading account is running.
2. In TWS, go to **Settings > API > Settings** and verify `Enable ActiveX and Socket Clients` is checked, and the Socket Port is `7497`.
3. Open the **TraderBot UI (Cockpit)**.
4. Set up your Watchlist.
5. Toggle the global **Bot Start** enablement flag on the UI. The Go runner immediately initializes the `scmhub/ibapi` TCP hook.

---

## 🚀 Compiling for Production & GitHub Actions

One of the vital milestones of the Go rewrite is generating cross-platform binary distributions for releases, ensuring you no longer need Python environments to run the bot.

### Local Build
To create a standalone production executable (e.g., `tws_traderbot_go.exe`, `.app`, or `.bin` depending on your OS):
```bash
wails build -clean -m
```
*The resulting executable is housed in the `build/bin/` folder. It bundles the Svelte assets directly inside the Go binary. You can port this file to any machine with TWS.*

### Deploying via GitHub Actions
A robust CI/CD workflow is provided inside `.github/workflows/release.yml`. It has been adapted specifically to replace the Python checks with native Golang validations.

**Triggering a Release:**
The pipeline automatically runs whenever you push a semantic versioning flag to your GitHub repository.

```bash
git tag v1.0.0
git push origin v1.0.0
```

**Pipeline Steps:**
1. **Tests Check**: Runs `go mod tidy` and executes all unit tests (`go test ./...`) making sure your Rules Engine doesn't throw a regression error. 
2. **Matrix Build**: Utilizing the `wailsapp/wails` action cache, it boots environments for `ubuntu-latest`, `windows-latest`, and `macos-latest`. 
3. **Node/Go Alignment**: Prepares SvelteKit Node requirements and compiles the embedded architecture via `wails build`.
4. **GitHub Releases**: Takes the final `.exe`, `.app` bundle, and linux binary, zipping them automatically into a new Github Release tagged with your version.
