package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"tws_traderbot/backend/models"
)

type ItemOverrideState struct {
	Enabled   bool
	Tier      string
	IsHolding bool
	SizeMode  string
	SizeValue float64
}

// GetWatchlists returns summaries of all registered watchlists for the Cockpit selector.
func (db *Database) GetWatchlists() ([]models.WatchlistSummary, error) {
	rows, err := db.conn.Query(`
		SELECT w.id, w.name, w.description, w.is_default, w.source, w.feed_url,
		       COUNT(i.id) AS item_count, w.updated_at
		FROM watchlists w
		LEFT JOIN watchlist_items i ON w.id = i.watchlist_id
		GROUP BY w.id
		ORDER BY w.is_default DESC, w.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []models.WatchlistSummary
	for rows.Next() {
		var s models.WatchlistSummary
		var desc, feedURL sql.NullString
		var updatedAt sql.NullString
		if err := rows.Scan(&s.ID, &s.Name, &desc, &s.IsDefault, &s.Source, &feedURL, &s.ItemCount, &updatedAt); err != nil {
			return nil, err
		}
		s.Description = desc.String
		s.FeedURL = feedURL.String
		s.UpdatedAt = updatedAt.String
		summaries = append(summaries, s)
	}
	return summaries, nil
}

// GetWatchlistByID reconstructs the full WatchlistResponse relationally.
func (db *Database) GetWatchlistByID(id string) (*models.WatchlistResponse, error) {
	var wl models.WatchlistSummary
	var desc, feedURL sql.NullString
	var updatedAt sql.NullString
	err := db.conn.QueryRow(`
		SELECT id, name, description, is_default, source, feed_url, updated_at
		FROM watchlists WHERE id = ?
	`, id).Scan(&wl.ID, &wl.Name, &desc, &wl.IsDefault, &wl.Source, &feedURL, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("watchlist %s not found: %w", id, err)
	}
	wl.Description = desc.String
	wl.FeedURL = feedURL.String
	wl.UpdatedAt = updatedAt.String

	rows, err := db.conn.Query(`
		SELECT id, watchlist_id, group_id, group_name, symbol, exchange, venue, instrument_id,
		       name, enabled, tier, is_orderable, is_regime_reference, is_holding,
		       size_mode, size_value, created_at, updated_at
		FROM watchlist_items
		WHERE watchlist_id = ?
		ORDER BY group_id ASC, id ASC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groupMap := make(map[string]*models.WatchlistGroup)
	var groupOrder []string
	var allSymbols []string

	for rows.Next() {
		var item models.WatchlistItem
		var grpID, grpName, createdAt, itemUpdatedAt string
		if err := rows.Scan(
			&item.ID, &item.WatchlistID, &grpID, &grpName, &item.Symbol, &item.Exchange,
			&item.Venue, &item.InstrumentID, &item.Name, &item.Enabled, &item.Tier,
			&item.IsOrderable, &item.IsRegimeReference, &item.IsHolding,
			&item.SizeMode, &item.SizeValue, &createdAt, &itemUpdatedAt,
		); err != nil {
			return nil, err
		}
		item.GroupID = grpID
		item.GroupName = grpName
		item.CreatedAt = createdAt
		item.UpdatedAt = itemUpdatedAt

		grp, exists := groupMap[grpID]
		if !exists {
			grp = &models.WatchlistGroup{
				ID:     grpID,
				Name:   grpName,
				Source: wl.Source,
				Items:  []models.WatchlistItem{},
			}
			groupMap[grpID] = grp
			groupOrder = append(groupOrder, grpID)
		}
		grp.Items = append(grp.Items, item)
		allSymbols = append(allSymbols, item.InstrumentID)
	}

	groups := make([]models.WatchlistGroup, 0, len(groupOrder))
	for _, gid := range groupOrder {
		groups = append(groups, *groupMap[gid])
	}

	var feed *models.WatchlistFeed
	if wl.FeedURL != "" {
		externalID := ""
		if strings.Contains(wl.FeedURL, "/watchlists/") {
			parts := strings.Split(wl.FeedURL, "/watchlists/")
			if len(parts) > 1 {
				num := strings.Trim(parts[1], "/")
				if num != "" {
					externalID = num
				}
			}
		}
		feed = &models.WatchlistFeed{
			Provider:        wl.Source,
			URL:             wl.FeedURL,
			Title:           wl.Name,
			ExternalID:      externalID,
			LastRefreshedAt: wl.UpdatedAt,
		}
	}

	return &models.WatchlistResponse{
		ID:          wl.ID,
		Name:        wl.Name,
		Description: wl.Description,
		IsDefault:   wl.IsDefault,
		Symbols:     allSymbols,
		Groups:      groups,
		Feed:        feed,
		UpdatedAt:   wl.UpdatedAt,
	}, nil
}

// MergeAndSaveWatchlist performs smart diff/merge.
// When preserveOverrides is true (e.g. during external feed refresh), existing local SQLite customizations
// (enabled, tier, is_holding, size_mode, size_value) are preserved over feed defaults.
// When preserveOverrides is false (e.g. during operator UI save/replace), incoming values are persisted directly.
func (db *Database) MergeAndSaveWatchlist(
	watchlistID string,
	name string,
	description string,
	feed *models.WatchlistFeed,
	incomingGroups []models.WatchlistGroup,
	preserveOverrides bool,
) (*models.WatchlistResponse, error) {
	// 1. Fetch existing state overrides before beginning the write transaction if preserveOverrides is true
	existingOverrides := make(map[string]ItemOverrideState)
	if preserveOverrides {
		rows, err := db.conn.Query(`
			SELECT instrument_id, enabled, tier, is_holding, size_mode, size_value
			FROM watchlist_items WHERE watchlist_id = ?
		`, watchlistID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var instID, tier, sizeMode string
			var enabled, isHolding bool
			var sizeVal float64
			if err := rows.Scan(&instID, &enabled, &tier, &isHolding, &sizeMode, &sizeVal); err == nil {
				existingOverrides[instID] = ItemOverrideState{
					Enabled:   enabled,
					Tier:      tier,
					IsHolding: isHolding,
					SizeMode:  sizeMode,
					SizeValue: sizeVal,
				}
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	feedURL := ""
	source := "manual"
	if feed != nil {
		feedURL = feed.URL
		source = feed.Provider
	}

	// 2. Begin atomic transaction for write operations
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO watchlists (id, name, description, is_default, source, feed_url, updated_at)
		VALUES (?, ?, ?, FALSE, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			source = excluded.source,
			feed_url = excluded.feed_url,
			updated_at = CURRENT_TIMESTAMP
	`, watchlistID, name, description, source, feedURL)
	if err != nil {
		return nil, err
	}

	// 3. Delete existing items to replace with merged set
	if _, err := tx.Exec("DELETE FROM watchlist_items WHERE watchlist_id = ?", watchlistID); err != nil {
		return nil, err
	}

	insertItem, err := tx.Prepare(`
		INSERT INTO watchlist_items (
			watchlist_id, group_id, group_name, symbol, exchange, venue, instrument_id,
			name, enabled, tier, is_orderable, is_regime_reference, is_holding,
			size_mode, size_value, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return nil, err
	}
	defer insertItem.Close()

	seen := make(map[string]bool)

	for _, grp := range incomingGroups {
		grpID := grp.ID
		if grpID == "" {
			grpID = "default"
		}
		grpName := grp.Name
		if grpName == "" {
			grpName = "Default"
		}

		for _, item := range grp.Items {
			raw := item.Symbol
			if item.Exchange != "" && !strings.Contains(raw, ":") && !strings.Contains(raw, ".") {
				raw = item.Exchange + ":" + item.Symbol
			}
			norm := models.NormalizeSymbol(raw)

			if seen[norm.InstrumentID] {
				continue // Deduplicate within same list
			}
			seen[norm.InstrumentID] = true

			// Apply Smart Merge: preserve existing user overrides if preserveOverrides is true
			enabled := item.Enabled
			tier := item.Tier
			isHolding := item.IsHolding
			sizeMode := item.SizeMode
			sizeValue := item.SizeValue

			if preserveOverrides {
				if prev, ok := existingOverrides[norm.InstrumentID]; ok {
					enabled = prev.Enabled
					tier = prev.Tier
					isHolding = prev.IsHolding
					sizeMode = prev.SizeMode
					sizeValue = prev.SizeValue
				} else {
					if tier == "" {
						tier = "2-Croissance"
					}
					if sizeMode == "" {
						sizeMode = "shares"
						sizeValue = 100.0
					}
				}
			} else {
				if tier == "" {
					tier = "2-Croissance"
				}
				if sizeMode == "" {
					sizeMode = "shares"
					if sizeValue == 0 {
						sizeValue = 100.0
					}
				}
			}

			itemName := item.Name
			if itemName == "" {
				itemName = norm.Symbol
			}

			if _, err := insertItem.Exec(
				watchlistID, grpID, grpName, norm.Symbol, norm.Exchange, norm.Venue, norm.InstrumentID,
				itemName, enabled, tier, norm.IsOrderable, norm.IsRegimeReference, isHolding,
				sizeMode, sizeValue,
			); err != nil {
				return nil, fmt.Errorf("failed to insert merged item %s: %w", norm.InstrumentID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	res, err := db.GetWatchlistByID(watchlistID)
	if err != nil {
		return nil, err
	}
	if feed != nil && feed.ExternalID != "" && res.Feed != nil {
		res.Feed.ExternalID = feed.ExternalID
	}
	return res, nil
}

// DeleteWatchlist drops a watchlist and cascades to its items.
func (db *Database) DeleteWatchlist(id string) error {
	_, err := db.conn.Exec("DELETE FROM watchlists WHERE id = ?", id)
	return err
}

// LoadWatchlistState retrieves default watchlist or fallback to cobalt-telegram.
func (db *Database) LoadWatchlistState() (*models.WatchlistResponse, error) {
	var defaultID string
	err := db.conn.QueryRow("SELECT id FROM watchlists WHERE is_default = TRUE LIMIT 1").Scan(&defaultID)
	if err != nil || defaultID == "" {
		defaultID = "cobalt-telegram"
	}
	return db.GetWatchlistByID(defaultID)
}

// SaveWatchlistState saves state relationally via MergeAndSaveWatchlist with preserveOverrides=false.
func (db *Database) SaveWatchlistState(state *models.WatchlistResponse) error {
	if state == nil {
		return fmt.Errorf("watchlist state cannot be nil")
	}
	id := state.ID
	if id == "" {
		id = "cobalt-telegram"
	}
	name := state.Name
	if name == "" {
		name = "Master Watchlist"
	}
	_, err := db.MergeAndSaveWatchlist(id, name, state.Description, state.Feed, state.Groups, false)
	return err
}

// GetSymbols returns matching symbols from the watchlist universe.
func (db *Database) GetSymbols(query models.SymbolQuery) (*models.SymbolsResponse, error) {
	q := strings.TrimSpace(query.Q)
	sqlQuery := "SELECT DISTINCT symbol, name, exchange FROM watchlist_items"
	var args []interface{}

	if q != "" {
		sqlQuery += " WHERE symbol LIKE ? OR name LIKE ? OR instrument_id LIKE ?"
		param := "%" + q + "%"
		args = append(args, param, param, param)
	}

	limit := query.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	sqlQuery += fmt.Sprintf(" ORDER BY symbol ASC LIMIT %d", limit)

	rows, err := db.conn.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.SymbolRecord
	for rows.Next() {
		var r models.SymbolRecord
		var name, exchange sql.NullString
		if err := rows.Scan(&r.Symbol, &name, &exchange); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.Exchange = exchange.String
		r.Type = "STK"
		records = append(records, r)
	}

	return &models.SymbolsResponse{
		Symbols:   records,
		Source:    "local_sqlite",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}
