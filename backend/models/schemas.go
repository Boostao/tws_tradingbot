package models

// This package mirrors the FastAPI Pydantic schemas from tws_traderbot/src/api/schemas.py

type WatchlistItem struct {
	Symbol       string `json:"symbol"`
	Exchange     string `json:"exchange"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	InstrumentID string `json:"instrument_id,omitempty"`
}

type WatchlistGroup struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Source string          `json:"source"`
	Items  []WatchlistItem `json:"items"`
}

type WatchlistFeed struct {
	Provider        string `json:"provider"`
	URL             string `json:"url"`
	Title           string `json:"title,omitempty"`
	ExternalID      string `json:"external_id,omitempty"`
	LastRefreshedAt string `json:"last_refreshed_at,omitempty"`
}

type WatchlistResponse struct {
	Symbols   []string         `json:"symbols"`
	Groups    []WatchlistGroup `json:"groups"`
	Feed      *WatchlistFeed   `json:"feed,omitempty"`
	UpdatedAt string           `json:"updated_at,omitempty"`
}

type CockpitStrategySummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	RuleCount        int    `json:"rule_count"`
	EnabledRuleCount int    `json:"enabled_rule_count"`
	Source           string `json:"source"`
}

type CockpitStrategySlot struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	StrategyID string `json:"strategy_id,omitempty"`
	Enabled    bool   `json:"enabled"`
}

type CockpitWorkspace struct {
	ID            string                `json:"id"`
	Name          string                `json:"name"`
	Kind          string                `json:"kind"`
	Enabled       bool                  `json:"enabled"`
	StrategySlots []CockpitStrategySlot `json:"strategy_slots"`
}

type CockpitStateResponse struct {
	GlobalEnabled     bool                     `json:"global_enabled"`
	ActiveWorkspaceID string                   `json:"active_workspace_id,omitempty"`
	Workspaces        []CockpitWorkspace       `json:"workspaces"`
	StrategyLibrary   []CockpitStrategySummary `json:"strategy_library"`
	Feed              *WatchlistFeed           `json:"feed,omitempty"`
	UpdatedAt         string                   `json:"updated_at,omitempty"`
}
