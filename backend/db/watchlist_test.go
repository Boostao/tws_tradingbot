package db

import (
	"sync"
	"testing"
	"tws_traderbot/backend/models"
)

func TestWatchlistState_Persistence(t *testing.T) {
	db := setupTestDB(t)

	// Save a state
	state := &models.WatchlistResponse{
		ID:   "custom-list",
		Name: "Custom List",
		Symbols: []string{"AAPL"},
		Groups: []models.WatchlistGroup{
			{ID: "g1", Name: "Tech", Source: "manual", Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "SMART", Name: "Apple", Enabled: true},
			}},
		},
	}
	if err := db.SaveWatchlistState(state); err != nil {
		t.Fatalf("Failed to save watchlist state: %v", err)
	}

	// Load and verify
	loaded, err := db.GetWatchlistByID("custom-list")
	if err != nil {
		t.Fatalf("Failed to load watchlist state: %v", err)
	}

	if len(loaded.Symbols) != 1 || loaded.Symbols[0] != "AAPL.SMART" {
		t.Errorf("Symbols mismatch, expected [AAPL.SMART], got: %v", loaded.Symbols)
	}
	if len(loaded.Groups) != 1 || loaded.Groups[0].Name != "Tech" {
		t.Errorf("Groups mismatch: %v", loaded.Groups)
	}
}

func TestWatchlist_RelationalSchemaAndCascadingDelete(t *testing.T) {
	db := setupTestDB(t)

	wlID := "test-cascade-wl"
	_, err := db.MergeAndSaveWatchlist(wlID, "Cascade Test", "Desc", nil, []models.WatchlistGroup{
		{
			ID:   "grp1",
			Name: "Group 1",
			Items: []models.WatchlistItem{
				{Symbol: "MSFT", Exchange: "NASDAQ", Enabled: true},
				{Symbol: "GOOGL", Exchange: "NASDAQ", Enabled: true},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("failed to insert watchlist: %v", err)
	}

	// Check items exist in DB
	var itemCount int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM watchlist_items WHERE watchlist_id = ?", wlID).Scan(&itemCount)
	if err != nil || itemCount != 2 {
		t.Fatalf("expected 2 items, got %d (err: %v)", itemCount, err)
	}

	// Delete parent watchlist
	if err := db.DeleteWatchlist(wlID); err != nil {
		t.Fatalf("failed to delete watchlist: %v", err)
	}

	// Verify CASCADE deleted child items
	err = db.conn.QueryRow("SELECT COUNT(*) FROM watchlist_items WHERE watchlist_id = ?", wlID).Scan(&itemCount)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if itemCount != 0 {
		t.Errorf("expected 0 items after cascade delete, got %d", itemCount)
	}
}

func TestWatchlist_PreSeededCobaltLists(t *testing.T) {
	db := setupTestDB(t)

	summaries, err := db.GetWatchlists()
	if err != nil {
		t.Fatalf("failed to get watchlists: %v", err)
	}

	expectedIDs := map[string]bool{
		"cobalt-telegram":    false,
		"cobalt-home-run":    false,
		"cblt-marches":       false,
		"premarkets":         false,
		"positions-detenues": false,
	}

	for _, s := range summaries {
		if _, ok := expectedIDs[s.ID]; ok {
			expectedIDs[s.ID] = true
			if s.ID == "positions-detenues" {
				if s.ItemCount != 0 {
					t.Errorf("seed watchlist %s should have 0 items before live TWS sync, got %d", s.ID, s.ItemCount)
				}
			} else if s.ItemCount == 0 {
				t.Errorf("seed watchlist %s has 0 items", s.ID)
			}
		}
	}

	for id, found := range expectedIDs {
		if !found {
			t.Errorf("expected pre-seeded watchlist %s was not found", id)
		}
	}
}

func TestWatchlist_SymbolNormalizationAndCollisions(t *testing.T) {
	db := setupTestDB(t)

	// Check collision symbols in cobalt-telegram
	wl, err := db.GetWatchlistByID("cobalt-telegram")
	if err != nil {
		t.Fatalf("failed to load cobalt-telegram: %v", err)
	}

	instruments := make(map[string]bool)
	for _, grp := range wl.Groups {
		for _, item := range grp.Items {
			instruments[item.InstrumentID] = true
		}
	}

	// Check 8 documented collision resolutions
	collisions := []string{
		"BTCUSD.BINANCE", "BTCUSD.COINBASE", "BTCUSD.INDEX",
		"ETHUSD.COINBASE", "ETHUSD.INDEX",
		"FETH.TSX", "FETH.CBOE",
		"AQN.TSX", "AQN.NYSE",
		"CNR.TSX", "CNR.NYSE",
		"NFLX.TSX", "NFLX.NASDAQ",
		"MDA.TSX", "MDA.NYSE",
	}

	for _, c := range collisions {
		if !instruments[c] {
			t.Errorf("expected collision-disambiguated instrument %s in cobalt-telegram", c)
		}
	}

	// Check SPX collision in cblt-marches
	marches, err := db.GetWatchlistByID("cblt-marches")
	if err != nil {
		t.Fatalf("failed to load cblt-marches: %v", err)
	}
	marchesInstruments := make(map[string]bool)
	for _, grp := range marches.Groups {
		for _, item := range grp.Items {
			marchesInstruments[item.InstrumentID] = true
			if item.InstrumentID == "VIX.TVC" || item.InstrumentID == "SPX.TVC" || item.InstrumentID == "TSX.TSX" {
				if item.IsOrderable {
					t.Errorf("expected %s to have IsOrderable=false", item.InstrumentID)
				}
				if !item.IsRegimeReference {
					t.Errorf("expected %s to have IsRegimeReference=true", item.InstrumentID)
				}
			}
		}
	}
	if !marchesInstruments["SPX.TVC"] {
		t.Errorf("expected SPX.TVC in cblt-marches")
	}
}

func TestWatchlist_SmartDiffMerge(t *testing.T) {
	db := setupTestDB(t)
	wlID := "test-diff-merge"

	// 1. Initial import
	initialGroups := []models.WatchlistGroup{
		{
			ID:   "g1",
			Name: "Group 1",
			Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, Tier: "1-Blue Chip", SizeMode: "shares", SizeValue: 100},
				{Symbol: "TSLA", Exchange: "NASDAQ", Enabled: true, Tier: "2-Croissance", SizeMode: "shares", SizeValue: 50},
			},
		},
	}
	_, err := db.MergeAndSaveWatchlist(wlID, "Test Diff Merge", "Desc", nil, initialGroups, false)
	if err != nil {
		t.Fatalf("initial merge failed: %v", err)
	}

	// 2. Operator toggles AAPL OFF and changes sizing to $5000, and flags TSLA as holding ($$$$)
	_, err = db.conn.Exec(`
		UPDATE watchlist_items
		SET enabled = FALSE, size_mode = 'dollar', size_value = 5000.0, tier = '4-Rejeté'
		WHERE watchlist_id = ? AND instrument_id = 'AAPL.NASDAQ'
	`, wlID)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	_, err = db.conn.Exec(`
		UPDATE watchlist_items
		SET is_holding = TRUE
		WHERE watchlist_id = ? AND instrument_id = 'TSLA.NASDAQ'
	`, wlID)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// 3. Incoming feed update re-imports AAPL, TSLA and a new ticker NVDA with default enabled=true
	incomingFeed := []models.WatchlistGroup{
		{
			ID:   "g1",
			Name: "Group 1",
			Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true, Tier: "1-Blue Chip", SizeMode: "shares", SizeValue: 100},
				{Symbol: "TSLA", Exchange: "NASDAQ", Enabled: true, Tier: "2-Croissance", SizeMode: "shares", SizeValue: 50},
				{Symbol: "NVDA", Exchange: "NASDAQ", Enabled: true, Tier: "1-Blue Chip", SizeMode: "shares", SizeValue: 100},
			},
		},
	}
	merged, err := db.MergeAndSaveWatchlist(wlID, "Test Diff Merge", "Desc", nil, incomingFeed, true)
	if err != nil {
		t.Fatalf("second merge failed: %v", err)
	}

	// Verify AAPL overrides are PRESERVED: enabled=false, size_mode='dollar', size_value=5000, tier='4-Rejeté'
	var aaplItem, tslaItem, nvdaItem *models.WatchlistItem
	for i := range merged.Groups[0].Items {
		item := &merged.Groups[0].Items[i]
		if item.InstrumentID == "AAPL.NASDAQ" {
			aaplItem = item
		} else if item.InstrumentID == "TSLA.NASDAQ" {
			tslaItem = item
		} else if item.InstrumentID == "NVDA.NASDAQ" {
			nvdaItem = item
		}
	}

	if aaplItem == nil {
		t.Fatalf("AAPL.NASDAQ not found in merged list")
	}
	if aaplItem.Enabled {
		t.Errorf("AAPL.NASDAQ should have remained DISABLED (enabled=false)")
	}
	if aaplItem.SizeMode != "dollar" || aaplItem.SizeValue != 5000.0 {
		t.Errorf("AAPL.NASDAQ sizing override lost: mode=%s, val=%f", aaplItem.SizeMode, aaplItem.SizeValue)
	}
	if aaplItem.Tier != "4-Rejeté" {
		t.Errorf("AAPL.NASDAQ tier override lost, expected 4-Rejeté, got %s", aaplItem.Tier)
	}

	if tslaItem == nil {
		t.Fatalf("TSLA.NASDAQ not found in merged list")
	}
	if !tslaItem.IsHolding {
		t.Errorf("TSLA.NASDAQ is_holding flag should have been preserved as true")
	}

	if nvdaItem == nil {
		t.Fatalf("NVDA.NASDAQ not found in merged list")
	}
	if !nvdaItem.Enabled {
		t.Errorf("new ticker NVDA.NASDAQ should be enabled by default")
	}
}

func TestWatchlist_SaveWatchlistState_RetainsDisabled(t *testing.T) {
	db := setupTestDB(t)
	wlID := "test-operator-save"

	// 1. Initial watchlist with an enabled item
	initialState := &models.WatchlistResponse{
		ID:   wlID,
		Name: "Operator Save Test",
		Groups: []models.WatchlistGroup{
			{
				ID:   "tech",
				Name: "Tech Stocks",
				Items: []models.WatchlistItem{
					{
						Symbol:    "AAPL",
						Exchange:  "NASDAQ",
						Name:      "Apple Inc",
						Enabled:   true,
						Tier:      "2-Croissance",
						SizeMode:  "shares",
						SizeValue: 100.0,
						IsHolding: false,
					},
				},
			},
		},
	}
	if err := db.SaveWatchlistState(initialState); err != nil {
		t.Fatalf("initial SaveWatchlistState failed: %v", err)
	}

	// Verify it was stored as enabled
	loaded, err := db.GetWatchlistByID(wlID)
	if err != nil {
		t.Fatalf("failed to load watchlist: %v", err)
	}
	if len(loaded.Groups) != 1 || len(loaded.Groups[0].Items) != 1 {
		t.Fatalf("unexpected structure in loaded watchlist: %+v", loaded)
	}
	if !loaded.Groups[0].Items[0].Enabled {
		t.Fatalf("expected item to start as enabled")
	}

	// 2. Operator modifies state: toggles Enabled to FALSE, changes tier, sizing, and holding
	modifiedState := &models.WatchlistResponse{
		ID:   wlID,
		Name: "Operator Save Test",
		Groups: []models.WatchlistGroup{
			{
				ID:   "tech",
				Name: "Tech Stocks",
				Items: []models.WatchlistItem{
					{
						Symbol:    "AAPL",
						Exchange:  "NASDAQ",
						Name:      "Apple Inc",
						Enabled:   false, // Operator disabled this item
						Tier:      "1-Blue Chip",
						SizeMode:  "dollar",
						SizeValue: 5000.0,
						IsHolding: true,
					},
				},
			},
		},
	}
	if err := db.SaveWatchlistState(modifiedState); err != nil {
		t.Fatalf("second SaveWatchlistState failed: %v", err)
	}

	// 3. Reload from DB and verify that Enabled = FALSE was retained (not reverted to previous true!)
	reloaded, err := db.GetWatchlistByID(wlID)
	if err != nil {
		t.Fatalf("failed to reload watchlist: %v", err)
	}

	item := reloaded.Groups[0].Items[0]
	if item.Enabled {
		t.Errorf("FAIL: SaveWatchlistState reverted Enabled=false back to Enabled=true!")
	}
	if item.Tier != "1-Blue Chip" {
		t.Errorf("FAIL: SaveWatchlistState reverted Tier to %s, expected 1-Blue Chip", item.Tier)
	}
	if item.SizeMode != "dollar" || item.SizeValue != 5000.0 {
		t.Errorf("FAIL: SaveWatchlistState reverted sizing: mode=%s, val=%f", item.SizeMode, item.SizeValue)
	}
	if !item.IsHolding {
		t.Errorf("FAIL: SaveWatchlistState reverted IsHolding to false, expected true")
	}
}

func TestSQLite_ConnectionPoolPRAGMAInheritance(t *testing.T) {
	db, err := Connect(":memory:")
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	var wg sync.WaitGroup
	var offCount int
	var mu sync.Mutex

	// Test 20 concurrent goroutines querying PRAGMA foreign_keys
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var fk int
			if err := db.conn.QueryRow("PRAGMA foreign_keys;").Scan(&fk); err != nil {
				t.Errorf("query failed: %v", err)
				return
			}
			if fk == 0 {
				mu.Lock()
				offCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if offCount > 0 {
		t.Errorf("CRITICAL DEFECT: %d/20 pooled connections ran with foreign_keys = 0", offCount)
	}
}

func TestWatchlist_SyncLivePositions_AndLegacyMigration(t *testing.T) {
	db := setupTestDB(t)

	// 1. Confirm positions-detenues starts empty (no AAPL.NASDAQ / SHOP.TSX)
	pd, err := db.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("failed to load positions-detenues: %v", err)
	}
	if len(pd.Symbols) != 0 || pd.Symbols == nil {
		t.Errorf("expected empty non-nil Symbols slice on positions-detenues, got %#v", pd.Symbols)
	}

	// 2. Sync live TWS positions: GOOGL (USD, SMART), BRK B (USD, SMART), AQN (CAD, SMART), PLTR (USD, SMART, unlisted)
	live := []models.BrokerPosition{
		{Symbol: "GOOGL", Venue: "SMART", InstrumentID: "GOOGL.SMART", Currency: "USD", Position: 10, AvgCost: 175.0},
		{Symbol: "BRK B", Venue: "SMART", InstrumentID: "BRK B.SMART", Currency: "USD", Position: 2, AvgCost: 410.0},
		{Symbol: "AQN", Venue: "SMART", InstrumentID: "AQN.SMART", Currency: "CAD", Position: 50, AvgCost: 7.5},
		{Symbol: "PLTR", Venue: "SMART", InstrumentID: "PLTR.SMART", Currency: "USD", Position: 25, AvgCost: 42.0},
		{Symbol: "MSFT", Venue: "SMART", InstrumentID: "MSFT.SMART", Currency: "USD", Position: 0, AvgCost: 0}, // flat pos ignored
	}
	if err := db.SyncLivePositions(live); err != nil {
		t.Fatalf("SyncLivePositions failed: %v", err)
	}

	// Verify positions-detenues contains GOOGL.NASDAQ, BRK.B.NYSE, AQN.TSX, PLTR.SMART (not MSFT)
	pd, err = db.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("failed to reload positions-detenues: %v", err)
	}
	pdMap := make(map[string]models.WatchlistItem)
	for _, g := range pd.Groups {
		for _, it := range g.Items {
			pdMap[it.InstrumentID] = it
		}
	}
	for _, expected := range []string{"GOOGL.NASDAQ", "BRK.B.NYSE", "AQN.TSX", "PLTR.SMART"} {
		if it, ok := pdMap[expected]; !ok || !it.IsHolding {
			t.Errorf("expected %s with IsHolding=true in positions-detenues, got %+v", expected, pdMap)
		}
	}
	if _, ok := pdMap["MSFT.SMART"]; ok {
		t.Errorf("flat position MSFT should not be in positions-detenues")
	}

	// Verify cross-watchlist is_holding flags and CAD/USD disambiguation on cobalt-telegram
	ct, err := db.GetWatchlistByID("cobalt-telegram")
	if err != nil {
		t.Fatalf("failed to load cobalt-telegram: %v", err)
	}
	ctMap := make(map[string]models.WatchlistItem)
	for _, g := range ct.Groups {
		for _, it := range g.Items {
			ctMap[it.InstrumentID] = it
		}
	}
	if !ctMap["GOOGL.NASDAQ"].IsHolding {
		t.Errorf("expected GOOGL.NASDAQ IsHolding=true in cobalt-telegram")
	}
	if !ctMap["BRK.B.NYSE"].IsHolding {
		t.Errorf("expected BRK.B.NYSE IsHolding=true in cobalt-telegram")
	}
	if !ctMap["AQN.TSX"].IsHolding {
		t.Errorf("expected AQN.TSX (CAD) IsHolding=true in cobalt-telegram")
	}
	if ctMap["AQN.NYSE"].IsHolding {
		t.Errorf("expected AQN.NYSE (USD) IsHolding=false when holding is CAD")
	}

	// 3. Modify operator override on GOOGL.NASDAQ in positions-detenues and re-sync
	if _, err := db.conn.Exec(`
		UPDATE watchlist_items SET enabled = FALSE, size_mode = 'dollar', size_value = 3500.0
		WHERE watchlist_id = 'positions-detenues' AND instrument_id = 'GOOGL.NASDAQ'
	`); err != nil {
		t.Fatalf("failed to update override: %v", err)
	}
	if err := db.SyncLivePositions([]models.BrokerPosition{
		{Symbol: "GOOGL", Venue: "SMART", Currency: "USD", Position: 15, AvgCost: 176.0},
	}); err != nil {
		t.Fatalf("second SyncLivePositions failed: %v", err)
	}

	pd2, err := db.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("failed to reload positions-detenues: %v", err)
	}
	if len(pd2.Groups) != 1 || len(pd2.Groups[0].Items) != 1 {
		t.Fatalf("expected 1 holding after second sync, got %+v", pd2.Groups)
	}
	googl := pd2.Groups[0].Items[0]
	if googl.Enabled || googl.SizeMode != "dollar" || googl.SizeValue != 3500.0 {
		t.Errorf("operator override not preserved on positions-detenues resync: %+v", googl)
	}

	// Verify legacy migration cleans AAPL.NASDAQ/SHOP.TSX and repairs clobbered cobalt-telegram name
	if _, err := db.conn.Exec("DELETE FROM system_config WHERE key = 'migration_cobalt_v2_clean_holdings'"); err != nil {
		t.Fatalf("delete migration flag: %v", err)
	}
	if _, err := db.conn.Exec("UPDATE watchlists SET name = 'cobalt-telegram' WHERE id = 'cobalt-telegram'"); err != nil {
		t.Fatalf("clobber name: %v", err)
	}
	if _, err := db.conn.Exec(`
		INSERT INTO watchlist_items (watchlist_id, group_id, group_name, symbol, exchange, venue, instrument_id, name, enabled, is_holding)
		VALUES ('positions-detenues', 'holding', 'Positions Détenues', 'AAPL', 'NASDAQ', 'SMART', 'AAPL.NASDAQ', 'AAPL', TRUE, TRUE)
	`); err != nil {
		t.Fatalf("insert legacy AAPL: %v", err)
	}
	db.InitSchema()

	ctAfter, err := db.GetWatchlistByID("cobalt-telegram")
	if err != nil {
		t.Fatalf("load cobalt-telegram after migration: %v", err)
	}
	if ctAfter.Name != "✝️@ Cobalt- TELEGRAM" {
		t.Errorf("expected repaired name, got %q", ctAfter.Name)
	}
	pdAfter, _ := db.GetWatchlistByID("positions-detenues")
	for _, g := range pdAfter.Groups {
		for _, it := range g.Items {
			if it.InstrumentID == "AAPL.NASDAQ" {
				t.Errorf("legacy AAPL.NASDAQ should have been cleaned up")
			}
		}
	}
}

func TestWatchlist_MergeDotClassExchangeAndSyncLiveVenueEdgeCases(t *testing.T) {
	db := setupTestDB(t)

	// 1. MergeAndSaveWatchlist preserves item.Exchange for dot-class tickers (BRK.B + NYSE -> BRK.B.NYSE)
	merged, err := db.MergeAndSaveWatchlist("wl-dotclass", "Dot Class", "", nil, []models.WatchlistGroup{
		{
			ID:   "core",
			Name: "Core",
			Items: []models.WatchlistItem{
				{Symbol: "BRK.B", Exchange: "NYSE", Enabled: true},
				{Symbol: "CTC.A", Exchange: "TSX", Enabled: true},
				{Symbol: "AQN", Exchange: "NASDAQ", Enabled: true},
			},
		},
	}, false)
	if err != nil {
		t.Fatalf("MergeAndSaveWatchlist failed: %v", err)
	}
	gotInsts := make(map[string]models.WatchlistItem)
	for _, g := range merged.Groups {
		for _, it := range g.Items {
			gotInsts[it.InstrumentID] = it
		}
	}
	if _, ok := gotInsts["BRK.B.NYSE"]; !ok {
		t.Errorf("expected BRK.B.NYSE when merging BRK.B with Exchange=NYSE, got %+v", merged.Symbols)
	}
	if _, ok := gotInsts["CTC.A.TSX"]; !ok {
		t.Errorf("expected CTC.A.TSX when merging CTC.A with Exchange=TSX, got %+v", merged.Symbols)
	}

	// 2. SyncLivePositions:
	//    - Unlisted XLE (Venue=ARCA, InstrumentID=XLE.ARCA, USD) -> XLE.ARCA (not XLE.ARCA.SMART)
	//    - Unlisted NEWCAD (Venue=SMART, CAD) -> NEWCAD.TSX (Venue=TSE, not NEWCAD.SMART)
	//    - Explicit AQN.NYSE (Venue=NYSE, InstrumentID=AQN.NYSE, USD) -> flags AQN.NYSE in cobalt-telegram,
	//      does NOT falsely flag AQN.NASDAQ in wl-dotclass or AQN.TSX in cobalt-telegram.
	if err := db.SyncLivePositions([]models.BrokerPosition{
		{Symbol: "XLE", Venue: "ARCA", InstrumentID: "XLE.ARCA", Currency: "USD", Position: 12, AvgCost: 90.0},
		{Symbol: "NEWCAD", Venue: "SMART", InstrumentID: "NEWCAD.SMART", Currency: "CAD", Position: 30, AvgCost: 15.0},
		{Symbol: "AQN", Venue: "NYSE", InstrumentID: "AQN.NYSE", Currency: "USD", Position: 40, AvgCost: 5.5},
	}); err != nil {
		t.Fatalf("SyncLivePositions failed: %v", err)
	}

	pd, err := db.GetWatchlistByID("positions-detenues")
	if err != nil {
		t.Fatalf("GetWatchlistByID(positions-detenues) failed: %v", err)
	}
	pdMap := make(map[string]models.WatchlistItem)
	for _, g := range pd.Groups {
		for _, it := range g.Items {
			pdMap[it.InstrumentID] = it
		}
	}
	if xle, ok := pdMap["XLE.ARCA"]; !ok || xle.Exchange != "ARCA" {
		t.Errorf("expected XLE.ARCA with Exchange=ARCA, got %+v (pdMap=%+v)", xle, pdMap)
	}
	if _, bad := pdMap["XLE.ARCA.SMART"]; bad {
		t.Errorf("unexpected mangled XLE.ARCA.SMART in positions-detenues")
	}
	if nc, ok := pdMap["NEWCAD.TSX"]; !ok || nc.Exchange != "TSX" || nc.Venue != "TSE" {
		t.Errorf("expected NEWCAD.TSX with Exchange=TSX, Venue=TSE, got %+v (pdMap=%+v)", nc, pdMap)
	}
	if _, ok := pdMap["AQN.NYSE"]; !ok {
		t.Errorf("expected AQN.NYSE in positions-detenues, got %+v", pdMap)
	}

	wlDot, err := db.GetWatchlistByID("wl-dotclass")
	if err != nil {
		t.Fatalf("GetWatchlistByID(wl-dotclass) failed: %v", err)
	}
	for _, g := range wlDot.Groups {
		for _, it := range g.Items {
			if it.InstrumentID == "AQN.NASDAQ" && it.IsHolding {
				t.Errorf("explicit AQN.NYSE position must not mark AQN.NASDAQ as IsHolding=true")
			}
		}
	}

	ct, err := db.GetWatchlistByID("cobalt-telegram")
	if err != nil {
		t.Fatalf("GetWatchlistByID(cobalt-telegram) failed: %v", err)
	}
	for _, g := range ct.Groups {
		for _, it := range g.Items {
			if it.InstrumentID == "AQN.NYSE" && !it.IsHolding {
				t.Errorf("expected AQN.NYSE IsHolding=true in cobalt-telegram")
			}
			if it.InstrumentID == "AQN.TSX" && it.IsHolding {
				t.Errorf("expected AQN.TSX IsHolding=false for AQN.NYSE USD holding")
			}
		}
	}
}
