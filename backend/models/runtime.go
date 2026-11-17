package models

type BrokerPosition struct {
	Symbol        string  `json:"symbol"`
	Venue         string  `json:"venue"`
	InstrumentID  string  `json:"instrument_id"`
	Currency      string  `json:"currency"`
	Position      float64 `json:"position"`
	AvgCost       float64 `json:"avg_cost"`
	MarketPrice   float64 `json:"market_price"`
	MarketValue   float64 `json:"market_value"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	RealizedPnL   float64 `json:"realized_pnl"`
}

type BrokerOrder struct {
	OrderID      int64   `json:"order_id"`
	Symbol       string  `json:"symbol"`
	Venue        string  `json:"venue"`
	InstrumentID string  `json:"instrument_id"`
	Action       string  `json:"action"`
	OrderType    string  `json:"order_type"`
	TotalQty     float64 `json:"total_qty"`
	FilledQty    float64 `json:"filled_qty"`
	RemainingQty float64 `json:"remaining_qty"`
	LimitPrice   float64 `json:"limit_price"`
	AvgFillPrice float64 `json:"avg_fill_price"`
	Status       string  `json:"status"`
	OutsideRTH   bool    `json:"outside_rth"`
}

type BrokerAlert struct {
	Code      int64  `json:"code"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Hint      string `json:"hint"`
	Timestamp string `json:"timestamp"`
}

type BotState struct {
	Status                  string                   `json:"status"`
	Running                 bool                     `json:"running,omitempty"`
	TwsConnected            bool                     `json:"tws_connected"`
	AccountID               string                   `json:"account_id,omitempty"`
	DryRun                  bool                     `json:"dry_run"`
	Equity                  float64                  `json:"equity"`
	BuyingPower             float64                  `json:"buying_power"`
	CashUSD                 float64                  `json:"cash_usd"`
	CashCAD                 float64                  `json:"cash_cad"`
	CashBase                float64                  `json:"cash_base"`
	DailyPnL                float64                  `json:"daily_pnl"`
	DailyPnLPercent         float64                  `json:"daily_pnl_percent"`
	TotalPnL                float64                  `json:"total_pnl"`
	RealizedPnL             float64                  `json:"realized_pnl"`
	UnrealizedPnL           float64                  `json:"unrealized_pnl"`
	DailyLossLimit          float64                  `json:"daily_loss_limit"`
	CircuitBreakerTripped   bool                     `json:"circuit_breaker_tripped"`
	ActiveWorkspaceID       string                   `json:"active_workspace_id,omitempty"`
	ActiveStrategyID        string                   `json:"active_strategy_id,omitempty"`
	ActiveStrategy          string                   `json:"active_strategy"`
	SessionStatus           string                   `json:"session_status,omitempty"`
	OpenPositionsCount      int                      `json:"open_positions_count"`
	PendingOrdersCount      int                      `json:"pending_orders_count"`
	OpenPositions           []BrokerPosition         `json:"open_positions,omitempty"`
	OpenOrders              []BrokerOrder            `json:"open_orders,omitempty"`
	BrokerAlerts            []BrokerAlert            `json:"broker_alerts,omitempty"`
	TradesToday             int                      `json:"trades_today"`
	WinRateToday            float64                  `json:"win_rate_today"`
	LastUpdate              string                   `json:"last_update,omitempty"`
	LastHeartbeat           string                   `json:"last_heartbeat,omitempty"`
	RecentLogs              []string                 `json:"recent_logs,omitempty"`
	RecentOrders            []map[string]interface{} `json:"recent_orders,omitempty"`
	RecentTrades            []map[string]interface{} `json:"recent_trades,omitempty"`
	LastDryRun              map[string]interface{}   `json:"last_dry_run,omitempty"`
	LastRuntimeReloadAt     string                   `json:"last_runtime_reload_at,omitempty"`
	LastRuntimeReloadReason string                   `json:"last_runtime_reload_reason,omitempty"`
	LastDisconnectAt        string                   `json:"last_disconnect_at,omitempty"`
	LastDisconnectReason    string                   `json:"last_disconnect_reason,omitempty"`
	ErrorMessage            string                   `json:"error_message,omitempty"`
	RunnerActive            bool                     `json:"runner_active,omitempty"`
}

type BotDryRunSubscription struct {
	Symbol    string `json:"symbol"`
	Timeframe string `json:"timeframe"`
}

type BotDryRun struct {
	Strategy      string                   `json:"strategy"`
	WorkspaceKind string                   `json:"workspace_kind"`
	Subscriptions []BotDryRunSubscription  `json:"subscriptions"`
	Positions     map[string]interface{}   `json:"positions"`
	OpenOrders    []map[string]interface{} `json:"open_orders"`
	PlannedOrders []map[string]interface{} `json:"planned_orders"`
	State         *BotState                `json:"state"`
	GeneratedAt   string                   `json:"generated_at"`
}
