package models

// AutopilotState represents the current state of the autopilot system.
type AutopilotState struct {
	ExecMode     int           `json:"exec_mode"`
	ActiveOrders []BrokerOrder `json:"active_orders"`
	Session      *AutopilotRun `json:"session"`
}

// AutopilotRun tracks the metadata of an active autopilot session.
type AutopilotRun struct {
	RunID       string `json:"run_id"`
	StrategyID  string `json:"strategy_id"`
	StartedAt   int64  `json:"started_at"`
	IsActive    bool   `json:"is_active"`
}

// AutopilotEvent records lifecycle events for an autopilot session.
type AutopilotEvent struct {
	EventID     int64  `json:"event_id"`
	RunID       string `json:"run_id"`
	DecisionID  string `json:"decision_id"`
	EventType   string `json:"event_type"`
	OrderID     int64  `json:"order_id"`
	SignalPrice float64 `json:"signal_price"`
	FillPrice   float64 `json:"fill_price"`
	PayloadJSON string `json:"payload_json"`
}
