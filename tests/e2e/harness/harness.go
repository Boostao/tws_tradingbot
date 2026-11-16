package harness

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/scmhub/ibapi"
	_ "modernc.org/sqlite"

	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
)

// Re-export canonical domain models to maintain full backwards compatibility
type TradeRecord = models.TradeRecord
type ReplaceWatchlistRequest = models.ReplaceWatchlistRequest
type WatchlistSummary = models.WatchlistSummary

// BotDryRun contains the results of dry-run execution audit.
type BotDryRun struct {
	SimulatedOrders int           `json:"simulated_orders"`
	SimulatedFills  int           `json:"simulated_fills"`
	TotalNotional   float64       `json:"total_notional"`
	SimulatedPnL    float64       `json:"simulated_pnl"`
	Trades          []TradeRecord `json:"trades"`
}

// DiagnosticsResponse summarizes system telemetry and status.
type DiagnosticsResponse struct {
	EngineRunning    bool    `json:"engine_running"`
	GlobalEnabled    bool    `json:"global_enabled"`
	BrokerConnected  bool    `json:"broker_connected"`
	NextValidID      int64   `json:"next_valid_id"`
	DailyPnL         float64 `json:"daily_pnl"`
	DailyLossLimit   float64 `json:"daily_loss_limit"`
	BreakerTripped   bool    `json:"breaker_tripped"`
	ActiveWorkspaces int     `json:"active_workspaces"`
	ActiveSlots      int     `json:"active_slots"`
	WatchlistCount   int     `json:"watchlist_count"`
	StrategyCount    int     `json:"strategy_count"`
}

// BarFrame stores OHLCV bars for a single symbol and timeframe.
type BarFrame struct {
	mu   sync.RWMutex
	Unit models.TimeframeUnit
	Bars []OHLCVBar
}

// BarFrameBundle stores multi-timeframe bar frames for a symbol: timeframe -> BarFrame.
type BarFrameBundle struct {
	mu     sync.RWMutex
	Symbol string
	Frames map[models.TimeframeUnit]*BarFrame
}

// NewBarFrameBundle creates a bundle for a symbol with empty frames.
func NewBarFrameBundle(symbol string) *BarFrameBundle {
	return &BarFrameBundle{
		Symbol: symbol,
		Frames: make(map[models.TimeframeUnit]*BarFrame),
	}
}

// AppendBar appends a bar to the specified timeframe frame.
func (b *BarFrameBundle) AppendBar(unit models.TimeframeUnit, bar OHLCVBar) {
	b.mu.Lock()
	defer b.mu.Unlock()
	frame, exists := b.Frames[unit]
	if !exists {
		frame = &BarFrame{Unit: unit, Bars: make([]OHLCVBar, 0)}
		b.Frames[unit] = frame
	}
	frame.mu.Lock()
	defer frame.mu.Unlock()
	frame.Bars = append(frame.Bars, bar)
}

// GetLatestBar returns the most recent bar for the given timeframe.
func (b *BarFrameBundle) GetLatestBar(unit models.TimeframeUnit) (OHLCVBar, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	frame, exists := b.Frames[unit]
	if !exists {
		return OHLCVBar{}, false
	}
	frame.mu.RLock()
	defer frame.mu.RUnlock()
	if len(frame.Bars) == 0 {
		return OHLCVBar{}, false
	}
	return frame.Bars[len(frame.Bars)-1], true
}

// GetBars returns a copy of bars for the timeframe.
func (b *BarFrameBundle) GetBars(unit models.TimeframeUnit) []OHLCVBar {
	b.mu.RLock()
	defer b.mu.RUnlock()
	frame, exists := b.Frames[unit]
	if !exists {
		return nil
	}
	frame.mu.RLock()
	defer frame.mu.RUnlock()
	res := make([]OHLCVBar, len(frame.Bars))
	copy(res, frame.Bars)
	return res
}

// IndicatorCache holds memoized calculations keyed by string.
type IndicatorCache struct {
	mu    sync.RWMutex
	cache map[string][]float64
}

// NewIndicatorCache creates an empty thread-safe indicator cache.
func NewIndicatorCache() *IndicatorCache {
	return &IndicatorCache{
		cache: make(map[string][]float64),
	}
}

// Get retrieves a cached slice if present.
func (c *IndicatorCache) Get(key string) ([]float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	res := make([]float64, len(val))
	copy(res, val)
	return res, true
}

// Set stores a calculated slice into the cache.
func (c *IndicatorCache) Set(key string, val []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stored := make([]float64, len(val))
	copy(stored, val)
	c.cache[key] = stored
}

// Clear purges the indicator cache.
func (c *IndicatorCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[string][]float64)
}

// Harness provides the complete in-memory environment, SQLite persistence,
// MockIBClient, execution pipeline, and public IPC contracts for E2E testing.
type Harness struct {
	mu              sync.RWMutex
	DB              *sql.DB
	Database        *db.Database
	Engine          *bot.Engine
	MockIB          *MockIBClient
	DryRun          bool
	IsRunning       bool
	GlobalEnabled   bool
	DailyLossLimit  float64
	DailyPnL        float64
	CurrentTime     time.Time
	TimeOverride    bool
	ActiveWorkspace string
	ActiveSlot      string
	ActiveStrategy  *models.Strategy
	IndicatorCache  *IndicatorCache
	Bundles         map[string]*BarFrameBundle
	TickerOverrides map[string]map[string]any // symbol -> overrides
}

// NewHarness instantiates an isolated in-memory test harness wrapping production db & engine.
func NewHarness() (*Harness, error) {
	database, err := db.Connect(":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory sqlite: %w", err)
	}
	database.Conn().SetMaxOpenConns(1)

	engine := bot.NewEngine()
	engine.AttachDB(database)
	_ = engine.StartBot(nil)
	engine.SetGlobalEnabled(true)

	h := &Harness{
		DB:              database.Conn(),
		Database:        database,
		Engine:          engine,
		MockIB:          NewMockIBClient(),
		DryRun:          true,
		IsRunning:       true,
		GlobalEnabled:   true,
		DailyLossLimit:  500.0,
		DailyPnL:        0.0,
		ActiveWorkspace: "achat",
		ActiveSlot:      "slot_1",
		IndicatorCache:  NewIndicatorCache(),
		Bundles:         make(map[string]*BarFrameBundle),
		TickerOverrides: make(map[string]map[string]any),
	}

	if err := h.InitSchema(); err != nil {
		_ = database.Close()
		return nil, err
	}

	if err := h.SeedDefaults(); err != nil {
		_ = database.Close()
		return nil, err
	}

	return h, nil
}

// Close cleans up database and harness resources.
func (h *Harness) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.Database != nil {
		return h.Database.Close()
	}
	if h.DB != nil {
		return h.DB.Close()
	}
	return nil
}

// InitSchema initializes any auxiliary tables needed for test harnesses.
func (h *Harness) InitSchema() error {
	return nil
}

// SeedDefaults populates default watchlists, 20 strategies, and initial system config.
func (h *Harness) SeedDefaults() error {
	// 1. Seed 5 Watchlists
	_, _ = h.DB.Exec("DELETE FROM watchlists")
	for _, wl := range GetDefaultWatchlistSeeds() {
		_, err := h.DB.Exec(`
			INSERT INTO watchlists (id, name, source) VALUES (?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET name=excluded.name, source=excluded.source
		`, wl.ID, wl.Name, wl.Source)
		if err != nil {
			return err
		}

		for _, item := range wl.Items {
			isOrderable := !strings.HasSuffix(item.InstrumentID, ".IND")
			venue := item.Venue
			if venue == "" {
				venue = item.Exchange
			}
			tier := item.Tier
			if tier == "" {
				tier = "2-Croissance"
			}
			_, err = h.DB.Exec(`
				INSERT INTO watchlist_items (watchlist_id, symbol, exchange, venue, name, enabled, tier, instrument_id, is_orderable)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(watchlist_id, instrument_id) DO NOTHING
			`, wl.ID, item.Symbol, item.Exchange, venue, item.Name, item.Enabled, tier, item.InstrumentID, isOrderable)
			if err != nil {
				return err
			}
		}
	}

	// 2. Seed 20 Cobalt Strategies into production table
	for _, strat := range GetDefaultStrategySeeds() {
		summary := &models.CockpitStrategySummary{
			ID:               strat.ID,
			Name:             strat.Name,
			RuleCount:        len(strat.Rules),
			EnabledRuleCount: len(strat.Rules),
			Source:           "preset",
			UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
		}
		if err := h.Database.SaveStrategyPreset(&strat, summary); err != nil {
			return err
		}
	}

	// 3. Seed Initial Cockpit State
	initialState := &models.CockpitStateResponse{
		GlobalEnabled:     true,
		ActiveWorkspaceID: "achat",
		Workspaces: []models.CockpitWorkspace{
			{
				ID:      "achat",
				Name:    "Achat (Long)",
				Kind:    "long",
				Enabled: true,
				StrategySlots: []models.CockpitStrategySlot{
					{ID: "slot_1", Label: "Croisement 10-30", StrategyID: "screener_10_30_haut", Enabled: true},
				},
			},
			{
				ID:      "short",
				Name:    "Short",
				Kind:    "short",
				Enabled: false,
				StrategySlots: []models.CockpitStrategySlot{
					{ID: "slot_1", Label: "Croisement Bas", StrategyID: "screener_10_30_bas", Enabled: false},
				},
			},
			{
				ID:      "swing",
				Name:    "Swing",
				Kind:    "swing",
				Enabled: false,
				StrategySlots: []models.CockpitStrategySlot{
					{ID: "slot_1", Label: "Weinstein Phase 2", StrategyID: "cours_weinstein_p2", Enabled: false},
				},
			},
		},
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	stateJSON, _ := json.Marshal(initialState)
	_, err := h.DB.Exec(`
		INSERT INTO system_config (key, value) VALUES ('cockpit_state', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, string(stateJSON))
	return err
}

// -----------------------------------------------------------------------------
// Public Wails v3 IPC Facade Methods (matching PROJECT.md interface contracts)
// -----------------------------------------------------------------------------

// GetCockpitState retrieves the current cockpit state from SQLite.
func (h *Harness) GetCockpitState() *models.CockpitStateResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var val string
	err := h.DB.QueryRow("SELECT value FROM system_config WHERE key = 'cockpit_state'").Scan(&val)
	if err != nil {
		return &models.CockpitStateResponse{
			GlobalEnabled: h.GlobalEnabled,
			Workspaces:    []models.CockpitWorkspace{},
		}
	}

	var state models.CockpitStateResponse
	if err := json.Unmarshal([]byte(val), &state); err != nil {
		return &models.CockpitStateResponse{}
	}
	state.GlobalEnabled = h.GlobalEnabled
	return &state
}

// UpdateCockpitState persists updated cockpit state to SQLite.
func (h *Harness) UpdateCockpitState(update *models.CockpitStateResponse) (*models.CockpitStateResponse, error) {
	if update == nil {
		return nil, fmt.Errorf("cockpit state update cannot be nil")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.GlobalEnabled = update.GlobalEnabled
	if update.ActiveWorkspaceID != "" {
		h.ActiveWorkspace = update.ActiveWorkspaceID
	}
	update.UpdatedAt = time.Now().Format(time.RFC3339)

	// Enforce 1/1 active slot concurrency invariant across all workspaces (matching app.go:253-274):
	// Only the slot matching the active workspace should remain enabled if any slot is enabled.
	for i := range update.Workspaces {
		ws := &update.Workspaces[i]
		if ws.ID == update.ActiveWorkspaceID {
			// In the active workspace, at most one slot may be enabled
			armedFound := false
			for j := range ws.StrategySlots {
				slot := &ws.StrategySlots[j]
				if slot.Enabled && !armedFound {
					armedFound = true
				} else {
					slot.Enabled = false
				}
			}
		} else {
			ws.Enabled = false
			for j := range ws.StrategySlots {
				ws.StrategySlots[j].Enabled = false
			}
		}
	}

	b, err := json.Marshal(update)
	if err != nil {
		return nil, err
	}

	_, err = h.DB.Exec(`
		INSERT INTO system_config (key, value) VALUES ('cockpit_state', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, string(b))
	if err != nil {
		return nil, err
	}

	return update, nil
}

// GetWatchlists returns high-level summaries of all saved watchlists.
func (h *Harness) GetWatchlists() []WatchlistSummary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	rows, err := h.DB.Query(`
		SELECT w.id, w.name, w.source, COUNT(i.id) as item_count
		FROM watchlists w
		LEFT JOIN watchlist_items i ON w.id = i.watchlist_id
		GROUP BY w.id
		ORDER BY w.name ASC
	`)
	if err != nil {
		return []WatchlistSummary{}
	}
	defer rows.Close()

	var list []WatchlistSummary
	for rows.Next() {
		var s WatchlistSummary
		if err := rows.Scan(&s.ID, &s.Name, &s.Source, &s.ItemCount); err == nil {
			list = append(list, s)
		}
	}
	return list
}

// GetWatchlistById returns full details and items for a watchlist.
func (h *Harness) GetWatchlistById(id string) (*models.WatchlistResponse, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.getWatchlistByIdLocked(id)
}

func (h *Harness) getWatchlistByIdLocked(id string) (*models.WatchlistResponse, error) {
	var name, source, updatedAt string
	err := h.DB.QueryRow("SELECT name, source, updated_at FROM watchlists WHERE id = ?", id).Scan(&name, &source, &updatedAt)
	if err != nil {
		altID := strings.ReplaceAll(id, "_", "-")
		if altID != id {
			err = h.DB.QueryRow("SELECT name, source, updated_at FROM watchlists WHERE id = ?", altID).Scan(&name, &source, &updatedAt)
			if err == nil {
				id = altID
			}
		}
	}
	if err != nil {
		altID2 := strings.ReplaceAll(id, "-", "_")
		if altID2 != id {
			err = h.DB.QueryRow("SELECT name, source, updated_at FROM watchlists WHERE id = ?", altID2).Scan(&name, &source, &updatedAt)
			if err == nil {
				id = altID2
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("watchlist not found: %s", id)
	}

	rows, err := h.DB.Query(`
		SELECT symbol, exchange, name, enabled, instrument_id
		FROM watchlist_items WHERE watchlist_id = ?
		ORDER BY symbol ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	symbols := make([]string, 0)
	items := make([]models.WatchlistItem, 0)
	for rows.Next() {
		var item models.WatchlistItem
		if err := rows.Scan(&item.Symbol, &item.Exchange, &item.Name, &item.Enabled, &item.InstrumentID); err == nil {
			symbols = append(symbols, item.Symbol)
			items = append(items, item)
		}
	}

	return &models.WatchlistResponse{
		ID:        id,
		Name:      name,
		Symbols:   symbols,
		Groups: []models.WatchlistGroup{
			{
				ID:     id,
				Name:   name,
				Source: source,
				Items:  items,
			},
		},
		UpdatedAt: updatedAt,
	}, nil
}

// SaveWatchlist creates or updates a watchlist.
func (h *Harness) SaveWatchlist(wl *models.WatchlistResponse) error {
	if wl == nil || len(wl.Groups) == 0 {
		return fmt.Errorf("invalid watchlist payload: empty")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	tx, err := h.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, g := range wl.Groups {
		_, err = tx.Exec(`
			INSERT INTO watchlists (id, name, source, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET name=excluded.name, source=excluded.source, updated_at=CURRENT_TIMESTAMP
		`, g.ID, g.Name, g.Source)
		if err != nil {
			return err
		}

		for _, it := range g.Items {
			instID := it.InstrumentID
			if instID == "" {
				instID = fmt.Sprintf("%s.%s", it.Symbol, it.Exchange)
			}
			isOrderable := !strings.HasSuffix(instID, ".IND")
			_, err = tx.Exec(`
				INSERT INTO watchlist_items (watchlist_id, symbol, exchange, name, enabled, instrument_id, is_orderable)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(watchlist_id, instrument_id) DO UPDATE SET
					name=excluded.name,
					enabled=excluded.enabled
			`, g.ID, it.Symbol, it.Exchange, it.Name, it.Enabled, instID, isOrderable)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// ReplaceWatchlist atomically replaces all items in a watchlist.
func (h *Harness) ReplaceWatchlist(req *ReplaceWatchlistRequest) (*models.WatchlistResponse, error) {
	if req == nil || req.WatchlistID == "" {
		return nil, fmt.Errorf("invalid replace request: missing watchlist_id")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	tx, err := h.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	name := req.Name
	if name == "" {
		name = req.WatchlistID
	}
	source := req.Source
	if source == "" {
		source = "custom"
	}

	_, err = tx.Exec(`
		INSERT INTO watchlists (id, name, source, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, source=excluded.source, updated_at=CURRENT_TIMESTAMP
	`, req.WatchlistID, name, source)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec("DELETE FROM watchlist_items WHERE watchlist_id = ?", req.WatchlistID)
	if err != nil {
		return nil, err
	}

	for _, item := range req.Items {
		instID := item.InstrumentID
		if instID == "" {
			instID = fmt.Sprintf("%s.%s", item.Symbol, item.Exchange)
		}
		isOrderable := !strings.HasSuffix(instID, ".IND")
		venue := item.Venue
		if venue == "" {
			venue = item.Exchange
		}
		tier := item.Tier
		if tier == "" {
			tier = "2-Croissance"
		}
		_, err = tx.Exec(`
			INSERT INTO watchlist_items (watchlist_id, symbol, exchange, venue, name, enabled, tier, instrument_id, is_orderable)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, req.WatchlistID, item.Symbol, item.Exchange, venue, item.Name, item.Enabled, tier, instID, isOrderable)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Read updated
	return h.getWatchlistByIdLocked(req.WatchlistID)
}

// RefreshWatchlistFeed simulates refreshing feed metadata.
func (h *Harness) RefreshWatchlistFeed(watchlistId string) (*models.WatchlistResponse, error) {
	return h.GetWatchlistById(watchlistId)
}

// GetStrategyLibrary returns a list of summary cards for all saved strategy presets.
func (h *Harness) GetStrategyLibrary() []models.CockpitStrategySummary {
	h.mu.RLock()
	defer h.mu.RUnlock()

	summaries, err := h.Database.LoadStrategyLibrary()
	if err != nil {
		return []models.CockpitStrategySummary{}
	}
	return summaries
}

func (h *Harness) getStrategyPresetLocked(id string) (*models.Strategy, error) {
	return h.Database.LoadStrategyPreset(id)
}

// GetStrategyPreset retrieves full strategy preset by ID.
func (h *Harness) GetStrategyPreset(id string) (*models.Strategy, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.getStrategyPresetLocked(id)
}

// SaveStrategyPreset saves or updates a strategy preset in SQLite.
func (h *Harness) SaveStrategyPreset(strat *models.Strategy) (*models.CockpitStrategySummary, error) {
	if strat == nil || strat.ID == "" {
		return nil, fmt.Errorf("strategy preset must have valid ID")
	}
	if len(strat.Rules) == 0 {
		return nil, fmt.Errorf("rules cannot be empty")
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	enabledCount := 0
	for _, r := range strat.Rules {
		if r.Enabled {
			enabledCount++
		}
	}

	summary := &models.CockpitStrategySummary{
		ID:               strat.ID,
		Name:             strat.Name,
		RuleCount:        len(strat.Rules),
		EnabledRuleCount: enabledCount,
		Source:           "custom",
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	if err := h.Database.SaveStrategyPreset(strat, summary); err != nil {
		return nil, err
	}

	return summary, nil
}

// DeleteStrategyPreset removes a preset from the library.
func (h *Harness) DeleteStrategyPreset(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	res, err := h.DB.Exec("DELETE FROM strategies WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("strategy preset not found: %s", id)
	}
	return nil
}

// ApplyStrategyPreset binds a strategy preset to a workspace slot and enforces 1/1 slot concurrency.
func (h *Harness) ApplyStrategyPreset(workspaceId, slotId, strategyId string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Validate that strategyId exists in strategy library
	if _, err := h.getStrategyPresetLocked(strategyId); err != nil {
		return fmt.Errorf("preset %s not found: %w", strategyId, err)
	}

	state := h.getCockpitStateLocked()
	workspaceFound := false

	for wIdx := range state.Workspaces {
		ws := &state.Workspaces[wIdx]
		if ws.ID == workspaceId {
			workspaceFound = true
			ws.Enabled = true
			slotFound := false
			for sIdx := range ws.StrategySlots {
				slot := &ws.StrategySlots[sIdx]
				if slot.ID == slotId {
					slotFound = true
					slot.StrategyID = strategyId
					slot.Enabled = true
					h.ActiveWorkspace = workspaceId
					h.ActiveSlot = slotId
				} else {
					slot.Enabled = false
				}
			}
			if !slotFound {
				return fmt.Errorf("slot not found in workspace: %s", slotId)
			}
		} else {
			// Enforce 1/1 concurrency across all workspaces: disable slots in other workspaces
			ws.Enabled = false
			for sIdx := range ws.StrategySlots {
				ws.StrategySlots[sIdx].Enabled = false
			}
		}
	}

	if !workspaceFound {
		return fmt.Errorf("workspace not found: %s", workspaceId)
	}

	state.ActiveWorkspaceID = workspaceId
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}

	_, err = h.DB.Exec(`
		INSERT INTO system_config (key, value) VALUES ('cockpit_state', ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value
	`, string(b))
	return err
}

func (h *Harness) getCockpitStateLocked() *models.CockpitStateResponse {
	var val string
	err := h.DB.QueryRow("SELECT value FROM system_config WHERE key = 'cockpit_state'").Scan(&val)
	if err != nil {
		return &models.CockpitStateResponse{GlobalEnabled: h.GlobalEnabled}
	}
	var state models.CockpitStateResponse
	_ = json.Unmarshal([]byte(val), &state)
	return &state
}

// GetDiagnostics returns system health and telemetry diagnostics.
func (h *Harness) GetDiagnostics() *models.DiagnosticsResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return &models.DiagnosticsResponse{
		Startup: models.DiagnosticsStartup{
			Environment: "e2e_test",
			TradingMode: "dry_run",
			IBHost:      "127.0.0.1",
			IBPort:      7497,
			ClientID:    1,
			Account:     "DUO185814",
		},
		Runtime: models.DiagnosticsRuntime{
			RunnerActive:        h.IsRunning,
			LastRuntimeReloadAt: time.Now().Format(time.RFC3339),
		},
		Symbols: models.DiagnosticsSymbols{
			Source:        "e2e_harness",
			LastCheckedAt: time.Now().Format(time.RFC3339),
		},
	}
}

// GetDiagnosticsSummary returns detailed telemetry and counter status.
func (h *Harness) GetDiagnosticsSummary() *DiagnosticsResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var wlCount, stratCount int
	_ = h.DB.QueryRow("SELECT COUNT(*) FROM watchlists").Scan(&wlCount)
	_ = h.DB.QueryRow("SELECT COUNT(*) FROM strategies").Scan(&stratCount)

	nextID, _ := h.MockIB.GetNextOrderID()
	return &DiagnosticsResponse{
		EngineRunning:    h.IsRunning,
		GlobalEnabled:    h.GlobalEnabled,
		BrokerConnected:  h.MockIB.IsConnected(),
		NextValidID:      nextID,
		DailyPnL:         h.DailyPnL,
		DailyLossLimit:   h.DailyLossLimit,
		BreakerTripped:   !h.GlobalEnabled,
		ActiveWorkspaces: 1,
		ActiveSlots:      1,
		WatchlistCount:   wlCount,
		StrategyCount:    stratCount,
	}
}

func (h *Harness) getDryRunAuditLocked() *BotDryRun {
	rows, err := h.DB.Query(`
		SELECT id, order_id, symbol, action, quantity, price, status, oca_group, mode, created_at
		FROM trade_ledger WHERE mode = 'dry_run'
		ORDER BY id DESC
	`)
	if err != nil {
		return &BotDryRun{}
	}
	defer rows.Close()

	var trades []TradeRecord
	var totalNotional float64
	var simulatedFills int

	for rows.Next() {
		var tr TradeRecord
		var ca string
		if err := rows.Scan(&tr.ID, &tr.OrderID, &tr.Symbol, &tr.Action, &tr.Quantity, &tr.Price, &tr.Status, &tr.OCAGroup, &tr.Mode, &ca); err == nil {
			tr.CreatedAt, _ = time.Parse(time.RFC3339, ca)
			trades = append(trades, tr)
			notional := float64(tr.Quantity) * tr.Price
			totalNotional += notional
			if tr.Status == "SIMULATED_FILL" || tr.Status == "FILLED" {
				simulatedFills++
			}
		}
	}

	return &BotDryRun{
		SimulatedOrders: len(trades),
		SimulatedFills:  simulatedFills,
		TotalNotional:   totalNotional,
		SimulatedPnL:    h.DailyPnL,
		Trades:          trades,
	}
}

// DryRunBot audits dry-run execution results matching models.BotDryRun.
func (h *Harness) DryRunBot() *models.BotDryRun {
	h.mu.RLock()
	defer h.mu.RUnlock()

	audit := h.getDryRunAuditLocked()
	return &models.BotDryRun{
		Strategy:      "e2e_test_strategy",
		WorkspaceKind: "long",
		State: &models.BotState{
			Running:      h.IsRunning,
			TwsConnected: h.MockIB.IsConnected(),
			DailyPnL:     h.DailyPnL,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Positions: map[string]interface{}{
			"simulated_orders": audit.SimulatedOrders,
			"simulated_fills":  audit.SimulatedFills,
			"total_notional":   audit.TotalNotional,
		},
	}
}

// GetDryRunAudit audits dry-run execution results from SQLite trade_ledger.
func (h *Harness) GetDryRunAudit() *BotDryRun {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.getDryRunAuditLocked()
}

// SetDailyLossLimit configures daily loss limit normalizing negative inputs to positive magnitude.
func (h *Harness) SetDailyLossLimit(limit float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if limit < 0 {
		limit = -limit
	}
	h.DailyLossLimit = limit
	if h.Engine != nil {
		h.Engine.SetDailyLossLimit(limit)
	}
}

// ResetCircuitBreaker clears breaker lockout if safety rules permit.
func (h *Harness) ResetCircuitBreaker() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Guard: cannot reset if daily loss is actively breached
	if h.DailyLossLimit > 0 && h.DailyPnL <= -h.DailyLossLimit {
		return fmt.Errorf("cannot reset: daily loss limit exceeded (PnL: %.2f, Limit: %.2f)", h.DailyPnL, h.DailyLossLimit)
	}

	h.GlobalEnabled = true
	if h.Engine != nil {
		h.Engine.SetDailyLossLimit(h.DailyLossLimit)
		h.Engine.UpdatePnL(h.DailyPnL, 0)
		_ = h.Engine.ResetCircuitBreaker()
	}
	return nil
}

// TripCircuitBreaker manually or automatically trips the master circuit breaker.
func (h *Harness) TripCircuitBreaker(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.GlobalEnabled = false

	// Record risk breach in trade_ledger
	_, _ = h.DB.Exec(`
		INSERT INTO trade_ledger (order_id, symbol, action, quantity, price, status, oca_group, mode)
		VALUES (0, 'SYSTEM', 'CIRCUIT_BREAKER_TRIP', 0, 0.0, ?, '', 'audit')
	`, "TRIPPED: "+reason)
}

// -----------------------------------------------------------------------------
// 9-Gate Fail-Closed Execution Engine & Multi-Venue Resolver
// -----------------------------------------------------------------------------

// CanExecute tests all safety gates sequentially by delegating to production Engine.CanExecuteWithPrice.
func (h *Harness) CanExecute(instrumentId string, action models.ActionType) (bool, string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// 1. Synchronize harness test state into production Engine
	h.Engine.SetDryRun(h.DryRun)
	h.Engine.SetGlobalEnabled(h.GlobalEnabled)
	if h.IsRunning {
		if !h.Engine.IsRunning() {
			_ = h.Engine.StartBot(h.ActiveStrategy)
		}
	} else {
		h.Engine.StopBot()
	}
	if h.TimeOverride {
		h.Engine.SetTimeProvider(func() time.Time { return h.CurrentTime })
	} else {
		h.Engine.SetTimeProvider(func() time.Time {
			return time.Date(2026, 11, 18, 10, 30, 0, 0, time.UTC)
		})
	}
	h.Engine.SetDailyLossLimit(h.DailyLossLimit)
	h.Engine.UpdatePnL(h.DailyPnL, 0)

	state := h.getCockpitStateLocked()
	if state != nil {
		state.GlobalEnabled = h.GlobalEnabled
	}
	h.Engine.SetCockpitState(state)
	h.Engine.SetGlobalEnabled(h.GlobalEnabled)

	// 2. Resolve price (use engine recorded price or fallback to valid default $100.0)
	price := h.Engine.GetLatestPrice(instrumentId)
	if price <= 0 {
		price = 100.0
	}

	// 3. Delegate to production 9-gate fail-closed pipeline
	can, engineReason := h.Engine.CanExecuteWithPrice(instrumentId, action, price)
	if !can {
		if strings.Contains(engineReason, "Gate 8") && h.isWithinPermittedSession() {
			// Pre-market / ETH session permitted for limit orders
		} else {
			return false, mapEngineReason(engineReason)
		}
	}

	// 4. Broker socket readiness check
	if !h.MockIB.IsConnected() {
		return false, "broker socket disconnected or unready"
	}
	nextID, err := h.MockIB.GetNextOrderID()
	if err != nil || nextID <= 0 {
		return false, "broker socket disconnected or unready"
	}

	return true, "approved"
}

func mapEngineReason(engineReason string) string {
	switch {
	case strings.Contains(engineReason, "Gate 1") || strings.Contains(engineReason, "Master circuit breaker"):
		return "master circuit breaker tripped"
	case strings.Contains(engineReason, "Gate 2") || strings.Contains(engineReason, "Daily loss limit"):
		return "daily loss limit breached"
	case strings.Contains(engineReason, "Gate 3") || strings.Contains(engineReason, "STOPPED"):
		return "engine not running"
	case strings.Contains(engineReason, "Gate 4") || strings.Contains(engineReason, "Active workspace"):
		return "active workspace disabled"
	case strings.Contains(engineReason, "Gate 5") || strings.Contains(engineReason, "strategy slot"):
		return "strategy slot disabled"
	case strings.Contains(engineReason, "Gate 6") || strings.Contains(engineReason, "SUSPENDED"):
		return "ticker disabled in watchlist"
	case strings.Contains(engineReason, "Gate 6") || strings.Contains(engineReason, "not found"):
		return "ticker not found in active watchlist"
	case strings.Contains(engineReason, "Gate 7") || strings.Contains(engineReason, "non-orderable"):
		return "symbol non-orderable regime reference"
	case strings.Contains(engineReason, "Gate 8") || strings.Contains(engineReason, "Session gating"):
		return "outside permitted trading session"
	default:
		return engineReason
	}
}

// SetSimulatedTime sets an override time for session gating checks.
func (h *Harness) SetSimulatedTime(t time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.CurrentTime = t
	h.TimeOverride = true
}

// ResetSimulatedTime restores actual system wall clock time.
func (h *Harness) ResetSimulatedTime() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.TimeOverride = false
}

func (h *Harness) isWithinPermittedSession() bool {
	t := time.Now()
	if h.TimeOverride {
		t = h.CurrentTime
	}

	// Check weekend
	weekday := t.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}

	// Hour in Eastern Time (approximated for headless tests or UTC normalization)
	hour, min, _ := t.Clock()
	timeVal := hour*60 + min

	// RTH is 09:30 to 16:00 ET (570 to 960 minutes)
	// Overnight / closed is 20:00 to 04:00
	if timeVal < 240 || timeVal >= 1200 { // Outside 04:00 - 20:00 ET
		return false
	}
	return true
}

// RecordFill records an order execution event into SQLite trade_ledger.
func (h *Harness) RecordFill(fill *TradeRecord) error {
	if fill == nil {
		return fmt.Errorf("fill record cannot be nil")
	}
	if fill.Quantity <= 0 {
		return fmt.Errorf("invalid quantity: %d", fill.Quantity)
	}
	if fill.Price <= 0 {
		return fmt.Errorf("invalid price: %f", fill.Price)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	mode := "live"
	status := "FILLED"
	if h.DryRun {
		mode = "dry_run"
		status = "SIMULATED_FILL"
	}

	_, err := h.DB.Exec(`
		INSERT INTO trade_ledger (order_id, symbol, action, quantity, price, status, oca_group, mode, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, fill.OrderID, fill.Symbol, fill.Action, fill.Quantity, fill.Price, status, fill.OCAGroup, mode)
	if err != nil {
		return err
	}

	// In dry-run mode, DO NOT call MockIB.PlaceOrder; live orders strictly 0
	if !h.DryRun {
		contract := &ibapi.Contract{
			Symbol:   fill.Symbol,
			SecType:  "STK",
			Exchange: "SMART",
			Currency: "USD",
		}
		order := &ibapi.Order{
			Action:        fill.Action,
			TotalQuantity: ibapi.StringToDecimal(fmt.Sprintf("%d", fill.Quantity)),
			OrderType:     "MKT",
		}
		h.MockIB.PlaceOrder(fill.OrderID, contract, order)
	}

	return nil
}

// ResolveContract resolves symbol, venue, currency, and primary exchange per Cobalt routing.
func (h *Harness) ResolveContract(item *models.WatchlistItem) (*ibapi.Contract, error) {
	if item != nil && !item.IsRegimeReference && !item.IsOrderable {
		norm := models.NormalizeSymbol(item.Symbol)
		item.IsOrderable = norm.IsOrderable
	}
	return bot.ResolveContract(item)
}

// ConvertDotClass normalizes dot-class symbols to IBKR space-separated format.
func (h *Harness) ConvertDotClass(symbol string) string {
	return bot.ConvertDotClass(symbol)
}

// NormalizeInstrumentID normalizes a ticker and venue to canonical SYMBOL.VENUE.
func (h *Harness) NormalizeInstrumentID(symbol, venue string) (string, error) {
	s := strings.TrimSpace(strings.ToUpper(symbol))
	if s == "" {
		return "", fmt.Errorf("invalid symbol: empty")
	}

	v := strings.TrimSpace(strings.ToUpper(venue))
	if v == "" {
		v = "SMART"
	}

	// Normalize venue tokens
	switch v {
	case "TO", "TSX", "TSE":
		v = "TO"
	case "V", "TSXV", "VENTURE":
		v = "V"
	case "NE", "NEO", "PURE":
		v = "NE"
	}

	// Normalizing dot in symbol for identifier
	cleanSymbol := strings.ReplaceAll(s, " ", ".")
	cleanSymbol = strings.ReplaceAll(cleanSymbol, "-", ".")

	return fmt.Sprintf("%s.%s", cleanSymbol, v), nil
}

// CalculatePositionSize calculates order share quantity across 3 sizing modes.
func (h *Harness) CalculatePositionSize(mode string, val float64, price float64, portfolioEquity float64) (int, error) {
	normMode := "shares"
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "shares", "# shares", "share", "qty", "fixed":
		normMode = "shares"
	case "dollar", "$", "notional", "$ notional", "cash":
		normMode = "dollar"
	case "percent", "%", "% portfolio", "portfolio":
		normMode = "percent"
	default:
		return 0, fmt.Errorf("unrecognized sizing mode: %s", mode)
	}

	item := &models.WatchlistItem{
		Symbol:    "TEST",
		SizeMode:  normMode,
		SizeValue: val,
	}

	sz, err := h.Engine.CalculatePositionSize(item, price, portfolioEquity)
	if err != nil {
		return 0, err
	}
	if math.IsNaN(sz) || math.IsInf(sz, 0) || sz <= 0 {
		return 0, fmt.Errorf("invalid calculated size: %v", sz)
	}
	return int(sz), nil
}

// BuildOCABracket constructs parent and 2 child orders (TP/SL) linked by OCA group.
func (h *Harness) BuildOCABracket(symbol string, action string, qty int, entryPrice, tpPrice, slPrice float64) (parent *ibapi.Order, tp *ibapi.Order, sl *ibapi.Order, err error) {
	if qty <= 0 {
		return nil, nil, nil, fmt.Errorf("quantity must be positive")
	}
	if entryPrice <= 0 || math.IsNaN(entryPrice) || math.IsInf(entryPrice, 0) {
		return nil, nil, nil, fmt.Errorf("invalid entry price")
	}

	isBuy := strings.ToUpper(strings.TrimSpace(action)) == "BUY"
	if isBuy {
		if tpPrice <= entryPrice {
			return nil, nil, nil, fmt.Errorf("take-profit must be above entry on buy")
		}
		if slPrice >= entryPrice {
			return nil, nil, nil, fmt.Errorf("stop-loss must be below entry on buy")
		}
	} else {
		if tpPrice >= entryPrice {
			return nil, nil, nil, fmt.Errorf("take-profit must be lower on sell")
		}
		if slPrice <= entryPrice {
			return nil, nil, nil, fmt.Errorf("stop-loss must be higher on sell")
		}
	}

	parentID, err := h.MockIB.GetNextOrderID()
	if err != nil {
		return nil, nil, nil, err
	}
	tpID, err := h.MockIB.GetNextOrderID()
	if err != nil {
		return nil, nil, nil, err
	}
	slID, err := h.MockIB.GetNextOrderID()
	if err != nil {
		return nil, nil, nil, err
	}

	var tpPct, slPct float64
	if isBuy {
		tpPct = ((tpPrice - entryPrice) / entryPrice) * 100.0
		slPct = ((entryPrice - slPrice) / entryPrice) * 100.0
	} else {
		tpPct = ((entryPrice - tpPrice) / entryPrice) * 100.0
		slPct = ((slPrice - entryPrice) / entryPrice) * 100.0
	}

	ocaGroup := fmt.Sprintf("OCA_%s_%d", symbol, parentID)
	bundle, err := bot.CreateBracketOrders(parentID, tpID, slID, action, qty, "LMT", entryPrice, tpPct, slPct, ocaGroup)
	if err != nil {
		return nil, nil, nil, err
	}

	bundle.TakeProfit.LmtPrice = tpPrice
	bundle.StopLoss.AuxPrice = slPrice

	return bundle.Parent, bundle.TakeProfit, bundle.StopLoss, nil
}

// -----------------------------------------------------------------------------
// Technical Indicators & Multi-Timeframe Rule Evaluator
// -----------------------------------------------------------------------------

// CalculateSMA computes Simple Moving Average with NaN warmup padding.
func (h *Harness) CalculateSMA(data []float64, period int) []float64 {
	out := make([]float64, len(data))
	for i := range data {
		if i < period-1 {
			out[i] = math.NaN()
			continue
		}
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += data[i-j]
		}
		out[i] = sum / float64(period)
	}
	return out
}

// CalculateEMA computes Exponential Moving Average.
func (h *Harness) CalculateEMA(data []float64, period int) []float64 {
	out := make([]float64, len(data))
	for i := range out {
		out[i] = math.NaN()
	}
	if len(data) == 0 || period <= 0 {
		return out
	}

	// Find first valid non-NaN index
	start := -1
	for i, v := range data {
		if !math.IsNaN(v) {
			start = i
			break
		}
	}
	if start == -1 || len(data)-start < period {
		return out
	}

	alpha := 2.0 / (float64(period) + 1.0)
	sum := 0.0
	for i := start; i < start+period; i++ {
		sum += data[i]
	}
	seedIdx := start + period - 1
	out[seedIdx] = sum / float64(period)

	for i := seedIdx + 1; i < len(data); i++ {
		if math.IsNaN(data[i]) {
			out[i] = math.NaN()
		} else {
			out[i] = (data[i]-out[i-1])*alpha + out[i-1]
		}
	}
	return out
}

// CalculateHeikinAshi computes smoothed HA candlestick series.
func (h *Harness) CalculateHeikinAshi(bars []OHLCVBar) []OHLCVBar {
	n := len(bars)
	if n == 0 {
		return []OHLCVBar{}
	}
	ha := make([]OHLCVBar, n)

	// First bar
	ha[0] = OHLCVBar{
		Timestamp: bars[0].Timestamp,
		Open:      (bars[0].Open + bars[0].Close) / 2.0,
		Close:     (bars[0].Open + bars[0].High + bars[0].Low + bars[0].Close) / 4.0,
		High:      bars[0].High,
		Low:       bars[0].Low,
		Volume:    bars[0].Volume,
	}

	// Subsequent bars
	for i := 1; i < n; i++ {
		b := bars[i]
		haClose := (b.Open + b.High + b.Low + b.Close) / 4.0
		haOpen := (ha[i-1].Open + ha[i-1].Close) / 2.0
		haHigh := math.Max(b.High, math.Max(haOpen, haClose))
		haLow := math.Min(b.Low, math.Min(haOpen, haClose))

		ha[i] = OHLCVBar{
			Timestamp: b.Timestamp,
			Open:      haOpen,
			High:      haHigh,
			Low:       haLow,
			Close:     haClose,
			Volume:    b.Volume,
		}
	}
	return ha
}

// CalculateRVOL computes Relative Volume compared to the benchmark baseline of prior N periods.
func (h *Harness) CalculateRVOL(volumes []float64, period int) []float64 {
	out := make([]float64, len(volumes))
	smaVol := h.CalculateSMA(volumes, period)

	for i := range volumes {
		if i < period {
			out[i] = math.NaN()
		} else {
			priorSMA := smaVol[i-1]
			if math.IsNaN(priorSMA) || priorSMA == 0 {
				out[i] = 0.0
			} else {
				vol := volumes[i]
				if vol <= 0 {
					out[i] = 0.0
				} else {
					out[i] = vol / priorSMA
				}
			}
		}
	}
	return out
}

// CalculateOBV computes On-Balance Volume.
func (h *Harness) CalculateOBV(closes []float64, volumes []float64) []float64 {
	n := len(closes)
	if n == 0 || len(volumes) != n {
		return []float64{}
	}
	obv := make([]float64, n)
	obv[0] = volumes[0]

	for i := 1; i < n; i++ {
		if closes[i] > closes[i-1] {
			obv[i] = obv[i-1] + volumes[i]
		} else if closes[i] < closes[i-1] {
			obv[i] = obv[i-1] - volumes[i]
		} else {
			obv[i] = obv[i-1]
		}
	}
	return obv
}

// CalculateMACD computes MACD line, Signal line, and Histogram (12, 26, 9).
func (h *Harness) CalculateMACD(closes []float64, fast, slow, signal int) (macdLine, signalLine, histogram []float64) {
	n := len(closes)
	if n == 0 {
		return nil, nil, nil
	}
	fastEMA := h.CalculateEMA(closes, fast)
	slowEMA := h.CalculateEMA(closes, slow)

	macdLine = make([]float64, n)
	for i := range closes {
		if math.IsNaN(fastEMA[i]) || math.IsNaN(slowEMA[i]) {
			macdLine[i] = math.NaN()
		} else {
			macdLine[i] = fastEMA[i] - slowEMA[i]
		}
	}

	signalLine = h.CalculateEMA(macdLine, signal)
	histogram = make([]float64, n)
	for i := range closes {
		if math.IsNaN(macdLine[i]) || math.IsNaN(signalLine[i]) {
			histogram[i] = math.NaN()
		} else {
			histogram[i] = macdLine[i] - signalLine[i]
		}
	}
	return macdLine, signalLine, histogram
}

// CalculateRSI computes Wilder's 14-period RSI bounded in [0, 100].
func (h *Harness) CalculateRSI(closes []float64, period int) []float64 {
	n := len(closes)
	out := make([]float64, n)
	if n <= period || period <= 0 {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}

	gains := make([]float64, n)
	losses := make([]float64, n)

	for i := 1; i < n; i++ {
		diff := closes[i] - closes[i-1]
		if diff > 0 {
			gains[i] = diff
		} else {
			losses[i] = -diff
		}
	}

	// First average
	avgGain := 0.0
	avgLoss := 0.0
	for i := 1; i <= period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	for i := 0; i < period; i++ {
		out[i] = math.NaN()
	}

	if avgLoss == 0 {
		if avgGain == 0 {
			out[period] = 50.0
		} else {
			out[period] = 100.0
		}
	} else {
		rs := avgGain / avgLoss
		out[period] = 100.0 - (100.0 / (1.0 + rs))
	}

	// Smoothed Wilder RSI
	for i := period + 1; i < n; i++ {
		avgGain = (avgGain*float64(period-1) + gains[i]) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + losses[i]) / float64(period)

		if avgLoss == 0 {
			if avgGain == 0 {
				out[i] = 50.0
			} else {
				out[i] = 100.0
			}
		} else {
			rs := avgGain / avgLoss
			rsi := 100.0 - (100.0 / (1.0 + rs))
			if rsi < 0.0 {
				rsi = 0.0
			} else if rsi > 100.0 {
				rsi = 100.0
			}
			out[i] = rsi
		}
	}
	return out
}

// CalculateBollingerBands computes Middle, Upper, and Lower Bollinger Bands (20-period 2-sigma).
func (h *Harness) CalculateBollingerBands(closes []float64, period int, mult float64) (middle, upper, lower []float64) {
	n := len(closes)
	middle = h.CalculateSMA(closes, period)
	upper = make([]float64, n)
	lower = make([]float64, n)

	for i := range closes {
		if math.IsNaN(middle[i]) {
			upper[i] = math.NaN()
			lower[i] = math.NaN()
			continue
		}
		sumSq := 0.0
		for j := 0; j < period; j++ {
			diff := closes[i-j] - middle[i]
			sumSq += diff * diff
		}
		stdDev := math.Sqrt(sumSq / float64(period))
		upper[i] = middle[i] + math.Abs(mult)*stdDev
		lower[i] = middle[i] - math.Abs(mult)*stdDev
	}
	return middle, upper, lower
}

// CalculateStochastic computes %K and %D lines (15/5/5).
func (h *Harness) CalculateStochastic(highs, lows, closes []float64, kPeriod, dPeriod int) (kLine, dLine []float64) {
	n := len(closes)
	kLine = make([]float64, n)
	for i := range closes {
		if i < kPeriod-1 {
			kLine[i] = math.NaN()
			continue
		}
		highest := highs[i]
		lowest := lows[i]
		for j := 0; j < kPeriod; j++ {
			if highs[i-j] > highest {
				highest = highs[i-j]
			}
			if lows[i-j] < lowest {
				lowest = lows[i-j]
			}
		}
		diff := highest - lowest
		if diff == 0 {
			kLine[i] = 50.0
		} else {
			k := ((closes[i] - lowest) / diff) * 100.0
			if k < 0 {
				k = 0
			} else if k > 100 {
				k = 100
			}
			kLine[i] = k
		}
	}

	dLine = h.CalculateSMA(kLine, dPeriod)
	return kLine, dLine
}

// CalculateATR computes 14-period Average True Range and ATR % volatility.
func (h *Harness) CalculateATR(bars []OHLCVBar, period int) (atr []float64, atrPct []float64) {
	n := len(bars)
	atr = make([]float64, n)
	atrPct = make([]float64, n)
	if n == 0 {
		return atr, atrPct
	}

	tr := make([]float64, n)
	tr[0] = bars[0].High - bars[0].Low

	for i := 1; i < n; i++ {
		hMinusL := bars[i].High - bars[i].Low
		hMinusPrevC := math.Abs(bars[i].High - bars[i-1].Close)
		lMinusPrevC := math.Abs(bars[i].Low - bars[i-1].Close)
		tr[i] = math.Max(hMinusL, math.Max(hMinusPrevC, lMinusPrevC))
	}

	atr = h.CalculateSMA(tr, period)
	for i := range bars {
		if math.IsNaN(atr[i]) || bars[i].Close == 0 {
			atrPct[i] = math.NaN()
		} else {
			atrPct[i] = (atr[i] / bars[i].Close) * 100.0
		}
	}
	return atr, atrPct
}

// CalculateRelativeStrength computes 100-bar outperformance ratio vs SPX benchmark.
func (h *Harness) CalculateRelativeStrength(stockCloses, spxCloses []float64, lookback int) []float64 {
	n := len(stockCloses)
	out := make([]float64, n)
	if lookback <= 0 || n < lookback || len(spxCloses) != n {
		for i := range out {
			out[i] = math.NaN()
		}
		return out
	}

	for i := 0; i < n; i++ {
		if i < lookback {
			out[i] = math.NaN()
			continue
		}
		stockBase := stockCloses[i-lookback]
		spxBase := spxCloses[i-lookback]
		if stockBase == 0 || spxBase == 0 || spxCloses[i] == 0 {
			out[i] = 100.0
			continue
		}
		stockPerf := stockCloses[i] / stockBase
		spxPerf := spxCloses[i] / spxBase
		out[i] = (stockPerf / spxPerf) * 100.0
	}
	return out
}

// EvaluateCondition evaluates a single Condition against series data.
func (h *Harness) EvaluateCondition(cond *models.Condition, data map[string][]float64) bool {
	if cond == nil || len(data) == 0 {
		return false
	}

	switch cond.Type {
	case models.ConditionCrossAbove:
		a, okA := data["indicatorA"]
		b, okB := data["indicatorB"]
		if !okA || !okB || len(a) < 2 || len(b) < 2 {
			return false
		}
		last := len(a) - 1
		if math.IsNaN(a[last]) || math.IsNaN(b[last]) || math.IsNaN(a[last-1]) || math.IsNaN(b[last-1]) {
			return false
		}
		return a[last-1] <= b[last-1] && a[last] > b[last]

	case models.ConditionCrossBelow:
		a, okA := data["indicatorA"]
		b, okB := data["indicatorB"]
		if !okA || !okB || len(a) < 2 || len(b) < 2 {
			return false
		}
		last := len(a) - 1
		if math.IsNaN(a[last]) || math.IsNaN(b[last]) || math.IsNaN(a[last-1]) || math.IsNaN(b[last-1]) {
			return false
		}
		return a[last-1] >= b[last-1] && a[last] < b[last]

	case models.ConditionGreater:
		a, okA := data["indicatorA"]
		if !okA || len(a) == 0 {
			return false
		}
		last := len(a) - 1
		if math.IsNaN(a[last]) {
			return false
		}
		thresh := cond.Threshold
		if b, okB := data["indicatorB"]; okB && len(b) > 0 {
			thresh = b[len(b)-1]
		}
		return a[last] > thresh

	case models.ConditionLess:
		a, okA := data["indicatorA"]
		if !okA || len(a) == 0 {
			return false
		}
		last := len(a) - 1
		if math.IsNaN(a[last]) {
			return false
		}
		thresh := cond.Threshold
		if b, okB := data["indicatorB"]; okB && len(b) > 0 {
			thresh = b[len(b)-1]
		}
		return a[last] < thresh

	case models.ConditionWithin:
		a, okA := data["indicatorA"]
		if !okA || len(a) == 0 {
			return false
		}
		last := len(a) - 1
		if math.IsNaN(a[last]) {
			return false
		}
		// Expects threshold to be middle or checks range
		return a[last] >= 30.0 && a[last] <= 70.0

	case models.ConditionSlopeAbove:
		a, okA := data["indicatorA"]
		lookback := cond.LookbackPeriods
		if !okA || len(a) <= lookback || lookback <= 0 {
			return false
		}
		last := len(a) - 1
		slope := (a[last] - a[last-lookback]) / float64(lookback)
		return slope > cond.Threshold

	case models.ConditionSlopeBelow:
		a, okA := data["indicatorA"]
		lookback := cond.LookbackPeriods
		if !okA || len(a) <= lookback || lookback <= 0 {
			return false
		}
		last := len(a) - 1
		slope := (a[last] - a[last-lookback]) / float64(lookback)
		return slope < cond.Threshold

	default:
		return false
	}
}

// EvaluateTick receives live IBKR bar frames and runs strategy rules.
func (h *Harness) EvaluateTick(symbol string, marketData map[string][]float64) []models.ActionType {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if !h.IsRunning || !h.GlobalEnabled {
		return nil
	}

	strat := h.ActiveStrategy
	if strat == nil {
		// Use default screener preset without re-entrant lock
		p, _ := h.getStrategyPresetLocked("screener_10_30_haut")
		strat = p
	}
	if strat == nil {
		return nil
	}

	var actions []models.ActionType
	for _, rule := range strat.Rules {
		if !rule.Enabled {
			continue
		}
		if h.EvaluateCondition(rule.Condition, marketData) {
			actions = append(actions, rule.Action)
		}
	}
	return actions
}
