package db

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite" // Pure Go SQLite driver (No CGO needed)
)

type Database struct {
	conn *sql.DB
}

func Connect(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	db := &Database{conn: conn}
	db.InitSchema()
	
	log.Println("SQLite database connected successfully")
	return db, nil
}

func (db *Database) InitSchema() {
	// Replicating DuckDB / SQLite schema needs from tws_traderbot
	query := `
	CREATE TABLE IF NOT EXISTS watchlist (
		symbol TEXT PRIMARY KEY,
		exchange TEXT,
		name TEXT,
		enabled BOOLEAN,
		instrument_id TEXT
	);

	CREATE TABLE IF NOT EXISTS system_config (
		key TEXT PRIMARY KEY,
		value JSON
	);
	`

	_, err := db.conn.Exec(query)
	if err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}
}
