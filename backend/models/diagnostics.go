package models

type DiagnosticsStartup struct {
	Environment     string `json:"environment,omitempty"`
	TradingMode     string `json:"trading_mode"`
	IBHost          string `json:"ib_host"`
	IBPort          int    `json:"ib_port"`
	ClientID        int    `json:"client_id"`
	Account         string `json:"account"`
	LogLevel        string `json:"log_level,omitempty"`
	LogFile         string `json:"log_file,omitempty"`
	WatchlistPath   string `json:"watchlist_path,omitempty"`
	StrategyPath    string `json:"strategy_path,omitempty"`
	SymbolCachePath string `json:"symbol_cache_path,omitempty"`
}

type DiagnosticsRuntime struct {
	RunnerActive            bool   `json:"runner_active"`
	LastRuntimeReloadAt     string `json:"last_runtime_reload_at,omitempty"`
	LastRuntimeReloadReason string `json:"last_runtime_reload_reason,omitempty"`
	LastDisconnectAt        string `json:"last_disconnect_at,omitempty"`
	LastDisconnectReason    string `json:"last_disconnect_reason,omitempty"`
}

type DiagnosticsSymbols struct {
	Source        string `json:"source,omitempty"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	LastWarning   string `json:"last_warning,omitempty"`
}

type DiagnosticsResponse struct {
	Startup            DiagnosticsStartup `json:"startup"`
	Runtime            DiagnosticsRuntime `json:"runtime"`
	Symbols            DiagnosticsSymbols `json:"symbols"`
	SessionStatus      string             `json:"session_status"`
	Account            string             `json:"account"`
	DryRun             bool               `json:"dry_run"`
	ActiveWorkspaceID  string             `json:"active_workspace_id,omitempty"`
	ActiveStrategyID   string             `json:"active_strategy_id,omitempty"`
	ActiveStrategyName string             `json:"active_strategy_name,omitempty"`
	BrokerAlerts       []BrokerAlert      `json:"broker_alerts,omitempty"`
}
