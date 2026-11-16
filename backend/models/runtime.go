package models

type BotState struct {
	Status                  string                   `json:"status"`
	Running                 bool                     `json:"running,omitempty"`
	TwsConnected            bool                     `json:"tws_connected"`
	Equity                  float64                  `json:"equity"`
	DailyPnL                float64                  `json:"daily_pnl"`
	DailyPnLPercent         float64                  `json:"daily_pnl_percent"`
	TotalPnL                float64                  `json:"total_pnl"`
	DailyLossLimit          float64                  `json:"daily_loss_limit"`
	CircuitBreakerTripped   bool                     `json:"circuit_breaker_tripped"`
	ActiveStrategy          string                   `json:"active_strategy"`
	OpenPositionsCount      int                      `json:"open_positions_count"`
	PendingOrdersCount      int                      `json:"pending_orders_count"`
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
