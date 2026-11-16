package bot

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The Engine abstracts the fail-closed execution loop, risk breaker, and safety gates.
type Engine struct {
	mu                 sync.RWMutex
	ActiveStrategy     *models.Strategy
	isRunning          atomic.Bool
	RiskBreakerTripped atomic.Bool
	dryRun             atomic.Bool
	globalEnabled      atomic.Bool

	CockpitState *models.CockpitStateResponse
	App          *application.App
	db           *db.Database

	sessionGate  *SessionGate
	timeProvider func() time.Time

	dailyLossLimit float64
	realizedPnL    float64
	unrealizedPnL  float64
	totalEquity    float64

	watchlistItems map[string]*models.WatchlistItem
	latestPrices   map[string]float64

	bracketEnabled bool
	takeProfitPct  float64
	stopLossPct    float64
}

// NewEngine creates an initialized Engine in fail-closed default state.
func NewEngine() *Engine {
	e := &Engine{
		sessionGate:    NewSessionGate(),
		timeProvider:   time.Now,
		dailyLossLimit: 500.0,
		totalEquity:    100000.0,
		watchlistItems: make(map[string]*models.WatchlistItem),
		latestPrices:   make(map[string]float64),
		takeProfitPct:  2.0,
		stopLossPct:    1.0,
	}

	e.isRunning.Store(false)
	e.RiskBreakerTripped.Store(false)
	e.dryRun.Store(true)        // Fail-safe: dry-run enabled by default
	e.globalEnabled.Store(false) // Coupe-circuit: disabled by default

	// Initialize default CockpitState with 3 workspaces and 1/1 strategy slot invariant
	e.CockpitState = &models.CockpitStateResponse{
		GlobalEnabled:     false,
		ActiveWorkspaceID: "achat",
		Workspaces: []models.CockpitWorkspace{
			{
				ID:      "achat",
				Name:    "Achat",
				Kind:    "long",
				Enabled: true,
				StrategySlots: []models.CockpitStrategySlot{
					{
						ID:         "slot_achat_1",
						Label:      "Slot 1",
						StrategyID: "default",
						Enabled:    true,
					},
				},
			},
			{
				ID:      "short",
				Name:    "Short",
				Kind:    "short",
				Enabled: false,
				StrategySlots: []models.CockpitStrategySlot{
					{
						ID:         "slot_short_1",
						Label:      "Slot 1",
						StrategyID: "",
						Enabled:    false,
					},
				},
			},
			{
				ID:      "swing",
				Name:    "Swing",
				Kind:    "swing",
				Enabled: false,
				StrategySlots: []models.CockpitStrategySlot{
					{
						ID:         "slot_swing_1",
						Label:      "Slot 1",
						StrategyID: "",
						Enabled:    false,
					},
				},
			},
		},
	}

	return e
}

// AttachDB binds the SQLite database for execution ledger and watchlist lookups.
func (e *Engine) AttachDB(database *db.Database) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.db = database
}

// AttachApp gives the engine access to Wails runtime events.
func (e *Engine) AttachApp(app *application.App) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.App = app
}

// SetDryRun configures dry-run execution mode.
func (e *Engine) SetDryRun(dryRun bool) {
	e.dryRun.Store(dryRun)
}

// IsDryRun reports whether dry-run execution guard is active.
func (e *Engine) IsDryRun() bool {
	return e.dryRun.Load()
}

// SetGlobalEnabled arms or disarms the master circuit breaker ("Coupe-circuit principal").
func (e *Engine) SetGlobalEnabled(enabled bool) {
	e.globalEnabled.Store(enabled)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.CockpitState != nil {
		e.CockpitState.GlobalEnabled = enabled
	}
}

// IsGlobalEnabled returns whether the master circuit breaker is armed.
func (e *Engine) IsGlobalEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.CockpitState != nil {
		return e.CockpitState.GlobalEnabled
	}
	return e.globalEnabled.Load()
}

// IsRunning returns whether the bot engine is currently running.
func (e *Engine) IsRunning() bool {
	return e.isRunning.Load()
}

// SetDailyLossLimit sets the threshold for automated daily loss risk breaker trip.
func (e *Engine) SetDailyLossLimit(limit float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit >= 0 {
		e.dailyLossLimit = limit
	}
}

// GetDailyLossLimit returns the daily loss limit.
func (e *Engine) GetDailyLossLimit() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.dailyLossLimit
}

// SetEquity sets the current portfolio equity for % position sizing.
func (e *Engine) SetEquity(equity float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if equity > 0 {
		e.totalEquity = equity
	}
}

// GetEquity returns the current portfolio equity.
func (e *Engine) GetEquity() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalEquity
}

// SetBracketConfig configures OCA bracket order parameters.
func (e *Engine) SetBracketConfig(enabled bool, tpPct, slPct float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bracketEnabled = enabled
	if tpPct > 0 {
		e.takeProfitPct = tpPct
	}
	if slPct > 0 {
		e.stopLossPct = slPct
	}
}

// IsBracketEnabled returns whether OCA child bracket orders are enabled.
func (e *Engine) IsBracketEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.bracketEnabled
}

// GetBracketPercentages returns take profit and stop loss percentages.
func (e *Engine) GetBracketPercentages() (float64, float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.takeProfitPct, e.stopLossPct
}

// SetTimeProvider overrides the time retrieval function for deterministic testing.
func (e *Engine) SetTimeProvider(fn func() time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if fn != nil {
		e.timeProvider = fn
	} else {
		e.timeProvider = time.Now
	}
}

// Now returns the current time from the configured time provider.
func (e *Engine) Now() time.Time {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.timeProvider != nil {
		return e.timeProvider()
	}
	return time.Now()
}

// SetLatestPrice records the most recent market price for a symbol.
func (e *Engine) SetLatestPrice(symbol string, price float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	norm := models.NormalizeSymbol(symbol)
	if price > 0 {
		e.latestPrices[symbol] = price
		e.latestPrices[norm.Symbol] = price
		e.latestPrices[norm.InstrumentID] = price
	} else {
		delete(e.latestPrices, symbol)
		delete(e.latestPrices, norm.Symbol)
		delete(e.latestPrices, norm.InstrumentID)
	}
}

// GetLatestPrice returns the latest recorded market price for a symbol.
func (e *Engine) GetLatestPrice(symbol string) float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if p, ok := e.latestPrices[symbol]; ok && p > 0 {
		return p
	}
	norm := models.NormalizeSymbol(symbol)
	if p, ok := e.latestPrices[norm.Symbol]; ok && p > 0 {
		return p
	}
	if p, ok := e.latestPrices[norm.InstrumentID]; ok && p > 0 {
		return p
	}
	return 0
}

// SetWatchlistItem stores or updates a watchlist item in the engine's registry.
func (e *Engine) SetWatchlistItem(item *models.WatchlistItem) {
	if item == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.watchlistItems[item.Symbol] = item
	if item.InstrumentID != "" {
		e.watchlistItems[item.InstrumentID] = item
	}
	norm := models.NormalizeSymbol(item.Symbol)
	e.watchlistItems[norm.InstrumentID] = item
	e.watchlistItems[norm.Symbol] = item
}

// GetWatchlistItem retrieves a watchlist item by symbol or instrument ID.
func (e *Engine) GetWatchlistItem(symbol string) *models.WatchlistItem {
	e.mu.RLock()
	if item, ok := e.watchlistItems[symbol]; ok {
		e.mu.RUnlock()
		return item
	}
	norm := models.NormalizeSymbol(symbol)
	if item, ok := e.watchlistItems[norm.Symbol]; ok {
		e.mu.RUnlock()
		return item
	}
	if item, ok := e.watchlistItems[norm.InstrumentID]; ok {
		e.mu.RUnlock()
		return item
	}
	dbRef := e.db
	e.mu.RUnlock()

	// If not found in cache and DB is attached, query SQLite
	if dbRef != nil && dbRef.Conn() != nil {
		row := dbRef.Conn().QueryRow(`
			SELECT id, watchlist_id, symbol, exchange, venue, instrument_id, name,
			       enabled, tier, is_orderable, is_regime_reference, is_holding, size_mode, size_value
			FROM watchlist_items
			WHERE symbol = ? OR instrument_id = ? OR symbol = ?
			LIMIT 1
		`, symbol, symbol, norm.Symbol)

		var it models.WatchlistItem
		err := row.Scan(
			&it.ID, &it.WatchlistID, &it.Symbol, &it.Exchange, &it.Venue, &it.InstrumentID, &it.Name,
			&it.Enabled, &it.Tier, &it.IsOrderable, &it.IsRegimeReference, &it.IsHolding, &it.SizeMode, &it.SizeValue,
		)
		if err == nil {
			e.SetWatchlistItem(&it)
			return &it
		}
	}

	return nil
}

// UpdatePnL updates realized and unrealized PnL and monitors against daily_loss_limit.
// Feature 12: Automated Daily Loss Risk Breaker trips automatically when breached.
func (e *Engine) UpdatePnL(realized, unrealized float64) {
	e.mu.Lock()
	e.realizedPnL = realized
	e.unrealizedPnL = unrealized
	total := realized + unrealized
	limit := e.dailyLossLimit
	app := e.App
	e.mu.Unlock()

	// Risk Breaker Check: if limit > 0 and total PnL <= -dailyLossLimit, automatic trip
	if limit > 0 && total <= -limit {
		e.RiskBreakerTripped.Store(true)
		e.SetGlobalEnabled(false)

		if app != nil {
			app.Event.Emit("circuit_breaker_tripped", map[string]interface{}{
				"realized":   realized,
				"unrealized": unrealized,
				"total_pnl":  total,
				"limit":      limit,
				"reason":     fmt.Sprintf("Daily loss limit breached: total PnL $%.2f <= -$%.2f", total, limit),
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			})

			msg := fmt.Sprintf("[%s] 🚨 COUPE-CIRCUIT DÉCLENCHÉ: Perte journalière ($%.2f) excède la limite ($%.2f). Exécution automatique suspendue.",
				time.Now().Format("15:04:05"), total, limit)
			app.Event.Emit("ledger_entry", msg)
		}
	}
}

// GetPnL returns realized, unrealized, and combined daily PnL.
func (e *Engine) GetPnL() (realized, unrealized, total float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.realizedPnL, e.unrealizedPnL, e.realizedPnL + e.unrealizedPnL
}

// ResetCircuitBreaker manually re-arms the risk breaker and master circuit breaker.
func (e *Engine) ResetCircuitBreaker() error {
	e.mu.RLock()
	limit := e.dailyLossLimit
	totalPnL := e.realizedPnL + e.unrealizedPnL
	app := e.App
	e.mu.RUnlock()

	// Guard: cannot reset if daily loss is actively breached
	if limit > 0 && totalPnL <= -limit {
		return fmt.Errorf("cannot reset circuit breaker: daily loss limit actively breached (PnL: %.2f, Limit: %.2f)", totalPnL, limit)
	}

	e.RiskBreakerTripped.Store(false)
	e.SetGlobalEnabled(true)

	if app != nil {
		app.Event.Emit("circuit_breaker_reset", map[string]interface{}{
			"status":    "ARMED",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		msg := fmt.Sprintf("[%s] 🛡️ Coupe-circuit réarmé manuellement. Prêt pour l'exécution.", time.Now().Format("15:04:05"))
		app.Event.Emit("ledger_entry", msg)
	}

	return nil
}

// SetCockpitState stores updated cockpit state and synchronizes globalEnabled flag.
func (e *Engine) SetCockpitState(state *models.CockpitStateResponse) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.CockpitState = state
	if state != nil {
		e.globalEnabled.Store(state.GlobalEnabled)
	}
}

// GetCockpitState retrieves current cockpit state.
func (e *Engine) GetCockpitState() *models.CockpitStateResponse {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.CockpitState
}

// GetActiveWorkspace returns the currently selected and active CockpitWorkspace.
func (e *Engine) GetActiveWorkspace() *models.CockpitWorkspace {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getActiveWorkspaceLocked()
}

// getActiveWorkspaceLocked returns the active workspace assuming e.mu (at least RLock) is already held.
func (e *Engine) getActiveWorkspaceLocked() *models.CockpitWorkspace {
	if e.CockpitState == nil || len(e.CockpitState.Workspaces) == 0 {
		return nil
	}

	targetID := e.CockpitState.ActiveWorkspaceID
	if targetID != "" {
		for i := range e.CockpitState.Workspaces {
			ws := &e.CockpitState.Workspaces[i]
			if ws.ID == targetID {
				return ws
			}
		}
		return nil
	}

	for i := range e.CockpitState.Workspaces {
		ws := &e.CockpitState.Workspaces[i]
		if ws.Enabled {
			return ws
		}
	}
	return nil
}

// GetActiveStrategy safely retrieves the currently active strategy.
func (e *Engine) GetActiveStrategy() *models.Strategy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ActiveStrategy
}

// CheckSessionHours delegates session validation to the SessionGate using the engine's current time.
func (e *Engine) CheckSessionHours(t time.Time) (bool, string) {
	return e.sessionGate.CheckSessionHours(t)
}

// CalculatePositionSize computes order size in shares based on watchlistItem sizing mode.
func (e *Engine) CalculatePositionSize(item *models.WatchlistItem, price float64, totalEquity float64) (float64, error) {
	return CalculatePositionSize(item, price, totalEquity)
}

// RecordFill records an execution fill into the SQLite trade_ledger and emits UI events.
func (e *Engine) RecordFill(fill *models.TradeRecord) error {
	if fill == nil {
		return fmt.Errorf("fill record cannot be nil")
	}

	if fill.CreatedAt.IsZero() {
		fill.CreatedAt = time.Now().UTC()
	}

	e.mu.RLock()
	dbRef := e.db
	app := e.App
	e.mu.RUnlock()

	if dbRef != nil {
		if err := dbRef.RecordFill(fill); err != nil {
			return fmt.Errorf("failed to insert trade fill into SQLite trade_ledger: %w", err)
		}
	}

	if app != nil {
		app.Event.Emit("trade_fill", fill)
		prefix := "[LIVE FILL]"
		if fill.Mode == "dry_run" {
			prefix = "[DRY-RUN SIMULATED FILL]"
		}
		msg := fmt.Sprintf("[%s] %s %s %d %s @ $%.2f (OrderID: %d, Status: %s)",
			fill.CreatedAt.Format("15:04:05"), prefix, fill.Action, fill.Quantity, fill.Symbol, fill.Price, fill.OrderID, fill.Status)
		app.Event.Emit("ledger_entry", msg)
	}

	return nil
}

// CanExecute evaluates the comprehensive 9-gate fail-closed pipeline.
// Feature 10: 9-Gate Fail-Closed Pipeline.
func (e *Engine) CanExecute(symbol string, action models.ActionType) (bool, string) {
	price := e.GetLatestPrice(symbol)
	return e.CanExecuteWithPrice(symbol, action, price)
}

// CanExecuteWithPrice evaluates all 9 safety gates in strict order.
func (e *Engine) CanExecuteWithPrice(symbol string, action models.ActionType, price float64) (bool, string) {
	normAction := strings.ToUpper(strings.TrimSpace(string(action)))
	if normAction != "BUY" && normAction != "SELL" {
		return false, fmt.Sprintf("Gate Rejected: Invalid or non-executable action %q", action)
	}

	// Gate 1: Master Circuit Breaker ("Coupe-circuit principal")
	if !e.IsGlobalEnabled() {
		return false, "Gate 1 Rejected: Master circuit breaker is OFF"
	}

	// Gate 2: Daily Loss Risk Breaker Check
	if e.RiskBreakerTripped.Load() {
		return false, "Gate 2 Rejected: Daily loss limit circuit breaker is TRIPPED"
	}

	// Gate 3: Engine Running Check
	if !e.IsRunning() {
		return false, "Gate 3 Rejected: Execution engine is STOPPED"
	}

	e.mu.RLock()
	cs := e.CockpitState
	if cs == nil {
		e.mu.RUnlock()
		return false, "Gate 4 Rejected: Cockpit state is uninitialized"
	}

	// Gate 4: Active Workspace Enabled Check
	ws := e.getActiveWorkspaceLocked()
	if ws == nil || !ws.Enabled {
		e.mu.RUnlock()
		return false, "Gate 4 Rejected: Active workspace is DISABLED"
	}

	// Gate 5: Active Strategy Slot Enabled (1/1 Concurrency Invariant)
	enabledSlotCount := 0
	var activeStrategyID string
	var hasActiveSlot bool
	for i := range cs.Workspaces {
		w := &cs.Workspaces[i]
		if !w.Enabled {
			continue
		}
		for j := range w.StrategySlots {
			slot := &w.StrategySlots[j]
			if slot.Enabled {
				enabledSlotCount++
				activeStrategyID = slot.StrategyID
				hasActiveSlot = true
			}
		}
	}
	e.mu.RUnlock()

	if enabledSlotCount == 0 {
		return false, "Gate 5 Rejected: No active strategy slot armed"
	}
	if enabledSlotCount > 1 {
		return false, "Gate 5 Rejected: Concurrency violation: multiple strategy slots active (must be 1/1)"
	}
	if !hasActiveSlot || activeStrategyID == "" {
		return false, "Gate 5 Rejected: Active strategy slot has no strategy assigned"
	}

	// Gate 6: Ticker Enabled in Watchlist
	item := e.GetWatchlistItem(symbol)
	if item == nil {
		return false, fmt.Sprintf("Gate 6 Rejected: Ticker %q not found in active watchlist", symbol)
	}
	if !item.Enabled {
		return false, fmt.Sprintf("Gate 6 Rejected: Ticker %q is SUSPENDED in watchlist", symbol)
	}

	// Gate 7: Orderable Equity Invariant (non-regime reference)
	if !item.IsOrderable || item.IsRegimeReference {
		return false, fmt.Sprintf("Gate 7 Rejected: Ticker %q is non-orderable regime reference", symbol)
	}

	// Gate 8: Session Gating (RTH vs ETH)
	now := e.Now()
	isRTH, sessReason := e.sessionGate.CheckSessionHours(now)
	if !isRTH {
		return false, fmt.Sprintf("Gate 8 Rejected: Session gating (%s)", sessReason)
	}

	// Gate 9: Price & Sizing Validity
	if price <= 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return false, "Gate 9 Rejected: Invalid market price for sizing (non-finite or <= 0)"
	}

	equity := e.GetEquity()
	size, err := e.CalculatePositionSize(item, price, equity)
	if err != nil {
		return false, fmt.Sprintf("Gate 9 Rejected: Sizing calculation failed: %v", err)
	}
	if size <= 0 || math.IsNaN(size) || math.IsInf(size, 0) {
		return false, "Gate 9 Rejected: Calculated position size is zero or invalid"
	}

	return true, "OK"
}

// StartBot flips the execution state to ON and registers active strategy.
func (e *Engine) StartBot(s *models.Strategy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isRunning.Load() {
		return fmt.Errorf("bot is already running")
	}

	e.ActiveStrategy = s
	e.isRunning.Store(true)

	// In test/headless runs without explicit strategy slot, arm the slot for the strategy
	if s != nil && s.ID != "" && e.CockpitState != nil {
		for i := range e.CockpitState.Workspaces {
			ws := &e.CockpitState.Workspaces[i]
			if ws.Enabled && len(ws.StrategySlots) > 0 {
				ws.StrategySlots[0].StrategyID = s.ID
				ws.StrategySlots[0].Enabled = true
			}
		}
	}

	fmt.Println("Bot engine started locally at", time.Now())
	return nil
}

// StopBot flips it off gracefully.
func (e *Engine) StopBot() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.isRunning.Store(false)
	fmt.Println("Bot engine stopped at", time.Now())
}

// EvaluateTick receives live IBKR bar frames and runs rules to decide buy/sell actions.
func (e *Engine) EvaluateTick(symbol string, marketData map[string][]float64) []models.ActionType {
	if !e.isRunning.Load() || e.RiskBreakerTripped.Load() || !e.IsGlobalEnabled() {
		return nil
	}

	e.mu.RLock()
	strat := e.ActiveStrategy
	app := e.App
	e.mu.RUnlock()

	if strat == nil {
		return nil
	}

	// Cache latest close price if present in marketData
	if closes, ok := marketData["Close"]; ok && len(closes) > 0 {
		e.SetLatestPrice(symbol, closes[len(closes)-1])
	}

	var actions []models.ActionType

	// Emit a pulse event so the UI knows we are actively scanning this symbol
	if app != nil {
		app.Event.Emit("pulse_tick", map[string]interface{}{
			"symbol": symbol,
			"time":   time.Now().Format(time.RFC3339),
		})
	}

	for _, rule := range strat.Rules {
		if !rule.Enabled {
			continue
		}

		if rule.Scope == models.ScopeSymbol && rule.Name != symbol {
			// Ignore if rules explicitly tied to other symbols
		}

		active := strategy.EvaluateConditions(rule.Condition, marketData)

		// Map rule evaluation to the UI Matrix Heatmap
		if app != nil {
			app.Event.Emit("matrix_update", map[string]interface{}{
				"symbol": symbol,
				"rule":   rule.Name,
				"active": active,
			})
		}

		if active {
			actions = append(actions, rule.Action)

			// Broadcast execution intent to UI Ledger
			if app != nil {
				msg := fmt.Sprintf("[%s] %s Action triggered by rule: %s",
					time.Now().Format("15:04:05"), rule.Action, rule.Name)
				app.Event.Emit("ledger_entry", msg)
			}
		}
	}

	return actions
}
