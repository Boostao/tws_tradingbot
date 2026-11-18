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

	// Single-writer/single-connection pool avoids SQLITE_BUSY on DEFERRED->RESERVED WAL lock upgrades.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	// Enable Foreign Key Enforcement and Write-Ahead Logging
	if _, err := conn.Exec("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;"); err != nil {
		log.Printf("Warning: failed to set SQLite PRAGMAs: %v\n", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, err
	}

	db := &Database{conn: conn}
	db.InitSchema()
	_ = InitAutopilotSchema(conn)

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
		pinned BOOLEAN NOT NULL DEFAULT 0,
		sort_order INTEGER NOT NULL DEFAULT 0,
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

	db.migrateStrategiesTable()

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

	db.migrateLegacyCobaltV2Data()

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

// migrateStrategiesTable idempotently adds pinned and sort_order columns and backfills initial sort_order.
func (db *Database) migrateStrategiesTable() {
	rows, err := db.conn.Query("PRAGMA table_info(strategies)")
	if err != nil {
		log.Printf("Warning: failed to inspect strategies table: %v\n", err)
		return
	}

	var hasPinned, hasSortOrder bool
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err == nil {
			switch name {
			case "pinned":
				hasPinned = true
			case "sort_order":
				hasSortOrder = true
			}
		}
	}
	rows.Close()

	if !hasPinned {
		if _, err := db.conn.Exec("ALTER TABLE strategies ADD COLUMN pinned BOOLEAN NOT NULL DEFAULT 0"); err != nil {
			log.Printf("Warning: failed adding strategies.pinned: %v\n", err)
		}
	}
	if !hasSortOrder {
		if _, err := db.conn.Exec("ALTER TABLE strategies ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0"); err != nil {
			log.Printf("Warning: failed adding strategies.sort_order: %v\n", err)
		}
	}

	var total, nonZero int
	if err := db.conn.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN sort_order != 0 THEN 1 ELSE 0 END), 0) FROM strategies").Scan(&total, &nonZero); err != nil {
		return
	}
	if total > 0 && (nonZero == 0 || !hasSortOrder) {
		tx, err := db.conn.Begin()
		if err != nil {
			return
		}
		defer tx.Rollback()

		idRows, err := tx.Query("SELECT id FROM strategies ORDER BY pinned DESC, name ASC, id ASC")
		if err != nil {
			return
		}
		var ids []string
		for idRows.Next() {
			var id string
			if err := idRows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		idRows.Close()

		for i, id := range ids {
			if _, err := tx.Exec("UPDATE strategies SET sort_order = ? WHERE id = ?", i+1, id); err != nil {
				return
			}
		}
		_ = tx.Commit()
	}
}

// migrateLegacyCobaltV2Data performs a one-time cleanup of legacy seeded holdings and clobbered watchlist names.
func (db *Database) migrateLegacyCobaltV2Data() {
	var migrated string
	err := db.conn.QueryRow("SELECT value FROM system_config WHERE key = 'migration_cobalt_v2_clean_holdings'").Scan(&migrated)
	if err == nil && migrated != "" {
		return
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		DELETE FROM watchlist_items
		WHERE watchlist_id = 'positions-detenues'
		  AND instrument_id IN ('AAPL.NASDAQ', 'SHOP.TSX')
	`); err != nil {
		return
	}

	if _, err := tx.Exec(`
		UPDATE watchlists
		SET name = '✝️@ Cobalt- TELEGRAM', updated_at = CURRENT_TIMESTAMP
		WHERE id = 'cobalt-telegram' AND name = 'cobalt-telegram'
	`); err != nil {
		return
	}

	if _, err := tx.Exec(`
		INSERT INTO system_config (key, value)
		VALUES ('migration_cobalt_v2_clean_holdings', '"true"')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`); err != nil {
		return
	}

	_ = tx.Commit()
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
