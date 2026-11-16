package strategy

import (
	"fmt"
	"time"
	"tws_traderbot/backend/db"
	"tws_traderbot/backend/models"
)

func init() {
	db.StrategyCatalogSeeder = SeedStrategyCatalog
}

// stringPtr returns a pointer to the passed string.
func stringPtr(s string) *string {
	return &s
}

// GetDefaultStrategySeeds returns all 20 canonical Cobalt presets:
// - 13 Chart models (style_01 .. style_12, style_f)
// - 4 Screener 2.0 presets (screener_52w_high, screener_golden_cross, screener_10_30_haut, screener_10_30_bas)
// - 3 Cours setups (cours_weinstein_p2, cours_revirement_30m, cours_jour_suivant)
func GetDefaultStrategySeeds() []models.Strategy {
	return []models.Strategy{
		// -------------------------------------------------------------------------
		// 13 TradingView Chart Models (Lesson 213 & Méthode 8.8)
		// -------------------------------------------------------------------------
		{
			ID:          "style_01",
			Name:        "Cobalt Modèle 1 — Mouvement de prix & Volume",
			Description: "Price breakout above Volume MA(100) on 1D",
			Rules: []models.Rule{
				{
					ID:       "s01_vol_surge",
					Name:     "Breakout Volume & Hausse de Prix",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionGreater,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorVolume,
							Timeframe: models.TimeframeD1,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeD1,
							Period:    100,
							Source:    "volume",
						},
					},
				},
			},
		},
		{
			ID:          "style_02",
			Name:        "Cobalt Modèle 2 — SMA 30-200 (Stage 2 Uptrend)",
			Description: "Price cross above SMA(30) with Price > SMA(200)",
			Rules: []models.Rule{
				{
					ID:       "s02_sma30_cross",
					Name:     "Entrée Croisement SMA 30",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorPrice,
							Timeframe: models.TimeframeD1,
							Source:    "close",
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeD1,
							Period:    30,
						},
					},
				},
			},
		},
		{
			ID:          "style_03",
			Name:        "Cobalt Modèle 3 — Croisement EMA 10-30 + OBV",
			Description: "EMA(10) crosses above EMA(30) with positive OBV slope",
			Rules: []models.Rule{
				{
					ID:       "s03_ema10_30_cross",
					Name:     "EMA 10/30 Crossover Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
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
		},
		{
			ID:          "style_04",
			Name:        "Cobalt Modèle 3G — Williams Alligator & MACD",
			Description: "Alligator Lips crosses above Teeth with positive MACD",
			Rules: []models.Rule{
				{
					ID:       "s04_alligator_cross",
					Name:     "Alligator Lips/Teeth Cross Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorAlligator,
							Timeframe: models.TimeframeD1,
							Component: stringPtr("lips"),
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorAlligator,
							Timeframe: models.TimeframeD1,
							Component: stringPtr("teeth"),
						},
					},
				},
			},
		},
		{
			ID:          "style_05",
			Name:        "Cobalt Modèle 3T — Triple Croisement EMA 9-21-55 & MACD",
			Description: "EMA(9) crosses above EMA(21) with EMA(21) > EMA(55)",
			Rules: []models.Rule{
				{
					ID:       "s05_triple_ema_cross",
					Name:     "EMA 9/21 Triple Cross Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeD1,
							Period:    9,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeD1,
							Period:    21,
						},
					},
				},
			},
		},
		{
			ID:          "style_06",
			Name:        "Cobalt Modèle 4 — SMA 100-150 & Filtre P/E",
			Description: "SMA(100) crosses above SMA(150)",
			Rules: []models.Rule{
				{
					ID:       "s06_sma100_150_cross",
					Name:     "SMA 100/150 Cross Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeD1,
							Period:    100,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeD1,
							Period:    150,
						},
					},
				},
			},
		},
		{
			ID:          "style_07",
			Name:        "Cobalt Modèle 4E — Cassure EMA 100, Volume & MACD",
			Description: "Price crosses above EMA(100) with volume confirmation",
			Rules: []models.Rule{
				{
					ID:       "s07_ema100_breakout",
					Name:     "EMA 100 Breakout Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorPrice,
							Timeframe: models.TimeframeD1,
							Source:    "close",
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeD1,
							Period:    100,
						},
					},
				},
			},
		},
		{
			ID:          "style_08",
			Name:        "Cobalt Modèle 5 — Golden Cross / Force Relative SPX",
			Description: "SMA(50) crosses above SMA(200) with RS vs SPX outperformance",
			Rules: []models.Rule{
				{
					ID:       "s08_golden_cross",
					Name:     "Golden Cross Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
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
		},
		{
			ID:          "style_09",
			Name:        "Cobalt Modèle 6 — Suracheté / Survendu (BB + RSI + Stoch)",
			Description: "Stochastic %K crosses %D with RSI < 35 oversold",
			Rules: []models.Rule{
				{
					ID:       "s09_stoch_cross",
					Name:     "Stochastic Reversal Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorStochastic,
							Timeframe: models.TimeframeD1,
							Component: stringPtr("k"),
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorStochastic,
							Timeframe: models.TimeframeD1,
							Component: stringPtr("d"),
						},
					},
				},
			},
		},
		{
			ID:          "style_10",
			Name:        "Cobalt Modèle 7 — Méthode de Guppy MMA Hebdomadaire",
			Description: "Guppy Fast Cluster crosses above Slow Cluster on 1W",
			Rules: []models.Rule{
				{
					ID:       "s10_guppy_cross",
					Name:     "Guppy Fast/Slow Expansion Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorGuppy,
							Timeframe: models.TimeframeW1,
							Component: stringPtr("fast_min"),
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorGuppy,
							Timeframe: models.TimeframeW1,
							Component: stringPtr("slow_max"),
						},
					},
				},
			},
		},
		{
			ID:          "style_11",
			Name:        "Cobalt Modèle 8 — EMAs Long Terme & Rendement Dividende",
			Description: "EMA(150) crosses above EMA(250) on 1D",
			Rules: []models.Rule{
				{
					ID:       "s11_ema150_250_cross",
					Name:     "Long-Term EMA 150/250 Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeD1,
							Period:    150,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeD1,
							Period:    250,
						},
					},
				},
			},
		},
		{
			ID:          "style_12",
			Name:        "Cobalt Modèle 9 — SMA 13-26 Semaines (Weinstein Phase 2)",
			Description: "Weekly SMA(13) crosses above SMA(26) with RS outperformance",
			Rules: []models.Rule{
				{
					ID:       "s12_sma13_26_cross",
					Name:     "Weekly SMA 13/26 Weinstein Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeW1,
							Period:    13,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeW1,
							Period:    26,
						},
					},
				},
			},
		},
		{
			ID:          "style_f",
			Name:        "Cobalt Modèle F — Ratios Fondamentaux, SMA 200 & Filtre Volatilité ATR",
			Description: "Price crosses above SMA(200) with ATR% <= 5.0% volatility guard",
			Rules: []models.Rule{
				{
					ID:       "sf_sma200_rebound",
					Name:     "SMA 200 Rebound Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorPrice,
							Timeframe: models.TimeframeD1,
							Source:    "close",
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorSMA,
							Timeframe: models.TimeframeD1,
							Period:    200,
						},
					},
				},
			},
		},

		// -------------------------------------------------------------------------
		// 4 Screener 2.0 Presets
		// -------------------------------------------------------------------------
		{
			ID:          "screener_52w_high",
			Name:        "Screener 2.0 — Cassure du Sommet 52 Semaines (US 52W High)",
			Description: "Price breakout to new 52-week highs with volume surge",
			Rules: []models.Rule{
				{
					ID:       "scr_52w_high_break",
					Name:     "52W High Breakout Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionGreater,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorPrice,
							Timeframe: models.TimeframeD1,
							Source:    "close",
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorHigh52W,
							Timeframe: models.TimeframeD1,
						},
					},
				},
			},
		},
		{
			ID:          "screener_golden_cross",
			Name:        "Screener 2.0 — Golden Cross 1J (SMA 50/200)",
			Description: "SMA(50) crosses above SMA(200) on daily frame",
			Rules: []models.Rule{
				{
					ID:       "scr_golden_cross_entry",
					Name:     "Golden Cross 1D Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
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
		},
		{
			ID:          "screener_10_30_haut",
			Name:        "Screener 2.0 — Croisement 10-30 Haut (Haussier 1J)",
			Description: "EMA(10) crosses above EMA(30) on daily frame",
			Rules: []models.Rule{
				{
					ID:       "scr_10_30_haut_entry",
					Name:     "EMA 10/30 Haut Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
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
		},
		{
			ID:          "screener_10_30_bas",
			Name:        "Screener 2.0 — Croisement 10-30 Bas (Baissier 1J / Vente à découvert)",
			Description: "EMA(10) crosses below EMA(30) on daily frame (Short)",
			Rules: []models.Rule{
				{
					ID:       "scr_10_30_bas_entry",
					Name:     "EMA 10/30 Bas Short Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionSell,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossBelow,
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
		},

		// -------------------------------------------------------------------------
		// 3 Cours-02/06 Setups
		// -------------------------------------------------------------------------
		{
			ID:          "cours_weinstein_p2",
			Name:        "Cours-02 — Weinstein Phase 2 Breakout",
			Description: "Top-down Phase 2 breakout with daily confirmation",
			Rules: []models.Rule{
				{
					ID:       "c02_weinstein_entry",
					Name:     "Phase 2 Breakout Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
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
		},
		{
			ID:          "cours_revirement_30m",
			Name:        "Cours-06 — Pattern du Revirement Graphique 30 min",
			Description: "MTF: Daily trend + 30m Heikin-Ashi reversal and EMA cross",
			Rules: []models.Rule{
				{
					ID:       "c06_revirement_30m_entry",
					Name:     "30m Reversal Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeM30,
							Period:    9,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeM30,
							Period:    21,
						},
					},
				},
			},
		},
		{
			ID:          "cours_jour_suivant",
			Name:        "Cours-06 — Achat le Jour Suivant un Break Out",
			Description: "Next-day market open continuation after prior day breakout",
			Rules: []models.Rule{
				{
					ID:       "c06_jour_suivant_entry",
					Name:     "Next Day Continuation Entry",
					Scope:    models.ScopeGlobal,
					Action:   models.ActionBuy,
					Enabled:  true,
					Priority: 50,
					Condition: &models.Condition{
						Type: models.ConditionCrossAbove,
						IndicatorA: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeM15,
							Period:    10,
						},
						IndicatorB: &models.Indicator{
							Type:      models.IndicatorEMA,
							Timeframe: models.TimeframeM15,
							Period:    30,
						},
					},
				},
			},
		},
	}
}

// GetDefaultCobaltPresets is an alias to GetDefaultStrategySeeds.
func GetDefaultCobaltPresets() []models.Strategy {
	return GetDefaultStrategySeeds()
}

// SeedStrategyCatalog seeds the 20 presets into SQLite database if fewer than 20 presets are stored.
func SeedStrategyCatalog(database *db.Database) error {
	if database == nil {
		return fmt.Errorf("database cannot be nil")
	}

	existing, err := database.LoadStrategyLibrary()
	if err == nil && len(existing) >= 20 {
		return nil // Already seeded
	}

	presets := GetDefaultStrategySeeds()
	for _, strat := range presets {
		s := strat
		summary := &models.CockpitStrategySummary{
			ID:               s.ID,
			Name:             s.Name,
			RuleCount:        len(s.Rules),
			EnabledRuleCount: len(s.Rules),
			Source:           "preset",
			UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
		}
		if err := database.SaveStrategyPreset(&s, summary); err != nil {
			return fmt.Errorf("failed to seed preset %s: %w", s.ID, err)
		}
	}
	return nil
}
