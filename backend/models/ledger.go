package models

import "time"

// TradeRecord represents an atomic execution event recorded in the SQLite trade_ledger.
type TradeRecord struct {
	ID        int64     `json:"id"`
	OrderID   int64     `json:"order_id"`
	Symbol    string    `json:"symbol"`
	Action    string    `json:"action"` // "BUY", "SELL"
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Status    string    `json:"status"` // "SIMULATED_FILL", "FILLED", "REJECTED"
	OCAGroup  string    `json:"oca_group,omitempty"`
	Mode      string    `json:"mode"`   // "dry_run", "live"
	CreatedAt time.Time `json:"created_at"`
}
