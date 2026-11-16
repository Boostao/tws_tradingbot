package db

import (
	"database/sql"
	"time"

	"tws_traderbot/backend/models"
)

// RecordFill records an execution fill event in the SQLite trade_ledger table.
func (db *Database) RecordFill(fill *models.TradeRecord) error {
	res, err := db.conn.Exec(`
		INSERT INTO trade_ledger (order_id, symbol, action, quantity, price, status, oca_group, mode)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, fill.OrderID, fill.Symbol, fill.Action, fill.Quantity, fill.Price, fill.Status, fill.OCAGroup, fill.Mode)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		fill.ID = id
	}
	return nil
}

// GetTradesByMode retrieves all trade ledger records matching the execution mode (dry_run, live).
func (db *Database) GetTradesByMode(mode string) ([]models.TradeRecord, error) {
	rows, err := db.conn.Query(`
		SELECT id, order_id, symbol, action, quantity, price, status, oca_group, mode, created_at
		FROM trade_ledger WHERE mode = ? ORDER BY id ASC
	`, mode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTradeRecords(rows)
}

// GetRecentTrades retrieves the most recent trade ledger records up to limit.
func (db *Database) GetRecentTrades(limit int) ([]models.TradeRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(`
		SELECT id, order_id, symbol, action, quantity, price, status, oca_group, mode, created_at
		FROM trade_ledger ORDER BY id DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTradeRecords(rows)
}

// GetTradeByID retrieves a single trade record by ID.
func (db *Database) GetTradeByID(id int64) (*models.TradeRecord, error) {
	row := db.conn.QueryRow(`
		SELECT id, order_id, symbol, action, quantity, price, status, oca_group, mode, created_at
		FROM trade_ledger WHERE id = ?
	`, id)

	var tr models.TradeRecord
	var oca sql.NullString
	var createdAt string
	if err := row.Scan(&tr.ID, &tr.OrderID, &tr.Symbol, &tr.Action, &tr.Quantity, &tr.Price, &tr.Status, &oca, &tr.Mode, &createdAt); err != nil {
		return nil, err
	}
	tr.OCAGroup = oca.String
	tr.CreatedAt = parseDBTime(createdAt)
	return &tr, nil
}

func scanTradeRecords(rows *sql.Rows) ([]models.TradeRecord, error) {
	var records []models.TradeRecord
	for rows.Next() {
		var tr models.TradeRecord
		var oca sql.NullString
		var createdAt string
		if err := rows.Scan(&tr.ID, &tr.OrderID, &tr.Symbol, &tr.Action, &tr.Quantity, &tr.Price, &tr.Status, &oca, &tr.Mode, &createdAt); err != nil {
			return nil, err
		}
		tr.OCAGroup = oca.String
		tr.CreatedAt = parseDBTime(createdAt)
		records = append(records, tr)
	}
	return records, rows.Err()
}

func parseDBTime(createdAt string) time.Time {
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05", createdAt); err == nil {
		return t
	}
	return time.Now().UTC()
}
