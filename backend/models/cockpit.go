package models

type CockpitStrategySummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	RuleCount        int    `json:"rule_count"`
	EnabledRuleCount int    `json:"enabled_rule_count"`
	Source           string `json:"source,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
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
