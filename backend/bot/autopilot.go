package bot

import (
	"database/sql"
	"fmt"
	"sync"
	"strings"
	"time"
	"github.com/scmhub/ibapi"
)

// AutopilotExecMode defines the operational state of the autopilot system.
type AutopilotExecMode int

const (
	ModeOff AutopilotExecMode = iota
	ModeShadow
	ModePaperLive
)

// AutopilotController manages the lifecycle of an autopilot session.
type AutopilotController struct {
	mu           sync.RWMutex
	mode         AutopilotExecMode
	client       *ibapi.EClient
	runID        string
	connEpoch    int64
	isActive     bool
	engine       *Engine
	quit         chan struct{}
	ordersPlaced int

	// Session bindings
	workspace    string
	slot         string
	strategyID   string
	watchlistID  string

	// Order tracking
	activeOrders map[int64]bool
	
	// Deduplication
	inMemoryDecisions sync.Map
}

func (c *AutopilotController) LoadDecisions(db *sql.DB) error {
	rows, err := db.Query("SELECT strategy_id, rule_id, instrument_id, timeframe, bar_ts, action FROM autopilot_decisions")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var sID, rID, iID, tf, action string
		var barTS int64
		if err := rows.Scan(&sID, &rID, &iID, &tf, &barTS, &action); err != nil {
			return err
		}
		key := fmt.Sprintf("%s:%s:%s:%s:%d:%s", sID, rID, iID, tf, barTS, action)
		c.inMemoryDecisions.Store(key, struct{}{})
	}
	return nil
}

func (c *AutopilotController) IsDuplicate(key string) bool {
	_, loaded := c.inMemoryDecisions.Load(key)
	return loaded
}

func (c *AutopilotController) RecordDecision(key string) {
	c.inMemoryDecisions.Store(key, struct{}{})
}

func (c *AutopilotController) SetClient(client *ibapi.EClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client = client
}

// Configure sets the session bindings.
func (c *AutopilotController) Configure(workspace, slot, strategyID, watchlistID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workspace = workspace
	c.slot = slot
	c.strategyID = strategyID
	c.watchlistID = watchlistID
}

func NewAutopilotController(e *Engine) *AutopilotController {
	return &AutopilotController{
		mode:         ModeOff,
		engine:       e,
		quit:         make(chan struct{}),
		activeOrders: make(map[int64]bool),
	}
}

// Start initiates an autopilot session with a specific mode.
func (c *AutopilotController) Start(mode AutopilotExecMode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isActive {
		return
	}
	c.mode = mode
	c.isActive = true
	c.ordersPlaced = 0
	c.quit = make(chan struct{})
	go c.runFlywheel()
}

// Stop halts all active autopilot operations.
func (c *AutopilotController) Stop() {
	c.FailsafeStop()
}

// State returns a snapshot of the autopilot controller state.
func (c *AutopilotController) State() (mode AutopilotExecMode, activeOrders []int64, runID, strategyID string, isActive bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	orders := make([]int64, 0, len(c.activeOrders))
	for id := range c.activeOrders {
		orders = append(orders, id)
	}
	
	return c.mode, orders, c.runID, c.strategyID, c.isActive
}

// FailsafeStop issues a secure cancellation of all active orders.
func (c *AutopilotController) FailsafeStop() {
	c.mu.Lock()
	// Cancel all active orders
	if c.client != nil {
		for orderID := range c.activeOrders {
			c.client.CancelOrder(orderID, ibapi.OrderCancel{})
		}
	}
	c.activeOrders = make(map[int64]bool)

	if !c.isActive {
		c.mu.Unlock()
		return
	}
	close(c.quit)
	c.isActive = false
	c.mode = ModeOff
	c.mu.Unlock()
}

// runFlywheel is the continuous feedback loop for autopilot evaluation.
func (c *AutopilotController) runFlywheel() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.quit:
			return
		case <-ticker.C:
			c.evaluateFeedback()
		}
	}
}

func (c *AutopilotController) evaluateFeedback() {
	c.mu.RLock()
	mode := c.mode
	c.mu.RUnlock()

	if mode == ModeOff {
		return
	}

	// Feedback flywheel: Check for risk breaker trip in live mode
	if c.engine.RiskBreakerTripped.Load() {
		fmt.Println("Autopilot flywheel: Risk breaker tripped, halting autopilot.")
		c.Stop()
	}
}

func (c *AutopilotController) ValidatePaperSafety() bool {
	if c.engine == nil {
		return false
	}
	// Check if risk breaker is tripped
	if c.engine.RiskBreakerTripped.Load() {
		return false
	}
	// Check if account ID is set and starts with 'DU' (common for paper accounts in IBKR)
	// Or other validation
	accountID := c.engine.GetAccountID()
	if accountID == "" || !strings.HasPrefix(accountID, "DU") {
		return false
	}
	return true
}

