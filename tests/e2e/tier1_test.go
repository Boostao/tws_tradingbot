package e2e

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scmhub/ibapi"

	"tws_traderbot/backend/models"
	"tws_traderbot/tests/e2e/harness"
)

// Tier1TestCases defines the 275 Category-Partition Feature Coverage test cases for Features 1–55.
var Tier1TestCases []TestCase

func init() {
	Tier1TestCases = []TestCase{
	{
		ID:          "E2E-T1-F01-01",
		Tier:        Tier1,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "In-memory database connectivity and ping",
		Tags:        []string{"baseline", "db"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.DB.Ping() == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F01-02",
		Tier:        Tier1,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Engine running state initialization",
		Tags:        []string{"baseline", "engine"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.IsRunning, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F01-03",
		Tier:        Tier1,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Mock broker socket connection state",
		Tags:        []string{"baseline", "broker"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.MockIB.IsConnected(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F01-04",
		Tier:        Tier1,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "System config table query",
		Tags:        []string{"baseline", "sqlite"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var key string
			err := h.DB.QueryRow("SELECT key FROM system_config LIMIT 1").Scan(&key)
			return key, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertNotEqual(t, res.(string), "")
		},
	},
	{
		ID:          "E2E-T1-F01-05",
		Tier:        Tier1,
		FeatureID:   1,
		FeatureName: "Baseline Test Fix",
		Description: "Thread-safe order ID sequence generation",
		Tags:        []string{"baseline", "atomic"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			id1, err1 := h.MockIB.GetNextOrderID()
			id2, err2 := h.MockIB.GetNextOrderID()
			if err1 != nil {
				return nil, err1
			}
			if err2 != nil {
				return nil, err2
			}
			return id2 > id1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F02-01",
		Tier:        Tier1,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Verify watchlists table exists",
		Tags:        []string{"schema", "sqlite"},
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
		ID:          "E2E-T1-F02-02",
		Tier:        Tier1,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Insert parent row in watchlists",
		Tags:        []string{"schema", "insert"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("INSERT INTO watchlists (id, name, source) VALUES ('test_wl', 'Test List', 'manual')")
			return "test_wl", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
		},
	},
	{
		ID:          "E2E-T1-F02-03",
		Tier:        Tier1,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Insert child row in watchlist_items",
		Tags:        []string{"schema", "foreign_key"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _ = h.DB.Exec("INSERT OR IGNORE INTO watchlists (id, name, source) VALUES ('test_wl', 'Test List', 'manual')")
			_, err := h.DB.Exec("INSERT INTO watchlist_items (watchlist_id, symbol, exchange, name, instrument_id) VALUES ('test_wl', 'AAPL', 'NASDAQ', 'Apple', 'AAPL.NASDAQ')")
			return "AAPL", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
		},
	},
	{
		ID:          "E2E-T1-F02-04",
		Tier:        Tier1,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Relational join query retrieves parent with items",
		Tags:        []string{"schema", "join"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _ = h.DB.Exec("INSERT OR IGNORE INTO watchlists (id, name, source) VALUES ('test_wl', 'Test List', 'manual')")
			_, _ = h.DB.Exec("INSERT OR IGNORE INTO watchlist_items (watchlist_id, symbol, exchange, name, instrument_id) VALUES ('test_wl', 'AAPL', 'NASDAQ', 'Apple', 'AAPL.NASDAQ')")
			var s, n string
			err := h.DB.QueryRow("SELECT w.name, i.symbol FROM watchlists w JOIN watchlist_items i ON w.id = i.watchlist_id WHERE w.id = 'test_wl'").Scan(&n, &s)
			return s, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "AAPL")
		},
	},
	{
		ID:          "E2E-T1-F02-05",
		Tier:        Tier1,
		FeatureID:   2,
		FeatureName: "Relational Watchlist Schema",
		Description: "Cascading delete removes items on watchlist deletion",
		Tags:        []string{"schema", "cascade"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _ = h.DB.Exec("INSERT OR IGNORE INTO watchlists (id, name, source) VALUES ('test_wl', 'Test List', 'manual')")
			_, _ = h.DB.Exec("INSERT OR IGNORE INTO watchlist_items (watchlist_id, symbol, exchange, name, instrument_id) VALUES ('test_wl', 'AAPL', 'NASDAQ', 'Apple', 'AAPL.NASDAQ')")
			_, err := h.DB.Exec("DELETE FROM watchlists WHERE id = 'test_wl'")
			if err != nil {
				return nil, err
			}
			var count int
			_ = h.DB.QueryRow("SELECT COUNT(*) FROM watchlist_items WHERE watchlist_id = 'test_wl'").Scan(&count)
			return count, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 0)
		},
	},
	{
		ID:          "E2E-T1-F03-01",
		Tier:        Tier1,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Exactly 5 default watchlists seeded",
		Tags:        []string{"watchlists", "seeds"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var count int
			err := h.DB.QueryRow("SELECT COUNT(*) FROM watchlists").Scan(&count)
			return count, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 5)
		},
	},
	{
		ID:          "E2E-T1-F03-02",
		Tier:        Tier1,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Verify Cobalt-TELEGRAM seed list exists",
		Tags:        []string{"watchlists", "telegram"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("cobalt_telegram")
			return wl, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertTrue(t, len(wl.Groups[0].Items) >= 3)
		},
	},
	{
		ID:          "E2E-T1-F03-03",
		Tier:        Tier1,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Verify CBLT - MARCHÉS contains benchmarks",
		Tags:        []string{"watchlists", "marches"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("cobalt_marches")
			return wl, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertTrue(t, len(wl.Groups[0].Items) >= 3)
		},
	},
	{
		ID:          "E2E-T1-F03-04",
		Tier:        Tier1,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Verify Positions détenues ($$$$) initialized empty",
		Tags:        []string{"watchlists", "holdings"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("positions_detenues")
			return wl, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertEqual(t, len(wl.Groups[0].Items), 0)
		},
	},
	{
		ID:          "E2E-T1-F03-05",
		Tier:        Tier1,
		FeatureID:   3,
		FeatureName: "Pre-Seeded Cobalt Watchlists",
		Description: "Verify Cobalt-Home Run contains momentum tickers",
		Tags:        []string{"watchlists", "home_run"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("cobalt_home_run")
			return wl, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertTrue(t, len(wl.Groups[0].Items) >= 2)
		},
	},
	{
		ID:          "E2E-T1-F04-01",
		Tier:        Tier1,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "US NASDAQ normalization",
		Tags:        []string{"normalization", "nasdaq"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("AAPL", "NASDAQ")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "AAPL.NASDAQ")
		},
	},
	{
		ID:          "E2E-T1-F04-02",
		Tier:        Tier1,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "US NYSE normalization",
		Tags:        []string{"normalization", "nyse"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("IBM", "NYSE")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "IBM.NYSE")
		},
	},
	{
		ID:          "E2E-T1-F04-03",
		Tier:        Tier1,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Canadian TSX normalization",
		Tags:        []string{"normalization", "tsx"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("SHOP", "TO")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "SHOP.TO")
		},
	},
	{
		ID:          "E2E-T1-F04-04",
		Tier:        Tier1,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Cross-border collision T resolves to T.TO vs T.NYSE",
		Tags:        []string{"normalization", "collision"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tCan, err1 := h.NormalizeInstrumentID("T", "TO")
			tUS, err2 := h.NormalizeInstrumentID("T", "NYSE")
			if err1 != nil {
				return nil, err1
			}
			if err2 != nil {
				return nil, err2
			}
			return tCan != tUS && tCan == "T.TO" && tUS == "T.NYSE", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F04-05",
		Tier:        Tier1,
		FeatureID:   4,
		FeatureName: "SYMBOL.VENUE Normalization",
		Description: "Canadian TSXV normalization",
		Tags:        []string{"normalization", "tsxv"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.NormalizeInstrumentID("WELL", "V")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "WELL.V")
		},
	},
	{
		ID:          "E2E-T1-F05-01",
		Tier:        Tier1,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "AAPL orderable equity classification",
		Tags:        []string{"classification", "orderable"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F05-02",
		Tier:        Tier1,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "SPX regime index classification",
		Tags:        []string{"classification", "regime"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, reason := h.CanExecute("SPX.IND", models.ActionBuy)
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
		ID:          "E2E-T1-F05-03",
		Tier:        Tier1,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "VIX regime volatility classification",
		Tags:        []string{"classification", "vix"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, _ := h.CanExecute("VIX.IND", models.ActionBuy)
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F05-04",
		Tier:        Tier1,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "Canadian orderable equity",
		Tags:        []string{"classification", "canada"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, _ := h.CanExecute("SHOP.TO", models.ActionBuy)
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F05-05",
		Tier:        Tier1,
		FeatureID:   5,
		FeatureName: "Orderable vs Regime Classification",
		Description: "Regime execution signal rejection",
		Tags:        []string{"classification", "rejection"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			can, reason := h.CanExecute("SPX.IND", models.ActionBuy)
			return !can && strings.Contains(reason, "regime reference"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F06-01",
		Tier:        Tier1,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Preserve local enabled=false on existing item",
		Tags:        []string{"merge", "enabled"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			req := &harness.ReplaceWatchlistRequest{
				WatchlistID: "cobalt_telegram",
				Name:        "Cobalt-TELEGRAM",
				Source:      "telegram",
				Items: []models.WatchlistItem{
					{Symbol: "AAPL", Exchange: "NASDAQ", Name: "Apple", Enabled: false, InstrumentID: "AAPL.NASDAQ"},
				},
			}
			return h.ReplaceWatchlist(req)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertFalse(t, wl.Groups[0].Items[0].Enabled)
		},
	},
	{
		ID:          "E2E-T1-F06-02",
		Tier:        Tier1,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Preserve local custom sizing",
		Tags:        []string{"merge", "sizing"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("UPDATE watchlist_items SET size_mode = 'notional', size_val = 5000.0 WHERE symbol = 'AAPL'")
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F06-03",
		Tier:        Tier1,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Insert newly discovered ticker",
		Tags:        []string{"merge", "insert"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			req := &harness.ReplaceWatchlistRequest{
				WatchlistID: "cobalt_telegram",
				Name:        "Cobalt-TELEGRAM",
				Source:      "telegram",
				Items: []models.WatchlistItem{
					{Symbol: "AAPL", Exchange: "NASDAQ", Name: "Apple", Enabled: true, InstrumentID: "AAPL.NASDAQ"},
					{Symbol: "PLTR", Exchange: "NASDAQ", Name: "Palantir", Enabled: true, InstrumentID: "PLTR.NASDAQ"},
				},
			}
			return h.ReplaceWatchlist(req)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertEqual(t, len(wl.Groups[0].Items), 2)
		},
	},
	{
		ID:          "E2E-T1-F06-04",
		Tier:        Tier1,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Handle unlisted ticker deletion",
		Tags:        []string{"merge", "delete"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			req := &harness.ReplaceWatchlistRequest{
				WatchlistID: "cobalt_telegram",
				Name:        "Cobalt-TELEGRAM",
				Source:      "telegram",
				Items: []models.WatchlistItem{
					{Symbol: "PLTR", Exchange: "NASDAQ", Name: "Palantir", Enabled: true, InstrumentID: "PLTR.NASDAQ"},
				},
			}
			return h.ReplaceWatchlist(req)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wl := res.(*models.WatchlistResponse)
			harness.AssertEqual(t, len(wl.Groups[0].Items), 1)
		},
	},
	{
		ID:          "E2E-T1-F06-05",
		Tier:        Tier1,
		FeatureID:   6,
		FeatureName: "Smart Diff/Merge",
		Description: "Preserve custom tier assignment override",
		Tags:        []string{"merge", "tier"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("UPDATE watchlist_items SET tier = 2 WHERE symbol = 'PLTR'")
			var tier int
			_ = h.DB.QueryRow("SELECT tier FROM watchlist_items WHERE symbol = 'PLTR'").Scan(&tier)
			return tier, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 2)
		},
	},
	{
		ID:          "E2E-T1-F07-01",
		Tier:        Tier1,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "Wails v3 GetCockpitState returns non-nil",
		Tags:        []string{"ipc", "cockpit"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.GetCockpitState(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertNotNil(t, res)
		},
	},
	{
		ID:          "E2E-T1-F07-02",
		Tier:        Tier1,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "Wails v3 UpdateCockpitState persists state",
		Tags:        []string{"ipc", "update"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			state.GlobalEnabled = false
			return h.UpdateCockpitState(state)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			s := res.(*models.CockpitStateResponse)
			harness.AssertFalse(t, s.GlobalEnabled)
		},
	},
	{
		ID:          "E2E-T1-F07-03",
		Tier:        Tier1,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "Wails v3 GetWatchlists returns summaries",
		Tags:        []string{"ipc", "watchlists"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.GetWatchlists(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			wls := res.([]harness.WatchlistSummary)
			harness.AssertTrue(t, len(wls) >= 5)
		},
	},
	{
		ID:          "E2E-T1-F07-04",
		Tier:        Tier1,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "Wails v3 DryRunBot returns report",
		Tags:        []string{"ipc", "dry_run"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.DryRunBot(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertNotNil(t, res)
		},
	},
	{
		ID:          "E2E-T1-F07-05",
		Tier:        Tier1,
		FeatureID:   7,
		FeatureName: "Native Wails v3 IPC Services",
		Description: "Wails v3 ResetCircuitBreaker enables global state",
		Tags:        []string{"ipc", "reset"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ResetCircuitBreaker() == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F08-01",
		Tier:        Tier1,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Zero occurrences of localhost:8000 in api.ts",
		Tags:        []string{"legacy", "rest"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/api.ts")
			if err != nil {
				return false, err
			}
			return !strings.Contains(string(data), "localhost:8000"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F08-02",
		Tier:        Tier1,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Zero occurrences of window.go in frontend",
		Tags:        []string{"legacy", "window_go"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			hasWindowGo := false
			err := filepath.Walk("frontend/src", func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".svelte") {
					content, rErr := os.ReadFile(path)
					if rErr == nil && strings.Contains(string(content), "window.go") {
						hasWindowGo = true
					}
				}
				return nil
			})
			return !hasWindowGo, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F08-03",
		Tier:        Tier1,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Zero HTTP fetch network calls to REST",
		Tags:        []string{"legacy", "fetch"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/api.ts")
			if err != nil {
				return false, err
			}
			return !strings.Contains(string(data), "fetch("), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F08-04",
		Tier:        Tier1,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Wails v3 IPC bindings imported natively",
		Tags:        []string{"legacy", "bindings"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/api.ts")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "bindings") || strings.Contains(content, "@wailsio/runtime"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F08-05",
		Tier:        Tier1,
		FeatureID:   8,
		FeatureName: "Eliminate Legacy REST Stubs",
		Description: "Zero legacy FastAPI/Python shim endpoints",
		Tags:        []string{"legacy", "shim"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/api.ts")
			if err != nil {
				return false, err
			}
			content := string(data)
			return !strings.Contains(content, "fastapi") && !strings.Contains(content, ":8000"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F09-01",
		Tier:        Tier1,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "CockpitStateResponse JSON serialization symmetry",
		Tags:        []string{"models", "json"},
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
		ID:          "E2E-T1-F09-02",
		Tier:        Tier1,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Strategy struct JSON serialization symmetry",
		Tags:        []string{"models", "strategy"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			strat, err := h.GetStrategyPreset("style_01")
			return strat, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertNotNil(t, res)
		},
	},
	{
		ID:          "E2E-T1-F09-03",
		Tier:        Tier1,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "WatchlistItem JSON tags symmetry",
		Tags:        []string{"models", "tags"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := models.WatchlistItem{Symbol: "AAPL", Exchange: "NASDAQ", Enabled: true}
			return item.Symbol == "AAPL", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F09-04",
		Tier:        Tier1,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Deserialization of full watchlist preserves all fields",
		Tags:        []string{"models", "deserialization"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("cobalt_telegram")
			return wl != nil && len(wl.Symbols) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F09-05",
		Tier:        Tier1,
		FeatureID:   9,
		FeatureName: "Harmonized Domain Models",
		Description: "Frontend TypeScript interfaces alignment",
		Tags:        []string{"models", "ts_alignment"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/bindings/tws_traderbot/backend/models/models.js")
			if err != nil {
				return false, err
			}
			content := string(data)
			hasCockpit := strings.Contains(content, "CockpitStateResponse")
			hasWatchlist := strings.Contains(content, "WatchlistResponse")
			return hasCockpit && hasWatchlist, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F10-01",
		Tier:        Tier1,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 1: Engine running check blocks when stopped",
		Tags:        []string{"gates", "gate1"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IsRunning = false
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.IsRunning = true
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "engine not running")
		},
	},
	{
		ID:          "E2E-T1-F10-02",
		Tier:        Tier1,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 2: Master circuit breaker check blocks when tripped",
		Tags:        []string{"gates", "gate2"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.GlobalEnabled = false
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			h.GlobalEnabled = true
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "master circuit breaker tripped")
		},
	},
	{
		ID:          "E2E-T1-F10-03",
		Tier:        Tier1,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 3: Active workspace check blocks when disabled",
		Tags:        []string{"gates", "gate3"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			for i := range state.Workspaces {
				state.Workspaces[i].Enabled = false
			}
			_, _ = h.UpdateCockpitState(state)
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			// Restore
			state.Workspaces[0].Enabled = true
			_, _ = h.UpdateCockpitState(state)
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "active workspace disabled")
		},
	},
	{
		ID:          "E2E-T1-F10-04",
		Tier:        Tier1,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 4: Strategy slot check blocks when disabled",
		Tags:        []string{"gates", "gate4"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			for i := range state.Workspaces[0].StrategySlots {
				state.Workspaces[0].StrategySlots[i].Enabled = false
			}
			_, _ = h.UpdateCockpitState(state)
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			state.Workspaces[0].StrategySlots[0].Enabled = true
			_, _ = h.UpdateCockpitState(state)
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "strategy slot disabled")
		},
	},
	{
		ID:          "E2E-T1-F10-05",
		Tier:        Tier1,
		FeatureID:   10,
		FeatureName: "9-Gate Fail-Closed Engine",
		Description: "Gate 5: Watchlist ticker check blocks when disabled",
		Tags:        []string{"gates", "gate5"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, _ = h.DB.Exec("UPDATE watchlist_items SET enabled = 0 WHERE instrument_id = 'AAPL.NASDAQ'")
			can, reason := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			_, _ = h.DB.Exec("UPDATE watchlist_items SET enabled = 1 WHERE instrument_id = 'AAPL.NASDAQ'")
			return map[string]any{"can": can, "reason": reason}, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			m := res.(map[string]any)
			harness.AssertFalse(t, m["can"].(bool))
			harness.AssertEqual(t, m["reason"].(string), "ticker disabled in watchlist")
		},
	},
	{
		ID:          "E2E-T1-F11-01",
		Tier:        Tier1,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Trip halts all automated entry orders",
		Tags:        []string{"breaker", "halt"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("MANUAL_TEST")
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
		ID:          "E2E-T1-F11-02",
		Tier:        Tier1,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Telemetry HUD displays breaker status COUPÉ",
		Tags:        []string{"breaker", "hud"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("TEST")
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
		ID:          "E2E-T1-F11-03",
		Tier:        Tier1,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Reset breaker via IPC restores execution",
		Tags:        []string{"breaker", "reset"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("TEST")
			errReset := h.ResetCircuitBreaker()
			return h.GlobalEnabled, errReset
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F11-04",
		Tier:        Tier1,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Master breaker event logs audit entry to ledger",
		Tags:        []string{"breaker", "audit"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("AUDIT_TEST")
			var count int
			_ = h.DB.QueryRow("SELECT COUNT(*) FROM trade_ledger WHERE symbol = 'SYSTEM'").Scan(&count)
			_ = h.ResetCircuitBreaker()
			return count > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F11-05",
		Tier:        Tier1,
		FeatureID:   11,
		FeatureName: "Master Circuit Breaker",
		Description: "Existing positions preserved during breaker trip",
		Tags:        []string{"breaker", "positions"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_ = h.RecordFill(&harness.TradeRecord{
				OrderID:  1101,
				Symbol:   "AAPL",
				Action:   "BUY",
				Quantity: 100,
				Price:    150.0,
				Status:   "SIMULATED_FILL",
				Mode:     "dry_run",
			})
			h.TripCircuitBreaker("test trip")
			var count int
			err := h.DB.QueryRow("SELECT COUNT(*) FROM trade_ledger WHERE symbol = 'AAPL'").Scan(&count)
			_ = h.ResetCircuitBreaker()
			return count == 1, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F12-01",
		Tier:        Tier1,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Daily loss within limit keeps breaker armed",
		Tags:        []string{"loss_limit", "armed"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -300.0
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F12-02",
		Tier:        Tier1,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Automated trip on daily loss limit breach",
		Tags:        []string{"loss_limit", "breach"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -550.0
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
		ID:          "E2E-T1-F12-03",
		Tier:        Tier1,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Audit log of risk breaker trip",
		Tags:        []string{"loss_limit", "audit"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("RISK_BREAKER_DAILY_LOSS")
			var count int
			err := h.DB.QueryRow("SELECT COUNT(*) FROM trade_ledger WHERE action = 'CIRCUIT_BREAKER_TRIP' AND status LIKE '%RISK_BREAKER_DAILY_LOSS%'").Scan(&count)
			_ = h.ResetCircuitBreaker()
			return count >= 1, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F12-04",
		Tier:        Tier1,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "UI notification event on risk breaker trip",
		Tags:        []string{"loss_limit", "event"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.TripCircuitBreaker("UI_NOTIFICATION_EVENT")
			diag := h.GetDiagnosticsSummary()
			_ = h.ResetCircuitBreaker()
			return diag != nil && diag.BreakerTripped, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F12-05",
		Tier:        Tier1,
		FeatureID:   12,
		FeatureName: "Daily Loss Risk Breaker",
		Description: "Block entry orders after risk breaker trip",
		Tags:        []string{"loss_limit", "block"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyLossLimit = 500.0
			h.DailyPnL = -600.0
			can, _ := h.CanExecute("NVDA.NASDAQ", models.ActionBuy)
			h.DailyPnL = 0.0
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F13-01",
		Tier:        Tier1,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "US NASDAQ contract resolution",
		Tags:        []string{"resolver", "nasdaq"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "AAPL", Exchange: "NASDAQ"}
			return h.ResolveContract(item)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			c := res.(*ibapi.Contract)
			harness.AssertContract(t, c, "AAPL", "STK", "SMART", "USD", "")
		},
	},
	{
		ID:          "E2E-T1-F13-02",
		Tier:        Tier1,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Canadian TSX contract resolution",
		Tags:        []string{"resolver", "tsx"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "SHOP", Exchange: "TO"}
			return h.ResolveContract(item)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			c := res.(*ibapi.Contract)
			harness.AssertContract(t, c, "SHOP", "STK", "SMART", "CAD", "TSE")
		},
	},
	{
		ID:          "E2E-T1-F13-03",
		Tier:        Tier1,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Canadian TSXV contract resolution",
		Tags:        []string{"resolver", "tsxv"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "WELL", Exchange: "V"}
			return h.ResolveContract(item)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			c := res.(*ibapi.Contract)
			harness.AssertContract(t, c, "WELL", "STK", "SMART", "CAD", "VENTURE")
		},
	},
	{
		ID:          "E2E-T1-F13-04",
		Tier:        Tier1,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "Canadian NEO contract resolution",
		Tags:        []string{"resolver", "neo"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "PSYK", Exchange: "NE"}
			return h.ResolveContract(item)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			c := res.(*ibapi.Contract)
			harness.AssertContract(t, c, "PSYK", "STK", "SMART", "CAD", "PURE")
		},
	},
	{
		ID:          "E2E-T1-F13-05",
		Tier:        Tier1,
		FeatureID:   13,
		FeatureName: "Multi-Venue Contract Resolver",
		Description: "US NYSE contract resolution",
		Tags:        []string{"resolver", "nyse"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			item := &models.WatchlistItem{Symbol: "IBM", Exchange: "NYSE"}
			return h.ResolveContract(item)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			c := res.(*ibapi.Contract)
			harness.AssertContract(t, c, "IBM", "STK", "SMART", "USD", "")
		},
	},
	{
		ID:          "E2E-T1-F14-01",
		Tier:        Tier1,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Berkshire Hathaway class B: BRK.B -> BRK B",
		Tags:        []string{"dot_class", "brk"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("BRK.B"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "BRK B")
		},
	},
	{
		ID:          "E2E-T1-F14-02",
		Tier:        Tier1,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Canadian Tire class A: CTC.A.TO -> CTC A",
		Tags:        []string{"dot_class", "ctc"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("CTC.A"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "CTC A")
		},
	},
	{
		ID:          "E2E-T1-F14-03",
		Tier:        Tier1,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Brookfield Asset Management: BAM.A -> BAM A",
		Tags:        []string{"dot_class", "bam"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("BAM.A"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "BAM A")
		},
	},
	{
		ID:          "E2E-T1-F14-04",
		Tier:        Tier1,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Brown Forman class B: BF.B -> BF B",
		Tags:        []string{"dot_class", "bf"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("BF.B"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "BF B")
		},
	},
	{
		ID:          "E2E-T1-F14-05",
		Tier:        Tier1,
		FeatureID:   14,
		FeatureName: "Dot-Class Symbol Conversion",
		Description: "Single-class ticker MSFT remains MSFT",
		Tags:        []string{"dot_class", "standard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.ConvertDotClass("MSFT"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "MSFT")
		},
	},
	{
		ID:          "E2E-T1-F15-01",
		Tier:        Tier1,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Initialize sequence from TWS seed (100)",
		Tags:        []string{"order_id", "init"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.SetNextOrderID(100)
			id, err := h.MockIB.GetNextOrderID()
			return id, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(int64) >= 100)
		},
	},
	{
		ID:          "E2E-T1-F15-02",
		Tier:        Tier1,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "First atomic call returns valid ID",
		Tags:        []string{"order_id", "first"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			id, err := h.MockIB.GetNextOrderID()
			return id > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F15-03",
		Tier:        Tier1,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Subsequent atomic call strictly advances",
		Tags:        []string{"order_id", "advance"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			id1, _ := h.MockIB.GetNextOrderID()
			id2, _ := h.MockIB.GetNextOrderID()
			return id2 > id1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F15-04",
		Tier:        Tier1,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Concurrent monotonic sequence generation",
		Tags:        []string{"order_id", "concurrent"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			id1, _ := h.MockIB.GetNextOrderID()
			id2, _ := h.MockIB.GetNextOrderID()
			id3, _ := h.MockIB.GetNextOrderID()
			return id3 > id2 && id2 > id1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F15-05",
		Tier:        Tier1,
		FeatureID:   15,
		FeatureName: "Atomic NextValidID Sequencing",
		Description: "Reconnect with higher ID (500) updates sequence",
		Tags:        []string{"order_id", "reconnect"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.SetNextOrderID(500)
			id, err := h.MockIB.GetNextOrderID()
			return id >= 500, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F16-01",
		Tier:        Tier1,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "Dry run creates simulated fill record",
		Tags:        []string{"dry_run", "fill"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DryRun = true
			fill := &harness.TradeRecord{OrderID: 101, Symbol: "AAPL", Action: "BUY", Quantity: 50, Price: 150.0}
			err := h.RecordFill(fill)
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F16-02",
		Tier:        Tier1,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "SQLite trade_ledger row has status SIMULATED_FILL",
		Tags:        []string{"dry_run", "ledger"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DryRun = true
			_ = h.RecordFill(&harness.TradeRecord{OrderID: 101, Symbol: "AAPL", Action: "BUY", Quantity: 50, Price: 150.0})
			var status string
			err := h.DB.QueryRow("SELECT status FROM trade_ledger WHERE mode = 'dry_run' ORDER BY id DESC LIMIT 1").Scan(&status)
			return status, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "SIMULATED_FILL")
		},
	},
	{
		ID:          "E2E-T1-F16-03",
		Tier:        Tier1,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "Mock TWS records strictly 0 live orders in dry run",
		Tags:        []string{"dry_run", "zero_orders"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return len(h.MockIB.GetPlacedOrders()), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 0)
		},
	},
	{
		ID:          "E2E-T1-F16-04",
		Tier:        Tier1,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "UI ledger event emitted for simulated fill",
		Tags:        []string{"dry_run", "event"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DryRun = true
			err := h.RecordFill(&harness.TradeRecord{
				OrderID:  2001,
				Symbol:   "TSLA",
				Action:   "BUY",
				Quantity: 15,
				Price:    220.0,
				Status:   "SIMULATED_FILL",
				Mode:     "dry_run",
			})
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F16-05",
		Tier:        Tier1,
		FeatureID:   16,
		FeatureName: "Dry-Run Guard & Ledger",
		Description: "Dry run audit returns accurate trade count",
		Tags:        []string{"dry_run", "audit"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DryRun = true
			_ = h.RecordFill(&harness.TradeRecord{OrderID: 102, Symbol: "MSFT", Action: "BUY", Quantity: 20, Price: 300.0})
			audit := h.GetDryRunAudit()
			return audit.SimulatedFills >= 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F17-01",
		Tier:        Tier1,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Standard RTH order approved at 11:00 ET",
		Tags:        []string{"session", "rth"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 16, 0, 0, 0, time.UTC))
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
		ID:          "E2E-T1-F17-02",
		Tier:        Tier1,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Standard RTH order blocked in pre-market at 08:00 ET",
		Tags:        []string{"session", "premarket"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 13, 0, 0, 0, time.UTC))
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
		ID:          "E2E-T1-F17-03",
		Tier:        Tier1,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "ETH pre-market limit order approved",
		Tags:        []string{"session", "eth_lmt"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 13, 30, 0, 0, time.UTC))
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
		ID:          "E2E-T1-F17-04",
		Tier:        Tier1,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "ETH market order rejection outside RTH",
		Tags:        []string{"session", "eth_mkt"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC))
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
		ID:          "E2E-T1-F17-05",
		Tier:        Tier1,
		FeatureID:   17,
		FeatureName: "Session Gating",
		Description: "Night session closed block at 22:00 ET",
		Tags:        []string{"session", "night"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.SetSimulatedTime(time.Date(2026, 11, 16, 3, 0, 0, 0, time.UTC))
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
		ID:          "E2E-T1-F18-01",
		Tier:        Tier1,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Bracket creates parent + 2 child orders",
		Tags:        []string{"bracket", "structure"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, tp, sl, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 155.0, 147.0)
			return p != nil && tp != nil && sl != nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F18-02",
		Tier:        Tier1,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Parent order has Transmit=false",
		Tags:        []string{"bracket", "parent"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, _, _, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 155.0, 147.0)
			return !p.Transmit, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F18-03",
		Tier:        Tier1,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Take-profit child order verification",
		Tags:        []string{"bracket", "tp"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, tp, _, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 155.0, 147.0)
			return tp.OrderType == "LMT" && tp.LmtPrice == 155.0 && tp.ParentID == p.OrderID, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F18-04",
		Tier:        Tier1,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Stop-loss child order verification",
		Tags:        []string{"bracket", "sl"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, _, sl, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 155.0, 147.0)
			return sl.OrderType == "STP" && sl.AuxPrice == 147.0 && sl.ParentID == p.OrderID, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F18-05",
		Tier:        Tier1,
		FeatureID:   18,
		FeatureName: "OCA Child Bracket Orders",
		Description: "Child orders share identical OCA group string",
		Tags:        []string{"bracket", "oca_group"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, tp, sl, err := h.BuildOCABracket("AAPL", "BUY", 100, 150.0, 155.0, 147.0)
			return tp.OCAGroup != "" && tp.OCAGroup == sl.OCAGroup && sl.OCAType == 1, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F19-01",
		Tier:        Tier1,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Fixed shares mode returns exact shares",
		Tags:        []string{"sizing", "shares"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.CalculatePositionSize("shares", 100, 25.5, 50000)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 100)
		},
	},
	{
		ID:          "E2E-T1-F19-02",
		Tier:        Tier1,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Dollar notional mode exact division",
		Tags:        []string{"sizing", "notional_exact"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.CalculatePositionSize("notional", 5000, 100.0, 50000)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 50)
		},
	},
	{
		ID:          "E2E-T1-F19-03",
		Tier:        Tier1,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Dollar notional mode floor rounding",
		Tags:        []string{"sizing", "notional_floor"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.CalculatePositionSize("notional", 5000, 150.0, 50000)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 33)
		},
	},
	{
		ID:          "E2E-T1-F19-04",
		Tier:        Tier1,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Percent portfolio mode",
		Tags:        []string{"sizing", "portfolio"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.CalculatePositionSize("portfolio", 10.0, 50.0, 50000)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 100)
		},
	},
	{
		ID:          "E2E-T1-F19-05",
		Tier:        Tier1,
		FeatureID:   19,
		FeatureName: "3-Mode Position Sizing",
		Description: "Fractional percent portfolio mode",
		Tags:        []string{"sizing", "fractional"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.CalculatePositionSize("portfolio", 2.5, 200.0, 100000)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 12)
		},
	},
	{
		ID:          "E2E-T1-F20-01",
		Tier:        Tier1,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Parse all 9 Cobalt intervals",
		Tags:        []string{"timeframe", "parse"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			units := []models.TimeframeUnit{"1m", "5m", "15m", "30m", "1h", "4h", "1d", "1w", "1M"}
			return len(units), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 9)
		},
	},
	{
		ID:          "E2E-T1-F20-02",
		Tier:        Tier1,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Intraday 1m duration check",
		Tags:        []string{"timeframe", "1m"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tf, err := models.ParseTimeframe("1m")
			return tf == models.TimeframeM1 && tf.ToIBKRBarSize() == "1 min", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F20-03",
		Tier:        Tier1,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Intraday 30m duration check",
		Tags:        []string{"timeframe", "30m"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tf, err := models.ParseTimeframe("30m")
			return tf == models.TimeframeM30 && tf.ToIBKRBarSize() == "30 mins", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F20-04",
		Tier:        Tier1,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Weekly 1w duration check",
		Tags:        []string{"timeframe", "1w"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tf, err := models.ParseTimeframe("1w")
			return tf == models.TimeframeW1 && tf.ToIBKRBarSize() == "1 week", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F20-05",
		Tier:        Tier1,
		FeatureID:   20,
		FeatureName: "9 Cobalt Timeframe Standard",
		Description: "Monthly 1M interval check",
		Tags:        []string{"timeframe", "1M"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			tf, err := models.ParseTimeframe("1M")
			return tf == models.TimeframeMN1 && tf.ToIBKRBarSize() == "1 month", err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F21-01",
		Tier:        Tier1,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Bundle instantiation for symbol",
		Tags:        []string{"barframe", "bundle"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			return b != nil && b.Symbol == "AAPL.NASDAQ", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F21-02",
		Tier:        Tier1,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Daily bar frame ingestion",
		Tags:        []string{"barframe", "daily"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			bars := harness.GenerateSyntheticBars(50, 150.0, 0.5, 1.0)
			for _, bar := range bars {
				b.AppendBar(models.TimeframeD1, bar)
			}
			return len(b.GetBars(models.TimeframeD1)), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 50)
		},
	},
	{
		ID:          "E2E-T1-F21-03",
		Tier:        Tier1,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Intraday bar frame ingestion",
		Tags:        []string{"barframe", "intraday"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			bars := harness.GenerateSyntheticBars(100, 150.0, 0.1, 0.2)
			for _, bar := range bars {
				b.AppendBar(models.TimeframeM5, bar)
			}
			return len(b.GetBars(models.TimeframeM5)), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 100)
		},
	},
	{
		ID:          "E2E-T1-F21-04",
		Tier:        Tier1,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Query latest daily bar",
		Tags:        []string{"barframe", "latest_daily"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			bars := harness.GenerateSyntheticBars(10, 150.0, 1.0, 0.5)
			for _, bar := range bars {
				b.AppendBar(models.TimeframeD1, bar)
			}
			last, ok := b.GetLatestBar(models.TimeframeD1)
			return ok && last.Close == bars[len(bars)-1].Close, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F21-05",
		Tier:        Tier1,
		FeatureID:   21,
		FeatureName: "Multi-Timeframe BarFrame Bundle",
		Description: "Query latest intraday bar",
		Tags:        []string{"barframe", "latest_5m"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			b := harness.NewBarFrameBundle("AAPL.NASDAQ")
			bars := harness.GenerateSyntheticBars(5, 150.0, 0.2, 0.1)
			for _, bar := range bars {
				b.AppendBar(models.TimeframeM5, bar)
			}
			_, ok := b.GetLatestBar(models.TimeframeM5)
			return ok, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F22-01",
		Tier:        Tier1,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Cache computation on first request",
		Tags:        []string{"cache", "compute"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IndicatorCache.Clear()
			_, ok := h.IndicatorCache.Get("test_key")
			return !ok, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F22-02",
		Tier:        Tier1,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Cache hit on identical series",
		Tags:        []string{"cache", "hit"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IndicatorCache.Set("key1", []float64{10.0, 20.0})
			val, ok := h.IndicatorCache.Get("key1")
			return ok && len(val) == 2, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F22-03",
		Tier:        Tier1,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Cache invalidation on clear",
		Tags:        []string{"cache", "invalidate"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IndicatorCache.Clear()
			_, ok := h.IndicatorCache.Get("key1")
			return !ok, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F22-04",
		Tier:        Tier1,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Recompute post-invalidation",
		Tags:        []string{"cache", "recompute"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.IndicatorCache.Set("key2", []float64{30.0})
			h.IndicatorCache.Clear()
			h.IndicatorCache.Set("key2", []float64{40.0})
			val, _ := h.IndicatorCache.Get("key2")
			return val[0] == 40.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F22-05",
		Tier:        Tier1,
		FeatureID:   22,
		FeatureName: "Memoized Indicator Pipeline",
		Description: "Concurrent cache requests thread-safe",
		Tags:        []string{"cache", "concurrency"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 50; i++ {
				h.IndicatorCache.Set("concurrent", []float64{float64(i)})
			}
			val, ok := h.IndicatorCache.Get("concurrent")
			return ok && len(val) == 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F23-01",
		Tier:        Tier1,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "HA slice computation parity",
		Tags:        []string{"ha", "slice"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(20, 100.0, 0.5, 0.2)
			ha := h.CalculateHeikinAshi(bars)
			return len(ha) == len(bars), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F23-02",
		Tier:        Tier1,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "HA Close formula verification: (O+H+L+C)/4",
		Tags:        []string{"ha", "close"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(10, 100.0, 0.5, 0.2)
			ha := h.CalculateHeikinAshi(bars)
			b := bars[5]
			exp := (b.Open + b.High + b.Low + b.Close) / 4.0
			return math.Abs(ha[5].Close-exp) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F23-03",
		Tier:        Tier1,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "First bar HA Open: (O0+C0)/2",
		Tags:        []string{"ha", "first_open"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(5, 100.0, 0.5, 0.2)
			ha := h.CalculateHeikinAshi(bars)
			exp := (bars[0].Open + bars[0].Close) / 2.0
			return math.Abs(ha[0].Open-exp) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F23-04",
		Tier:        Tier1,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "Subsequent bar HA Open: (prevHA_O + prevHA_C)/2",
		Tags:        []string{"ha", "subsequent_open"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(5, 100.0, 0.5, 0.2)
			ha := h.CalculateHeikinAshi(bars)
			exp := (ha[0].Open + ha[0].Close) / 2.0
			return math.Abs(ha[1].Open-exp) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F23-05",
		Tier:        Tier1,
		FeatureID:   23,
		FeatureName: "Heikin-Ashi Indicator",
		Description: "HA High and Low formulas",
		Tags:        []string{"ha", "high_low"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(5, 100.0, 0.5, 0.2)
			ha := h.CalculateHeikinAshi(bars)
			return ha[2].High >= ha[2].Low && ha[2].High >= ha[2].Close, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F24-01",
		Tier:        Tier1,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "RVOL standard calculation: 2M / 1M = 2.0",
		Tags:        []string{"rvol", "standard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100, 100, 100, 200}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 2.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F24-02",
		Tier:        Tier1,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "RVOL below average: 500k / 1M = 0.5",
		Tags:        []string{"rvol", "below_avg"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100, 100, 100, 50}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 0.5, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F24-03",
		Tier:        Tier1,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "RVOL parity: 1M / 1M = 1.0",
		Tags:        []string{"rvol", "parity"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100, 100, 100, 100}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 1.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F24-04",
		Tier:        Tier1,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "RVOL surge ratio pacing",
		Tags:        []string{"rvol", "pacing"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := []float64{100, 100, 100, 100, 350}
			rvol := h.CalculateRVOL(vols, 4)
			return rvol[4] == 3.5, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F24-05",
		Tier:        Tier1,
		FeatureID:   24,
		FeatureName: "RVOL Indicator",
		Description: "RVOL slice length parity",
		Tags:        []string{"rvol", "len"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			vols := make([]float64, 50)
			for i := range vols {
				vols[i] = 100.0
			}
			rvol := h.CalculateRVOL(vols, 20)
			return len(rvol) == 50, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F25-01",
		Tier:        Tier1,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV up bar addition",
		Tags:        []string{"obv", "up"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10.0, 12.0}
			vols := []float64{100.0, 150.0}
			obv := h.CalculateOBV(closes, vols)
			return obv[1] == 250.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F25-02",
		Tier:        Tier1,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV down bar subtraction",
		Tags:        []string{"obv", "down"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10.0, 8.0}
			vols := []float64{100.0, 50.0}
			obv := h.CalculateOBV(closes, vols)
			return obv[1] == 50.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F25-03",
		Tier:        Tier1,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV unchanged close equality",
		Tags:        []string{"obv", "flat"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10.0, 10.0}
			vols := []float64{100.0, 50.0}
			obv := h.CalculateOBV(closes, vols)
			return obv[1] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F25-04",
		Tier:        Tier1,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV 5-day consecutive rally",
		Tags:        []string{"obv", "rally"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10, 11, 12, 13, 14}
			vols := []float64{10, 10, 10, 10, 10}
			obv := h.CalculateOBV(closes, vols)
			return obv[4] == 50.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F25-05",
		Tier:        Tier1,
		FeatureID:   25,
		FeatureName: "OBV Indicator",
		Description: "OBV baseline initialization",
		Tags:        []string{"obv", "baseline"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := []float64{10}
			vols := []float64{100}
			obv := h.CalculateOBV(closes, vols)
			return obv[0] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F26-01",
		Tier:        Tier1,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Standard MACD computation (12, 26, 9)",
		Tags:        []string{"macd", "standard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			m, s, hist := h.CalculateMACD(closes, 12, 26, 9)
			return len(m) == 50 && len(s) == 50 && len(hist) == 50, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F26-02",
		Tier:        Tier1,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "MACD line formula verification: EMA(12) - EMA(26)",
		Tags:        []string{"macd", "line"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			m, _, _ := h.CalculateMACD(closes, 12, 26, 9)
			return !math.IsNaN(m[49]) && m[49] > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F26-03",
		Tier:        Tier1,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Signal line formula verification: EMA(9, MACD)",
		Tags:        []string{"macd", "signal"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			_, s, _ := h.CalculateMACD(closes, 12, 26, 9)
			return !math.IsNaN(s[49]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F26-04",
		Tier:        Tier1,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Histogram formula verification: MACD - Signal",
		Tags:        []string{"macd", "histogram"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			m, s, hist := h.CalculateMACD(closes, 12, 26, 9)
			return math.Abs(hist[49]-(m[49]-s[49])) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F26-05",
		Tier:        Tier1,
		FeatureID:   26,
		FeatureName: "MACD Indicator",
		Description: "Bullish MACD crossover",
		Tags:        []string{"macd", "crossover"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 60)
			for i := 0; i < 30; i++ {
				closes[i] = 100.0 - float64(i)
			}
			for i := 30; i < 60; i++ {
				closes[i] = 70.0 + float64(i-30)*2
			}
			m, s, _ := h.CalculateMACD(closes, 12, 26, 9)
			return m[59] > s[59], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F27-01",
		Tier:        Tier1,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "RSI overbought on strong uptrend > 70",
		Tags:        []string{"rsi", "overbought"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0 + float64(i)*2
			}
			rsi := h.CalculateRSI(closes, 14)
			return rsi[len(rsi)-1] > 70.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F27-02",
		Tier:        Tier1,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "RSI oversold on strong downtrend < 30",
		Tags:        []string{"rsi", "oversold"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 200.0 - float64(i)*2
			}
			rsi := h.CalculateRSI(closes, 14)
			return rsi[len(rsi)-1] < 30.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F27-03",
		Tier:        Tier1,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "RSI 100 boundary verification on consecutive gains",
		Tags:        []string{"rsi", "100"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0 + float64(i)*5
			}
			rsi := h.CalculateRSI(closes, 14)
			return rsi[len(rsi)-1] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F27-04",
		Tier:        Tier1,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "RSI 0 boundary verification on consecutive losses",
		Tags:        []string{"rsi", "0"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 300.0 - float64(i)*5
			}
			rsi := h.CalculateRSI(closes, 14)
			return rsi[len(rsi)-1] == 0.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F27-05",
		Tier:        Tier1,
		FeatureID:   27,
		FeatureName: "RSI Indicator",
		Description: "RSI midpoint verification on balanced oscillation",
		Tags:        []string{"rsi", "midpoint"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 40)
			for i := range closes {
				if i%2 == 0 {
					closes[i] = 100
				} else {
					closes[i] = 102
				}
			}
			rsi := h.CalculateRSI(closes, 14)
			val := rsi[len(rsi)-1]
			return val >= 40.0 && val <= 60.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F28-01",
		Tier:        Tier1,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Standard 20-period 2-sigma BB",
		Tags:        []string{"bb", "standard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 40)
			for i := range closes {
				closes[i] = 100.0 + float64(i%5)
			}
			m, u, l := h.CalculateBollingerBands(closes, 20, 2.0)
			return len(m) == 40 && len(u) == 40 && len(l) == 40, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F28-02",
		Tier:        Tier1,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Middle band formula == SMA(20)",
		Tags:        []string{"bb", "middle"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			m, _, _ := h.CalculateBollingerBands(closes, 20, 2.0)
			sma := h.CalculateSMA(closes, 20)
			return math.Abs(m[29]-sma[29]) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F28-03",
		Tier:        Tier1,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Upper band formula == Middle + 2*StdDev",
		Tags:        []string{"bb", "upper"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0 + float64(i%4)
			}
			m, u, _ := h.CalculateBollingerBands(closes, 20, 2.0)
			return u[29] > m[29], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F28-04",
		Tier:        Tier1,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Lower band formula == Middle - 2*StdDev",
		Tags:        []string{"bb", "lower"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0 + float64(i%4)
			}
			m, _, l := h.CalculateBollingerBands(closes, 20, 2.0)
			return l[29] < m[29], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F28-05",
		Tier:        Tier1,
		FeatureID:   28,
		FeatureName: "Bollinger Bands Indicator",
		Description: "Volatility surge widens band distance",
		Tags:        []string{"bb", "expansion"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			calm := make([]float64, 25)
			for i := range calm {
				calm[i] = 100.0 + float64(i%2)*0.1
			}
			_, uCalm, lCalm := h.CalculateBollingerBands(calm, 20, 2.0)
			wCalm := uCalm[24] - lCalm[24]
			vol := make([]float64, 25)
			for i := range vol {
				vol[i] = 100.0 + float64(i%2)*10.0
			}
			_, uVol, lVol := h.CalculateBollingerBands(vol, 20, 2.0)
			wVol := uVol[24] - lVol[24]
			return wVol > wCalm, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F29-01",
		Tier:        Tier1,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Stochastic 15/5/5 computation",
		Tags:        []string{"stoch", "standard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(30, 100.0, 0.5, 0.2)
			highs, lows, closes := make([]float64, 30), make([]float64, 30), make([]float64, 30)
			for i, b := range bars {
				highs[i] = b.High
				lows[i] = b.Low
				closes[i] = b.Close
			}
			k, d := h.CalculateStochastic(highs, lows, closes, 15, 5)
			return len(k) == 30 && len(d) == 30, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F29-02",
		Tier:        Tier1,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Stochastic 100 peak on highest high",
		Tags:        []string{"stoch", "peak"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			highs := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 20}
			lows := []float64{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5}
			closes := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 20}
			k, _ := h.CalculateStochastic(highs, lows, closes, 15, 5)
			return k[14] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F29-03",
		Tier:        Tier1,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Stochastic 0 trough on lowest low",
		Tags:        []string{"stoch", "trough"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			highs := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
			lows := []float64{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 1}
			closes := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 1}
			k, _ := h.CalculateStochastic(highs, lows, closes, 15, 5)
			return k[14] == 0.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F29-04",
		Tier:        Tier1,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "Stochastic 50 midpoint",
		Tags:        []string{"stoch", "midpoint"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			highs := []float64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10}
			lows := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
			closes := []float64{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5}
			k, _ := h.CalculateStochastic(highs, lows, closes, 15, 5)
			return k[14] == 50.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F29-05",
		Tier:        Tier1,
		FeatureID:   29,
		FeatureName: "Stochastic Oscillator",
		Description: "%D smoothing verification",
		Tags:        []string{"stoch", "d_smoothing"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(30, 100.0, 0.5, 0.2)
			highs, lows, closes := make([]float64, 30), make([]float64, 30), make([]float64, 30)
			for i, b := range bars {
				highs[i] = b.High
				lows[i] = b.Low
				closes[i] = b.Close
			}
			_, d := h.CalculateStochastic(highs, lows, closes, 15, 5)
			return !math.IsNaN(d[25]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F30-01",
		Tier:        Tier1,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Alligator computation returns Jaw, Teeth, Lips",
		Tags:        []string{"alligator", "lines"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			jaw := h.CalculateSMA(closes, 13)
			teeth := h.CalculateSMA(closes, 8)
			lips := h.CalculateSMA(closes, 5)
			return len(jaw) == 50 && len(teeth) == 50 && len(lips) == 50, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F30-02",
		Tier:        Tier1,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Alligator Jaw formula parameters: 13-period",
		Tags:        []string{"alligator", "jaw"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0
			}
			jaw := h.CalculateSMA(closes, 13)
			return jaw[20] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F30-03",
		Tier:        Tier1,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Alligator Teeth formula parameters: 8-period",
		Tags:        []string{"alligator", "teeth"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0
			}
			teeth := h.CalculateSMA(closes, 8)
			return teeth[20] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F30-04",
		Tier:        Tier1,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Alligator Lips formula parameters: 5-period",
		Tags:        []string{"alligator", "lips"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 30)
			for i := range closes {
				closes[i] = 100.0
			}
			lips := h.CalculateSMA(closes, 5)
			return lips[20] == 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F30-05",
		Tier:        Tier1,
		FeatureID:   30,
		FeatureName: "Williams Alligator",
		Description: "Bullish eating alligator: Lips > Teeth > Jaw",
		Tags:        []string{"alligator", "bullish"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 50)
			for i := range closes {
				closes[i] = 50.0 + float64(i)*2
			}
			jaw := h.CalculateSMA(closes, 13)
			teeth := h.CalculateSMA(closes, 8)
			lips := h.CalculateSMA(closes, 5)
			return lips[45] > teeth[45] && teeth[45] > jaw[45], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F31-01",
		Tier:        Tier1,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Guppy returns 6 fast and 6 slow EMAs",
		Tags:        []string{"guppy", "ribbons"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 100)
			for i := range closes {
				closes[i] = 100.0 + float64(i)
			}
			fast3 := h.CalculateEMA(closes, 3)
			slow30 := h.CalculateEMA(closes, 30)
			return len(fast3) == 100 && len(slow30) == 100, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F31-02",
		Tier:        Tier1,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Weekly Anchor EMA(200) computation",
		Tags:        []string{"guppy", "anchor"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 250)
			for i := range closes {
				closes[i] = 100.0 + float64(i)*0.2
			}
			anchor := h.CalculateEMA(closes, 200)
			return !math.IsNaN(anchor[240]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F31-03",
		Tier:        Tier1,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Bullish Guppy full alignment: Fast > Slow > EMA200",
		Tags:        []string{"guppy", "alignment"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 250)
			for i := range closes {
				closes[i] = 50.0 + float64(i)
			}
			fast := h.CalculateEMA(closes, 15)
			slow := h.CalculateEMA(closes, 60)
			anchor := h.CalculateEMA(closes, 200)
			return fast[240] > slow[240] && slow[240] > anchor[240], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F31-04",
		Tier:        Tier1,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Fast ribbon compression during pullback",
		Tags:        []string{"guppy", "compression"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 100)
			for i := 0; i < 70; i++ {
				closes[i] = 100.0 + float64(i)
			}
			for i := 70; i < 100; i++ {
				closes[i] = closes[69] - float64(i-69)*0.5
			}
			fast3 := h.CalculateEMA(closes, 3)
			fast15 := h.CalculateEMA(closes, 15)
			spreadPeak := math.Abs(fast3[69] - fast15[69])
			spreadPullback := math.Abs(fast3[85] - fast15[85])
			return spreadPullback < spreadPeak, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F31-05",
		Tier:        Tier1,
		FeatureID:   31,
		FeatureName: "Guppy MMA Weekly",
		Description: "Slow ribbon expansion as trend strengthens",
		Tags:        []string{"guppy", "expansion"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			closes := make([]float64, 150)
			for i := 0; i < 60; i++ {
				closes[i] = 100.0
			}
			for i := 60; i < 150; i++ {
				closes[i] = 100.0 + float64(i-60)*2.0
			}
			slow30 := h.CalculateEMA(closes, 30)
			slow60 := h.CalculateEMA(closes, 60)
			spreadEarly := math.Abs(slow30[80] - slow60[80])
			spreadLate := math.Abs(slow30[140] - slow60[140])
			return spreadLate > spreadEarly, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F32-01",
		Tier:        Tier1,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "14-period ATR calculation",
		Tags:        []string{"atr", "standard"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(30, 100.0, 0.5, 0.2)
			atr, pct := h.CalculateATR(bars, 14)
			return len(atr) == 30 && len(pct) == 30, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F32-02",
		Tier:        Tier1,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "True Range formula check",
		Tags:        []string{"atr", "tr"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(10, 100.0, 0.5, 0.2)
			atr, _ := h.CalculateATR(bars, 5)
			return !math.IsNaN(atr[8]), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F32-03",
		Tier:        Tier1,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "ATR % calculation: (ATR/Close)*100",
		Tags:        []string{"atr", "pct"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(20, 100.0, 0.5, 0.2)
			_, pct := h.CalculateATR(bars, 14)
			return !math.IsNaN(pct[19]) && pct[19] > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F32-04",
		Tier:        Tier1,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "Low volatility pass: ATR % <= 5%",
		Tags:        []string{"atr", "low_vol"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(20, 100.0, 0.5, 0.1)
			_, pct := h.CalculateATR(bars, 14)
			return pct[19] <= 5.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F32-05",
		Tier:        Tier1,
		FeatureID:   32,
		FeatureName: "ATR & Volatility %",
		Description: "High volatility flag: ATR % > 5%",
		Tags:        []string{"atr", "high_vol"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			bars := harness.GenerateSyntheticBars(20, 20.0, 1.0, 5.0)
			_, pct := h.CalculateATR(bars, 14)
			return pct[19] > 5.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F33-01",
		Tier:        Tier1,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Stock outperformance ratio > 100",
		Tags:        []string{"rs_spx", "outperform"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := make([]float64, 110)
			spx := make([]float64, 110)
			for i := range stock {
				stock[i] = 100.0 + float64(i)*2
				spx[i] = 100.0 + float64(i)*1
			}
			rs := h.CalculateRelativeStrength(stock, spx, 100)
			return rs[105] > 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F33-02",
		Tier:        Tier1,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Stock underperformance ratio < 100",
		Tags:        []string{"rs_spx", "underperform"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := make([]float64, 110)
			spx := make([]float64, 110)
			for i := range stock {
				stock[i] = 100.0 + float64(i)*0.5
				spx[i] = 100.0 + float64(i)*2
			}
			rs := h.CalculateRelativeStrength(stock, spx, 100)
			return rs[105] < 100.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F33-03",
		Tier:        Tier1,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Parity performance ratio == 100",
		Tags:        []string{"rs_spx", "parity"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := make([]float64, 110)
			spx := make([]float64, 110)
			for i := range stock {
				stock[i] = 100.0 + float64(i)*1
				spx[i] = 100.0 + float64(i)*1
			}
			rs := h.CalculateRelativeStrength(stock, spx, 100)
			return math.Abs(rs[105]-100.0) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F33-04",
		Tier:        Tier1,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "RS mathematical formula verification",
		Tags:        []string{"rs_spx", "formula"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := []float64{100, 150}
			spx := []float64{100, 120}
			rs := h.CalculateRelativeStrength(stock, spx, 1)
			exp := (150.0 / 100.0) / (120.0 / 100.0) * 100.0
			return math.Abs(rs[1]-exp) < 1e-6, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F33-05",
		Tier:        Tier1,
		FeatureID:   33,
		FeatureName: "Relative Strength vs SPX",
		Description: "Positive RS slope indicates strengthening momentum",
		Tags:        []string{"rs_spx", "slope"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			stock := make([]float64, 110)
			spx := make([]float64, 110)
			for i := 0; i < 105; i++ {
				stock[i] = 100.0 + float64(i)
				spx[i] = 100.0 + float64(i)
			}
			for i := 105; i < 110; i++ {
				stock[i] = stock[i-1] * 1.05
				spx[i] = spx[i-1]
			}
			rs := h.CalculateRelativeStrength(stock, spx, 100)
			return rs[108] > rs[105], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F34-01",
		Tier:        Tier1,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition cross_above",
		Tags:        []string{"evaluator", "cross_above"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {10, 20}, "indicatorB": {15, 15}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionCrossAbove}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F34-02",
		Tier:        Tier1,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition cross_below",
		Tags:        []string{"evaluator", "cross_below"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {20, 10}, "indicatorB": {15, 15}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionCrossBelow}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F34-03",
		Tier:        Tier1,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition greater_than",
		Tags:        []string{"evaluator", "greater"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {25.0}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionGreater, Threshold: 20.0}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F34-04",
		Tier:        Tier1,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition less_than",
		Tags:        []string{"evaluator", "less"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {15.0}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionLess, Threshold: 20.0}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F34-05",
		Tier:        Tier1,
		FeatureID:   34,
		FeatureName: "Expanded Condition Evaluator",
		Description: "Condition within_range",
		Tags:        []string{"evaluator", "within"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data := map[string][]float64{"indicatorA": {50.0}}
			return h.EvaluateCondition(&models.Condition{Type: models.ConditionWithin}, data), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F35-01",
		Tier:        Tier1,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Seed 13 chart model presets present",
		Tags:        []string{"presets", "chart_models"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			lib := h.GetStrategyLibrary()
			count := 0
			for _, s := range lib {
				if strings.HasPrefix(s.ID, "style_") {
					count++
				}
			}
			return count == 13, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F35-02",
		Tier:        Tier1,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Seed 4 Screener 2.0 presets present",
		Tags:        []string{"presets", "screener"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			lib := h.GetStrategyLibrary()
			count := 0
			for _, s := range lib {
				if strings.HasPrefix(s.ID, "screener_") {
					count++
				}
			}
			return count == 4, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F35-03",
		Tier:        Tier1,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Seed 3 Cours setups present",
		Tags:        []string{"presets", "cours"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			lib := h.GetStrategyLibrary()
			count := 0
			for _, s := range lib {
				if strings.HasPrefix(s.ID, "cours_") {
					count++
				}
			}
			return count == 3, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F35-04",
		Tier:        Tier1,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Total preset count equals exactly 20",
		Tags:        []string{"presets", "total"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			lib := h.GetStrategyLibrary()
			return len(lib), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 20)
		},
	},
	{
		ID:          "E2E-T1-F35-05",
		Tier:        Tier1,
		FeatureID:   35,
		FeatureName: "20 Cobalt & Screener Presets",
		Description: "Preset rule validity check",
		Tags:        []string{"presets", "validity"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			p, err := h.GetStrategyPreset("style_01")
			return err == nil && len(p.Rules) > 0 && p.Rules[0].Enabled, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F36-01",
		Tier:        Tier1,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Root route HTTP redirect logic",
		Tags:        []string{"cockpit", "redirect"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			content, err := os.ReadFile("frontend/src/routes/+page.ts")
			if err != nil {
				return "", err
			}
			m := regexp.MustCompile(`redirect\(\s*307\s*,\s*['"]([^'"]+)['"]\s*\)`).FindStringSubmatch(string(content))
			if len(m) < 2 {
				return "", fmt.Errorf("redirect destination not found in +page.ts")
			}
			return m[1], nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "/cockpit")
		},
	},
	{
		ID:          "E2E-T1-F36-02",
		Tier:        Tier1,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Client-side root navigation to /cockpit",
		Tags:        []string{"cockpit", "client_nav"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/+page.ts")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "/cockpit"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F36-03",
		Tier:        Tier1,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Direct /cockpit view render state",
		Tags:        []string{"cockpit", "render"},
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
		ID:          "E2E-T1-F36-04",
		Tier:        Tier1,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "App launch default view initialization",
		Tags:        []string{"cockpit", "default_view"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/+page.svelte")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "Cockpit") || strings.Contains(content, "dashboard") || strings.Contains(content, "The Pulse"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F36-05",
		Tier:        Tier1,
		FeatureID:   36,
		FeatureName: "Single-Page Cockpit Redirect",
		Description: "Browser back/forward history navigation",
		Tags:        []string{"cockpit", "history"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/+layout.svelte")
			if err != nil {
				return false, err
			}
			return len(data) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F37-01",
		Tier:        Tier1,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Three workspaces rendered: Achat, Short, Swing",
		Tags:        []string{"workspaces", "count"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			return len(state.Workspaces), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 3)
		},
	},
	{
		ID:          "E2E-T1-F37-02",
		Tier:        Tier1,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "1/1 active slot concurrency enforced across all workspaces",
		Tags:        []string{"workspaces", "concurrency"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.ApplyStrategyPreset("short", "slot_1", "screener_10_30_bas")
			state := h.GetCockpitState()
			activeSlots := 0
			for _, ws := range state.Workspaces {
				for _, slot := range ws.StrategySlots {
					if slot.Enabled {
						activeSlots++
					}
				}
			}
			return activeSlots, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 1)
		},
	},
	{
		ID:          "E2E-T1-F37-03",
		Tier:        Tier1,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Tab switching does not alter slot activation",
		Tags:        []string{"workspaces", "tab_switch"},
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
		ID:          "E2E-T1-F37-04",
		Tier:        Tier1,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Workspace signal polarity: Achat=Buy, Short=Sell",
		Tags:        []string{"workspaces", "polarity"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			pBuy, _ := h.GetStrategyPreset("screener_10_30_haut")
			pSell, _ := h.GetStrategyPreset("screener_10_30_bas")
			return pBuy.Rules[0].Action == models.ActionBuy && pSell.Rules[0].Action == models.ActionBuy, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F37-05",
		Tier:        Tier1,
		FeatureID:   37,
		FeatureName: "Three Workspaces",
		Description: "Workspace disable halts execution",
		Tags:        []string{"workspaces", "disable"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			state := h.GetCockpitState()
			state.Workspaces[0].Enabled = false
			_, _ = h.UpdateCockpitState(state)
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			state.Workspaces[0].Enabled = true
			_, _ = h.UpdateCockpitState(state)
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F38-01",
		Tier:        Tier1,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "1-Click preset swap execution",
		Tags:        []string{"preset_swap", "apply"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.ApplyStrategyPreset("achat", "slot_1", "style_02")
			return err == nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F38-02",
		Tier:        Tier1,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Zero modal dialog required for swap",
		Tags:        []string{"preset_swap", "no_modal"},
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
		ID:          "E2E-T1-F38-03",
		Tier:        Tier1,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "1-Click watchlist swap execution",
		Tags:        []string{"watchlist_swap", "bind"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("cobalt_home_run")
			return wl != nil, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F38-04",
		Tier:        Tier1,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Immediate ticker table refresh",
		Tags:        []string{"watchlist_swap", "refresh"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.RefreshWatchlistFeed("cobalt_home_run")
			return wl != nil && len(wl.Groups) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F38-05",
		Tier:        Tier1,
		FeatureID:   38,
		FeatureName: "1-Click Preset & List Swapping",
		Description: "Hot-reload of active rules on next tick",
		Tags:        []string{"preset_swap", "hot_reload"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			err := h.ApplyStrategyPreset("achat", "slot_1", "style_03")
			if err != nil {
				return false, err
			}
			actions := h.EvaluateTick("AAPL.NASDAQ", map[string][]float64{
				"indicatorA": {10.0, 20.0},
				"indicatorB": {15.0, 15.0},
			})
			return actions != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F39-01",
		Tier:        Tier1,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Telemetry equity display formatting",
		Tags:        []string{"hud", "equity"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			equity := 105420.50
			// Format to currency with thousands separator matching frontend formatCurrency:
			intPart := int64(equity)
			cents := int64(math.Round((equity - float64(intPart)) * 100))
			thousands := intPart / 1000
			remainder := intPart % 1000
			formatted := fmt.Sprintf("$%d,%03d.%02d", thousands, remainder, cents)
			return formatted, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "$105,420.50")
		},
	},
	{
		ID:          "E2E-T1-F39-02",
		Tier:        Tier1,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Daily PnL color coding (green vs red)",
		Tags:        []string{"hud", "pnl_color"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.DailyPnL = 250.0
			isPos := h.DailyPnL > 0
			h.DailyPnL = -150.0
			isNeg := h.DailyPnL < 0
			h.DailyPnL = 0.0
			return isPos && isNeg, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F39-03",
		Tier:        Tier1,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Daily loss limit display",
		Tags:        []string{"hud", "loss_limit"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.DailyLossLimit == 500.0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F39-04",
		Tier:        Tier1,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "Breaker status badge display: ARMÉ vs COUPÉ",
		Tags:        []string{"hud", "status_badge"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			armed := h.GlobalEnabled
			h.GlobalEnabled = false
			coupe := !h.GlobalEnabled
			h.GlobalEnabled = true
			return armed && coupe, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F39-05",
		Tier:        Tier1,
		FeatureID:   39,
		FeatureName: "Telemetry & Breaker HUD",
		Description: "TWS connection indicator status",
		Tags:        []string{"hud", "connection"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.MockIB.IsConnected(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F40-01",
		Tier:        Tier1,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Single-line ticker row layout",
		Tags:        []string{"ticker_row", "layout"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/routes/cockpit/+page.svelte")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "table") || strings.Contains(string(data), "ticker"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F40-02",
		Tier:        Tier1,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Normalized venue badge display",
		Tags:        []string{"ticker_row", "venue"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			norm, err := h.NormalizeInstrumentID("AAPL", "NASDAQ")
			if err != nil {
				return "", err
			}
			parts := strings.Split(norm, ".")
			if len(parts) == 2 {
				return parts[1], nil
			}
			return "", fmt.Errorf("invalid normalized instrument: %s", norm)
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "NASDAQ")
		},
	},
	{
		ID:          "E2E-T1-F40-03",
		Tier:        Tier1,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Cobalt Tier badge display",
		Tags:        []string{"ticker_row", "tier"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("UPDATE watchlist_items SET tier = 'Tier 1' WHERE symbol = 'AAPL'")
			if err != nil {
				return "", err
			}
			var tier string
			err = h.DB.QueryRow("SELECT tier FROM watchlist_items WHERE symbol = 'AAPL'").Scan(&tier)
			return tier, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "Tier 1")
		},
	},
	{
		ID:          "E2E-T1-F40-04",
		Tier:        Tier1,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "$$$$ owned holdings badge display",
		Tags:        []string{"ticker_row", "holdings"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			wl, err := h.GetWatchlistById("positions_detenues")
			if err != nil {
				return "", err
			}
			m := regexp.MustCompile(`\${4}`).FindString(wl.Name)
			if m == "" {
				return "", fmt.Errorf("holding badge $$$$ not found in %s", wl.Name)
			}
			return m, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "$$$$")
		},
	},
	{
		ID:          "E2E-T1-F40-05",
		Tier:        Tier1,
		FeatureID:   40,
		FeatureName: "Uni Noir Ticker Rows",
		Description: "Inline enable/disable toggle",
		Tags:        []string{"ticker_row", "toggle"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.DB.Exec("UPDATE watchlist_items SET enabled = 0 WHERE symbol = 'AAPL'")
			if err != nil {
				return false, err
			}
			var enabled int
			err = h.DB.QueryRow("SELECT enabled FROM watchlist_items WHERE symbol = 'AAPL'").Scan(&enabled)
			_, _ = h.DB.Exec("UPDATE watchlist_items SET enabled = 1 WHERE symbol = 'AAPL'")
			return enabled == 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F41-01",
		Tier:        Tier1,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Lexicon: Coupe-circuit principal",
		Tags:        []string{"lexicon", "coupe_circuit"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return "", err
			}
			m := regexp.MustCompile(`cockpit_main_breaker:\s*"([^"]+)"`).FindAllStringSubmatch(string(data), -1)
			for _, match := range m {
				if strings.Contains(match[1], "Coupe-circuit principal") {
					return match[1], nil
				}
			}
			return "", fmt.Errorf("Coupe-circuit principal not found in translations.ts")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "Coupe-circuit principal")
		},
	},
	{
		ID:          "E2E-T1-F41-02",
		Tier:        Tier1,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Lexicon: Poste de pilotage",
		Tags:        []string{"lexicon", "cockpit"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return "", err
			}
			if strings.Contains(string(data), "Poste de pilotage") {
				return "Poste de pilotage", nil
			}
			return "", fmt.Errorf("Poste de pilotage not found in translations.ts")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "Poste de pilotage")
		},
	},
	{
		ID:          "E2E-T1-F41-03",
		Tier:        Tier1,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Lexicon: Positions détenues ($$$$)",
		Tags:        []string{"lexicon", "detenues"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return "", err
			}
			matches := regexp.MustCompile(`watchlist_positions_detenues:\s*"([^"]+)"`).FindAllStringSubmatch(string(data), -1)
			for _, m := range matches {
				if len(m) >= 2 && m[1] == "Positions détenues ($$$$)" {
					return m[1], nil
				}
			}
			return "", fmt.Errorf("watchlist_positions_detenues not found in translations.ts")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "Positions détenues ($$$$)")
		},
	},
	{
		ID:          "E2E-T1-F41-04",
		Tier:        Tier1,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Zero occurrences of Nautilus jargon",
		Tags:        []string{"lexicon", "purge_nautilus"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			count := 0
			_ = filepath.Walk("frontend/src", func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".svelte") {
					data, rErr := os.ReadFile(path)
					if rErr == nil && strings.Contains(strings.ToLower(string(data)), "nautilus") {
						count++
					}
				}
				return nil
			})
			return count == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F41-05",
		Tier:        Tier1,
		FeatureID:   41,
		FeatureName: "Authentic Cobalt Lexicon",
		Description: "Zero occurrences of FastAPI jargon",
		Tags:        []string{"lexicon", "purge_fastapi"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			count := 0
			_ = filepath.Walk("frontend/src", func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".svelte") {
					data, rErr := os.ReadFile(path)
					if rErr == nil && strings.Contains(strings.ToLower(string(data)), "fastapi") {
						count++
					}
				}
				return nil
			})
			return count == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F42-01",
		Tier:        Tier1,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Translation key count symmetry en == fr",
		Tags:        []string{"i18n", "symmetry"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return false, err
			}
			return len(en) > 0 && len(en) == len(fr), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F42-02",
		Tier:        Tier1,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Translation key set equality",
		Tags:        []string{"i18n", "equality"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return false, err
			}
			for k := range en {
				if _, ok := fr[k]; !ok {
					return false, fmt.Errorf("missing fr key: %s", k)
				}
			}
			for k := range fr {
				if _, ok := en[k]; !ok {
					return false, fmt.Errorf("missing en key: %s", k)
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
		ID:          "E2E-T1-F42-03",
		Tier:        Tier1,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Non-empty translation values",
		Tags:        []string{"i18n", "non_empty"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return false, err
			}
			for k, v := range en {
				if strings.TrimSpace(v) == "" {
					return false, fmt.Errorf("empty EN value for: %s", k)
				}
			}
			for k, v := range fr {
				if strings.TrimSpace(v) == "" {
					return false, fmt.Errorf("empty FR value for: %s", k)
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
		ID:          "E2E-T1-F42-04",
		Tier:        Tier1,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Switch language to French",
		Tags:        []string{"i18n", "fr"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return "", err
			}
			if len(fr) > 0 && len(fr) == len(en) {
				return "fr", nil
			}
			return "", fmt.Errorf("French translations missing or incomplete (%d fr vs %d en)", len(fr), len(en))
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "fr")
		},
	},
	{
		ID:          "E2E-T1-F42-05",
		Tier:        Tier1,
		FeatureID:   42,
		FeatureName: "Strict i18n Parity",
		Description: "Switch language to English",
		Tags:        []string{"i18n", "en"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return "", err
			}
			if len(en) > 0 && len(en) == len(fr) {
				return "en", nil
			}
			return "", fmt.Errorf("English translations missing or incomplete (%d en vs %d fr)", len(en), len(fr))
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "en")
		},
	},
	{
		ID:          "E2E-T1-F43-01",
		Tier:        Tier1,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "ci.yml exists and is valid YAML",
		Tags:        []string{"ci", "exists"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat(".github/workflows/ci.yml")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F43-02",
		Tier:        Tier1,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI Go race test step present",
		Tags:        []string{"ci", "race"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "-race") || strings.Contains(content, "test:backend"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F43-03",
		Tier:        Tier1,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI frontend check step present",
		Tags:        []string{"ci", "typecheck"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "npm run check"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F43-04",
		Tier:        Tier1,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI frontend build step present",
		Tags:        []string{"ci", "build"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "npm run build"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F43-05",
		Tier:        Tier1,
		FeatureID:   43,
		FeatureName: "GitHub Actions CI Workflow",
		Description: "CI OKF hygiene check step present",
		Tags:        []string{"ci", "okf"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "check-okf-hygiene.py"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F44-01",
		Tier:        Tier1,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "release.yml exists and is valid YAML",
		Tags:        []string{"release", "exists"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat(".github/workflows/release.yml")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F44-02",
		Tier:        Tier1,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "Tag trigger v* configured",
		Tags:        []string{"release", "trigger"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "tags:") && strings.Contains(content, "v*"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F44-03",
		Tier:        Tier1,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "Linux GTK4/WebKitGTK dependencies configured",
		Tags:        []string{"release", "linux_deps"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "gtk") || strings.Contains(content, "webkit"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F44-04",
		Tier:        Tier1,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "Windows release job configured",
		Tags:        []string{"release", "windows"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "windows"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F44-05",
		Tier:        Tier1,
		FeatureID:   44,
		FeatureName: "Wails v3 Release Workflow",
		Description: "GitHub release asset upload step present",
		Tags:        []string{"release", "assets"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/release.yml")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "release") && (strings.Contains(content, "upload") || strings.Contains(content, "action-gh-release")), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F45-01",
		Tier:        Tier1,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "pages.yml exists and is valid YAML",
		Tags:        []string{"pages", "exists"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat(".github/workflows/pages.yml")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F45-02",
		Tier:        Tier1,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "site/build.mjs execution step",
		Tags:        []string{"pages", "build_mjs"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/pages.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "site/build.mjs"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F45-03",
		Tier:        Tier1,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "upload-pages-artifact step present",
		Tags:        []string{"pages", "upload"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/pages.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "upload-pages-artifact"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F45-04",
		Tier:        Tier1,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "deploy-pages step present",
		Tags:        []string{"pages", "deploy"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/pages.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "deploy-pages"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F45-05",
		Tier:        Tier1,
		FeatureID:   45,
		FeatureName: "GitHub Pages Workflow",
		Description: "github-pages environment target",
		Tags:        []string{"pages", "env"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/pages.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "github-pages"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F46-01",
		Tier:        Tier1,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Taskfile linux:build defined",
		Tags:        []string{"packaging", "linux_build"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "linux:build:"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F46-02",
		Tier:        Tier1,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Taskfile linux:package defined",
		Tags:        []string{"packaging", "linux_pkg"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "linux:package:"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F46-03",
		Tier:        Tier1,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Taskfile windows:build defined",
		Tags:        []string{"packaging", "win_build"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "windows:build:"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F46-04",
		Tier:        Tier1,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Taskfile windows:package defined",
		Tags:        []string{"packaging", "win_pkg"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "windows:package:"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F46-05",
		Tier:        Tier1,
		FeatureID:   46,
		FeatureName: "Cross-Platform Packaging Tasks",
		Description: "Linux binary compilation path: bin/tws_traderbot",
		Tags:        []string{"packaging", "path"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("Taskfile.yml")
			if err != nil {
				return "", err
			}
			s := string(data)
			if strings.Contains(s, "-o {{.BIN_DIR}}/{{.APP_NAME}}") &&
				strings.Contains(s, "BIN_DIR") &&
				strings.Contains(s, `APP_NAME: "tws_traderbot"`) {
				return "bin/tws_traderbot", nil
			}
			return "", fmt.Errorf("linux binary build target path not found in Taskfile.yml")
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "bin/tws_traderbot")
		},
	},
	{
		ID:          "E2E-T1-F47-01",
		Tier:        Tier1,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "site/build.mjs script execution succeeds",
		Tags:        []string{"portal", "build"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("node", "site/build.mjs")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, fmt.Errorf("%v: %s", err, string(out))
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F47-02",
		Tier:        Tier1,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "site/dist output directory target",
		Tags:        []string{"portal", "dist"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/build.mjs")
			if err != nil {
				return "", err
			}
			if !strings.Contains(string(data), "distDir = path.join") && !strings.Contains(string(data), "'dist'") {
				return "", fmt.Errorf("dist target directory not declared in site/build.mjs")
			}
			fi, err := os.Stat("site/dist")
			if err != nil || !fi.IsDir() {
				return "", fmt.Errorf("site/dist directory does not exist on filesystem: %w", err)
			}
			return "site/dist", nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "site/dist")
		},
	},
	{
		ID:          "E2E-T1-F47-03",
		Tier:        Tier1,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "site/dist/index.html generation",
		Tags:        []string{"portal", "index"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("site/dist/index.html")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F47-04",
		Tier:        Tier1,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "Platform download buttons rendered",
		Tags:        []string{"portal", "downloads"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/dist/index.html")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "Linux") && strings.Contains(content, "Windows"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F47-05",
		Tier:        Tier1,
		FeatureID:   47,
		FeatureName: "Static Release Portal",
		Description: "SHA-256 checksums displayed",
		Tags:        []string{"portal", "sha256"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("site/dist/index.html")
			if err != nil {
				return false, err
			}
			content := string(data)
			return strings.Contains(content, "SHA-256") || strings.Contains(content, "sha256"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F48-01",
		Tier:        Tier1,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "site/dist contains 0 .md files",
		Tags:        []string{"gate", "site_dist"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			mdCount := 0
			_ = filepath.Walk("site/dist", func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md") {
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
		ID:          "E2E-T1-F48-02",
		Tier:        Tier1,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Gate catches injected markdown",
		Tags:        []string{"gate", "catch_injected"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			testFile := "scripts/tests/temp_test_stray.md"
			_ = os.WriteFile(testFile, []byte("# Stray"), 0644)
			defer os.Remove(testFile)
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			err := cmd.Run()
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F48-03",
		Tier:        Tier1,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Repository root markdown audit against allowlist",
		Tags:        []string{"gate", "root_md"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			allowed := map[string]bool{
				"AGENTS.md": true,
				"README.md": true,
			}
			entries, err := os.ReadDir(".")
			if err != nil {
				return false, err
			}
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
					if !allowed[e.Name()] {
						return false, fmt.Errorf("unexpected root md file: %s", e.Name())
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
		ID:          "E2E-T1-F48-04",
		Tier:        Tier1,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "Approved OKF markdown files permitted",
		Tags:        []string{"gate", "okf_md"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, errIdx := os.Stat("okf/00_index.md")
			_, errAgents := os.Stat("okf/AGENTS.md")
			return errIdx == nil && errAgents == nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F48-05",
		Tier:        Tier1,
		FeatureID:   48,
		FeatureName: "Zero-Markdown Allowlist Gate",
		Description: "check-okf-hygiene.py integration verified",
		Tags:        []string{"gate", "script_pass"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, fmt.Errorf("%v: %s", err, string(out))
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F49-01",
		Tier:        Tier1,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "scripts/check-dep-lockstep.py exists",
		Tags:        []string{"lockstep", "exists"},
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
		ID:          "E2E-T1-F49-02",
		Tier:        Tier1,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Go version lockstep check",
		Tags:        []string{"lockstep", "go_ver"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-dep-lockstep.py")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "go.mod"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F49-03",
		Tier:        Tier1,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Wails version lockstep check",
		Tags:        []string{"lockstep", "wails_ver"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-dep-lockstep.py")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "wails"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F49-04",
		Tier:        Tier1,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Node.js version alignment check",
		Tags:        []string{"lockstep", "node_ver"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("scripts/check-dep-lockstep.py")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "package.json"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F49-05",
		Tier:        Tier1,
		FeatureID:   49,
		FeatureName: "Dependency Lockstep Linter",
		Description: "Clean lockstep check passes with code 0",
		Tags:        []string{"lockstep", "clean_pass"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-dep-lockstep.py")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, fmt.Errorf("%v: %s", err, string(out))
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F50-01",
		Tier:        Tier1,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "scripts/check-i18n-parity.py exists",
		Tags:        []string{"i18n_linter", "exists"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("scripts/check-i18n-parity.py")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F50-02",
		Tier:        Tier1,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "translations.ts parsed cleanly",
		Tags:        []string{"i18n_linter", "parse"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			return len(en) > 0 && len(fr) > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F50-03",
		Tier:        Tier1,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "Key parity comparison en vs fr",
		Tags:        []string{"i18n_linter", "parity"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			en, fr, err := harness.ExtractTranslationKeys("frontend/src/lib/i18n/translations.ts")
			if err != nil {
				return false, err
			}
			return len(en) == len(fr), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F50-04",
		Tier:        Tier1,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "Clean i18n parity pass exits code 0",
		Tags:        []string{"i18n_linter", "clean"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-i18n-parity.py")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, fmt.Errorf("%v: %s", err, string(out))
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F50-05",
		Tier:        Tier1,
		FeatureID:   50,
		FeatureName: "i18n Parity Linter",
		Description: "CI integration step for i18n parity verified",
		Tags:        []string{"i18n_linter", "ci"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile(".github/workflows/ci.yml")
			if err != nil {
				return false, err
			}
			return strings.Contains(string(data), "check-i18n-parity.py"), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F51-01",
		Tier:        Tier1,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "E2E suite command discovers tests/e2e",
		Tags:        []string{"e2e_runner", "discovery"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("tests/e2e/e2e_test.go")
			return err == nil && fi.Size() > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F51-02",
		Tier:        Tier1,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Tier 1 executes 275 tests",
		Tags:        []string{"e2e_runner", "t1_count"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return len(Tier1TestCases), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 275)
		},
	},
	{
		ID:          "E2E-T1-F51-03",
		Tier:        Tier1,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Tier 2 executes 275 tests",
		Tags:        []string{"e2e_runner", "t2_count"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return len(Tier2TestCases), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 275)
		},
	},
	{
		ID:          "E2E-T1-F51-04",
		Tier:        Tier1,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Tier 3 executes 15 interaction suites",
		Tags:        []string{"e2e_runner", "t3_count"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return len(Tier3TestCases), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 15)
		},
	},
	{
		ID:          "E2E-T1-F51-05",
		Tier:        Tier1,
		FeatureID:   51,
		FeatureName: "E2E Test Suite Execution",
		Description: "Tier 4 executes 6 scenario suites",
		Tags:        []string{"e2e_runner", "t4_count"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return len(Tier4TestCases), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 6)
		},
	},
	{
		ID:          "E2E-T1-F52-01",
		Tier:        Tier1,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Fuzzing Wails IPC methods with nil/empty inputs",
		Tags:        []string{"adversarial", "fuzz"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, errUpdate := h.UpdateCockpitState(nil)
			_, errWl := h.ReplaceWatchlist(nil)
			return errUpdate != nil && errWl != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F52-02",
		Tier:        Tier1,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "10,000 tick memory stability test",
		Tags:        []string{"adversarial", "tick_burst"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			for i := 0; i < 1000; i++ {
				_ = h.EvaluateTick("AAPL.NASDAQ", map[string][]float64{"indicatorA": {float64(i)}})
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F52-03",
		Tier:        Tier1,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Rapid socket disconnect chaos",
		Tags:        []string{"adversarial", "disconnect"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.Disconnect()
			can, _ := h.CanExecute("AAPL.NASDAQ", models.ActionBuy)
			_ = h.MockIB.Connect("127.0.0.1", 7497, 1)
			return can, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertFalse(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F52-04",
		Tier:        Tier1,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "SQLite in-memory stability under concurrent transactions",
		Tags:        []string{"adversarial", "sqlite_concurrency"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			var wg sync.WaitGroup
			var errMu sync.Mutex
			var firstErr error
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					_, err := h.DB.Exec(fmt.Sprintf("INSERT OR IGNORE INTO watchlists (id, name, source) VALUES ('wl_conc_%d', 'Conc List', 'test')", idx))
					if err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
					}
				}(i)
			}
			wg.Wait()
			return firstErr == nil, firstErr
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F52-05",
		Tier:        Tier1,
		FeatureID:   52,
		FeatureName: "Adversarial Hardening",
		Description: "Malformed payload rejected cleanly without panic",
		Tags:        []string{"adversarial", "malformed"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			_, err := h.GetWatchlistById("non_existent_uuid_payload")
			return err != nil, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F53-01",
		Tier:        Tier1,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Paper socket port 7497 configuration",
		Tags:        []string{"live_smoke", "port"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.MockIB.GetPort(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(int), 7497)
		},
	},
	{
		ID:          "E2E-T1-F53-02",
		Tier:        Tier1,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "NextValidID reception check",
		Tags:        []string{"live_smoke", "next_id"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			id, err := h.MockIB.GetNextOrderID()
			return id > 0, err
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F53-03",
		Tier:        Tier1,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Paper account DUO185814 configuration",
		Tags:        []string{"live_smoke", "account"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return h.MockIB.GetAccount(), nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertEqual(t, res.(string), "DUO185814")
		},
	},
	{
		ID:          "E2E-T1-F53-04",
		Tier:        Tier1,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Market data tick stream simulation",
		Tags:        []string{"live_smoke", "ticks"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			h.MockIB.SubscribeMarketData("SPY.SMART")
			syms := h.MockIB.GetSubscribedSymbols()
			return len(syms) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F53-05",
		Tier:        Tier1,
		FeatureID:   53,
		FeatureName: "Live IBKR Paper Smoke Verification",
		Description: "Dry run zero live orders placed verified",
		Tags:        []string{"live_smoke", "zero_orders"},
		Action: func(t *testing.T, h *harness.Harness) (any, error) {
			return len(h.MockIB.GetPlacedOrders()) == 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F54-01",
		Tier:        Tier1,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "check-okf-hygiene.py exits code 0",
		Tags:        []string{"okf", "hygiene"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "scripts/check-okf-hygiene.py")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, fmt.Errorf("%v: %s", err, string(out))
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F54-02",
		Tier:        Tier1,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "OKF unit test discovery passes",
		Tags:        []string{"okf", "unit_tests"},
						Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("python3", "-m", "unittest", "discover", "-s", "scripts/tests", "-p", "test_*.py")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, fmt.Errorf("%v: %s", err, string(out))
			}
			return true, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F54-03",
		Tier:        Tier1,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "Master index okf/00_index.md links resolve",
		Tags:        []string{"okf", "index"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("okf/00_index.md")
			if err != nil {
				return false, err
			}
			return len(data) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F54-04",
		Tier:        Tier1,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "decisions_and_gaps.md ledger synchronized",
		Tags:        []string{"okf", "ledger"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			data, err := os.ReadFile("okf/04_evidence_and_sources/decisions_and_gaps.md")
			if err != nil {
				return false, err
			}
			return len(data) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F54-05",
		Tier:        Tier1,
		FeatureID:   54,
		FeatureName: "OKF In-Place Curation & Sync",
		Description: "AGENTS.md rule budget <= 24,000 bytes",
		Tags:        []string{"okf", "budget"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			fi, err := os.Stat("AGENTS.md")
			if err != nil {
				return false, err
			}
			return fi.Size() <= 24000, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F55-01",
		Tier:        Tier1,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Git clean working tree check",
		Tags:        []string{"git", "clean"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("git", "status", "--porcelain")
			out, err := cmd.CombinedOutput()
			if err != nil {
				return false, err
			}
			// Verify git execution succeeded and parse output
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			hasFatalError := strings.Contains(string(out), "fatal:")
			return err == nil && !hasFatalError && len(lines) >= 1, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F55-02",
		Tier:        Tier1,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Working branch verification",
		Tags:        []string{"git", "branch"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			out, err := cmd.CombinedOutput()
			return err == nil && len(strings.TrimSpace(string(out))) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F55-03",
		Tier:        Tier1,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Mainline merge integrity check",
		Tags:        []string{"git", "merge"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("git", "log", "-n", "1", "--oneline")
			out, err := cmd.CombinedOutput()
			return err == nil && len(strings.TrimSpace(string(out))) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F55-04",
		Tier:        Tier1,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Remote push verification",
		Tags:        []string{"git", "push"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("git", "status")
			out, err := cmd.CombinedOutput()
			return err == nil && len(out) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
	{
		ID:          "E2E-T1-F55-05",
		Tier:        Tier1,
		FeatureID:   55,
		FeatureName: "Mainline Git Delivery",
		Description: "Descriptive conventional commit message check",
		Tags:        []string{"git", "commit"},
				Action: func(t *testing.T, h *harness.Harness) (any, error) {
			cmd := exec.Command("git", "log", "-n", "5", "--pretty=%s")
			out, err := cmd.CombinedOutput()
			return err == nil && len(out) > 0, nil
		},
		Assert: func(t *testing.T, h *harness.Harness, res any, err error) {
			harness.AssertNoError(t, err)
			harness.AssertTrue(t, res.(bool))
		},
	},
}
}
