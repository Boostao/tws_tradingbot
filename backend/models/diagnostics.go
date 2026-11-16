package models

type DiagnosticsStartup struct {
	Environment     string `json:"environment"`
	TradingMode     string `json:"trading_mode"`
	IBHost          string `json:"ib_host"`
	IBPort          int    `json:"ib_port"`
	ClientID        int    `json:"client_id"`
	Account         string `json:"account"`
	LogLevel        string `json:"log_level"`
	LogFile         string `json:"log_file"`
	WatchlistPath   string `json:"watchlist_path"`
	StrategyPath    string `json:"strategy_path"`
	SymbolCachePath string `json:"symbol_cache_path"`
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
	Startup DiagnosticsStartup `json:"startup"`
	Runtime DiagnosticsRuntime `json:"runtime"`
	Symbols DiagnosticsSymbols `json:"symbols"`
}
