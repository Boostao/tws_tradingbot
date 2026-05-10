package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"tws_traderbot/backend/models"
)

// Runner ties the IBKR Client and the Engine together
type Runner struct {
	Engine *Engine
	Client *IBKRClient
}

func NewRunner() *Runner {
	engine := NewEngine()
	return &Runner{
		Engine: engine,
		Client: NewIBKRClient(engine),
	}
}

// Start reads a strategy from json, connects to IBKR, and wires them up.
func (r *Runner) Start(strategyPath string) error {
	// 1. Load Strategy
	data, err := os.ReadFile(strategyPath)
	if err != nil {
		return fmt.Errorf("failed to read strategy config: %v", err)
	}

	var strat models.Strategy
	if err := json.Unmarshal(data, &strat); err != nil {
		return fmt.Errorf("failed to parse strategy config: %v", err)
	}

	// 2. Connect to IBKR (For now using 127.0.0.1:7497; can be tied to UI payload later)
	if err := r.Client.Connect("127.0.0.1", 7497, 1); err != nil {
		return err
	}

	// Wire up the engine to the IB wrapper
	r.Client.wrapper.engine = r.Engine

	// 3. Start Bot Logic
	if err := r.Engine.StartBot(&strat); err != nil {
		return err
	}

	fmt.Printf("Bot runner successfully initialized with strategy: %s\n", strat.Name)
	return nil
}

func (r *Runner) Stop() {
	r.Engine.StopBot()
	r.Client.Disconnect()
}
