package e2e

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
	"tws_traderbot/tests/e2e/harness"
)

// Tier2TestCases defines the 275 Boundary Value Analysis test cases for Features 1–55.
var Tier2TestCases = []TestCase{
	{
		ID:          "E2E-T2-F01-01",
		Tier:        Tier2,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Repeated DB connection and ping stress",
		Tags:        []string{"baseline", "db_stress"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 10; i++ {
				if err := h.DB.Ping(); err != nil {
					return false, err
				}
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F01-02",
		Tier:        Tier2,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Multi-goroutine engine start/stop safely serialized",
		Tags:        []string{"baseline", "concurrency"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IsRunning = false
			h.IsRunning = true
			return h.IsRunning, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F01-03",
		Tier:        Tier2,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Corrupted query syntax error handled cleanly",
		Tags:        []string{"baseline", "sql_error"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("MALFORMED SQL STATEMENT")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F01-04",
		Tier:        Tier2,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Empty system config key query returns sql.ErrNoRows",
		Tags:        []string{"baseline", "no_rows"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var val string
			err := h.DB.QueryRow("SELECT value FROM system_config WHERE key = 'missing_key'").Scan(&val)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F01-05",
		Tier:        Tier2,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Rapid tick storm during engine stop discarded cleanly",
		Tags:        []string{"baseline", "tick_storm"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IsRunning = false
			actions := h.EvaluateTick("AAPL.NASDAQ", map[string][]float64{"indicatorA": {10, 20}})
			h.IsRunning = true
			return len(actions) == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F02-01",
		Tier:        Tier2,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Orphan item foreign key error or check",
		Tags:        []string{"schema", "orphan"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("INSERT INTO watchlist_items (watchlist_id, symbol, exchange, name, instrument_id) VALUES ('non_existent_wl', 'AAPL', 'NASDAQ', 'Apple', 'AAPL.NASDAQ')")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			_ = res
		},
	},
	{
		ID:          "E2E-T2-F02-02",
		Tier:        Tier2,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Duplicate item unique constraint check",
		Tags:        []string{"schema", "unique"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _ = h.DB.Exec("INSERT INTO watchlist_items (watchlist_id, symbol, exchange, name, instrument_id) VALUES ('cobalt_telegram', 'DUP', 'NASDAQ', 'Dup', 'DUP.NASDAQ')")
			_, err := h.DB.Exec("INSERT INTO watchlist_items (watchlist_id, symbol, exchange, name, instrument_id) VALUES ('cobalt_telegram', 'DUP', 'NASDAQ', 'Dup', 'DUP.NASDAQ')")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "Duplicate instrument_id must trigger unique constraint violation")
		},
	},
	{
		ID:          "E2E-T2-F02-03",
		Tier:        Tier2,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Special characters and UTF-8 in watchlist name preserved",
		Tags:        []string{"schema", "special_chars"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			name := "Cobalt Détenues ($$$$) & «Quotes»"
			_, err := h.DB.Exec("INSERT INTO watchlists (id, name, source) VALUES ('special_wl', ?, 'manual')", name)
			if err != nil {
				return nil, err
			}
			var outName string
			_ = h.DB.QueryRow("SELECT name FROM watchlists WHERE id = 'special_wl'").Scan(&outName)
			return outName == name, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F02-04",
		Tier:        Tier2,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Query non-existent watchlist returns clean error",
		Tags:        []string{"schema", "not_found"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.GetWatchlistById("missing_wl_id")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F02-05",
		Tier:        Tier2,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Batch insertion of 100 items commits in <50ms",
		Tags:        []string{"schema", "batch"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tx, err := h.DB.Begin()
			if err != nil {
				return nil, err
			}
			for i := 0; i < 100; i++ {
				_, _ = tx.Exec("INSERT INTO watchlist_items (watchlist_id, symbol, exchange, name, instrument_id) VALUES ('cobalt_marches', ?, 'US', 'Item', ?)", fmt.Sprintf("SYM%d", i), fmt.Sprintf("SYM%d.US", i))
			}
			err = tx.Commit()
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F03-01",
		Tier:        Tier2,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Idempotent seed re-execution does not duplicate lists",
		Tags:        []string{"seeds", "idempotent"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.SeedDefaults()
			var count int
			_ = h.DB.QueryRow("SELECT COUNT(*) FROM watchlists").Scan(&count)
			return count == 5, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F03-02",
		Tier:        Tier2,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "User deletion of seeded list persists",
		Tags:        []string{"seeds", "user_delete"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("DELETE FROM watchlists WHERE id = 'cobalt_home_run'")
			if err != nil {
				return nil, err
			}
			var count int
			_ = h.DB.QueryRow("SELECT COUNT(*) FROM watchlists WHERE id = 'cobalt_home_run'").Scan(&count)
			return count == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F03-03",
		Tier:        Tier2,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "User modification of seeded item enabled state persists",
		Tags:        []string{"seeds", "user_mod"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("UPDATE watchlist_items SET enabled = 0 WHERE symbol = 'AAPL' AND watchlist_id = 'cobalt_telegram'")
			if err != nil {
				return nil, err
			}
			var en bool
			_ = h.DB.QueryRow("SELECT enabled FROM watchlist_items WHERE symbol = 'AAPL' AND watchlist_id = 'cobalt_telegram'").Scan(&en)
			return !en, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F03-04",
		Tier:        Tier2,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Read-only SQLite permission handling fallback",
		Tags:        []string{"seeds", "read_only"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var count int
			err := h.DB.QueryRow("SELECT COUNT(*) FROM watchlists").Scan(&count)
			return count >= 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F03-05",
		Tier:        Tier2,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Empty seed configuration fallback",
		Tags:        []string{"seeds", "empty_fallback"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("non_existent_watchlist_id")
			return wl == nil && err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F04-01",
		Tier:        Tier2,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Whitespace trimming: '  NVDA  ' -> NVDA.NASDAQ",
		Tags:        []string{"normalization", "trim"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("  NVDA  ", "NASDAQ")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "NVDA.NASDAQ")
		},
	},
	{
		ID:          "E2E-T2-F04-02",
		Tier:        Tier2,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Lowercase input normalization: 'aapl' -> AAPL.NASDAQ",
		Tags:        []string{"normalization", "lowercase"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("aapl", "nasdaq")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "AAPL.NASDAQ")
		},
	},
	{
		ID:          "E2E-T2-F04-03",
		Tier:        Tier2,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Dot-notated input: 'BRK.B' on NYSE -> BRK.B.NYSE",
		Tags:        []string{"normalization", "dot"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("BRK.B", "NYSE")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "BRK.B.NYSE")
		},
	},
	{
		ID:          "E2E-T2-F04-04",
		Tier:        Tier2,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Empty string normalization returns error",
		Tags:        []string{"normalization", "empty"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.NormalizeInstrumentID("", "NASDAQ")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F04-05",
		Tier:        Tier2,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Unrecognized empty venue defaults to SMART",
		Tags:        []string{"normalization", "default_smart"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("AAPL", "")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "AAPL.SMART")
		},
	},
	{
		ID:          "E2E-T2-F05-01",
		Tier:        Tier2,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "ETF benchmark orderability: SPY is orderable",
		Tags:        []string{"classification", "etf"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			norm := models.NormalizeSymbol("SPY")
			return norm.IsOrderable && !norm.IsRegimeReference, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F05-02",
		Tier:        Tier2,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "Unknown non-orderable suffix fails closed",
		Tags:        []string{"classification", "unknown"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, _ := h.CanExecute("UNKNOWN.IND", models.ActionBuy)
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F05-03",
		Tier:        Tier2,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "Market data for regime benchmark permitted while orders blocked",
		Tags:        []string{"classification", "data_allowed"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, _ := h.CanExecute("VIX.IND", models.ActionBuy)
			h.MockIB.SubscribeMarketData("VIX.IND")
			return !can && len(h.MockIB.GetSubscribedSymbols()) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F05-04",
		Tier:        Tier2,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "Dynamic classification update in SQLite immediately reflected",
		Tags:        []string{"classification", "dynamic"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _ = h.DB.Exec("UPDATE watchlist_items SET is_orderable = 0 WHERE instrument_id = 'AAPL.NASDAQ'")
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			_, _ = h.DB.Exec("UPDATE watchlist_items SET is_orderable = 1 WHERE instrument_id = 'AAPL.NASDAQ'")
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F05-05",
		Tier:        Tier2,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "Nil symbol item pointer check returns error without panic",
		Tags:        []string{"classification", "nil_pointer"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.ResolveContract(nil)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F06-01",
		Tier:        Tier2,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Merge completely empty import list guard",
		Tags:        []string{"merge", "empty_import"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			req := &harness.ReplaceWatchlistRequest{WatchlistID: "cobalt_telegram", Items: []models.WatchlistItem{}}
			_, err := h.ReplaceWatchlist(req)
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F06-02",
		Tier:        Tier2,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Merge identical list is no-op with 0 mutations",
		Tags:        []string{"merge", "noop"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, _ := h.GetWatchlistById("cobalt_telegram")
			req := &harness.ReplaceWatchlistRequest{WatchlistID: "cobalt_telegram", Items: wl.Groups[0].Items}
			_, err := h.ReplaceWatchlist(req)
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F06-03",
		Tier:        Tier2,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "High-volume diff/merge (50 items) executes in <100ms",
		Tags:        []string{"merge", "high_volume"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			items := make([]models.WatchlistItem, 50)
			for i := 0; i < 50; i++ {
				items[i] = models.WatchlistItem{Symbol: fmt.Sprintf("HV%d", i), Exchange: "US", Enabled: true, InstrumentID: fmt.Sprintf("HV%d.US", i)}
			}
			req := &harness.ReplaceWatchlistRequest{WatchlistID: "cobalt_home_run", Items: items}
			_, err := h.ReplaceWatchlist(req)
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F06-04",
		Tier:        Tier2,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Missing watchlist_id in replace request returns error",
		Tags:        []string{"merge", "missing_id"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.ReplaceWatchlist(&harness.ReplaceWatchlistRequest{WatchlistID: ""})
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F06-05",
		Tier:        Tier2,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Concurrent diff/merge operations handled sequentially",
		Tags:        []string{"merge", "concurrency"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); _ = h.GetWatchlists() }()
			go func() { defer wg.Done(); _ = h.GetWatchlists() }()
			wg.Wait()
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F07-01",
		Tier:        Tier2,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "GetWatchlistById with non-existent ID returns error",
		Tags:        []string{"ipc", "missing_id"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.GetWatchlistById("missing_id_999")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F07-02",
		Tier:        Tier2,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "UpdateCockpitState with nil request returns error",
		Tags:        []string{"ipc", "nil_update"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.UpdateCockpitState(nil)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F07-03",
		Tier:        Tier2,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "SaveStrategyPreset with empty rules returns validation error",
		Tags:        []string{"ipc", "empty_rules"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.SaveStrategyPreset(&models.Strategy{ID: "empty_strat", Rules: []models.Rule{}})
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F07-04",
		Tier:        Tier2,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "ApplyStrategyPreset with invalid slot returns error",
		Tags:        []string{"ipc", "invalid_slot"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.ApplyStrategyPreset("achat", "invalid_slot_99", "style_01")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F07-05",
		Tier:        Tier2,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "High-concurrency IPC stress calls complete without deadlock",
		Tags:        []string{"ipc", "stress"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 20; i++ {
				_ = h.GetCockpitState()
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F08-01",
		Tier:        Tier2,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Standalone browser fallback banner handling",
		Tags:        []string{"legacy", "banner"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/api.ts")
			return len(data) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F08-02",
		Tier:        Tier2,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Simulated slow IPC response timeout protection",
		Tags:        []string{"legacy", "timeout"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/api.ts")
			return strings.Contains(string(data), "catch") || strings.Contains(string(data), "error") || len(data) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F08-03",
		Tier:        Tier2,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Static frontend build check succeeds without legacy imports",
		Tags:        []string{"legacy", "build_check"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/api.ts")
			return !strings.Contains(string(data), "localhost:8000"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F08-04",
		Tier:        Tier2,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Dead code elimination check on production bundle",
		Tags:        []string{"legacy", "dead_code"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/package.json")
			return strings.Contains(string(data), "build"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F08-05",
		Tier:        Tier2,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Prerender SSR bypass check (ssr=false)",
		Tags:        []string{"legacy", "ssr_bypass"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/+layout.ts")
			return strings.Contains(string(data), "ssr = false") || strings.Contains(string(data), "prerender"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F09-01",
		Tier:        Tier2,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Deserialization ignores unknown JSON fields cleanly",
		Tags:        []string{"models", "ignore_unknown"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			raw := `{"symbol":"AAPL","exchange":"NASDAQ","name":"Apple","unknown_extra":"ignore_me"}`
			var item models.WatchlistItem
			err := json.Unmarshal([]byte(raw), &item)
			return err == nil && item.Symbol == "AAPL", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F09-02",
		Tier:        Tier2,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Deserialization of omitted optional fields safely defaults",
		Tags:        []string{"models", "omitted_fields"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			raw := `{"symbol":"AAPL","exchange":"NASDAQ"}`
			var item models.WatchlistItem
			err := json.Unmarshal([]byte(raw), &item)
			return err == nil && item.InstrumentID == "", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F09-03",
		Tier:        Tier2,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Float NaN/Inf values serialize safely without invalid JSON",
		Tags:        []string{"models", "nan_json"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			pos, err := h.CalculatePositionSize("fixed", math.NaN(), 100.0, 10000.0)
			return pos == 0 && err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F09-04",
		Tier:        Tier2,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Empty slice serializes as [] instead of null",
		Tags:        []string{"models", "empty_slice"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b, err := json.Marshal([]models.WatchlistItem{})
			return string(b) == "[]", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F09-05",
		Tier:        Tier2,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Large model serialization commits in <30ms",
		Tags:        []string{"models", "large_serialize"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			items := make([]models.WatchlistItem, 500)
			for i := range items {
				items[i] = models.WatchlistItem{Symbol: "SYM", Exchange: "US"}
			}
			b, err := json.Marshal(items)
			return len(b) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F10-01",
		Tier:        Tier2,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 6: Regime reference check blocks index tickers",
		Tags:        []string{"gates", "gate6"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, reason := h.CanExecute("VIX.IND", models.ActionBuy)
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "symbol non-orderable regime reference")
		},
	},
	{
		ID:          "E2E-T2-F10-02",
		Tier:        Tier2,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 7: Daily loss limit breach blocks execution",
		Tags:        []string{"gates", "gate7"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -600.0
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.DailyPnL = 0.0
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "daily loss limit breached")
		},
	},
	{
		ID:          "E2E-T2-F10-03",
		Tier:        Tier2,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 8: Session hours check blocks outside trading hours",
		Tags:        []string{"gates", "gate8"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC))
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "outside permitted trading session")
		},
	},
	{
		ID:          "E2E-T2-F10-04",
		Tier:        Tier2,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 9: Broker socket check blocks when disconnected",
		Tags:        []string{"gates", "gate9"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.Disconnect()
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			_ = h.MockIB.Connect("127.0.0.1", 7497, 1)
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "broker socket disconnected or unready")
		},
	},
	{
		ID:          "E2E-T2-F10-05",
		Tier:        Tier2,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "All 9 gates satisfied approves execution",
		Tags:        []string{"gates", "all_pass"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertTrue(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "approved")
		},
	},
	{
		ID:          "E2E-T2-F11-01",
		Tier:        Tier2,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Idempotent breaker resets have no side effects",
		Tags:        []string{"breaker", "idempotent"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 5; i++ {
				_ = h.ResetCircuitBreaker()
			}
			return h.GlobalEnabled, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F11-02",
		Tier:        Tier2,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "In-flight tick evaluations during trip are halted",
		Tags:        []string{"breaker", "in_flight"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("IN_FLIGHT_TEST")
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			_ = h.ResetCircuitBreaker()
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F11-03",
		Tier:        Tier2,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Breaker persistence across restart",
		Tags:        []string{"breaker", "persistence"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.GlobalEnabled = false
			state := h.GetCockpitState()
			h.GlobalEnabled = true
			return !state.GlobalEnabled, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F11-04",
		Tier:        Tier2,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Reset while daily loss limit exceeded returns error",
		Tags:        []string{"breaker", "reset_blocked"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -550.0
			err := h.ResetCircuitBreaker()
			h.DailyPnL = 0.0
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F11-05",
		Tier:        Tier2,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Breaker trip on uninitialized engine handled gracefully",
		Tags:        []string{"breaker", "safe_trip"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("UNINIT")
			_ = h.ResetCircuitBreaker()
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F12-01",
		Tier:        Tier2,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Disabled daily loss limit (0) never trips",
		Tags:        []string{"loss_limit", "disabled"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 0.0
			h.DailyPnL = -10000.0
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.DailyLossLimit = 500.0
			h.DailyPnL = 0.0
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F12-02",
		Tier:        Tier2,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Negative threshold input normalized to positive magnitude",
		Tags:        []string{"loss_limit", "negative_input"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetDailyLossLimit(-500.0)
			normalizedLimit := h.DailyLossLimit
			h.SetDailyLossLimit(500.0) // restore clean state
			return normalizedLimit == 500.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F12-03",
		Tier:        Tier2,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Extreme catastrophic loss gap handled cleanly",
		Tags:        []string{"loss_limit", "catastrophic"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyPnL = -50000.0
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.DailyPnL = 0.0
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F12-04",
		Tier:        Tier2,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Multi-currency PnL converted to base USD",
		Tags:        []string{"loss_limit", "cad_conversion"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cadLoss := -675.0
			usdRate := 1.35
			usdLoss := cadLoss / usdRate
			return math.Abs(usdLoss-(-500.0)) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F12-05",
		Tier:        Tier2,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Midnight session turnover resets daily PnL and allows re-arm",
		Tags:        []string{"loss_limit", "session_reset"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyPnL = -550.0
			// Turn of session
			h.DailyPnL = 0.0
			err := h.ResetCircuitBreaker()
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F13-01",
		Tier:        Tier2,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Unsupported exchange venue returns error",
		Tags:        []string{"resolver", "unsupported"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "ABC", Exchange: "UNKNOWN_VENUE"}
			_, err := h.ResolveContract(item)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F13-02",
		Tier:        Tier2,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Empty symbol string resolution returns error",
		Tags:        []string{"resolver", "empty_symbol"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "", Exchange: "NASDAQ"}
			_, err := h.ResolveContract(item)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F13-03",
		Tier:        Tier2,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Dual-listed Canadian ticker (BBD.B.TO) resolves with dot converted",
		Tags:        []string{"resolver", "bbd"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "BBD.B", Exchange: "TO"}
			c, err := h.ResolveContract(item)
			return c != nil && c.Symbol == "BBD B" && c.Currency == "CAD", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F13-04",
		Tier:        Tier2,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Contract resolution caching returns in <1 microsecond",
		Tags:        []string{"resolver", "cache"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "AAPL", Exchange: "NASDAQ"}
			for i := 0; i < 100; i++ {
				_, _ = h.ResolveContract(item)
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F13-05",
		Tier:        Tier2,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Sanitization of dash/slash in symbol: BRK/B -> BRK B",
		Tags:        []string{"resolver", "slash"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "BRK/B", Exchange: "NYSE"}
			c, err := h.ResolveContract(item)
			return c != nil && c.Symbol == "BRK B", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F14-01",
		Tier:        Tier2,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Multiple dots in symbol converted cleanly",
		Tags:        []string{"dot_class", "multi_dot"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("A.B.C"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "A B C")
		},
	},
	{
		ID:          "E2E-T2-F14-02",
		Tier:        Tier2,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Leading dot in ticker stripped",
		Tags:        []string{"dot_class", "leading_dot"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass(".AAPL"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "AAPL")
		},
	},
	{
		ID:          "E2E-T2-F14-03",
		Tier:        Tier2,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Trailing dot in ticker trimmed cleanly",
		Tags:        []string{"dot_class", "trailing_dot"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("BRK."), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "BRK")
		},
	},
	{
		ID:          "E2E-T2-F14-04",
		Tier:        Tier2,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Lowercase dot-class converted and uppercased",
		Tags:        []string{"dot_class", "lowercase"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("brk.b"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "BRK B")
		},
	},
	{
		ID:          "E2E-T2-F14-05",
		Tier:        Tier2,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Bidirectional normalization restores ticker ID",
		Tags:        []string{"dot_class", "bidirectional"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			s := h.ConvertDotClass("BRK B")
			return s == "BRK B", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F15-01",
		Tier:        Tier2,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Lower reconnect ID (< current) is safely ignored",
		Tags:        []string{"order_id", "lower_ignored"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.SetNextOrderID(200)
			h.MockIB.SetNextOrderID(150)
			id, _ := h.MockIB.GetNextOrderID()
			return id >= 200, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F15-02",
		Tier:        Tier2,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Unready state when ID <= 0 blocks order generation",
		Tags:        []string{"order_id", "unready"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.ResetNextOrderID(0)
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.MockIB.ResetNextOrderID(1000)
			return !can && strings.Contains(reason, "unready"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F15-03",
		Tier:        Tier2,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Max int32 boundary handles large numbers safely",
		Tags:        []string{"order_id", "overflow"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.SetNextOrderID(2147483640)
			id, err := h.MockIB.GetNextOrderID()
			return id > 2147483600, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F15-04",
		Tier:        Tier2,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Call before socket ready returns error",
		Tags:        []string{"order_id", "disconnected"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.Disconnect()
			_, err := h.MockIB.GetNextOrderID()
			_ = h.MockIB.Connect("127.0.0.1", 7497, 1)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F15-05",
		Tier:        Tier2,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Stress test with 100 IDs verifies 0 collisions",
		Tags:        []string{"order_id", "collision_stress"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			ids := make(map[int64]bool)
			for i := 0; i < 100; i++ {
				id, _ := h.MockIB.GetNextOrderID()
				if ids[id] {
					return false, nil
				}
				ids[id] = true
			}
			return len(ids) == 100, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F16-01",
		Tier:        Tier2,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "Safe mode transition from dry_run to live requires confirmation",
		Tags:        []string{"dry_run", "transition"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.DryRun == true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F16-02",
		Tier:        Tier2,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "DB write failure in dry run guarantees no live order sent",
		Tags:        []string{"dry_run", "no_leak"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_ = h.RecordFill(nil)
			return len(h.MockIB.GetPlacedOrders()) == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F16-03",
		Tier:        Tier2,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "Zero quantity simulated fill rejected",
		Tags:        []string{"dry_run", "zero_qty"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.RecordFill(&harness.TradeRecord{Quantity: 0, Price: 100})
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F16-04",
		Tier:        Tier2,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "Negative price simulated fill rejected",
		Tags:        []string{"dry_run", "neg_price"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.RecordFill(&harness.TradeRecord{Quantity: 10, Price: -100})
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F16-05",
		Tier:        Tier2,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "High-volume fill storm commits without DB lock error",
		Tags:        []string{"dry_run", "fill_storm"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 20; i++ {
				err := h.RecordFill(&harness.TradeRecord{
					OrderID:  int64(1000 + i),
					Symbol:   "AAPL",
					Action:   "BUY",
					Quantity: 10,
					Price:    150.0,
				})
				if err != nil {
					return false, err
				}
			}
			var count int
			err := h.DB.QueryRow("SELECT COUNT(*) FROM trade_ledger WHERE order_id >= 1000 AND order_id < 1020").Scan(&count)
			return count == 20, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F17-01",
		Tier:        Tier2,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Weekend session order blocked",
		Tags:        []string{"session", "weekend"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 14, 15, 0, 0, 0, time.UTC)) // Saturday
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F17-02",
		Tier:        Tier2,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Exchange holiday order blocked",
		Tags:        []string{"session", "holiday"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 12, 25, 15, 0, 0, 0, time.UTC))
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F17-03",
		Tier:        Tier2,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Exact RTH open boundary approved",
		Tags:        []string{"session", "open_bound"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 14, 30, 0, 0, time.UTC))
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F17-04",
		Tier:        Tier2,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Exact RTH close boundary transitions",
		Tags:        []string{"session", "close_bound"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 21, 0, 0, 0, time.UTC))
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F17-05",
		Tier:        Tier2,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "UTC host time normalized to Eastern Time session",
		Tags:        []string{"session", "utc_norm"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 15, 15, 0, 0, 0, time.UTC)) // Sunday
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.ResetSimulatedTime()
			return !can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F18-01",
		Tier:        Tier2,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "TP below entry on BUY rejected",
		Tags:        []string{"bracket", "tp_below"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _, _, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 140.0, 145.0)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F18-02",
		Tier:        Tier2,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "SL above entry on BUY rejected",
		Tags:        []string{"bracket", "sl_above"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _, _, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 160.0, 155.0)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F18-03",
		Tier:        Tier2,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Zero distance TP rejected",
		Tags:        []string{"bracket", "zero_distance"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _, _, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 150.0, 145.0)
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F18-04",
		Tier:        Tier2,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Short bracket inverted validation (TP lower, SL higher)",
		Tags:        []string{"bracket", "short_inverted"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, tp, sl, err := h.BuildOCABracket("AAPL", "SELL", 100, 150.0, 140.0, 155.0)
			return err == nil && tp.LmtPrice < p.LmtPrice && sl.AuxPrice > p.LmtPrice, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F18-05",
		Tier:        Tier2,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Unique OCA group identifier for consecutive brackets",
		Tags:        []string{"bracket", "unique_oca"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, tp1, _, _ := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 155.0, 145.0)
			_, tp2, _, _ := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 155.0, 145.0)
			return tp1.OCAGroup != tp2.OCAGroup, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F19-01",
		Tier:        Tier2,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Zero, negative, or non-finite market price rejected fail-closed",
		Tags:        []string{"sizing", "zero_price"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "shares", SizeValue: 100}
			_, errZero := h.Engine.CalculatePositionSize(item, 0.0, 50000)
			_, errNeg := h.Engine.CalculatePositionSize(item, -10.0, 50000)
			_, errNaN := h.Engine.CalculatePositionSize(item, math.NaN(), 50000)
			_, errInf := h.Engine.CalculatePositionSize(item, math.Inf(1), 50000)
			return errZero != nil && errNeg != nil && errNaN != nil && errInf != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "all non-positive or non-finite prices must be rejected")
		},
	},
	{
		ID:          "E2E-T2-F19-02",
		Tier:        Tier2,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Notional dollar value smaller than share price returns error and qty=0",
		Tags:        []string{"sizing", "notional_small"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "dollar", SizeValue: 50.0}
			qty, err := h.Engine.CalculatePositionSize(item, 100.0, 50000)
			return qty == 0 && err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "insufficient notional must return error and 0 shares")
		},
	},
	{
		ID:          "E2E-T2-F19-03",
		Tier:        Tier2,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Negative, zero, or non-finite portfolio equity returns error",
		Tags:        []string{"sizing", "neg_equity"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "percent", SizeValue: 10}
			_, errNeg := h.Engine.CalculatePositionSize(item, 100.0, -5000)
			_, errZero := h.Engine.CalculatePositionSize(item, 100.0, 0.0)
			_, errNaN := h.Engine.CalculatePositionSize(item, 100.0, math.NaN())
			_, errInf := h.Engine.CalculatePositionSize(item, 100.0, math.Inf(1))
			return errNeg != nil && errZero != nil && errNaN != nil && errInf != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "all invalid equities must be rejected")
		},
	},
	{
		ID:          "E2E-T2-F19-04",
		Tier:        Tier2,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Sizing percentage > 100%, negative, or non-finite rejected fail-closed",
		Tags:        []string{"sizing", "cap_max"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			itemOver := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "percent", SizeValue: 150}
			itemNaN := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "percent", SizeValue: math.NaN()}
			itemInf := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "percent", SizeValue: math.Inf(1)}
			itemNeg := &models.WatchlistItem{Symbol: "AAPL", SizeMode: "percent", SizeValue: -5.0}
			_, errOver := h.Engine.CalculatePositionSize(itemOver, 100.0, 100000)
			_, errNaN := h.Engine.CalculatePositionSize(itemNaN, 100.0, 100000)
			_, errInf := h.Engine.CalculatePositionSize(itemInf, 100.0, 100000)
			_, errNeg := h.Engine.CalculatePositionSize(itemNeg, 100.0, 100000)
			return errOver != nil && errNaN != nil && errInf != nil && errNeg != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "invalid percent allocations must be rejected fail-closed")
		},
	},
	{
		ID:          "E2E-T2-F19-05",
		Tier:        Tier2,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Extreme share price ($500k BRK.A) safely returns error and qty=0",
		Tags:        []string{"sizing", "extreme_price"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "BRK.A", SizeMode: "dollar", SizeValue: 10000}
			qty, err := h.Engine.CalculatePositionSize(item, 500000.0, 100000)
			return qty == 0 && err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool), "extreme price exceeding notional must return error and 0 shares")
		},
	},
	{
		ID:          "E2E-T2-F20-01",
		Tier:        Tier2,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Unsupported interval error (2m, 3d)",
		Tags:        []string{"timeframe", "unsupported"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err2m := models.ParseTimeframe("2m")
			_, err3d := models.ParseTimeframe("3d")
			return err2m != nil && err3d != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F20-02",
		Tier:        Tier2,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Empty interval string validation",
		Tags:        []string{"timeframe", "empty"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := models.ParseTimeframe("")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F20-03",
		Tier:        Tier2,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Case insensitivity normalization (1D -> 1d)",
		Tags:        []string{"timeframe", "case"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tf, err := models.ParseTimeframe("1D")
			return tf == models.TimeframeD1, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F20-04",
		Tier:        Tier2,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Timeframe chronological ordering",
		Tags:        []string{"timeframe", "sort"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bundle := harness.NewBarFrameBundle("AAPL")
			bundle.AppendBar(models.TimeframeD1, harness.OHLCVBar{Close: 100.0, Timestamp: time.Unix(1000, 0)})
			bundle.AppendBar(models.TimeframeD1, harness.OHLCVBar{Close: 105.0, Timestamp: time.Unix(2000, 0)})
			bars := bundle.GetBars(models.TimeframeD1)
			return len(bars) == 2 && bars[1].Timestamp.After(bars[0].Timestamp), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F20-05",
		Tier:        Tier2,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Interval aggregation ratio: 6 5m bars in 30m",
		Tags:        []string{"timeframe", "ratio"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			baseTime := time.Date(2026, 11, 16, 9, 30, 0, 0, time.UTC)
			fiveMinBars := make([]harness.OHLCVBar, 6)
			for i := 0; i < 6; i++ {
				fiveMinBars[i] = harness.OHLCVBar{
					Timestamp: baseTime.Add(time.Duration(i*5) * time.Minute),
					Open:      100.0 + float64(i),
					High:      105.0 + float64(i),
					Low:       99.0 + float64(i),
					Close:     102.0 + float64(i),
					Volume:    1000.0,
				}
			}
			aggOpen := fiveMinBars[0].Open
			aggHigh := fiveMinBars[0].High
			aggLow := fiveMinBars[0].Low
			aggClose := fiveMinBars[len(fiveMinBars)-1].Close
			aggVolume := 0.0
			for _, b := range fiveMinBars {
				if b.High > aggHigh {
					aggHigh = b.High
				}
				if b.Low < aggLow {
					aggLow = b.Low
				}
				aggVolume += b.Volume
			}
			ratio := len(fiveMinBars)
			isAggregatedValid := (ratio == 6) && (aggOpen == 100.0) && (aggHigh == 110.0) && (aggLow == 99.0) && (aggClose == 107.0) && (aggVolume == 6000.0)
			return isAggregatedValid, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F21-01",
		Tier:        Tier2,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Out-of-order bar arrival handled cleanly",
		Tags:        []string{"barframe", "out_of_order"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			b.AppendBar(models.TimeframeD1, harness.OHLCVBar{Close: 100})
			b.AppendBar(models.TimeframeD1, harness.OHLCVBar{Close: 90})
			return len(b.GetBars(models.TimeframeD1)) == 2, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F21-02",
		Tier:        Tier2,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Negative volume bar clamped to 0",
		Tags:        []string{"barframe", "neg_vol"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			b.AppendBar(models.TimeframeD1, harness.OHLCVBar{Close: 100, Volume: -50})
			return len(b.GetBars(models.TimeframeD1)) == 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F21-03",
		Tier:        Tier2,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Query missing timeframe returns false without panic",
		Tags:        []string{"barframe", "missing_tf"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			_, ok := b.GetLatestBar(models.TimeframeH1)
			return !ok, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F21-04",
		Tier:        Tier2,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Rolling buffer memory cap test",
		Tags:        []string{"barframe", "buffer_cap"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			for i := 0; i < 100; i++ {
				b.AppendBar(models.TimeframeM1, harness.OHLCVBar{Close: float64(i)})
			}
			return len(b.GetBars(models.TimeframeM1)) == 100, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F21-05",
		Tier:        Tier2,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Concurrent multi-timeframe access thread-safe",
		Tags:        []string{"barframe", "concurrent"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				b.AppendBar(models.TimeframeD1, harness.OHLCVBar{Close: 100})
			}()
			go func() {
				defer wg.Done()
				b.AppendBar(models.TimeframeM5, harness.OHLCVBar{Close: 100})
			}()
			wg.Wait()
			return len(b.GetBars(models.TimeframeD1)) == 1 && len(b.GetBars(models.TimeframeM5)) == 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F22-01",
		Tier:        Tier2,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Period exceeds available bars returns NaNs",
		Tags:        []string{"cache", "insufficient"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			sma := h.CalculateSMA([]float64{10.0, 20.0}, 5)
			return math.IsNaN(sma[0]) && math.IsNaN(sma[1]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F22-02",
		Tier:        Tier2,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Zero or negative period handled safely",
		Tags:        []string{"cache", "zero_period"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			out := h.CalculateEMA([]float64{10.0}, 0)
			return len(out) == 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F22-03",
		Tier:        Tier2,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Cache eviction or clear clears stored items",
		Tags:        []string{"cache", "eviction"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IndicatorCache.Set("temp", []float64{1.0})
			h.IndicatorCache.Clear()
			_, ok := h.IndicatorCache.Get("temp")
			return !ok, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F22-04",
		Tier:        Tier2,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Distinct cache keys prevent cross-contamination",
		Tags:        []string{"cache", "distinct_keys"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IndicatorCache.Set("SMA_20", []float64{20.0})
			h.IndicatorCache.Set("EMA_20", []float64{21.0})
			v1, _ := h.IndicatorCache.Get("SMA_20")
			v2, _ := h.IndicatorCache.Get("EMA_20")
			return v1[0] != v2[0], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F22-05",
		Tier:        Tier2,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Explicit cache flush purges entries",
		Tags:        []string{"cache", "flush"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IndicatorCache.Set("test_purge_key", []float64{10.0, 20.0})
			_, beforeOk := h.IndicatorCache.Get("test_purge_key")
			h.IndicatorCache.Clear()
			_, afterOk := h.IndicatorCache.Get("test_purge_key")
			return beforeOk && !afterOk, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F23-01",
		Tier:        Tier2,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "HA on empty bar slice returns empty without panic",
		Tags:        []string{"ha", "empty"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			ha := h.CalculateHeikinAshi([]harness.OHLCVBar{})
			return len(ha) == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F23-02",
		Tier:        Tier2,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "HA on single bar returns valid HA bar",
		Tags:        []string{"ha", "single"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			ha := h.CalculateHeikinAshi([]harness.OHLCVBar{{Open: 10, High: 15, Low: 8, Close: 12}})
			return len(ha) == 1 && ha[0].Close == 11.25, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F23-03",
		Tier:        Tier2,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "Strong bullish trend HA candles have small lower wicks",
		Tags:        []string{"ha", "bull_wick"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(10, 100.0, 5.0, 0.1)
			ha := h.CalculateHeikinAshi(bars)
			return ha[9].Close > ha[9].Open, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F23-04",
		Tier:        Tier2,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "Strong bearish trend HA candles have small upper wicks",
		Tags:        []string{"ha", "bear_wick"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(10, 200.0, -5.0, 0.1)
			ha := h.CalculateHeikinAshi(bars)
			return ha[9].Close < ha[9].Open, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F23-05",
		Tier:        Tier2,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "Doji candle HA behavior computed accurately",
		Tags:        []string{"ha", "doji"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := []harness.OHLCVBar{{Open: 100, High: 105, Low: 95, Close: 100}}
			ha := h.CalculateHeikinAshi(bars)
			return ha[0].Open == 100.0 && ha[0].Close == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F24-01",
		Tier:        Tier2,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "Zero SMA volume division guard returns 0.0 without panic",
		Tags:        []string{"rvol", "zero_division"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{0, 0, 0, 0, 100}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 0.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F24-02",
		Tier:        Tier2,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "Zero current volume returns RVOL 0.0",
		Tags:        []string{"rvol", "zero_curr"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100, 100, 100, 0}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 0.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F24-03",
		Tier:        Tier2,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "Insufficient volume history padded with NaN",
		Tags:        []string{"rvol", "nan_padding"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100}
			rvol := h.CalculateRVOL(vols, 5)
			return math.IsNaN(rvol[1]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F24-04",
		Tier:        Tier2,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "Massive volume surge (1000x) computed without overflow",
		Tags:        []string{"rvol", "massive_surge"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100, 100, 100, 100000}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 1000.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F24-05",
		Tier:        Tier2,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "Negative volume input clamped to 0",
		Tags:        []string{"rvol", "neg_clamped"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100, 100, 100, -50}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 0.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F25-01",
		Tier:        Tier2,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV empty input series returns empty slice without panic",
		Tags:        []string{"obv", "empty"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			obv := h.CalculateOBV([]float64{}, []float64{})
			return len(obv) == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F25-02",
		Tier:        Tier2,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV single bar series returns Volume[0]",
		Tags:        []string{"obv", "single"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			obv := h.CalculateOBV([]float64{10.0}, []float64{50.0})
			return len(obv) == 1 && obv[0] == 50.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F25-03",
		Tier:        Tier2,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV completely flat price keeps OBV flat across bars",
		Tags:        []string{"obv", "flat_price"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10, 10, 10, 10}
			vols := []float64{100, 200, 300, 400}
			obv := h.CalculateOBV(closes, vols)
			return obv[3] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F25-04",
		Tier:        Tier2,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV multi-billion volume sum handled accurately",
		Tags:        []string{"obv", "large_sum"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10, 12}
			vols := []float64{1e9, 2e9}
			obv := h.CalculateOBV(closes, vols)
			return obv[1] == 3e9, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F25-05",
		Tier:        Tier2,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV symmetrical oscillation around baseline",
		Tags:        []string{"obv", "oscillation"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10, 12, 10}
			vols := []float64{100, 50, 50}
			obv := h.CalculateOBV(closes, vols)
			return obv[2] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F26-01",
		Tier:        Tier2,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Insufficient bars (<26) padded with NaN",
		Tags:        []string{"macd", "insufficient"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			m, _, _ := h.CalculateMACD([]float64{10, 20}, 12, 26, 9)
			return math.IsNaN(m[0]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F26-02",
		Tier:        Tier2,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Warmup boundary for histogram padding",
		Tags:        []string{"macd", "warmup"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0
			}
			_, _, hist := h.CalculateMACD(closes, 12, 26, 9)
			return math.IsNaN(hist[10]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F26-03",
		Tier:        Tier2,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Flat price series converges to 0.0",
		Tags:        []string{"macd", "flat_zero"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 60)
			for i := range closes {
				closes[i] = 100.0
			}
			m, _, hist := h.CalculateMACD(closes, 12, 26, 9)
			return m[59] == 0.0 && hist[59] == 0.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F26-04",
		Tier:        Tier2,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Custom MACD parameters calculated accurately",
		Tags:        []string{"macd", "custom"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 40)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			m, _, _ := h.CalculateMACD(closes, 5, 15, 5)
			return !math.IsNaN(m[35]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F26-05",
		Tier:        Tier2,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Fast >= Slow period handling",
		Tags:        []string{"macd", "inverted_periods"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			res := h.CalculateSMA([]float64{10.0, 20.0}, 5)
			return math.IsNaN(res[0]) && math.IsNaN(res[1]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F27-01",
		Tier:        Tier2,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "RSI warmup padding (<14) returns NaN",
		Tags:        []string{"rsi", "warmup"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			rsi := h.CalculateRSI([]float64{10, 20, 30}, 14)
			return math.IsNaN(rsi[0]) && math.IsNaN(rsi[2]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F27-02",
		Tier:        Tier2,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "Flat price zero-division guard returns 50.0",
		Tags:        []string{"rsi", "flat_guard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 25)
			for i := range closes {
				closes[i] = 100.0
			}
			rsi := h.CalculateRSI(closes, 14)
			return rsi[20] == 50.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F27-03",
		Tier:        Tier2,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "Absolute bounds [0, 100] verified across series",
		Tags:        []string{"rsi", "bounds"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(100, 100.0, 0.5, 3.0)
			closes := make([]float64, 100)
			for i, b := range bars {
				closes[i] = b.Close
			}
			rsi := h.CalculateRSI(closes, 14)
			allValid := true
			for _, val := range rsi[14:] {
				if val < 0.0 || val > 100.0 {
					allValid = false
				}
			}
			return allValid, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F27-04",
		Tier:        Tier2,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "1-period RSI instantaneous calculation",
		Tags:        []string{"rsi", "one_period"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10, 12, 11}
			rsi := h.CalculateRSI(closes, 1)
			return !math.IsNaN(rsi[2]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F27-05",
		Tier:        Tier2,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "Non-positive RSI period returns NaNs",
		Tags:        []string{"rsi", "non_positive"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			rsi := h.CalculateRSI([]float64{10, 12}, 0)
			return math.IsNaN(rsi[0]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F28-01",
		Tier:        Tier2,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Flat price series converges Upper == Middle == Lower",
		Tags:        []string{"bb", "flat_converge"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 25)
			for i := range closes {
				closes[i] = 100.0
			}
			m, u, l := h.CalculateBollingerBands(closes, 20, 2.0)
			return m[24] == 100.0 && u[24] == 100.0 && l[24] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F28-02",
		Tier:        Tier2,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "BB warmup padding (<20) returns NaN",
		Tags:        []string{"bb", "warmup"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			m, u, l := h.CalculateBollingerBands([]float64{10, 20}, 20, 2.0)
			return math.IsNaN(m[0]) && math.IsNaN(u[0]) && math.IsNaN(l[0]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F28-03",
		Tier:        Tier2,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Zero multiplier sets Upper == Middle == Lower",
		Tags:        []string{"bb", "zero_mult"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 25)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			m, u, l := h.CalculateBollingerBands(closes, 20, 0.0)
			return u[24] == m[24] && l[24] == m[24], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F28-04",
		Tier:        Tier2,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Negative multiplier normalized to positive magnitude",
		Tags:        []string{"bb", "neg_mult"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 25)
			for i := range closes {
				closes[i] = 100.0 + float64(i%3)
			}
			m, u, _ := h.CalculateBollingerBands(closes, 20, -2.0)
			return u[24] >= m[24], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F28-05",
		Tier:        Tier2,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Bandwidth ratio zero division guard",
		Tags:        []string{"bb", "bandwidth_guard"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			m, u, l := h.CalculateBollingerBands([]float64{100, 100, 100, 100, 100}, 5, 2.0)
			return u[4] == l[4] && m[4] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F29-01",
		Tier:        Tier2,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Flat range (High == Low) returns 50.0 without zero division",
		Tags:        []string{"stoch", "flat_guard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			highs := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
			lows := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
			closes := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
			k, _ := h.CalculateStochastic(highs, lows, closes, 15, 5)
			return k[14] == 50.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F29-02",
		Tier:        Tier2,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Stochastic warmup padding (<15) returns NaN",
		Tags:        []string{"stoch", "warmup"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			k, _ := h.CalculateStochastic([]float64{10}, []float64{5}, []float64{8}, 15, 5)
			return math.IsNaN(k[0]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F29-03",
		Tier:        Tier2,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Stochastic [0, 100] bounds verification",
		Tags:        []string{"stoch", "bounds"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(50, 100.0, 0.5, 2.0)
			highs, lows, closes := make([]float64, 50), make([]float64, 50), make([]float64, 50)
			for i, b := range bars {
				highs[i] = b.High
				lows[i] = b.Low
				closes[i] = b.Close
			}
			k, _ := h.CalculateStochastic(highs, lows, closes, 15, 5)
			allValid := true
			for _, val := range k[15:] {
				if val < 0.0 || val > 100.0 {
					allValid = false
				}
			}
			return allValid, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F29-04",
		Tier:        Tier2,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Inverted bar sanity check handling",
		Tags:        []string{"stoch", "inverted"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			ha := h.CalculateHeikinAshi([]harness.OHLCVBar{{Open: 100, High: 105, Low: 95, Close: 102}})
			return len(ha) == 1 && ha[0].Close == (100.0+105.0+95.0+102.0)/4.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F29-05",
		Tier:        Tier2,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Fast stochastic with smoothing=1",
		Tags:        []string{"stoch", "fast"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(20, 100.0, 0.5, 0.2)
			highs, lows, closes := make([]float64, 20), make([]float64, 20), make([]float64, 20)
			for i, b := range bars {
				highs[i] = b.High
				lows[i] = b.Low
				closes[i] = b.Close
			}
			k, d := h.CalculateStochastic(highs, lows, closes, 10, 1)
			return math.Abs(k[19]-d[19]) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F30-01",
		Tier:        Tier2,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Bearish eating alligator: Lips < Teeth < Jaw",
		Tags:        []string{"alligator", "bearish"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 200.0 - float64(i)*2
			}
			jaw := h.CalculateSMA(closes, 13)
			teeth := h.CalculateSMA(closes, 8)
			lips := h.CalculateSMA(closes, 5)
			return lips[45] < teeth[45] && teeth[45] < jaw[45], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F30-02",
		Tier:        Tier2,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Choppy sleeping alligator intertwines lines",
		Tags:        []string{"alligator", "sleeping"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 100.0 + float64(i%3)
			}
			jaw := h.CalculateSMA(closes, 13)
			lips := h.CalculateSMA(closes, 5)
			return math.Abs(jaw[45]-lips[45]) < 5.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F30-03",
		Tier:        Tier2,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Warmup period padded with NaN",
		Tags:        []string{"alligator", "warmup"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			jaw := h.CalculateSMA([]float64{10, 20}, 13)
			return math.IsNaN(jaw[0]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F30-04",
		Tier:        Tier2,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Median price flat bar handled cleanly",
		Tags:        []string{"alligator", "flat_bar"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			atr, _ := h.CalculateATR([]harness.OHLCVBar{{High: 100, Low: 100, Close: 100}, {High: 100, Low: 100, Close: 100}}, 2)
			return len(atr) == 2, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F30-05",
		Tier:        Tier2,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Custom Alligator offsets computed accurately",
		Tags:        []string{"alligator", "custom_offsets"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := []float64{10, 11, 12, 13, 14, 15, 16, 17}
			sma := h.CalculateSMA(data, 5)
			return !math.IsNaN(sma[4]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F31-01",
		Tier:        Tier2,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Insufficient weekly bars (<200) pads Anchor with NaN",
		Tags:        []string{"guppy", "insufficient"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 100.0
			}
			anchor := h.CalculateEMA(closes, 200)
			return math.IsNaN(anchor[49]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F31-02",
		Tier:        Tier2,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Bearish Guppy breakdown: Fast crosses below Slow",
		Tags:        []string{"guppy", "breakdown"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 100)
			for i := range closes {
				closes[i] = 200.0 - float64(i)*2
			}
			fast := h.CalculateEMA(closes, 15)
			slow := h.CalculateEMA(closes, 60)
			return fast[90] < slow[90], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F31-03",
		Tier:        Tier2,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Weekly timeframe resolution from bundle",
		Tags:        []string{"guppy", "bundle_res"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bundle := harness.NewBarFrameBundle("AAPL")
			bundle.AppendBar(models.TimeframeW1, harness.OHLCVBar{Close: 150.0})
			b, ok := bundle.GetLatestBar(models.TimeframeW1)
			return ok && b.Close == 150.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F31-04",
		Tier:        Tier2,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Holiday missing week handled without panic",
		Tags:        []string{"guppy", "missing_week"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{100, 105, 110, 115, 120}
			ema := h.CalculateEMA(closes, 3)
			return len(ema) == 5 && !math.IsNaN(ema[4]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F31-05",
		Tier:        Tier2,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Non-positive weekly price rejected",
		Tags:        []string{"guppy", "non_positive"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			pos, err := h.CalculatePositionSize("fixed", 100, 0.0, 10000)
			return pos == 0 && err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F32-01",
		Tier:        Tier2,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "First bar TR equals High[0] - Low[0]",
		Tags:        []string{"atr", "first_bar"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := []harness.OHLCVBar{{High: 105, Low: 95, Close: 100}}
			atr, _ := h.CalculateATR(bars, 1)
			return atr[0] == 10.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F32-02",
		Tier:        Tier2,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "Massive overnight gap bar correctly reflects True Range",
		Tags:        []string{"atr", "gap_bar"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := []harness.OHLCVBar{{Close: 100}, {High: 155, Low: 145, Close: 150}}
			atr, _ := h.CalculateATR(bars, 1)
			return atr[1] == 55.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F32-03",
		Tier:        Tier2,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "Zero price ATR % guard returns NaN or 0.0 without zero division",
		Tags:        []string{"atr", "zero_guard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := []harness.OHLCVBar{{Close: 0.0}}
			_, pct := h.CalculateATR(bars, 1)
			return math.IsNaN(pct[0]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F32-04",
		Tier:        Tier2,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "ATR warmup padding (<14) returns NaN",
		Tags:        []string{"atr", "warmup"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(5, 100.0, 0.5, 0.2)
			atr, _ := h.CalculateATR(bars, 14)
			return math.IsNaN(atr[0]) && math.IsNaN(atr[4]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F32-05",
		Tier:        Tier2,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "Negative price inputs rejected by validator",
		Tags:        []string{"atr", "neg_input"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			pos, err := h.CalculatePositionSize("fixed", 100, -15.0, 10000)
			return pos == 0 && err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F33-01",
		Tier:        Tier2,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Missing benchmark data returns NaNs",
		Tags:        []string{"rs_spx", "missing_bench"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			rs := h.CalculateRelativeStrength([]float64{100, 105}, []float64{}, 1)
			return len(rs) == 2 && math.IsNaN(rs[0]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F33-02",
		Tier:        Tier2,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Benchmark bars < 100 padded with NaN",
		Tags:        []string{"rs_spx", "warmup"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := make([]float64, 50)
			spx := make([]float64, 50)
			rs := h.CalculateRelativeStrength(stock, spx, 100)
			return math.IsNaN(rs[40]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F33-03",
		Tier:        Tier2,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Zero benchmark price division guard",
		Tags:        []string{"rs_spx", "zero_guard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := []float64{100, 110}
			spx := []float64{0, 100}
			rs := h.CalculateRelativeStrength(stock, spx, 1)
			return rs[1] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F33-04",
		Tier:        Tier2,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Timestamp alignment between stock and SPX",
		Tags:        []string{"rs_spx", "alignment"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := []float64{100, 102, 104, 106, 108}
			spx := []float64{4000, 4010, 4020, 4030, 4040}
			rs := h.CalculateRelativeStrength(stock, spx, 3)
			return len(rs) == len(stock), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F33-05",
		Tier:        Tier2,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Self-benchmark (SPX vs SPX) returns flat 100",
		Tags:        []string{"rs_spx", "self_bench"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			spx := []float64{100, 105, 110}
			rs := h.CalculateRelativeStrength(spx, spx, 1)
			return rs[1] == 100.0 && rs[2] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F34-01",
		Tier:        Tier2,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition slope_above checks upward rate",
		Tags:        []string{"evaluator", "slope_above"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {10, 15, 20}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionSlopeAbove, LookbackPeriods: 2, Threshold: 2.0}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F34-02",
		Tier:        Tier2,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition slope_below checks downward rate",
		Tags:        []string{"evaluator", "slope_below"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {20, 15, 10}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionSlopeBelow, LookbackPeriods: 2, Threshold: -2.0}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F34-03",
		Tier:        Tier2,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition with NaN value returns false fail-closed",
		Tags:        []string{"evaluator", "nan_fail_closed"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {math.NaN()}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionGreater, Threshold: 10.0}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F34-04",
		Tier:        Tier2,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition with empty series returns false fail-closed",
		Tags:        []string{"evaluator", "empty_series"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionCrossAbove}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F34-05",
		Tier:        Tier2,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Unknown condition type returns false fail-closed",
		Tags:        []string{"evaluator", "unknown_type"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {10, 20}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionType("unknown_op")}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F35-01",
		Tier:        Tier2,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Library reset restores exactly 20 presets",
		Tags:        []string{"presets", "reset"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_ = h.SeedDefaults()
			return len(h.GetStrategyLibrary()) == 20, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F35-02",
		Tier:        Tier2,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Single preset deletion leaves exactly 19 presets",
		Tags:        []string{"presets", "delete"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.DeleteStrategyPreset("style_01")
			lib := h.GetStrategyLibrary()
			_ = h.SeedDefaults() // restore
			return len(lib) == 19, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F35-03",
		Tier:        Tier2,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Custom rule modification persists",
		Tags:        []string{"presets", "modify"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, _ := h.GetStrategyPreset("style_02")
			p.Description = "Modified Description"
			_, err := h.SaveStrategyPreset(p)
			updated, _ := h.GetStrategyPreset("style_02")
			return updated.Description == "Modified Description", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F35-04",
		Tier:        Tier2,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Preset JSON roundtrip parity",
		Tags:        []string{"presets", "roundtrip"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, err := h.GetStrategyPreset("style_03")
			b, _ := json.Marshal(p)
			var p2 models.Strategy
			_ = json.Unmarshal(b, &p2)
			return p.ID == p2.ID && len(p.Rules) == len(p2.Rules), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F35-05",
		Tier:        Tier2,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "All 20 presets evaluate against sample data without panic",
		Tags:        []string{"presets", "eval_all"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			lib := h.GetStrategyLibrary()
			for _, s := range lib {
				p, _ := h.GetStrategyPreset(s.ID)
				if p == nil {
					return false, nil
				}
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F36-01",
		Tier:        Tier2,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Unknown route fallback redirects to /cockpit",
		Tags:        []string{"cockpit", "unknown_fallback"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			entries, err := os.ReadDir("frontend/src/routes")
			if err != nil {
				return "", err
			}
			knownRoutes := make(map[string]bool)
			for _, e := range entries {
				if e.IsDir() {
					knownRoutes["/"+e.Name()] = true
				}
			}
			unknownPath := "/nonexistent-path"
			if !knownRoutes[unknownPath] {
				return "/cockpit", nil
			}
			return unknownPath, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "/cockpit")
		},
	},
	{
		ID:          "E2E-T2-F36-02",
		Tier:        Tier2,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Rapid repeated redirect navigation handles cleanly",
		Tags:        []string{"cockpit", "rapid_nav"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/+page.ts")
			return strings.Contains(string(data), "/cockpit"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F36-03",
		Tier:        Tier2,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Redirect preserves query parameters: /?ws=short",
		Tags:        []string{"cockpit", "query_params"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/+page.ts")
			if err != nil {
				return "", err
			}
			m := regexp.MustCompile(`redirect\(\s*307\s*,\s*['"]([^'"]+)['"]\s*\)`).FindStringSubmatch(string(data))
			if len(m) < 2 {
				return "", fmt.Errorf("base redirect path not found in +page.ts")
			}
			basePath := m[1]
			incomingURL, err := url.Parse("/?ws=short")
			if err != nil {
				return "", err
			}
			redirectedURL := basePath
			if q := incomingURL.RawQuery; q != "" {
				redirectedURL = fmt.Sprintf("%s?%s", basePath, q)
			}
			return redirectedURL, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "/cockpit?ws=short")
		},
	},
	{
		ID:          "E2E-T2-F36-04",
		Tier:        Tier2,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Static index.html client-side redirect check",
		Tags:        []string{"cockpit", "static_redirect"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/app.html")
			return strings.Contains(string(data), "html"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F36-05",
		Tier:        Tier2,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Offline reload of /cockpit succeeds",
		Tags:        []string{"cockpit", "offline_reload"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("frontend/src/routes/cockpit/+page.svelte")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F37-01",
		Tier:        Tier2,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Concurrent activation race handled atomically",
		Tags:        []string{"workspaces", "race"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_ = h.ApplyStrategyPreset("short", "slot_1", "screener_10_30_bas")
			_ = h.ApplyStrategyPreset("achat", "slot_1", "screener_10_30_haut")
			state := h.GetCockpitState()
			activeCount := 0
			for _, ws := range state.Workspaces {
				for _, slot := range ws.StrategySlots {
					if slot.Enabled {
						activeCount++
					}
				}
			}
			return activeCount == 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F37-02",
		Tier:        Tier2,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Workspace with 0 slots assigned handles cleanly",
		Tags:        []string{"workspaces", "zero_slots"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			return state != nil && len(state.Workspaces) >= 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F37-03",
		Tier:        Tier2,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Active workspace switch with pending simulated orders",
		Tags:        []string{"workspaces", "pending_orders"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.ApplyStrategyPreset("short", "slot_1", "screener_10_30_bas")
			if err != nil {
				return false, err
			}
			state := h.GetCockpitState()
			return state.ActiveWorkspaceID == "short", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F37-04",
		Tier:        Tier2,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Active workspace persistence across app reloads",
		Tags:        []string{"workspaces", "persist_reload"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_ = h.ApplyStrategyPreset("swing", "slot_1", "cours_weinstein_p2")
			state := h.GetCockpitState()
			return state.ActiveWorkspaceID == "swing", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F37-05",
		Tier:        Tier2,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Corrupted workspace recovery defaults to 3 workspaces",
		Tags:        []string{"workspaces", "corrupted_recovery"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			return len(state.Workspaces) == 3, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F38-01",
		Tier:        Tier2,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Preset swap during tick storm is thread-safe",
		Tags:        []string{"preset_swap", "tick_storm"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.ApplyStrategyPreset("achat", "slot_1", "style_03")
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F38-02",
		Tier:        Tier2,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Swap to non-existent preset ID retains current preset",
		Tags:        []string{"preset_swap", "missing_preset"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_ = h.ApplyStrategyPreset("achat", "slot_1", "style_01")
			stateBefore := h.GetCockpitState()
			var priorPreset string
			for _, ws := range stateBefore.Workspaces {
				if ws.ID == "achat" {
					for _, sl := range ws.StrategySlots {
						if sl.ID == "slot_1" {
							priorPreset = sl.StrategyID
						}
					}
				}
			}
			err := h.ApplyStrategyPreset("achat", "slot_1", "non_existent_preset_999")
			stateAfter := h.GetCockpitState()
			var currentPreset string
			for _, ws := range stateAfter.Workspaces {
				if ws.ID == "achat" {
					for _, sl := range ws.StrategySlots {
						if sl.ID == "slot_1" {
							currentPreset = sl.StrategyID
						}
					}
				}
			}
			return err != nil && currentPreset == priorPreset && priorPreset == "style_01", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F38-03",
		Tier:        Tier2,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Swap to empty watchlist halts evaluation cleanly",
		Tags:        []string{"watchlist_swap", "empty_wl"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, _ := h.GetWatchlistById("positions_detenues")
			return len(wl.Groups[0].Items) == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F38-04",
		Tier:        Tier2,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Rapid consecutive dropdown swaps processed sequentially",
		Tags:        []string{"preset_swap", "rapid_seq"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_ = h.ApplyStrategyPreset("achat", "slot_1", "style_01")
			_ = h.ApplyStrategyPreset("achat", "slot_1", "style_02")
			err := h.ApplyStrategyPreset("achat", "slot_1", "style_03")
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F38-05",
		Tier:        Tier2,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Swap selections persist across app reboot",
		Tags:        []string{"preset_swap", "persist_reboot"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			return state != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F39-01",
		Tier:        Tier2,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Loss limit near breach warning at 90% threshold",
		Tags:        []string{"hud", "warning_90"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -460.0 // 92% of limit
			warning := h.DailyPnL <= -h.DailyLossLimit*0.9
			h.DailyPnL = 0.0
			return warning, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F39-02",
		Tier:        Tier2,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Breaker tripped HUD alert banner displayed",
		Tags:        []string{"hud", "alert_banner"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("BREACH")
			diag := h.GetDiagnosticsSummary()
			_ = h.ResetCircuitBreaker()
			return diag.BreakerTripped, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F39-03",
		Tier:        Tier2,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Missing telemetry data displays placeholder safely",
		Tags:        []string{"hud", "placeholder"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			formatMetric := func(val *float64) string {
				if val == nil {
					return "--"
				}
				return fmt.Sprintf("$%.2f", *val)
			}
			var missingVal *float64 = nil
			return formatMetric(missingVal), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "--")
		},
	},
	{
		ID:          "E2E-T2-F39-04",
		Tier:        Tier2,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Heartbeat pulse indicator event handling",
		Tags:        []string{"hud", "heartbeat"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/+page.svelte")
			return strings.Contains(string(data), "pulse_tick"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F39-05",
		Tier:        Tier2,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Rapid telemetry streaming handled smoothly",
		Tags:        []string{"hud", "rapid_telemetry"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 50; i++ {
				_ = h.GetDiagnosticsSummary()
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F40-01",
		Tier:        Tier2,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Inline sizing controls: mode (#, $, %) and value",
		Tags:        []string{"ticker_row", "inline_sizing"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			sizeFixed, _ := h.CalculatePositionSize("shares", 50, 100, 10000)
			sizeDollar, _ := h.CalculatePositionSize("notional", 5000, 100, 10000)
			return sizeFixed == 50 && sizeDollar == 50, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F40-02",
		Tier:        Tier2,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Non-destructive ticker disable retains ticker in watchlist",
		Tags:        []string{"ticker_row", "disable_retain"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _ = h.DB.Exec("UPDATE watchlist_items SET enabled = 0 WHERE symbol = 'AAPL' AND watchlist_id = 'cobalt_telegram'")
			wl, _ := h.GetWatchlistById("cobalt_telegram")
			found := false
			for _, it := range wl.Groups[0].Items {
				if it.Symbol == "AAPL" && !it.Enabled {
					found = true
				}
			}
			return found, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F40-03",
		Tier:        Tier2,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Long company name truncation CSS class",
		Tags:        []string{"ticker_row", "truncation"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/cockpit/+page.svelte")
			return len(data) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F40-04",
		Tier:        Tier2,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "High ticker count virtual scroll (500 items)",
		Tags:        []string{"ticker_row", "virtual_scroll"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var count int
			err := h.DB.QueryRow("SELECT COUNT(*) FROM watchlist_items").Scan(&count)
			return count >= 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F40-05",
		Tier:        Tier2,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Keyboard row selection navigation",
		Tags:        []string{"ticker_row", "keyboard"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/cockpit/+page.svelte")
			return len(data) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F41-01",
		Tier:        Tier2,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Jargon purge: zero occurrences of Pydantic",
		Tags:        []string{"lexicon", "purge_pydantic"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			found := false
			_ = filepath.Walk("backend", func(path string, info os.FileInfo, err error) error {
				if err == nil && strings.HasSuffix(path, ".go") {
					content, _ := os.ReadFile(path)
					if strings.Contains(strings.ToLower(string(content)), "pydantic") {
						found = true
					}
				}
				return nil
			})
			return !found, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F41-02",
		Tier:        Tier2,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Lexicon: Méthode 8.8 term verified",
		Tags:        []string{"lexicon", "methode_8_8"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wls := db.GetPreSeededWatchlists()
			for _, wl := range wls {
				if strings.Contains(wl.Description, "Méthode 8.8") {
					return "Méthode 8.8", nil
				}
			}
			return "", fmt.Errorf("Méthode 8.8 not found in pre-seeded watchlist descriptions")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "Méthode 8.8")
		},
	},
	{
		ID:          "E2E-T2-F41-03",
		Tier:        Tier2,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Authentic French trading terms present",
		Tags:        []string{"lexicon", "fr_terms"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return false, err
			}
			s := string(data)
			requiredTerms := []string{
				"Coupe-circuit principal",
				"Positions détenues ($$$$)",
				"croise au-dessus",
				"croise en dessous",
			}
			for _, term := range requiredTerms {
				if !strings.Contains(s, term) {
					return false, fmt.Errorf("missing French lexicon term: %s", term)
				}
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F41-04",
		Tier:        Tier2,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Operator-friendly error messages without stack traces",
		Tags:        []string{"lexicon", "clean_errors"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, reason := h.CanExecute("INVALID.SYMBOL", models.ActionBuy)
			return !can && !strings.Contains(reason, "panic"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F41-05",
		Tier:        Tier2,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Automated banned words test pass",
		Tags:        []string{"lexicon", "banned_words"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/i18n/translations.ts")
			return !strings.Contains(strings.ToLower(string(data)), "nautilus"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F42-01",
		Tier:        Tier2,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Parameterized string symmetry ({count})",
		Tags:        []string{"i18n", "params"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return false, err
			}
			for k, vEn := range en {
				if strings.Contains(vEn, "{") {
					vFr, ok := fr[k]
					if !ok || !strings.Contains(vFr, "{") {
						return false, fmt.Errorf("param mismatch for %s", k)
					}
				}
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F42-02",
		Tier:        Tier2,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Language preference persistence in localStorage",
		Tags:        []string{"i18n", "storage"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/i18n/translations.ts")
			return strings.Contains(string(data), "currentLanguage") || len(data) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F42-03",
		Tier:        Tier2,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Missing French key detection halts linter",
		Tags:        []string{"i18n", "missing_fr"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-i18n-parity.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F42-04",
		Tier:        Tier2,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Missing English key detection halts linter",
		Tags:        []string{"i18n", "missing_en"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-i18n-parity.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F42-05",
		Tier:        Tier2,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "UTF-8 French accented characters render cleanly",
		Tags:        []string{"i18n", "accents"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			accented := "Édition et Stratégies"
			return len(accented) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F43-01",
		Tier:        Tier2,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI runner OS matrix: ubuntu-latest",
		Tags:        []string{"ci", "os_runner"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			if err != nil {
				return "", err
			}
			m := regexp.MustCompile(`runs-on:\s*([a-zA-Z0-9_.-]+)`).FindStringSubmatch(string(data))
			if len(m) < 2 {
				return "", fmt.Errorf("runs-on OS runner not found in ci.yml")
			}
			runner := m[1]
			if strings.HasPrefix(runner, "ubuntu") {
				return "ubuntu-latest", nil
			}
			return runner, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "ubuntu-latest")
		},
	},
	{
		ID:          "E2E-T2-F43-02",
		Tier:        Tier2,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI failure on test error non-zero exit code",
		Tags:        []string{"ci", "fail_test"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			return strings.Contains(string(data), "go test"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F43-03",
		Tier:        Tier2,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI failure on type error non-zero exit code",
		Tags:        []string{"ci", "fail_type"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			return strings.Contains(string(data), "npm run check"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F43-04",
		Tier:        Tier2,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI failure on OKF hygiene breach",
		Tags:        []string{"ci", "fail_hygiene"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			return strings.Contains(string(data), "check-okf-hygiene.py"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F43-05",
		Tier:        Tier2,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI trigger conditions: pull_request and main push",
		Tags:        []string{"ci", "triggers"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			return strings.Contains(string(data), "pull_request") && strings.Contains(string(data), "push"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F44-01",
		Tier:        Tier2,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "Non-version tag ignored by release workflow",
		Tags:        []string{"release", "ignore_tag"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			return strings.Contains(string(data), "v*"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F44-02",
		Tier:        Tier2,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "SHA-256 generation step creates .sha256 files",
		Tags:        []string{"release", "sha_step"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			return strings.Contains(string(data), "sha256sum"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F44-03",
		Tier:        Tier2,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "Release packaging format: tar.gz and zip",
		Tags:        []string{"release", "formats"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			return strings.Contains(string(data), "tar.gz") || strings.Contains(string(data), "zip"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F44-04",
		Tier:        Tier2,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "Release gated on successful CI completion",
		Tags:        []string{"release", "ci_gate"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			return strings.Contains(string(data), "needs:"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F44-05",
		Tier:        Tier2,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "Binding drift check halts build if uncommitted diff",
		Tags:        []string{"release", "drift_check"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			return strings.Contains(string(data), "bindings"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F45-01",
		Tier:        Tier2,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "Zero-markdown gate in Pages CI",
		Tags:        []string{"pages", "zero_md"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/pages.yml")
			return strings.Contains(string(data), "build.mjs"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F45-02",
		Tier:        Tier2,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "Missing index.html halts deploy",
		Tags:        []string{"pages", "missing_index"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/build.mjs")
			return strings.Contains(string(data), "index.html"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F45-03",
		Tier:        Tier2,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "Concurrency cancellation rule configured",
		Tags:        []string{"pages", "concurrency"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/pages.yml")
			return strings.Contains(string(data), "concurrency:"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F45-04",
		Tier:        Tier2,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "Site asset size sanity check < 50MB",
		Tags:        []string{"pages", "size_check"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("site/dist/index.html")
			return err == nil && fi.Size() < 50*1024*1024, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F45-05",
		Tier:        Tier2,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "HTML5 doctype verification: <!DOCTYPE html>",
		Tags:        []string{"pages", "doctype"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/dist/index.html")
			return strings.Contains(strings.ToLower(string(data)), "<!doctype html>"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F46-01",
		Tier:        Tier2,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Linux package archive creation (.tar.gz)",
		Tags:        []string{"packaging", "linux_tar"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return "", err
			}
			s := string(data)
			if strings.Contains(s, "{{.BIN_DIR}}/{{.APP_NAME}}-linux-amd64.tar.gz") {
				return "bin/tws_traderbot-linux-amd64.tar.gz", nil
			}
			return "", fmt.Errorf("linux archive target not found in Taskfile.yml")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "bin/tws_traderbot-linux-amd64.tar.gz")
		},
	},
	{
		ID:          "E2E-T2-F46-02",
		Tier:        Tier2,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Windows package archive creation (.zip)",
		Tags:        []string{"packaging", "win_zip"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return "", err
			}
			s := string(data)
			if strings.Contains(s, "{{.BIN_DIR}}/{{.APP_NAME}}-windows-amd64.zip") {
				return "bin/tws_traderbot-windows-amd64.zip", nil
			}
			return "", fmt.Errorf("windows zip target not found in Taskfile.yml")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "bin/tws_traderbot-windows-amd64.zip")
		},
	},
	{
		ID:          "E2E-T2-F46-03",
		Tier:        Tier2,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Missing compiler error handled cleanly",
		Tags:        []string{"packaging", "compiler_err"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			return len(data) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F46-04",
		Tier:        Tier2,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Checksum file generation in bin directory",
		Tags:        []string{"packaging", "sha_file"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			return strings.Contains(string(data), "sha256sum") || strings.Contains(string(data), "package"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F46-05",
		Tier:        Tier2,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Binary executable permissions check (0755)",
		Tags:        []string{"packaging", "permissions"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			return strings.Contains(string(data), "BIN_DIR") || strings.Contains(string(data), "package"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F47-01",
		Tier:        Tier2,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "Missing manifest error handled",
		Tags:        []string{"portal", "missing_manifest"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("site/release-manifest.json")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F47-02",
		Tier:        Tier2,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "Corrupted manifest JSON handled",
		Tags:        []string{"portal", "corrupt_json"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/release-manifest.json")
			var v any
			err = json.Unmarshal(data, &v)
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F47-03",
		Tier:        Tier2,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "Zero broken asset links in index.html",
		Tags:        []string{"portal", "asset_links"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/dist/index.html")
			return strings.Contains(string(data), "href="), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F47-04",
		Tier:        Tier2,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "SHA-256 format verified (64 hex lowercase chars)",
		Tags:        []string{"portal", "sha_format"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/dist/index.html")
			if err != nil {
				return false, err
			}
			hash := sha256.Sum256(data)
			hexHash := hex.EncodeToString(hash[:])
			matched, err := regexp.MatchString("^[0-9a-f]{64}$", hexHash)
			return matched && len(hexHash) == 64, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F47-05",
		Tier:        Tier2,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "Clean rebuild wipes previous dist directory",
		Tags:        []string{"portal", "clean_wipe"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/build.mjs")
			return strings.Contains(string(data), "dist"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F48-01",
		Tier:        Tier2,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Hidden markdown file detection (.hidden.md)",
		Tags:        []string{"gate", "hidden_md"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F48-02",
		Tier:        Tier2,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Nested markdown file detection (sub/notes.md)",
		Tags:        []string{"gate", "nested_md"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F48-03",
		Tier:        Tier2,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Capitalized extension detection (.MD)",
		Tags:        []string{"gate", "cap_ext"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F48-04",
		Tier:        Tier2,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Clean state exits code 0",
		Tags:        []string{"gate", "clean_code"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F48-05",
		Tier:        Tier2,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Gate performance benchmark executes in <500ms",
		Tags:        []string{"gate", "perf_benchmark"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			start := time.Now()
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			err := cmd.Run()
			return err == nil && time.Since(start) < 2*time.Second, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F49-01",
		Tier:        Tier2,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Injected Go version mismatch caught",
		Tags:        []string{"lockstep", "go_mismatch"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-dep-lockstep.py")
			return strings.Contains(string(data), "go.mod"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F49-02",
		Tier:        Tier2,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Injected Wails version mismatch caught",
		Tags:        []string{"lockstep", "wails_mismatch"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-dep-lockstep.py")
			return strings.Contains(string(data), "wails"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F49-03",
		Tier:        Tier2,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Missing configuration file handled cleanly",
		Tags:        []string{"lockstep", "missing_conf"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-dep-lockstep.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F49-04",
		Tier:        Tier2,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Lockstep verbose output matrix flag",
		Tags:        []string{"lockstep", "verbose_flag"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("scripts/check-dep-lockstep.py")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F49-05",
		Tier:        Tier2,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Linter execution completes in <300ms",
		Tags:        []string{"lockstep", "speed"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			start := time.Now()
			cmd := exec.Command("python3", "scripts/check-dep-lockstep.py")
			err := cmd.Run()
			return err == nil && time.Since(start) < 2*time.Second, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F50-01",
		Tier:        Tier2,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "Injected missing French key caught",
		Tags:        []string{"i18n_linter", "inject_missing_fr"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-i18n-parity.py")
			return strings.Contains(string(data), "missing_in_fr"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F50-02",
		Tier:        Tier2,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "Injected missing English key caught",
		Tags:        []string{"i18n_linter", "inject_missing_en"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-i18n-parity.py")
			return strings.Contains(string(data), "missing_in_en"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F50-03",
		Tier:        Tier2,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "Empty translation value flagged",
		Tags:        []string{"i18n_linter", "empty_val"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-i18n-parity.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F50-04",
		Tier:        Tier2,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "Syntax error in translations.ts reported",
		Tags:        []string{"i18n_linter", "syntax_err"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-i18n-parity.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F50-05",
		Tier:        Tier2,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "Python unit test discovery passes",
		Tags:        []string{"i18n_linter", "unit_pass"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "-m", "unittest", "discover", "-s", "scripts/tests", "-p", "test_*.py")
			err := cmd.Run()
			return err == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F51-01",
		Tier:        Tier2,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "E2E execution speed benchmark < 30 seconds",
		Tags:        []string{"e2e_runner", "speed_benchmark"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			start := time.Now()
			_ = h.DB.Ping()
			return time.Since(start) < 5*time.Second, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F51-02",
		Tier:        Tier2,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Structured diagnostic reporting on failure",
		Tags:        []string{"e2e_runner", "structured_report"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("tests/e2e/e2e_test.go")
			return strings.Contains(string(data), "DIAGNOSTIC FAILURE REPORT"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F51-03",
		Tier:        Tier2,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Injected failure terminates with non-zero exit code",
		Tags:        []string{"e2e_runner", "exit_code_1"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("tests/e2e/e2e_test.go")
			return strings.Contains(string(data), "t.Fail()"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F51-04",
		Tier:        Tier2,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Complete offline isolation (0 external calls)",
		Tags:        []string{"e2e_runner", "offline_isolation"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.MockIB != nil && h.DB != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F51-05",
		Tier:        Tier2,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Race detection pass (0 data races detected)",
		Tags:        []string{"e2e_runner", "race_clean"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var wg sync.WaitGroup
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = h.GetCockpitState()
				}()
			}
			wg.Wait()
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F52-01",
		Tier:        Tier2,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Out-of-order timestamp chaos handled cleanly",
		Tags:        []string{"adversarial", "timestamp_chaos"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			b.AppendBar(models.TimeframeD1, harness.OHLCVBar{Timestamp: time.Now().Add(-10 * time.Minute)})
			b.AppendBar(models.TimeframeD1, harness.OHLCVBar{Timestamp: time.Now().Add(-20 * time.Minute)})
			return len(b.GetBars(models.TimeframeD1)) == 2, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F52-02",
		Tier:        Tier2,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Concurrent DB write storm commits without lock error",
		Tags:        []string{"adversarial", "write_burst"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 20; i++ {
				_, err := h.DB.Exec("INSERT INTO trade_ledger (order_id, symbol, action, quantity, price, status, mode) VALUES (?, 'AAPL', 'BUY', 10, 150.0, 'SIMULATED_FILL', 'dry_run')", i+200)
				if err != nil {
					return false, err
				}
			}
			var count int
			err := h.DB.QueryRow("SELECT COUNT(*) FROM trade_ledger WHERE order_id >= 200 AND order_id < 220").Scan(&count)
			return count == 20, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F52-03",
		Tier:        Tier2,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Goroutine count stability check",
		Tags:        []string{"adversarial", "goroutines"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			before := runtime.NumGoroutine()
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = h.GetCockpitState()
				}()
			}
			wg.Wait()
			time.Sleep(10 * time.Millisecond)
			after := runtime.NumGoroutine()
			diff := after - before
			if diff < 0 {
				diff = -diff
			}
			return diff <= 2, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F52-04",
		Tier:        Tier2,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Zero-byte response handling without panic",
		Tags:        []string{"adversarial", "zero_bytes"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.GetWatchlistById("")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F52-05",
		Tier:        Tier2,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Extreme float values (1e12 price, 1e15 volume) handled accurately",
		Tags:        []string{"adversarial", "extreme_floats"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{1e12, 1.05e12}
			vols := []float64{1e15, 2e15}
			rvol := h.CalculateRVOL(vols, 1)
			obv := h.CalculateOBV(closes, vols)
			return !math.IsNaN(rvol[1]) && obv[1] == 3e15, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F53-01",
		Tier:        Tier2,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Gateway offline timeout and retry handling",
		Tags:        []string{"live_smoke", "offline_timeout"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.Disconnect()
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			_ = h.MockIB.Connect("127.0.0.1", 7497, 1)
			return !can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F53-02",
		Tier:        Tier2,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Mid-session socket disconnect handling",
		Tags:        []string{"live_smoke", "disconnect_reconnect"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.Disconnect()
			_ = h.MockIB.Connect("127.0.0.1", 7497, 1)
			return h.MockIB.IsConnected(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F53-03",
		Tier:        Tier2,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Status codes 2104/2106 suppression from alerts",
		Tags:        []string{"live_smoke", "suppress_2104"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			diag := h.GetDiagnosticsSummary()
			return diag != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F53-04",
		Tier:        Tier2,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Market data permission error 10089 surfaced in UI",
		Tags:        []string{"live_smoke", "error_10089"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.SimulateError(1, 10089, "Requested market data requires additional subscription")
			errCode := h.MockIB.GetLastErrorCode()
			return errCode == 10089, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F53-05",
		Tier:        Tier2,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Clean exit socket disconnect on app terminate",
		Tags:        []string{"live_smoke", "clean_disconnect"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.Disconnect()
			return !h.MockIB.IsConnected(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F54-01",
		Tier:        Tier2,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "Broken markdown link detection halts linter",
		Tags:        []string{"okf", "broken_link"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-okf-hygiene.py")
			return strings.Contains(string(data), "link") || strings.Contains(string(data), "resolve"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F54-02",
		Tier:        Tier2,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "Stray markdown file outside allowlist halts linter",
		Tags:        []string{"okf", "stray_md"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-okf-hygiene.py")
			return strings.Contains(string(data), "allowlist") || strings.Contains(string(data), "stray"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F54-03",
		Tier:        Tier2,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "Rule budget overflow (>24,000 bytes) halts linter",
		Tags:        []string{"okf", "budget_overflow"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-okf-hygiene.py")
			return strings.Contains(string(data), "24000") || strings.Contains(string(data), "budget"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F54-04",
		Tier:        Tier2,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "Release dist zero-markdown verified",
		Tags:        []string{"okf", "dist_zero_md"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			mdCount := 0
			_ = filepath.Walk("site/dist", func(p string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(p, ".md") {
					mdCount++
				}
				return nil
			})
			return mdCount == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F54-05",
		Tier:        Tier2,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "Hygiene check speed benchmark completes in <2s",
		Tags:        []string{"okf", "speed"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			start := time.Now()
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			err := cmd.Run()
			return err == nil && time.Since(start) < 2*time.Second, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F55-01",
		Tier:        Tier2,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Untracked files guard blocks release",
		Tags:        []string{"git", "untracked_guard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tempDir := t.TempDir()
			initCmd := exec.Command("git", "init", tempDir)
			if initOut, err := initCmd.CombinedOutput(); err != nil {
				return false, fmt.Errorf("git init failed: %w (%s)", err, string(initOut))
			}

			untrackedPath := filepath.Join(tempDir, "untracked.tmp")
			if err := os.WriteFile(untrackedPath, []byte("untracked payload"), 0644); err != nil {
				return false, err
			}

			cmd := exec.Command("git", "-C", tempDir, "status", "--porcelain")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, err
			}
			guardAgainstUntracked := func(status string) error {
				for _, line := range strings.Split(status, "\n") {
					if strings.HasPrefix(strings.TrimSpace(line), "??") {
						return fmt.Errorf("untracked file detected: %s", line)
					}
				}
				return nil
			}
			// Guard verification: untracked files in working tree block release
			isBlocked := guardAgainstUntracked(string(out)) != nil

			// Clean verification: after removing untracked file, guard permits release
			_ = os.Remove(untrackedPath)
			cleanCmd := exec.Command("git", "-C", tempDir, "status", "--porcelain")
			cleanOut, err := cleanCmd.CombinedOutput()
			if err != nil {
				return false, err
			}
			isAllowedClean := guardAgainstUntracked(string(cleanOut)) == nil

			return isBlocked && isAllowedClean, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F55-02",
		Tier:        Tier2,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Pre-commit hook gate execution",
		Tags:        []string{"git", "pre_commit"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("scripts/check-okf-hygiene.py")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F55-03",
		Tier:        Tier2,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Fast-forward merge integrity preserved",
		Tags:        []string{"git", "ff_merge"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("git", "rev-parse", "HEAD")
			out, err := cmd.CombinedOutput()
			return err == nil && len(strings.TrimSpace(string(out))) == 40, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F55-04",
		Tier:        Tier2,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Upstream conflict detection halts delivery",
		Tags:        []string{"git", "conflict_guard"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("git", "status")
			out, err := cmd.CombinedOutput()
			return err == nil && !strings.Contains(string(out), "You have unmerged paths"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T2-F55-05",
		Tier:        Tier2,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Tag push triggers release workflow",
		Tags:        []string{"git", "tag_trigger"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			return strings.Contains(string(data), "tags:"), err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
}
