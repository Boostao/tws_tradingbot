package bot

import (
	"fmt"
	"time"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"
)

// The Engine abstracts the execution loop reading IBKR bars and firing strategies.
type Engine struct {
	ActiveStrategy *models.Strategy
	IsRunning      bool
}

func NewEngine() *Engine {
	return &Engine{
		IsRunning: false,
	}
}

// EvaluateTick receives live IBKR bar frames and runs rules to decide buy/sell actions
func (e *Engine) EvaluateTick(symbol string, marketData map[string][]float64) []models.ActionType {
	if !e.IsRunning || e.ActiveStrategy == nil {
		return nil
	}

	var actions []models.ActionType

	for _, rule := range e.ActiveStrategy.Rules {
		if !rule.Enabled {
			continue
		}

		if rule.Scope == models.ScopeSymbol && rule.Name != symbol { // Simplistic matching
            		// Ignore if rules explicitly tied to other symbols
		}

		active := strategy.EvaluateConditions(rule.Condition, marketData)
		if active {
			actions = append(actions, rule.Action)
		}
	}

	return actions
}

// StartBot flips the global execution state to ON
func (e *Engine) StartBot(s *models.Strategy) error {
	if e.IsRunning {
		return fmt.Errorf("bot is already running")
	}
	e.ActiveStrategy = s
	e.IsRunning = true
	fmt.Println("Bot engine started locally at", time.Now())
	return nil
}

// StopBot flips it off gracefully
func (e *Engine) StopBot() {
	e.IsRunning = false
	fmt.Println("Bot engine stopped at", time.Now())
}
