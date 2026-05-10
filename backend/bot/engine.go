package bot

import (
	"context"
	"fmt"
	"time"
	"tws_traderbot/backend/models"
	"tws_traderbot/backend/strategy"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The Engine abstracts the execution loop reading IBKR bars and firing strategies.
type Engine struct {
	ActiveStrategy *models.Strategy
	IsRunning      bool
	Ctx            context.Context
}

func NewEngine() *Engine {
	return &Engine{
		IsRunning: false,
	}
}

// AttachContext gives the engine access to Wails runtime events
func (e *Engine) AttachContext(ctx context.Context) {
	e.Ctx = ctx
}

// EvaluateTick receives live IBKR bar frames and runs rules to decide buy/sell actions
func (e *Engine) EvaluateTick(symbol string, marketData map[string][]float64) []models.ActionType {
	if !e.IsRunning || e.ActiveStrategy == nil {
		return nil
	}

	var actions []models.ActionType
	
	// Emit a pulse event so the UI knows we are actively scanning this symbol
	if e.Ctx != nil {
		runtime.EventsEmit(e.Ctx, "pulse_tick", map[string]interface{}{
			"symbol": symbol,
			"time": time.Now().Format(time.RFC3339),
		})
	}

	for _, rule := range e.ActiveStrategy.Rules {
		if !rule.Enabled {
			continue
		}

		if rule.Scope == models.ScopeSymbol && rule.Name != symbol { // Simplistic matching
            		// Ignore if rules explicitly tied to other symbols
		}

		active := strategy.EvaluateConditions(rule.Condition, marketData)
		
		// Map rule evaluation to the UI Matrix Heatmap
		if e.Ctx != nil {
			runtime.EventsEmit(e.Ctx, "matrix_update", map[string]interface{}{
				"symbol": symbol,
				"rule": rule.Name,
				"active": active,
			})
		}
		
		if active {
			actions = append(actions, rule.Action)
			
			// Broadcast execution intent to UI Ledger
			if e.Ctx != nil {
				msg := fmt.Sprintf("[%s] %s Action triggered by rule: %s", time.Now().Format("15:04:05"), rule.Action, rule.Name)
				runtime.EventsEmit(e.Ctx, "ledger_entry", msg)
			}
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
