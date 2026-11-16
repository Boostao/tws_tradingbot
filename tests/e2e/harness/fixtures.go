package harness

import (
	"time"

	"tws_traderbot/backend/models"
)

// OHLCVBar represents a single candlestick bar in a timeframe.
type OHLCVBar struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

// SeededWatchlistDefinition holds metadata and symbols for default seeded watchlists.
type SeededWatchlistDefinition struct {
	ID     string
	Name   string
	Source string
	Items  []models.WatchlistItem
}

// GetDefaultWatchlistSeeds returns definitions of the 5 canonical Cobalt watchlists.
func GetDefaultWatchlistSeeds() []SeededWatchlistDefinition {
	return []SeededWatchlistDefinition{
		{
			ID:     "cobalt_telegram",
			Name:   "Cobalt-TELEGRAM",
			Source: "telegram",
			Items: []models.WatchlistItem{
				{Symbol: "AAPL", Exchange: "NASDAQ", Name: "Apple Inc.", Enabled: true, InstrumentID: "AAPL.NASDAQ"},
				{Symbol: "MSFT", Exchange: "NASDAQ", Name: "Microsoft Corp.", Enabled: true, InstrumentID: "MSFT.NASDAQ"},
				{Symbol: "NVDA", Exchange: "NASDAQ", Name: "NVIDIA Corp.", Enabled: true, InstrumentID: "NVDA.NASDAQ"},
				{Symbol: "SHOP", Exchange: "TO", Name: "Shopify Inc.", Enabled: true, InstrumentID: "SHOP.TO"},
			},
		},
		{
			ID:     "cobalt_home_run",
			Name:   "Cobalt-Home Run",
			Source: "screener",
			Items: []models.WatchlistItem{
				{Symbol: "PLTR", Exchange: "NASDAQ", Name: "Palantir Tech", Enabled: true, InstrumentID: "PLTR.NASDAQ"},
				{Symbol: "APP", Exchange: "NASDAQ", Name: "AppLovin Corp", Enabled: true, InstrumentID: "APP.NASDAQ"},
				{Symbol: "WELL", Exchange: "V", Name: "WELL Health", Enabled: true, InstrumentID: "WELL.V"},
			},
		},
		{
			ID:     "cobalt_marches",
			Name:   "CBLT - MARCHÉS",
			Source: "system",
			Items: []models.WatchlistItem{
				{Symbol: "SPX", Exchange: "IND", Name: "S&P 500 Index", Enabled: true, InstrumentID: "SPX.IND"},
				{Symbol: "QQQ", Exchange: "NASDAQ", Name: "Invesco QQQ Trust", Enabled: true, InstrumentID: "QQQ.NASDAQ"},
				{Symbol: "TSX", Exchange: "IND", Name: "S&P/TSX Composite", Enabled: true, InstrumentID: "TSX.IND"},
				{Symbol: "VIX", Exchange: "IND", Name: "CBOE Volatility Index", Enabled: true, InstrumentID: "VIX.IND"},
			},
		},
		{
			ID:     "premarkets",
			Name:   "Prémarkets",
			Source: "scanner",
			Items: []models.WatchlistItem{
				{Symbol: "TSLA", Exchange: "NASDAQ", Name: "Tesla Inc.", Enabled: true, InstrumentID: "TSLA.NASDAQ"},
				{Symbol: "AMD", Exchange: "NASDAQ", Name: "Advanced Micro Devices", Enabled: true, InstrumentID: "AMD.NASDAQ"},
			},
		},
		{
			ID:     "positions_detenues",
			Name:   "Positions détenues ($$$$)",
			Source: "portfolio",
			Items:  []models.WatchlistItem{}, // Initialized empty for active holdings
		},
	}
}

// GetDefaultStrategySeeds returns all 20 Cobalt presets (13 chart models, 4 Screeners, 3 Cours setups).
func GetDefaultStrategySeeds() []models.Strategy {
	presets := make([]models.Strategy, 0, 20)

	// 13 Chart Models (Styles 1..9, F, etc.)
	chartStyles := []struct {
		ID   string
		Name string
	}{
		{"style_01", "Style 1 - Momentum EMA"},
		{"style_02", "Style 2 - Pullback EMA 10-30"},
		{"style_03", "Style 3 - Rebond Bollinger Bands"},
		{"style_04", "Style 4 - Crossover MACD"},
		{"style_05", "Style 5 - RSI Reversal"},
		{"style_06", "Style 6 - Confluence Triplet (RSI+BB+MACD)"},
		{"style_07", "Style 7 - Alligator Trend Continuation"},
		{"style_08", "Style 8 - Guppy MMA Ribbon Expansion"},
		{"style_09", "Style 9 - Volume Breakout RVOL"},
		{"style_10", "Style 10 - Support Resistance Bounce"},
		{"style_11", "Style 11 - ATR Volatility Expansion"},
		{"style_12", "Style 12 - Heikin-Ashi Trend Pilot"},
		{"style_f", "Style F - Outperformance RS vs SPX"},
	}

	for _, cs := range chartStyles {
		presets = append(presets, models.Strategy{
			ID:          cs.ID,
			Name:        cs.Name,
			Description: "Cobalt Methodology Chart Model " + cs.Name,
			Rules: []models.Rule{
				{
					Name:    cs.Name + " Entry",
					Scope:   models.ScopeGlobal,
					Action:  models.ActionBuy,
					Enabled: true,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeD1,
							Period:    10,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeD1,
							Period:    30,
						},
					},
				},
			},
		})
	}

	// 4 Screener 2.0 Presets
	screenerPresets := []struct {
		ID   string
		Name string
		Cond models.ConditionType
	}{
		{"screener_52w_high", "US 52W High", models.ConditionGreater},
		{"screener_golden_cross", "Golden Cross", models.ConditionCrossAbove},
		{"screener_10_30_haut", "10-30 Haut", models.ConditionCrossAbove},
		{"screener_10_30_bas", "10-30 Bas", models.ConditionCrossBelow},
	}

	for _, sp := range screenerPresets {
		presets = append(presets, models.Strategy{
			ID:          sp.ID,
			Name:        sp.Name,
			Description: "Screener 2.0 Preset " + sp.Name,
			Rules: []models.Rule{
				{
					Name:    sp.Name + " Trigger",
					Scope:   models.ScopeGlobal,
					Action:  models.ActionBuy,
					Enabled: true,
					Condition: &models.Condition{
						Type: sp.Cond,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeD1,
							Period:    50,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeD1,
							Period:    200,
						},
					},
				},
			},
		})
	}

	// 3 Cours-02/06 Setups
	coursSetups := []struct {
		ID   string
		Name string
	}{
		{"cours_weinstein_p2", "Weinstein Phase 2 Breakout"},
		{"cours_revirement_30m", "Revirement 30m"},
		{"cours_jour_suivant", "Jour Suivant Setup"},
	}

	for _, c := range coursSetups {
		presets = append(presets, models.Strategy{
			ID:          c.ID,
			Name:        c.Name,
			Description: "Cobalt Cours Setup " + c.Name,
			Rules: []models.Rule{
				{
					Name:    c.Name + " Setup Rule",
					Scope:   models.ScopeGlobal,
					Action:  models.ActionBuy,
					Enabled: true,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeM5,
							Period:    10,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeM5,
							Period:    30,
						},
					},
				},
			},
		})
	}

	return presets
}

// GenerateSyntheticBars creates a slice of n bars with a given base price, trend slope, and volatility.
func GenerateSyntheticBars(n int, basePrice, slope, volatility float64) []OHLCVBar {
	bars := make([]OHLCVBar, n)
	curr := basePrice
	startTime := time.Now().Add(-time.Duration(n) * 24 * time.Hour)

	for i := 0; i < n; i++ {
		t := startTime.Add(time.Duration(i) * 24 * time.Hour)
		change := slope + (float64(i%5)-2.0)*volatility
		open := curr
		closeP := curr + change
		if closeP < 1.0 {
			closeP = 1.0
		}
		high := open
		if closeP > high {
			high = closeP + volatility
		} else {
			high = open + volatility
		}
		low := open
		if closeP < low {
			low = closeP - volatility
		} else {
			low = open - volatility
		}
		if low < 0.5 {
			low = 0.5
		}

		vol := 100000.0 + float64(i%10)*20000.0

		bars[i] = OHLCVBar{
			Timestamp: t,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeP,
			Volume:    vol,
		}
		curr = closeP
	}
	return bars
}
