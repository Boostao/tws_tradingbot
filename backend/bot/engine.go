package bot

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"
	"github.com/scmhub/ibapi"
)

// The Engine abstracts the fail-closed execution loop, risk breaker, and safety gates.
type Engine struct {
	mu                 sync.RWMutex
	ActiveStrategy     *models.Strategy
	isRunning          atomic.Bool
	RiskBreakerTripped atomic.Bool
	dryRun             atomic.Bool
	globalEnabled      atomic.Bool

	AutopilotController *AutopilotController
	CockpitState        *models.CockpitStateResponse
	Emitter             Emitter
	Journal             *JournalWriter
	db                  *db.Database

	sessionGate  *SessionGate
	timeProvider func() time.Time

	dailyLossLimit  float64
	realizedPnL     float64
	unrealizedPnL   float64
	dailyPnL        float64
	hasLiveDailyPnL bool
	totalEquity     float64

	accountID               string
	buyingPower             float64
	cashUSD                 float64
	cashCAD                 float64
	cashBase                float64
	hasCashUSD              bool
	hasCashCAD              bool
	hasCashBase             bool
	equityWithLoan          float64
	excessLiquidity         float64
	equityCurrency          string
	buyingPowerCurrency     string
	equityWithLoanCurrency  string
	excessLiquidityCurrency string
	preferredBaseCurrency   string

	positions               map[string]models.BrokerPosition
	openOrders              map[int64]models.BrokerOrder
	brokerAlerts            []models.BrokerAlert
	lastDisconnectAt        string
	lastDisconnectReason    string
	lastRuntimeReloadAt     string
	lastRuntimeReloadReason string
	lastDryRun              map[string]interface{}

	watchlistItems map[string]*models.WatchlistItem
	latestPrices   map[string]float64
	barBundle      *strategy.BarFrameBundle

	Autopilot *AutopilotController

	bracketEnabled bool
	takeProfitPct  float64
	stopLossPct    float64
}

// NewEngine creates an initialized Engine in fail-closed default state.
func NewEngine() *Engine {
	e := &Engine{
		sessionGate:         NewSessionGate(),
		timeProvider:        time.Now,
		dailyLossLimit:      500.0,
		totalEquity:    0.0,
		positions:      make(map[string]models.BrokerPosition),
		openOrders:     make(map[int64]models.BrokerOrder),
		brokerAlerts:   make([]models.BrokerAlert, 0),
		watchlistItems: make(map[string]*models.WatchlistItem),
		latestPrices:   make(map[string]float64),
		barBundle:      strategy.NewBarFrameBundle(),
		takeProfitPct:  2.0,
		stopLossPct:    1.0,
	}
	e.AutopilotController = NewAutopilotController(e)
	e.Autopilot = e.AutopilotController

	e.isRunning.Store(false)
	e.RiskBreakerTripped.Store(false)
	e.dryRun.Store(true)         // Fail-safe: dry-run enabled by default
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
	if e.AutopilotController != nil {
		_ = e.AutopilotController.LoadDecisions(database.Conn())
	}
}

// AttachEmitter gives the engine access to event emission.
func (e *Engine) AttachEmitter(emitter Emitter) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Emitter = emitter
}

// GetEmitter safely retrieves the attached event emitter reference.
func (e *Engine) GetEmitter() Emitter {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.Emitter
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
	e.mu.Lock()
	defer e.mu.Unlock()
	e.globalEnabled.Store(enabled)
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

// InitJournal initializes the journal writer.
func (e *Engine) InitJournal(path string) error {
	jw, err := NewJournalWriter(path)
	if err != nil {
		return err
	}
	e.Journal = jw
	return nil
}

// LogEvent logs an event to the journal asynchronously.
func (e *Engine) LogEvent(name string, data any) {
	if e.Journal != nil {
		e.Journal.Write(name, data)
	}
}

// SetDailyLossLimit sets the threshold for automated daily loss risk breaker trip.
func (e *Engine) SetDailyLossLimit(limit float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if limit >= 0 && !math.IsNaN(limit) && !math.IsInf(limit, 0) {
		e.dailyLossLimit = limit
	}
}

// GetDailyLossLimit returns the daily loss limit.
func (e *Engine) GetDailyLossLimit() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.dailyLossLimit
}

// SetBaseCurrency sets the preferred account base currency used when TWS AccountSummary omits "BASE".
func (e *Engine) SetBaseCurrency(currency string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.preferredBaseCurrency = strings.ToUpper(strings.TrimSpace(currency))
}

// acceptAccountMetricCurrency determines whether an incoming account summary metric currency
// should update the stored metric. "BASE" takes highest priority, followed by preferredBase if
// configured; otherwise an established primary currency accepts updates only for that same currency.
func acceptAccountMetricCurrency(currentCurr *string, incomingCurr, preferredBase string) bool {
	norm := strings.ToUpper(strings.TrimSpace(incomingCurr))
	cur := strings.ToUpper(strings.TrimSpace(*currentCurr))
	pref := strings.ToUpper(strings.TrimSpace(preferredBase))
	if cur == "" {
		if norm == "" {
			*currentCurr = "BASE"
		} else {
			*currentCurr = norm
		}
		return true
	}
	if norm == "BASE" {
		*currentCurr = "BASE"
		return true
	}
	if cur == "BASE" {
		return false
	}
	if pref != "" {
		if norm == pref && cur != pref {
			*currentCurr = norm
			return true
		}
		if cur == pref && norm != pref {
			return false
		}
	}
	return norm == cur
}

// SetEquity sets the current portfolio equity for % position sizing (allows 0 reset on disconnect).
func (e *Engine) SetEquity(equity float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if equity >= 0 && !math.IsNaN(equity) && !math.IsInf(equity, 0) {
		e.totalEquity = equity
		if equity == 0 {
			e.equityCurrency = ""
		}
	}
}

// SetEquityWithCurrency updates NetLiquidation while preventing secondary-currency rows from
// overwriting BASE or an already-established primary currency.
func (e *Engine) SetEquityWithCurrency(equity float64, currency string) {
	if equity < 0 || math.IsNaN(equity) || math.IsInf(equity, 0) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if acceptAccountMetricCurrency(&e.equityCurrency, currency, e.preferredBaseCurrency) {
		e.totalEquity = equity
	}
}

// GetEquity returns the current portfolio equity.
func (e *Engine) GetEquity() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalEquity
}

// SetAccountID records the live TWS account identifier.
func (e *Engine) SetAccountID(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.accountID = id
}

// GetAccountID returns the captured TWS account identifier.
func (e *Engine) GetAccountID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.accountID
}

// PersistCapturedAccount saves the captured TWS account ID under config.ib.account without clobbering host/port/client_id.
func (e *Engine) PersistCapturedAccount(account string) {
	account = strings.TrimSpace(account)
	if account == "" {
		return
	}
	e.SetAccountID(account)

	e.mu.RLock()
	dbRef := e.db
	e.mu.RUnlock()
	if dbRef == nil {
		return
	}

	cfg, err := dbRef.LoadSystemConfig("config")
	if err != nil || cfg == nil {
		cfg = map[string]interface{}{}
	}
	DeepMergeMap(cfg, map[string]interface{}{
		"ib": map[string]interface{}{
			"account": account,
		},
	})
	_ = dbRef.SaveSystemConfig("config", cfg)
}

// SetBuyingPower stores the live broker buying power.
func (e *Engine) SetBuyingPower(bp float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if bp >= 0 && !math.IsNaN(bp) && !math.IsInf(bp, 0) {
		e.buyingPower = bp
		if bp == 0 {
			e.buyingPowerCurrency = ""
		}
	}
}

// SetBuyingPowerWithCurrency stores BuyingPower respecting primary/BASE currency priority.
func (e *Engine) SetBuyingPowerWithCurrency(bp float64, currency string) {
	if bp < 0 || math.IsNaN(bp) || math.IsInf(bp, 0) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if acceptAccountMetricCurrency(&e.buyingPowerCurrency, currency, e.preferredBaseCurrency) {
		e.buyingPower = bp
	}
}

// SetEquityWithLoan stores EquityWithLoanValue from AccountSummary.
func (e *Engine) SetEquityWithLoan(v float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !math.IsNaN(v) && !math.IsInf(v, 0) {
		e.equityWithLoan = v
	}
}

// SetEquityWithLoanWithCurrency stores EquityWithLoanValue respecting primary/BASE currency priority.
func (e *Engine) SetEquityWithLoanWithCurrency(v float64, currency string) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if acceptAccountMetricCurrency(&e.equityWithLoanCurrency, currency, e.preferredBaseCurrency) {
		e.equityWithLoan = v
	}
}

// SetExcessLiquidity stores ExcessLiquidity from AccountSummary.
func (e *Engine) SetExcessLiquidity(v float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !math.IsNaN(v) && !math.IsInf(v, 0) {
		e.excessLiquidity = v
	}
}

// SetExcessLiquidityWithCurrency stores ExcessLiquidity respecting primary/BASE currency priority.
func (e *Engine) SetExcessLiquidityWithCurrency(v float64, currency string) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if acceptAccountMetricCurrency(&e.excessLiquidityCurrency, currency, e.preferredBaseCurrency) {
		e.excessLiquidity = v
	}
}

// SetCashBalance stores native per-currency CashBalance from UpdateAccountValue.
func (e *Engine) SetCashBalance(currency string, val float64) {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "USD":
		e.cashUSD = val
		e.hasCashUSD = true
	case "CAD":
		e.cashCAD = val
		e.hasCashCAD = true
	case "BASE", "":
		e.cashBase = val
		e.hasCashBase = true
	}
}

// SetCashBalanceFallback stores TotalCashValue from AccountSummary without overriding native CashBalance.
func (e *Engine) SetCashBalanceFallback(currency string, val float64) {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "BASE", "":
		if !e.hasCashBase {
			e.cashBase = val
		}
	case "USD":
		if !e.hasCashUSD {
			e.cashUSD = val
		}
	case "CAD":
		if !e.hasCashCAD {
			e.cashCAD = val
		}
	}
}

// GetAccountTelemetry returns equity, buyingPower, cashUSD, cashCAD, and cashBase atomically.
func (e *Engine) GetAccountTelemetry() (equity, buyingPower, cashUSD, cashCAD, cashBase float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalEquity, e.buyingPower, e.cashUSD, e.cashCAD, e.cashBase
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

// PlaceAutopilotOrder routes autopilot order requests through the controller.
func (e *Engine) PlaceAutopilotOrder(orderID int64, contract *ibapi.Contract, action string, qty int, price float64, symbol string, decisionKey string) {
	e.mu.RLock()
	controller := e.AutopilotController
	e.mu.RUnlock()
	if controller == nil {
		return
	}
	controller.PlaceAutopilotOrder(orderID, contract, action, qty, price, symbol, decisionKey)
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

// GetSessionStatus returns the formatted NY/Toronto Eastern Time market session string.
func (e *Engine) GetSessionStatus() string {
	return e.sessionGate.FormatSessionStatus(e.Now())
}

// GetBarBundle returns the Engine's multi-timeframe BarFrameBundle.
func (e *Engine) GetBarBundle() *strategy.BarFrameBundle {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.barBundle
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
	emitter := e.Emitter
	e.mu.Unlock()

	e.evaluateDailyLossBreaker(realized, unrealized, total, limit, emitter)
}

// UpdateLivePnL stores streaming TWS ReqPnL updates and evaluates the daily loss breaker.
func (e *Engine) UpdateLivePnL(daily, unrealized, realized float64) {
	e.mu.Lock()
	e.dailyPnL = daily
	e.hasLiveDailyPnL = true
	e.unrealizedPnL = unrealized
	e.realizedPnL = realized
	effectiveTotal := daily
	if effectiveTotal == 0 && (realized != 0 || unrealized != 0) {
		effectiveTotal = realized + unrealized
	}
	limit := e.dailyLossLimit
	emitter := e.Emitter
	e.mu.Unlock()

	e.evaluateDailyLossBreaker(realized, unrealized, effectiveTotal, limit, emitter)
}

func (e *Engine) evaluateDailyLossBreaker(realized, unrealized, total, limit float64, emitter Emitter) {
	// Risk Breaker Check: if limit > 0 and total PnL <= -dailyLossLimit, automatic trip
	if limit > 0 && total <= -limit {
		e.RiskBreakerTripped.Store(true)
		e.SetGlobalEnabled(false)

		e.mu.RLock()
		dbRef := e.db
		e.mu.RUnlock()
		if dbRef != nil {
			if cs, err := dbRef.LoadCockpitState(); err == nil && cs != nil && cs.GlobalEnabled {
				cs.GlobalEnabled = false
				_ = dbRef.SaveCockpitState(cs)
			}
		}

		if emitter != nil {
			emitter.Emit("circuit_breaker_tripped", map[string]interface{}{
				"realized":   realized,
				"unrealized": unrealized,
				"total_pnl":  total,
				"limit":      limit,
				"reason":     fmt.Sprintf("Daily loss limit breached: total PnL $%.2f <= -$%.2f", total, limit),
				"timestamp":  time.Now().UTC().Format(time.RFC3339),
			})

			msg := fmt.Sprintf("[%s] 🚨 COUPE-CIRCUIT DÉCLENCHÉ: Perte journalière ($%.2f) excède la limite ($%.2f). Exécution automatique suspendue.",
				time.Now().Format("15:04:05"), total, limit)
			emitter.Emit("ledger_entry", msg)
		}
	}
}

// getPnLLocked returns realized, unrealized, and combined daily PnL while holding e.mu.RLock or e.mu.Lock.
func (e *Engine) getPnLLocked() (realized, unrealized, total float64) {
	if e.hasLiveDailyPnL {
		total = e.dailyPnL
		if total == 0 && (e.realizedPnL != 0 || e.unrealizedPnL != 0) {
			total = e.realizedPnL + e.unrealizedPnL
		}
		return e.realizedPnL, e.unrealizedPnL, total
	}
	return e.realizedPnL, e.unrealizedPnL, e.realizedPnL + e.unrealizedPnL
}

// GetPnL returns realized, unrealized, and combined daily PnL.
func (e *Engine) GetPnL() (realized, unrealized, total float64) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.getPnLLocked()
}

// GetDailyPnL returns live daily PnL when available, else realized + unrealized PnL.
func (e *Engine) GetDailyPnL() float64 {
	_, _, total := e.GetPnL()
	return total
}

// UpsertPosition stores or updates a broker position from IBWrapper.Position.
func (e *Engine) UpsertPosition(key string, pos models.BrokerPosition) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if pos.Position == 0 {
		delete(e.positions, key)
		return
	}
	if existing, ok := e.positions[key]; ok {
		if pos.MarketPrice == 0 && existing.MarketPrice != 0 {
			pos.MarketPrice = existing.MarketPrice
		}
		if pos.MarketValue == 0 && existing.MarketValue != 0 {
			pos.MarketValue = existing.MarketValue
		}
		if pos.UnrealizedPnL == 0 && existing.UnrealizedPnL != 0 {
			pos.UnrealizedPnL = existing.UnrealizedPnL
		}
		if pos.RealizedPnL == 0 && existing.RealizedPnL != 0 {
			pos.RealizedPnL = existing.RealizedPnL
		}
		if (pos.Venue == "" || pos.Venue == "SMART") && existing.Venue != "" && existing.Venue != "SMART" {
			pos.Venue = existing.Venue
			pos.InstrumentID = existing.InstrumentID
		}
	}
	e.positions[key] = pos
	if pos.MarketPrice > 0 {
		norm := models.NormalizeSymbol(pos.Symbol)
		e.latestPrices[pos.Symbol] = pos.MarketPrice
		e.latestPrices[norm.Symbol] = pos.MarketPrice
		e.latestPrices[pos.InstrumentID] = pos.MarketPrice
	}
}

// UpdatePortfolioPosition merges valuation fields from IBWrapper.UpdatePortfolio.
func (e *Engine) UpdatePortfolioPosition(key string, pos models.BrokerPosition) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if pos.Position == 0 {
		delete(e.positions, key)
		return
	}
	if existing, ok := e.positions[key]; ok {
		if pos.AvgCost == 0 && existing.AvgCost != 0 {
			pos.AvgCost = existing.AvgCost
		}
		if (pos.Venue == "" || pos.Venue == "SMART") && existing.Venue != "" && existing.Venue != "SMART" {
			pos.Venue = existing.Venue
			pos.InstrumentID = existing.InstrumentID
		}
	}
	e.positions[key] = pos
	if pos.MarketPrice > 0 {
		norm := models.NormalizeSymbol(pos.Symbol)
		e.latestPrices[pos.Symbol] = pos.MarketPrice
		e.latestPrices[norm.Symbol] = pos.MarketPrice
		e.latestPrices[pos.InstrumentID] = pos.MarketPrice
	}
}

// GetOpenPositions returns a deterministically sorted snapshot of open broker positions.
func (e *Engine) GetOpenPositions() []models.BrokerPosition {
	e.mu.RLock()
	defer e.mu.RUnlock()
	res := make([]models.BrokerPosition, 0, len(e.positions))
	for _, p := range e.positions {
		if p.Position != 0 {
			res = append(res, p)
		}
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].InstrumentID != res[j].InstrumentID {
			return res[i].InstrumentID < res[j].InstrumentID
		}
		return res[i].Symbol < res[j].Symbol
	})
	return res
}

func isTerminalOrderStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "filled", "cancelled", "apicancelled":
		return true
	default:
		return false
	}
}

// UpsertOpenOrder stores or updates a working broker order from IBWrapper.OpenOrder.
func (e *Engine) UpsertOpenOrder(ord models.BrokerOrder) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if isTerminalOrderStatus(ord.Status) {
		delete(e.openOrders, ord.OrderID)
		return
	}
	if existing, ok := e.openOrders[ord.OrderID]; ok {
		if ord.FilledQty == 0 && existing.FilledQty > 0 {
			ord.FilledQty = existing.FilledQty
		}
		if ord.RemainingQty == 0 && existing.RemainingQty > 0 {
			ord.RemainingQty = existing.RemainingQty
		}
		if ord.AvgFillPrice == 0 && existing.AvgFillPrice > 0 {
			ord.AvgFillPrice = existing.AvgFillPrice
		}
		if ord.Status == "" && existing.Status != "" {
			ord.Status = existing.Status
		}
	}
	e.openOrders[ord.OrderID] = ord
}

// UpdateOrderStatus updates fill/remaining/status for an open order and removes terminal orders.
func (e *Engine) UpdateOrderStatus(orderID int64, status string, filled, remaining, avgFillPrice float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if isTerminalOrderStatus(status) {
		delete(e.openOrders, orderID)
		return
	}
	ord := e.openOrders[orderID]
	ord.OrderID = orderID
	ord.Status = status
	ord.FilledQty = filled
	ord.RemainingQty = remaining
	ord.AvgFillPrice = avgFillPrice
	e.openOrders[orderID] = ord
}

// GetOpenOrders returns a deterministically sorted snapshot of hydrated active broker orders.
func (e *Engine) GetOpenOrders() []models.BrokerOrder {
	e.mu.RLock()
	defer e.mu.RUnlock()
	res := make([]models.BrokerOrder, 0, len(e.openOrders))
	for _, o := range e.openOrders {
		if o.Symbol == "" && o.Action == "" {
			continue
		}
		res = append(res, o)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].OrderID < res[j].OrderID
	})
	return res
}

// AddBrokerAlert records a structured broker alert (deduplicated consecutively, capped at 25).
func (e *Engine) AddBrokerAlert(alert models.BrokerAlert) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if alert.Timestamp == "" {
		alert.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if n := len(e.brokerAlerts); n > 0 {
		last := &e.brokerAlerts[n-1]
		if last.Code == alert.Code && last.Message == alert.Message {
			last.Timestamp = alert.Timestamp
			last.Hint = alert.Hint
			last.Severity = alert.Severity
			return
		}
	}
	e.brokerAlerts = append(e.brokerAlerts, alert)
	if len(e.brokerAlerts) > 25 {
		e.brokerAlerts = append([]models.BrokerAlert(nil), e.brokerAlerts[len(e.brokerAlerts)-25:]...)
	}
}

// GetBrokerAlerts returns a copy of recorded broker alerts.
func (e *Engine) GetBrokerAlerts() []models.BrokerAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.brokerAlerts) == 0 {
		return nil
	}
	out := make([]models.BrokerAlert, len(e.brokerAlerts))
	copy(out, e.brokerAlerts)
	return out
}

// RecordDisconnect stores the timestamp and reason of the latest disconnect.
func (e *Engine) RecordDisconnect(reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastDisconnectAt = time.Now().UTC().Format(time.RFC3339)
	e.lastDisconnectReason = reason
}

// GetLastDisconnect returns the last disconnect timestamp and reason.
func (e *Engine) GetLastDisconnect() (string, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastDisconnectAt, e.lastDisconnectReason
}

// RecordRuntimeReload records a startup, config, cockpit, or dry-run reload timestamp and reason.
func (e *Engine) RecordRuntimeReload(reason string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastRuntimeReloadAt = time.Now().UTC().Format(time.RFC3339)
	e.lastRuntimeReloadReason = reason
}

// GetLastRuntimeReload returns the last runtime reload timestamp and reason.
func (e *Engine) GetLastRuntimeReload() (string, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastRuntimeReloadAt, e.lastRuntimeReloadReason
}

// SetLastDryRun stores the most recent dry-run summary snapshot.
func (e *Engine) SetLastDryRun(summary map[string]interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if summary == nil {
		e.lastDryRun = nil
		return
	}
	cp := make(map[string]interface{}, len(summary))
	for k, v := range summary {
		cp[k] = v
	}
	e.lastDryRun = cp
}

// GetLastDryRun returns a copy of the last dry-run summary snapshot.
func (e *Engine) GetLastDryRun() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.lastDryRun == nil {
		return nil
	}
	cp := make(map[string]interface{}, len(e.lastDryRun))
	for k, v := range e.lastDryRun {
		cp[k] = v
	}
	return cp
}

// ResetBrokerTelemetry clears live broker balances, PnL, positions, and open orders on disconnect
// while preserving brokerAlerts and lastDisconnect metadata for operator diagnostics.
func (e *Engine) ResetBrokerTelemetry() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.totalEquity = 0.0
	e.buyingPower = 0.0
	e.cashUSD = 0.0
	e.cashCAD = 0.0
	e.cashBase = 0.0
	e.hasCashUSD = false
	e.hasCashCAD = false
	e.hasCashBase = false
	e.equityWithLoan = 0.0
	e.excessLiquidity = 0.0
	e.equityCurrency = ""
	e.buyingPowerCurrency = ""
	e.equityWithLoanCurrency = ""
	e.excessLiquidityCurrency = ""
	e.dailyPnL = 0.0
	e.hasLiveDailyPnL = false
	e.realizedPnL = 0.0
	e.unrealizedPnL = 0.0
	e.positions = make(map[string]models.BrokerPosition)
	e.openOrders = make(map[int64]models.BrokerOrder)
}

// SyncLivePositionsToDB persists current open positions to SQLite watchlists without holding e.mu.
func (e *Engine) SyncLivePositionsToDB() error {
	e.mu.RLock()
	dbRef := e.db
	emitter := e.Emitter
	e.mu.RUnlock()

	if dbRef == nil {
		return nil
	}
	positions := e.GetOpenPositions()
	if err := dbRef.SyncLivePositions(positions); err != nil {
		return err
	}
	if emitter != nil {
		emitter.Emit("positions_synced", map[string]interface{}{
			"count":     len(positions),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
	return nil
}

// ResetCircuitBreaker manually re-arms the risk breaker and master circuit breaker.
func (e *Engine) ResetCircuitBreaker() error {
	e.mu.RLock()
	limit := e.dailyLossLimit
	_, _, totalPnL := e.getPnLLocked()
	emitter := e.Emitter
	e.mu.RUnlock()

	// Guard: cannot reset if daily loss is actively breached
	if limit > 0 && totalPnL <= -limit {
		return fmt.Errorf("cannot reset circuit breaker: daily loss limit actively breached (PnL: %.2f, Limit: %.2f)", totalPnL, limit)
	}

	e.RiskBreakerTripped.Store(false)
	e.SetGlobalEnabled(true)

	if emitter != nil {
		emitter.Emit("circuit_breaker_reset", map[string]interface{}{
			"status":    "ARMED",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		msg := fmt.Sprintf("[%s] 🛡️ Coupe-circuit réarmé manuellement. Prêt pour l'exécution.", time.Now().Format("15:04:05"))
		emitter.Emit("ledger_entry", msg)
	}

	return nil
}

func cloneCockpitState(s *models.CockpitStateResponse) *models.CockpitStateResponse {
	if s == nil {
		return nil
	}
	cp := *s
	if s.Workspaces != nil {
		cp.Workspaces = make([]models.CockpitWorkspace, len(s.Workspaces))
		for i, ws := range s.Workspaces {
			cp.Workspaces[i] = ws
			if ws.StrategySlots != nil {
				cp.Workspaces[i].StrategySlots = make([]models.CockpitStrategySlot, len(ws.StrategySlots))
				copy(cp.Workspaces[i].StrategySlots, ws.StrategySlots)
			}
		}
	}
	if s.StrategyLibrary != nil {
		cp.StrategyLibrary = make([]models.CockpitStrategySummary, len(s.StrategyLibrary))
		copy(cp.StrategyLibrary, s.StrategyLibrary)
	}
	if s.Feed != nil {
		feedCopy := *s.Feed
		cp.Feed = &feedCopy
	}
	return &cp
}

// SetCockpitState stores a deep clone of updated cockpit state and synchronizes globalEnabled flag.
func (e *Engine) SetCockpitState(state *models.CockpitStateResponse) {
	cloned := cloneCockpitState(state)
	e.mu.Lock()
	defer e.mu.Unlock()
	if cloned != nil && e.RiskBreakerTripped.Load() {
		cloned.GlobalEnabled = false
	}
	e.CockpitState = cloned
	if cloned != nil {
		e.globalEnabled.Store(cloned.GlobalEnabled)
	}
}

// GetCockpitState retrieves a deep clone of current cockpit state.
func (e *Engine) GetCockpitState() *models.CockpitStateResponse {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return cloneCockpitState(e.CockpitState)
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

// SetActiveStrategy sets the currently selected strategy without modifying isRunning.
func (e *Engine) SetActiveStrategy(s *models.Strategy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ActiveStrategy = s
}

// GetActiveStrategy safely retrieves the currently active strategy.
func (e *Engine) GetActiveStrategy() *models.Strategy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.ActiveStrategy
}

// ResolveSelectedStrategy determines the active workspace ID, selected strategy ID, and loaded strategy
// even when the workspace or engine is paused.
func (e *Engine) ResolveSelectedStrategy(state *models.CockpitStateResponse) (wsID, stratID string, strat *models.Strategy) {
	if state == nil {
		state = e.GetCockpitState()
	}
	if state == nil || len(state.Workspaces) == 0 {
		return "", "", nil
	}

	var targetWS *models.CockpitWorkspace
	if state.ActiveWorkspaceID != "" {
		for i := range state.Workspaces {
			if state.Workspaces[i].ID == state.ActiveWorkspaceID {
				targetWS = &state.Workspaces[i]
				break
			}
		}
	}
	if targetWS == nil {
		for i := range state.Workspaces {
			if state.Workspaces[i].Enabled {
				targetWS = &state.Workspaces[i]
				break
			}
		}
	}
	if targetWS == nil {
		targetWS = &state.Workspaces[0]
	}

	wsID = targetWS.ID
	for _, slot := range targetWS.StrategySlots {
		if slot.Enabled && slot.StrategyID != "" {
			stratID = slot.StrategyID
			break
		}
	}
	if stratID == "" {
		for _, slot := range targetWS.StrategySlots {
			if slot.StrategyID != "" {
				stratID = slot.StrategyID
				break
			}
		}
	}

	if stratID != "" {
		e.mu.RLock()
		dbRef := e.db
		e.mu.RUnlock()
		if dbRef != nil {
			if loaded, err := dbRef.LoadStrategyPreset(stratID); err == nil {
				strat = loaded
			}
		}
	}

	return wsID, stratID, strat
}

// DeepMergeMap recursively merges src into dst without overwriting sibling map keys.
func DeepMergeMap(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = map[string]interface{}{}
	}
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, exists := dst[k].(map[string]interface{}); exists {
				dst[k] = DeepMergeMap(dstMap, srcMap)
				continue
			}
			newSub := map[string]interface{}{}
			dst[k] = DeepMergeMap(newSub, srcMap)
			continue
		}
		dst[k] = v
	}
	return dst
}

func asFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		if !math.IsNaN(n) && !math.IsInf(n, 0) {
			return n, true
		}
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// SyncConfigMap applies runtime and IB config settings to the Engine.
func (e *Engine) SyncConfigMap(cfg map[string]interface{}) {
	if cfg == nil {
		return
	}

	if rt, ok := cfg["runtime"].(map[string]interface{}); ok {
		if dry, ok := rt["dry_run"].(bool); ok {
			e.SetDryRun(dry)
		}
		if limit, ok := asFloat(rt["daily_loss_limit"]); ok && limit >= 0 {
			e.SetDailyLossLimit(limit)
		}

		tpPct, slPct := e.GetBracketPercentages()
		bracketEnabled := e.IsBracketEnabled()
		bracketChanged := false
		if be, ok := rt["bracket_enabled"].(bool); ok {
			bracketEnabled = be
			bracketChanged = true
		}
		if tp, ok := asFloat(rt["take_profit_pct"]); ok && tp > 0 {
			tpPct = tp
			bracketChanged = true
		}
		if sl, ok := asFloat(rt["stop_loss_pct"]); ok && sl > 0 {
			slPct = sl
			bracketChanged = true
		}
		if bc, ok := rt["base_currency"].(string); ok && strings.TrimSpace(bc) != "" {
			e.SetBaseCurrency(bc)
		}
		if bracketChanged {
			e.SetBracketConfig(bracketEnabled, tpPct, slPct)
		}
	}

	if ibCfg, ok := cfg["ib"].(map[string]interface{}); ok {
		if acct, ok := ibCfg["account"].(string); ok {
			acct = strings.TrimSpace(acct)
			if acct != "" && e.GetAccountID() == "" {
				e.SetAccountID(acct)
			}
		}
		if bc, ok := ibCfg["base_currency"].(string); ok && strings.TrimSpace(bc) != "" {
			e.SetBaseCurrency(bc)
		}
	}

	e.RecordRuntimeReload("config_update")
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
	emitter := e.Emitter
	e.mu.RUnlock()

	if dbRef != nil {
		if err := dbRef.RecordFill(fill); err != nil {
			return fmt.Errorf("failed to insert trade fill into SQLite trade_ledger: %w", err)
		}
	}

	if emitter != nil {
		emitter.Emit("trade_fill", fill)
		prefix := "[LIVE FILL]"
		if fill.Mode == "dry_run" {
			prefix = "[DRY-RUN SIMULATED FILL]"
		}
		msg := fmt.Sprintf("[%s] %s %s %d %s @ $%.2f (OrderID: %d, Status: %s)",
			fill.CreatedAt.Format("15:04:05"), prefix, fill.Action, fill.Quantity, fill.Symbol, fill.Price, fill.OrderID, fill.Status)
		emitter.Emit("ledger_entry", msg)
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
	return e.canExecuteInternal(symbol, nil, action, price)
}

// CanExecuteWithItem evaluates all 9 safety gates in strict order using an explicit WatchlistItem
// from the active workspace's bound watchlist without unscoped cross-watchlist lookups.
func (e *Engine) CanExecuteWithItem(item *models.WatchlistItem, action models.ActionType, currentPrice float64) (bool, string) {
	symbol := ""
	if item != nil {
		symbol = item.Symbol
		if symbol == "" {
			symbol = item.InstrumentID
		}
	}
	return e.canExecuteInternal(symbol, item, action, currentPrice)
}

func (e *Engine) canExecuteInternal(symbol string, explicitItem *models.WatchlistItem, action models.ActionType, price float64) (bool, string) {
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
	item := explicitItem
	if item == nil {
		item = e.GetWatchlistItem(symbol)
	}
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
	emitter := e.Emitter
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
	if emitter != nil {
		emitter.Emit("pulse_tick", map[string]interface{}{
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
			continue
		}

		active := strategy.EvaluateConditions(rule.Condition, marketData)

		// Map rule evaluation to the UI Matrix Heatmap
		if emitter != nil {
			emitter.Emit("matrix_update", map[string]interface{}{
				"symbol": symbol,
				"rule":   rule.Name,
				"active": active,
			})
		}

		if active {
			actions = append(actions, rule.Action)

			// Broadcast execution intent to UI Ledger
			if emitter != nil {
				msg := fmt.Sprintf("[%s] %s Action triggered by rule: %s",
					time.Now().Format("15:04:05"), rule.Action, rule.Name)
				emitter.Emit("ledger_entry", msg)
			}
		}
	}

	return actions
}

// BuildRuntimeState assembles a full live BotState snapshot from Engine and DB state.
func (e *Engine) BuildRuntimeState(twsConnected bool) *models.BotState {
	running := e.IsRunning()
	dryRun := e.IsDryRun()
	equity, buyingPower, cashUSD, cashCAD, cashBase := e.GetAccountTelemetry()
	lossLimit := e.GetDailyLossLimit()
	realized, unrealized, totalPnL := e.GetPnL()
	breakerTripped := e.RiskBreakerTripped.Load()
	globalEnabled := e.IsGlobalEnabled()
	accountID := e.GetAccountID()
	sessionStatus := e.GetSessionStatus()

	cockpit := e.GetCockpitState()
	activeWSID, activeStratID, resolvedStrat := e.ResolveSelectedStrategy(cockpit)

	var strat *models.Strategy
	if running {
		strat = e.GetActiveStrategy()
		if strat == nil {
			strat = resolvedStrat
		}
	} else {
		strat = resolvedStrat
		if strat == nil {
			strat = e.GetActiveStrategy()
		}
	}

	activeStratName := ""
	if strat != nil {
		if strat.ID != "" {
			activeStratID = strat.ID
		}
		activeStratName = strat.Name
		if activeStratName == "" {
			activeStratName = strat.ID
		}
	} else if activeStratID != "" && cockpit != nil {
		for _, s := range cockpit.StrategyLibrary {
			if s.ID == activeStratID {
				activeStratName = s.Name
				break
			}
		}
		if activeStratName == "" {
			activeStratName = activeStratID
		}
	}

	openPositions := e.GetOpenPositions()
	openOrders := e.GetOpenOrders()
	brokerAlerts := e.GetBrokerAlerts()
	lastDiscAt, lastDiscReason := e.GetLastDisconnect()
	lastReloadAt, lastReloadReason := e.GetLastRuntimeReload()
	lastDryRun := e.GetLastDryRun()

	var recentOrders []map[string]interface{}
	if len(openOrders) > 0 {
		recentOrders = make([]map[string]interface{}, 0, len(openOrders))
		for _, ord := range openOrders {
			recentOrders = append(recentOrders, map[string]interface{}{
				"order_id":       ord.OrderID,
				"symbol":         ord.Symbol,
				"instrument_id":  ord.InstrumentID,
				"action":         ord.Action,
				"order_type":     ord.OrderType,
				"total_qty":      ord.TotalQty,
				"filled_qty":     ord.FilledQty,
				"remaining_qty":  ord.RemainingQty,
				"limit_price":    ord.LimitPrice,
				"avg_fill_price": ord.AvgFillPrice,
				"status":         ord.Status,
			})
		}
	}

	var recentTrades []map[string]interface{}
	tradesToday := 0
	winRateToday := 0.0

	e.mu.RLock()
	dbRef := e.db
	e.mu.RUnlock()

	if dbRef != nil {
		if accountID == "" {
			if cfg, err := dbRef.LoadSystemConfig("config"); err == nil && cfg != nil {
				if ibCfg, ok := cfg["ib"].(map[string]interface{}); ok {
					if acct, ok := ibCfg["account"].(string); ok {
						accountID = strings.TrimSpace(acct)
					}
				}
			}
		}
		if trades, err := dbRef.GetRecentTrades(50); err == nil && len(trades) > 0 {
			todayET := e.sessionGate.InET(e.Now()).Format("2006-01-02")
			recentTrades = make([]map[string]interface{}, 0, len(trades))
			for _, tr := range trades {
				createdUTC := tr.CreatedAt.UTC().Format(time.RFC3339)
				if e.sessionGate.InET(tr.CreatedAt).Format("2006-01-02") == todayET {
					tradesToday++
				}
				recentTrades = append(recentTrades, map[string]interface{}{
					"id":         tr.ID,
					"order_id":   tr.OrderID,
					"symbol":     tr.Symbol,
					"action":     tr.Action,
					"side":       tr.Action,
					"quantity":   tr.Quantity,
					"price":      tr.Price,
					"status":     tr.Status,
					"oca_group":  tr.OCAGroup,
					"mode":       tr.Mode,
					"created_at": createdUTC,
					"closed_at":  createdUTC,
				})
			}
		}
	}

	errMsg := ""
	for i := len(brokerAlerts) - 1; i >= 0; i-- {
		if brokerAlerts[i].Severity == "error" {
			errMsg = brokerAlerts[i].Message
			break
		}
	}

	dailyPnLPct := 0.0
	if equity > 0 {
		dailyPnLPct = (totalPnL / equity) * 100.0
	}

	status := "STOPPED"
	if breakerTripped {
		status = "CIRCUIT_BREAKER"
	} else if running {
		status = "RUNNING"
	} else if globalEnabled {
		status = "ARMED"
	}

	now := time.Now().UTC().Format(time.RFC3339)

	return &models.BotState{
		Status:                  status,
		Running:                 running,
		RunnerActive:            running,
		TwsConnected:            twsConnected,
		AccountID:               accountID,
		DryRun:                  dryRun,
		Equity:                  equity,
		BuyingPower:             buyingPower,
		CashUSD:                 cashUSD,
		CashCAD:                 cashCAD,
		CashBase:                cashBase,
		DailyPnL:                totalPnL,
		DailyPnLPercent:         dailyPnLPct,
		TotalPnL:                totalPnL,
		RealizedPnL:             realized,
		UnrealizedPnL:           unrealized,
		DailyLossLimit:          lossLimit,
		CircuitBreakerTripped:   breakerTripped,
		ActiveWorkspaceID:       activeWSID,
		ActiveStrategyID:        activeStratID,
		ActiveStrategy:          activeStratName,
		SessionStatus:           sessionStatus,
		OpenPositionsCount:      len(openPositions),
		PendingOrdersCount:      len(openOrders),
		OpenPositions:           openPositions,
		OpenOrders:              openOrders,
		BrokerAlerts:            brokerAlerts,
		TradesToday:             tradesToday,
		WinRateToday:            winRateToday,
		LastUpdate:              now,
		LastHeartbeat:           now,
		RecentOrders:            recentOrders,
		RecentTrades:            recentTrades,
		LastDryRun:              lastDryRun,
		LastRuntimeReloadAt:     lastReloadAt,
		LastRuntimeReloadReason: lastReloadReason,
		LastDisconnectAt:        lastDiscAt,
		LastDisconnectReason:    lastDiscReason,
		ErrorMessage:            errMsg,
	}
}

// BuildDiagnostics returns clean, de-noised startup, runtime, session, and broker alert diagnostics.
func (e *Engine) BuildDiagnostics() (*models.DiagnosticsResponse, error) {
	host := "127.0.0.1"
	port := 7497
	clientID := 1
	tradingMode := ""
	cfgAccount := ""
	lastCheckedAt := ""

	e.mu.RLock()
	dbRef := e.db
	e.mu.RUnlock()

	if dbRef != nil {
		if twsCfg, err := dbRef.LoadSystemConfig("tws_connection"); err == nil && twsCfg != nil {
			if h, ok := twsCfg["host"].(string); ok && strings.TrimSpace(h) != "" {
				host = strings.TrimSpace(h)
			}
			if p, ok := asFloat(twsCfg["port"]); ok && p > 0 {
				port = int(p)
			}
			if c, ok := asFloat(twsCfg["client_id"]); ok && c >= 0 {
				clientID = int(c)
			}
		}
		if cfg, err := dbRef.LoadSystemConfig("config"); err == nil && cfg != nil {
			if ibCfg, ok := cfg["ib"].(map[string]interface{}); ok {
				if m, ok := ibCfg["trading_mode"].(string); ok && strings.TrimSpace(m) != "" {
					tradingMode = strings.ToLower(strings.TrimSpace(m))
				}
				if a, ok := ibCfg["account"].(string); ok && strings.TrimSpace(a) != "" {
					cfgAccount = strings.TrimSpace(a)
				}
			}
		}
		if wl, err := dbRef.LoadWatchlistState(); err == nil && wl != nil {
			lastCheckedAt = wl.UpdatedAt
		}
	}

	if tradingMode == "" {
		if port == 7496 || port == 4001 {
			tradingMode = "live"
		} else {
			tradingMode = "paper"
		}
	}

	account := e.GetAccountID()
	if account == "" {
		account = cfgAccount
	}

	cockpit := e.GetCockpitState()
	activeWSID, activeStratID, resolvedStrat := e.ResolveSelectedStrategy(cockpit)
	var strat *models.Strategy
	if e.IsRunning() {
		strat = e.GetActiveStrategy()
		if strat == nil {
			strat = resolvedStrat
		}
	} else {
		strat = resolvedStrat
		if strat == nil {
			strat = e.GetActiveStrategy()
		}
	}

	activeStratName := ""
	if strat != nil {
		if strat.ID != "" {
			activeStratID = strat.ID
		}
		activeStratName = strat.Name
		if activeStratName == "" {
			activeStratName = strat.ID
		}
	} else if activeStratID != "" && cockpit != nil {
		for _, s := range cockpit.StrategyLibrary {
			if s.ID == activeStratID {
				activeStratName = s.Name
				break
			}
		}
		if activeStratName == "" {
			activeStratName = activeStratID
		}
	}

	lastReloadAt, lastReloadReason := e.GetLastRuntimeReload()
	lastDiscAt, lastDiscReason := e.GetLastDisconnect()

	return &models.DiagnosticsResponse{
		Startup: models.DiagnosticsStartup{
			TradingMode: tradingMode,
			IBHost:      host,
			IBPort:      port,
			ClientID:    clientID,
			Account:     account,
		},
		Runtime: models.DiagnosticsRuntime{
			RunnerActive:            e.IsRunning(),
			LastRuntimeReloadAt:     lastReloadAt,
			LastRuntimeReloadReason: lastReloadReason,
			LastDisconnectAt:        lastDiscAt,
			LastDisconnectReason:    lastDiscReason,
		},
		Symbols: models.DiagnosticsSymbols{
			LastCheckedAt: lastCheckedAt,
		},
		SessionStatus:      e.GetSessionStatus(),
		Account:            account,
		DryRun:             e.IsDryRun(),
		ActiveWorkspaceID:  activeWSID,
		ActiveStrategyID:   activeStratID,
		ActiveStrategyName: activeStratName,
		BrokerAlerts:       e.GetBrokerAlerts(),
	}, nil
}

// RunDryRun evaluates the active workspace, selected strategy, bound/default watchlist,
// live positions/orders, and cached BarFrameBundle without placing any live orders.
func (e *Engine) RunDryRun(twsConnected bool) (*models.BotDryRun, error) {
	e.mu.RLock()
	dbRef := e.db
	bundle := e.barBundle
	e.mu.RUnlock()

	cockpit := e.GetCockpitState()
	if cockpit == nil && dbRef != nil {
		if loaded, err := dbRef.LoadCockpitState(); err == nil {
			cockpit = loaded
		}
	}

	activeWSID, _, resolvedStrat := e.ResolveSelectedStrategy(cockpit)
	wsKind := "long"
	boundWatchlistID := ""
	if cockpit != nil {
		for _, ws := range cockpit.Workspaces {
			if ws.ID == activeWSID {
				if ws.Kind != "" {
					wsKind = ws.Kind
				}
				boundWatchlistID = ws.WatchlistID
				break
			}
		}
	}

	var strat *models.Strategy
	if e.IsRunning() {
		strat = e.GetActiveStrategy()
		if strat == nil {
			strat = resolvedStrat
		}
	} else {
		strat = resolvedStrat
		if strat == nil {
			strat = e.GetActiveStrategy()
		}
	}

	activeStratName := ""
	if strat != nil {
		activeStratName = strat.Name
		if activeStratName == "" {
			activeStratName = strat.ID
		}
	}

	// Collect unique timeframes from enabled strategy rules
	tfSeen := make(map[models.TimeframeUnit]bool)
	var timeframes []models.TimeframeUnit
	if strat != nil {
		for _, rule := range strat.Rules {
			if !rule.Enabled || rule.Condition == nil {
				continue
			}
			for _, ind := range []*models.Indicator{rule.Condition.IndicatorA, rule.Condition.IndicatorB} {
				if ind != nil && ind.Timeframe != "" && !tfSeen[ind.Timeframe] {
					tfSeen[ind.Timeframe] = true
					timeframes = append(timeframes, ind.Timeframe)
				}
			}
		}
	}
	if len(timeframes) == 0 {
		timeframes = []models.TimeframeUnit{models.TimeframeD1}
	}

	// Load bound watchlist or default watchlist
	var wl *models.WatchlistResponse
	if dbRef != nil {
		if boundWatchlistID != "" {
			wl, _ = dbRef.GetWatchlistByID(boundWatchlistID)
		}
		if wl == nil {
			wl, _ = dbRef.LoadWatchlistState()
		}
	}

	subscriptions := make([]models.BotDryRunSubscription, 0)
	plannedOrders := make([]map[string]interface{}, 0)
	var evalItems []models.WatchlistItem

	if wl != nil {
		seenItem := make(map[string]bool)
		for _, grp := range wl.Groups {
			for _, item := range grp.Items {
				if !item.Enabled || !item.IsOrderable || item.IsRegimeReference {
					continue
				}
				key := item.InstrumentID
				if key == "" {
					key = item.Symbol
				}
				if seenItem[key] {
					continue
				}
				seenItem[key] = true
				evalItems = append(evalItems, item)
				for _, tf := range timeframes {
					subscriptions = append(subscriptions, models.BotDryRunSubscription{
						Symbol:    key,
						Timeframe: string(tf),
					})
				}
			}
		}
	}

	// Live positions & open orders snapshot
	positionsMap := make(map[string]interface{})
	for _, p := range e.GetOpenPositions() {
		key := p.InstrumentID
		if key == "" {
			key = p.Symbol
		}
		positionsMap[key] = map[string]interface{}{
			"symbol":         p.Symbol,
			"venue":          p.Venue,
			"instrument_id":  p.InstrumentID,
			"currency":       p.Currency,
			"position":       p.Position,
			"avg_cost":       p.AvgCost,
			"market_price":   p.MarketPrice,
			"market_value":   p.MarketValue,
			"unrealized_pnl": p.UnrealizedPnL,
			"realized_pnl":   p.RealizedPnL,
		}
	}

	openOrders := e.GetOpenOrders()
	openOrdersList := make([]map[string]interface{}, 0, len(openOrders))
	for _, o := range openOrders {
		openOrdersList = append(openOrdersList, map[string]interface{}{
			"order_id":       o.OrderID,
			"symbol":         o.Symbol,
			"venue":          o.Venue,
			"instrument_id":  o.InstrumentID,
			"action":         o.Action,
			"order_type":     o.OrderType,
			"total_qty":      o.TotalQty,
			"filled_qty":     o.FilledQty,
			"remaining_qty":  o.RemainingQty,
			"limit_price":    o.LimitPrice,
			"avg_fill_price": o.AvgFillPrice,
			"status":         o.Status,
			"outside_rth":    o.OutsideRTH,
		})
	}

	// Evaluate rules against BarFrameBundle (zero live orders)
	if strat != nil && bundle != nil {
		now := e.Now()
		for i := range evalItems {
			item := evalItems[i]
			evalSymbol := item.Symbol
			if item.InstrumentID != "" {
				for _, tf := range timeframes {
					if frame := bundle.GetFrame(item.InstrumentID, tf); frame != nil && frame.Len() > 0 {
						evalSymbol = item.InstrumentID
						break
					}
				}
			}

			// Evaluate ActionFilter rules first: all applicable enabled filter rules must pass
			filterPassed := true
			for _, rule := range strat.Rules {
				if !rule.Enabled || rule.Action != models.ActionFilter {
					continue
				}
				if rule.Scope == models.ScopeSymbol && rule.Name != item.Symbol && rule.Name != item.InstrumentID {
					continue
				}
				if !strategy.EvaluateRule(&rule, bundle, evalSymbol, now) {
					filterPassed = false
					break
				}
			}
			if !filterPassed {
				continue
			}

			signals := strategy.EvaluateRulesAt(strat, bundle, evalSymbol, now, item.Symbol, item.InstrumentID)
			for _, sig := range signals {
				if sig.Action != models.ActionBuy && sig.Action != models.ActionSell {
					continue
				}

				price := 0.0
				if item.InstrumentID != "" {
					price = e.GetLatestPrice(item.InstrumentID)
				}
				if price <= 0 {
					price = e.GetLatestPrice(item.Symbol)
				}
				if price <= 0 {
					for _, tf := range timeframes {
						if item.InstrumentID != "" {
							if frame := bundle.GetFrame(item.InstrumentID, tf); frame != nil && len(frame.Bars) > 0 {
								price = frame.Bars[len(frame.Bars)-1].Close
								break
							}
						}
						if frame := bundle.GetFrame(item.Symbol, tf); frame != nil && len(frame.Bars) > 0 {
							price = frame.Bars[len(frame.Bars)-1].Close
							break
						}
					}
				}

				qty := 0.0
				if price > 0 {
					if calc, err := e.CalculatePositionSize(&item, price, e.GetEquity()); err == nil {
						qty = calc
					}
				}
				canExec, gateReason := e.CanExecuteWithItem(&item, sig.Action, price)

				plannedOrders = append(plannedOrders, map[string]interface{}{
					"symbol":        item.Symbol,
					"instrument_id": item.InstrumentID,
					"action":        strings.ToUpper(string(sig.Action)),
					"rule":          sig.RuleName,
					"quantity":      qty,
					"price":         price,
					"can_execute":   canExec,
					"gate_reason":   gateReason,
					"dry_run":       true,
				})
			}
		}
	}

	generatedAt := time.Now().UTC().Format(time.RFC3339)
	e.SetLastDryRun(map[string]interface{}{
		"strategy":       activeStratName,
		"workspace_kind": wsKind,
		"subscriptions":  len(subscriptions),
		"planned_orders": len(plannedOrders),
		"generated_at":   generatedAt,
	})
	e.RecordRuntimeReload("dry_run")

	return &models.BotDryRun{
		Strategy:      activeStratName,
		WorkspaceKind: wsKind,
		Subscriptions: subscriptions,
		Positions:     positionsMap,
		OpenOrders:    openOrdersList,
		PlannedOrders: plannedOrders,
		State:         e.BuildRuntimeState(twsConnected),
		GeneratedAt:   generatedAt,
	}, nil
}
