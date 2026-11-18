package api

import (
	"tws_traderbot/backend/bot"
	"tws_traderbot/backend/models"
)

// GetAutopilotState returns the current state of the autopilot system.
func GetAutopilotState(engine *bot.Engine) *models.AutopilotState {
	if engine == nil || engine.AutopilotController == nil {
		return &models.AutopilotState{
			ExecMode:     int(bot.ModeOff),
			ActiveOrders: []models.BrokerOrder{},
			Session:      &models.AutopilotRun{},
		}
	}

	mode, orderIDs, runID, strategyID, isActive := engine.AutopilotController.State()

	openOrders := engine.GetOpenOrders()
	activeOrdersMap := make(map[int64]models.BrokerOrder)
	for _, order := range openOrders {
		activeOrdersMap[order.OrderID] = order
	}

	activeOrders := []models.BrokerOrder{}
	for _, id := range orderIDs {
		if order, ok := activeOrdersMap[id]; ok {
			activeOrders = append(activeOrders, order)
		}
	}

	return &models.AutopilotState{
		ExecMode:     int(mode),
		ActiveOrders: activeOrders,
		Session: &models.AutopilotRun{
			RunID:      runID,
			StrategyID: strategyID,
			IsActive:   isActive,
		},
	}
}
