package bot

import (
	"os"
	"testing"
	"time"
	"tws_traderbot/backend/models"

	"github.com/scmhub/ibapi"
)

// TestRunner_HeadlessMockFlow tests strategy loading, engine evaluation,
// and simulated order execution without any external TWS network socket.
func TestRunner_HeadlessMockFlow(t *testing.T) {
	eng := NewEngine()
	eng.SetGlobalEnabled(true)
	eng.SetWatchlistItem(&models.WatchlistItem{
		Symbol:      "SYM_1",
		Exchange:    "SMART",
		Venue:       "SMART",
		Enabled:     true,
		IsOrderable: true,
		SizeMode:    "shares",
		SizeValue:   100.0,
	})
	eng.SetLatestPrice("SYM_1", 100.0)
	eng.SetTimeProvider(func() time.Time {
		// Wednesday 10:00 AM ET (RTH)
		return time.Date(2026, 11, 18, 10, 0, 0, 0, eng.sessionGate.Location())
	})

	strat := &models.Strategy{
		ID:   "strat_mock",
		Name: "Momentum SPY Test",
		Rules: []models.Rule{
			{
				Name:  "Test Buy Rule",
				Scope: models.ScopeGlobal,
				Condition: &models.Condition{
					Type:            models.ConditionSlopeBelow,
					Threshold:       1000.0,
					LookbackPeriods: 2,
				},
				Action:  models.ActionBuy,
				Enabled: true,
			},
		},
	}

	if err := eng.StartBot(strat); err != nil {
		t.Fatalf("failed to start engine: %v", err)
	}
	defer eng.StopBot()

	wrapper := &IBWrapper{
		engine:     eng,
		marketData: make(map[string][]float64),
	}
	wrapper.nextOrderID.Store(500)

	// Push synthetic bars into reqID 1
	wrapper.HistoricalData(1, &ibapi.Bar{Close: 96, Open: 95})
	wrapper.HistoricalData(1, &ibapi.Bar{Close: 98, Open: 97})
	wrapper.HistoricalData(1, &ibapi.Bar{Close: 102, Open: 99})
	wrapper.HistoricalDataEnd(1, "", "")

	// Verify order was placed
	if wrapper.nextOrderID.Load() <= 500 {
		t.Errorf("expected order ID to be incremented after rule trigger, got %d", wrapper.nextOrderID.Load())
	}
}

func TestFullRunnerFlow(t *testing.T) {
	if os.Getenv("TWS_LIVE_TEST") != "1" {
		t.Skip("Skipping full live runner flow test; set TWS_LIVE_TEST=1 to enable")
	}

	strat := models.Strategy{
		ID:   "strat_prod",
		Name: "Momentum SPY",
		Rules: []models.Rule{
			{
				Name:  "Test",
				Scope: models.ScopeGlobal,
				Condition: &models.Condition{
					Type:            models.ConditionSlopeBelow,
					Threshold:       1000.0,
					LookbackPeriods: 2,
				},
				Action:  models.ActionBuy,
				Enabled: true,
			},
		},
	}

	runner := NewRunner()
	_ = runner.Engine.StartBot(&strat)
	runner.Stop()
}
