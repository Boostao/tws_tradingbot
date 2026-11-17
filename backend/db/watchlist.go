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
	allSymbols := make([]string, 0)

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
			name = CASE WHEN excluded.name != '' THEN excluded.name ELSE watchlists.name END,
			description = CASE WHEN excluded.description != '' THEN excluded.description ELSE watchlists.description END,
			source = CASE WHEN excluded.feed_url != '' THEN excluded.source ELSE watchlists.source END,
			feed_url = CASE WHEN excluded.feed_url != '' THEN excluded.feed_url ELSE watchlists.feed_url END,
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
			raw := strings.TrimSpace(item.Symbol)
			if item.Exchange != "" && !strings.Contains(raw, ":") && !models.HasExchangeSuffix(raw) {
				raw = strings.TrimSpace(item.Exchange) + ":" + raw
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
	desc := state.Description
	feed := state.Feed
	if name == "" || desc == "" || feed == nil {
		if existing, err := db.GetWatchlistByID(id); err == nil && existing != nil {
			if name == "" {
				name = existing.Name
			}
			if desc == "" {
				desc = existing.Description
			}
			if feed == nil {
				feed = existing.Feed
			}
		}
	}
	if name == "" {
		name = "Master Watchlist"
	}
	_, err := db.MergeAndSaveWatchlist(id, name, desc, feed, state.Groups, false)
	return err
}

type holdingOverride struct {
	Enabled   bool
	Tier      string
	SizeMode  string
	SizeValue float64
	SizeVal   float64
	Name      string
}

type resolvedHolding struct {
	Symbol       string
	Exchange     string
	Venue        string
	InstrumentID string
	Name         string
}

// SyncLivePositions atomically reconciles is_holding across all watchlists and replaces positions-detenues
// strictly with live TWS positions (pos != 0), preserving operator overrides.
func (db *Database) SyncLivePositions(positions []models.BrokerPosition) error {
	if db == nil || db.conn == nil {
		return nil
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Clear existing is_holding flags across all watchlists
	if _, err := tx.Exec("UPDATE watchlist_items SET is_holding = FALSE WHERE is_holding = TRUE"); err != nil {
		return fmt.Errorf("failed to reset is_holding flags: %w", err)
	}

	// 2. Ensure positions-detenues watchlist header exists
	if _, err := tx.Exec(`
		INSERT INTO watchlists (id, name, description, is_default, source, feed_url, updated_at)
		VALUES ('positions-detenues', 'Positions détenues ($$$$)', 'Active held portfolio positions tracked according to Méthode 8.8 Step E', FALSE, 'manual', '', CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO NOTHING
	`); err != nil {
		return fmt.Errorf("failed to ensure positions-detenues watchlist: %w", err)
	}

	// 3. Snapshot operator overrides currently on positions-detenues
	prevByInst := make(map[string]holdingOverride)
	prevBySym := make(map[string]holdingOverride)
	snapRows, err := tx.Query(`
		SELECT instrument_id, symbol, enabled, tier, size_mode, size_value, size_val, name
		FROM watchlist_items
		WHERE watchlist_id = 'positions-detenues'
	`)
	if err != nil {
		return fmt.Errorf("failed to snapshot positions-detenues overrides: %w", err)
	}
	for snapRows.Next() {
		var instID, sym, tier, sizeMode, name string
		var enabled bool
		var sizeValue, sizeVal float64
		if err := snapRows.Scan(&instID, &sym, &enabled, &tier, &sizeMode, &sizeValue, &sizeVal, &name); err == nil {
			ov := holdingOverride{
				Enabled:   enabled,
				Tier:      tier,
				SizeMode:  sizeMode,
				SizeValue: sizeValue,
				SizeVal:   sizeVal,
				Name:      name,
			}
			if instID != "" {
				prevByInst[strings.ToUpper(instID)] = ov
			}
			if sym != "" {
				prevBySym[strings.ToUpper(sym)] = ov
			}
		}
	}
	if err := snapRows.Err(); err != nil {
		snapRows.Close()
		return err
	}
	snapRows.Close()

	// 4. Match active holdings across all non-regime watchlists and resolve canonical instruments
	var resolved []resolvedHolding
	seenInst := make(map[string]bool)

	for _, pos := range positions {
		if pos.Position == 0 || strings.TrimSpace(pos.Symbol) == "" {
			continue
		}

		rawSym := strings.ToUpper(strings.TrimSpace(pos.Symbol))
		dotSym := strings.ReplaceAll(rawSym, " ", ".")
		venue := strings.ToUpper(strings.TrimSpace(pos.Venue))
		if venue == "ISLAND" {
			venue = "NASDAQ"
		}
		instInput := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(pos.InstrumentID)), " ", ".")
		if strings.HasSuffix(instInput, ".ISLAND") {
			instInput = strings.TrimSuffix(instInput, ".ISLAND") + ".NASDAQ"
		}

		raw := dotSym
		if venue != "" && venue != "SMART" {
			raw = venue + ":" + dotSym
		} else if instInput != "" && strings.Contains(instInput, ".") {
			raw = instInput
		}
		norm := models.NormalizeSymbol(raw)

		currency := strings.ToUpper(strings.TrimSpace(pos.Currency))
		if currency == "" {
			currency = norm.Currency
		}

		// Candidate matching in existing non-regime watchlists (excluding positions-detenues)
		type matchRow struct {
			id           int64
			exchange     string
			venue        string
			instrumentID string
			name         string
		}
		var matches []matchRow

		var rows *sql.Rows
		if norm.Exchange != "" && norm.Exchange != "SMART" {
			rows, err = tx.Query(`
				SELECT id, exchange, venue, instrument_id, name
				FROM watchlist_items
				WHERE watchlist_id != 'positions-detenues'
				  AND is_regime_reference = FALSE
				  AND (
				      instrument_id = ?
				      OR ((symbol = ? OR symbol = ?) AND exchange = ?)
				  )
			`, norm.InstrumentID, dotSym, rawSym, norm.Exchange)
		} else {
			if currency == "CAD" {
				rows, err = tx.Query(`
					SELECT id, exchange, venue, instrument_id, name
					FROM watchlist_items
					WHERE watchlist_id != 'positions-detenues'
					  AND is_regime_reference = FALSE
					  AND (symbol = ? OR symbol = ?)
					  AND exchange IN ('TSX', 'TSXV', 'CSE', 'NEO')
				`, dotSym, rawSym)
			} else {
				rows, err = tx.Query(`
					SELECT id, exchange, venue, instrument_id, name
					FROM watchlist_items
					WHERE watchlist_id != 'positions-detenues'
					  AND is_regime_reference = FALSE
					  AND (symbol = ? OR symbol = ?)
					  AND exchange IN ('NASDAQ', 'NYSE', 'AMEX', 'ARCA', 'BATS', 'CBOE', 'SMART', 'IEX', 'ISLAND')
				`, dotSym, rawSym)
			}
		}
		if err != nil {
			return fmt.Errorf("failed to query watchlist matches for %s: %w", norm.InstrumentID, err)
		}

		for rows.Next() {
			var mr matchRow
			if err := rows.Scan(&mr.id, &mr.exchange, &mr.venue, &mr.instrumentID, &mr.name); err == nil {
				matches = append(matches, mr)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()

		canonicalExchange := norm.Exchange
		canonicalVenue := norm.Venue
		canonicalInst := norm.InstrumentID
		canonicalName := dotSym

		for _, mr := range matches {
			if _, err := tx.Exec("UPDATE watchlist_items SET is_holding = TRUE, updated_at = CURRENT_TIMESTAMP WHERE id = ?", mr.id); err != nil {
				return fmt.Errorf("failed to flag is_holding on item %d: %w", mr.id, err)
			}
			if canonicalExchange == "SMART" && mr.exchange != "" && mr.exchange != "SMART" {
				canonicalExchange = mr.exchange
				canonicalVenue = mr.venue
				canonicalInst = mr.instrumentID
			}
			if mr.name != "" && canonicalName == dotSym {
				canonicalName = mr.name
			}
		}

		if (canonicalExchange == "" || canonicalExchange == "SMART") && currency == "CAD" {
			canonicalExchange = "TSX"
			canonicalVenue = "TSE"
			canonicalInst = dotSym + "." + canonicalExchange
		} else if canonicalExchange == "" {
			canonicalExchange = "SMART"
			canonicalVenue = "SMART"
			canonicalInst = dotSym + "." + canonicalExchange
		}

		if seenInst[canonicalInst] {
			continue
		}
		seenInst[canonicalInst] = true

		resolved = append(resolved, resolvedHolding{
			Symbol:       dotSym,
			Exchange:     canonicalExchange,
			Venue:        canonicalVenue,
			InstrumentID: canonicalInst,
			Name:         canonicalName,
		})
	}

	// 5. Replace positions-detenues items strictly with current live holdings
	if _, err := tx.Exec("DELETE FROM watchlist_items WHERE watchlist_id = 'positions-detenues'"); err != nil {
		return fmt.Errorf("failed to clear positions-detenues: %w", err)
	}

	if len(resolved) > 0 {
		insertStmt, err := tx.Prepare(`
			INSERT INTO watchlist_items (
				watchlist_id, group_id, group_name, symbol, exchange, venue, instrument_id,
				name, enabled, tier, is_orderable, is_regime_reference, is_holding,
				size_mode, size_value, size_val, created_at, updated_at
			) VALUES ('positions-detenues', 'holding', 'Positions Détenues', ?, ?, ?, ?, ?, ?, ?, TRUE, FALSE, TRUE, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`)
		if err != nil {
			return err
		}
		defer insertStmt.Close()

		for _, h := range resolved {
			enabled := true
			tier := "1-Blue Chip"
			sizeMode := "shares"
			sizeValue := 100.0
			sizeVal := 100.0
			itemName := h.Name
			if itemName == "" {
				itemName = h.Symbol
			}

			if ov, ok := prevByInst[strings.ToUpper(h.InstrumentID)]; ok {
				enabled = ov.Enabled
				if ov.Tier != "" {
					tier = ov.Tier
				}
				if ov.SizeMode != "" {
					sizeMode = ov.SizeMode
				}
				if ov.SizeValue > 0 {
					sizeValue = ov.SizeValue
				}
				if ov.SizeVal > 0 {
					sizeVal = ov.SizeVal
				} else {
					sizeVal = sizeValue
				}
				if ov.Name != "" {
					itemName = ov.Name
				}
			} else if ov, ok := prevBySym[strings.ToUpper(h.Symbol)]; ok {
				enabled = ov.Enabled
				if ov.Tier != "" {
					tier = ov.Tier
				}
				if ov.SizeMode != "" {
					sizeMode = ov.SizeMode
				}
				if ov.SizeValue > 0 {
					sizeValue = ov.SizeValue
				}
				if ov.SizeVal > 0 {
					sizeVal = ov.SizeVal
				} else {
					sizeVal = sizeValue
				}
				if ov.Name != "" {
					itemName = ov.Name
				}
			}

			if _, err := insertStmt.Exec(
				h.Symbol, h.Exchange, h.Venue, h.InstrumentID, itemName,
				enabled, tier, sizeMode, sizeValue, sizeVal,
			); err != nil {
				return fmt.Errorf("failed to insert holding %s into positions-detenues: %w", h.InstrumentID, err)
			}
		}
	}

	return tx.Commit()
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
