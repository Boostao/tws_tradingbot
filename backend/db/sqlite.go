package db

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"

	"tws_traderbot/backend/models"

	_ "modernc.org/sqlite" // Pure Go SQLite driver (No CGO needed)
)

type Database struct {
	conn *sql.DB
}

func Connect(dbPath string) (*Database, error) {
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	} else {
		dsn += "&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	}
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Enable Foreign Key Enforcement and Write-Ahead Logging
	if _, err := conn.Exec("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;"); err != nil {
		log.Printf("Warning: failed to set SQLite PRAGMAs: %v\n", err)
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
	query := `
	-- 1. Watchlists table
	CREATE TABLE IF NOT EXISTS watchlists (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		is_default BOOLEAN DEFAULT FALSE,
		source TEXT DEFAULT 'manual',
		feed_url TEXT DEFAULT '',
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 2. Watchlist Items table (child relation with CASCADE)
	CREATE TABLE IF NOT EXISTS watchlist_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		watchlist_id TEXT NOT NULL,
		group_id TEXT DEFAULT 'default',
		group_name TEXT DEFAULT 'Default',
		symbol TEXT NOT NULL,
		exchange TEXT NOT NULL,
		venue TEXT DEFAULT 'SMART',
		instrument_id TEXT NOT NULL,
		name TEXT DEFAULT '',
		enabled BOOLEAN DEFAULT TRUE,
		tier TEXT DEFAULT '2-Croissance',
		is_orderable BOOLEAN DEFAULT TRUE,
		is_regime_reference BOOLEAN DEFAULT FALSE,
		is_holding BOOLEAN DEFAULT FALSE,
		size_mode TEXT DEFAULT 'shares',
		size_value REAL DEFAULT 100.0,
		size_val REAL DEFAULT 100.0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (watchlist_id) REFERENCES watchlists(id) ON DELETE CASCADE,
		UNIQUE(watchlist_id, instrument_id)
	);

	-- 3. Indexes for fast lookup, filtering, and execution scans
	CREATE INDEX IF NOT EXISTS idx_watchlist_items_wid ON watchlist_items(watchlist_id);
	CREATE INDEX IF NOT EXISTS idx_watchlist_items_inst ON watchlist_items(instrument_id);
	CREATE INDEX IF NOT EXISTS idx_watchlist_items_enabled ON watchlist_items(watchlist_id, enabled);
	CREATE INDEX IF NOT EXISTS idx_watchlist_items_orderable ON watchlist_items(is_orderable);
	CREATE INDEX IF NOT EXISTS idx_watchlist_items_tier ON watchlist_items(tier);

	-- 4. Global system configuration table
	CREATE TABLE IF NOT EXISTS system_config (
		key TEXT PRIMARY KEY,
		value JSON
	);

	-- 5. Strategy Presets Library
	CREATE TABLE IF NOT EXISTS strategies (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		summary JSON,
		data JSON,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 6. Execution Audit Ledger (Dry-run & Live fills)
	CREATE TABLE IF NOT EXISTS trade_ledger (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		order_id INTEGER NOT NULL,
		symbol TEXT NOT NULL,
		action TEXT NOT NULL,
		quantity INTEGER NOT NULL,
		price REAL NOT NULL,
		status TEXT NOT NULL,
		oca_group TEXT DEFAULT '',
		mode TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_trade_ledger_symbol ON trade_ledger(symbol);
	CREATE INDEX IF NOT EXISTS idx_trade_ledger_mode ON trade_ledger(mode);
	`

	_, err := db.conn.Exec(query)
	if err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// Check if watchlists table is empty
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM watchlists").Scan(&count)
	if err == nil && count == 0 {
		// Check for legacy JSON in system_config
		var legacyVal string
		legErr := db.conn.QueryRow("SELECT value FROM system_config WHERE key = 'watchlist_state'").Scan(&legacyVal)
		if legErr == nil && legacyVal != "" {
			var legacy models.WatchlistResponse
			if err := json.Unmarshal([]byte(legacyVal), &legacy); err == nil && len(legacy.Groups) > 0 {
				log.Println("Migrating legacy watchlist_state into relational tables...")
				_ = db.MigrateLegacyWatchlist(&legacy)
			}
		}

		// Seed the 5 official Cobalt lists
		log.Println("Seeding official Cobalt watchlists into SQLite...")
		if err := db.SeedCobaltWatchlists(); err != nil {
			log.Printf("Error seeding Cobalt watchlists: %v\n", err)
		}
	}

	// Check if strategies table needs seeding
	var stratCount int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM strategies").Scan(&stratCount)
	if err == nil && stratCount < 20 && StrategyCatalogSeeder != nil {
		log.Println("Seeding official Cobalt strategies into SQLite...")
		if err := StrategyCatalogSeeder(db); err != nil {
			log.Printf("Error seeding Cobalt strategies: %v\n", err)
		}
	}
}

func (db *Database) MigrateLegacyWatchlist(legacy *models.WatchlistResponse) error {
	if legacy == nil || len(legacy.Groups) == 0 {
		return nil
	}
	id := legacy.ID
	if id == "" {
		id = "cobalt-telegram"
	}
	name := legacy.Name
	if name == "" {
		name = "✝️@ Cobalt- TELEGRAM"
	}
	_, err := db.MergeAndSaveWatchlist(id, name, legacy.Description, legacy.Feed, legacy.Groups, false)
	return err
}

func (db *Database) Conn() *sql.DB {
	return db.conn
}

func (db *Database) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

