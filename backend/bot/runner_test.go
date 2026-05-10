package bot

import (
	"encoding/json"
	"os"
	"testing"
	"tws_traderbot/backend/models"
    "time"
    "fmt"
    "github.com/scmhub/ibapi"
)

func TestFullRunnerFlow(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping full runner flow test in CI environment")
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
					Threshold:       1000.0, // generous pass
					LookbackPeriods: 2,
				},
				Action:  models.ActionBuy,
				Enabled: true,
			},
		},
	}

	b, _ := json.Marshal(strat)
	_ = os.WriteFile("dummy_strat.json", b, 0644)
	defer os.Remove("dummy_strat.json")

	runner := NewRunner()
	err := runner.Start("dummy_strat.json")
	if err != nil {
		t.Skipf("TWS not available, skipping full runner flow test: %v", err)
	}

    // Now Mock pushing request
	fmt.Println("Requesting historical data for SPY (conId: 756733) via TWS")
	contract := &ibapi.Contract{
		Symbol:   "SPY",
		SecType:  "STK",
		Exchange: "SMART",
		Currency: "USD",
	}

	// Request historical data
	// func (c *EClient) ReqHistoricalData(reqID int64, contract *Contract, endDateTime string, duration string, barSize string, whatToShow string, useRTH bool, formatDate int, keepUpToDate bool, chartOptions []TagValue)
	runner.Client.client.ReqHistoricalData(1, contract, "", "1 D", "1 hour", "TRADES", true, 1, false, nil)
	
    // Let TWS return data 
    time.Sleep(6 * time.Second)

	runner.Stop()
}
