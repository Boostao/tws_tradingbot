package models

import (
	"fmt"
	"strings"
)

type RuleScope string

const (
	ScopeGlobal     RuleScope = "global"
	ScopeSymbol     RuleScope = "symbol"
	ScopeInstrument RuleScope = "instrument"
	ScopePerTicker  RuleScope = "per_ticker"
)

type ActionType string

const (
	ActionBuy    ActionType = "buy"
	ActionSell   ActionType = "sell"
	ActionFilter ActionType = "filter"
	ActionAlert  ActionType = "alert"
)

type ConditionType string

const (
	ConditionCrossAbove   ConditionType = "cross_above"
	ConditionCrossesAbove ConditionType = "crosses_above"
	ConditionCrossBelow   ConditionType = "cross_below"
	ConditionCrossesBelow ConditionType = "crosses_below"
	ConditionGreater      ConditionType = "greater_than"
	ConditionLess         ConditionType = "less_than"
	ConditionSlopeAbove   ConditionType = "slope_above"
	ConditionSlopeBelow   ConditionType = "slope_below"
	ConditionWithin       ConditionType = "within_range"
)

type IndicatorType string

const (
	IndicatorPrice               IndicatorType = "price"
	IndicatorVolume              IndicatorType = "volume"
	IndicatorEMA                 IndicatorType = "ema"
	IndicatorSMA                 IndicatorType = "sma"
	IndicatorRSI                 IndicatorType = "rsi"
	IndicatorMACD                IndicatorType = "macd"
	IndicatorVIX                 IndicatorType = "vix"
	IndicatorTime                IndicatorType = "time"
	IndicatorBollinger           IndicatorType = "bollinger"
	IndicatorStochastic          IndicatorType = "stochastic"
	IndicatorOBV                 IndicatorType = "obv"
	IndicatorAlligator           IndicatorType = "alligator"
	IndicatorHeikinAshi          IndicatorType = "heikin_ashi"
	IndicatorRVOL                IndicatorType = "rvol"
	IndicatorGuppy               IndicatorType = "guppy"
	IndicatorATR                 IndicatorType = "atr"
	IndicatorRelativeStrength    IndicatorType = "relative_strength"
	IndicatorHigh52W             IndicatorType = "high_52w"
	IndicatorDividendYield       IndicatorType = "dividend_yield"
	IndicatorPERatio             IndicatorType = "pe_ratio"
	IndicatorRelativePerformance IndicatorType = "relative_performance"
	IndicatorMLSignal            IndicatorType = "ml_signal"
)

type TimeframeUnit string

const (
	TimeframeM1  TimeframeUnit = "1m"
	TimeframeM5  TimeframeUnit = "5m"
	TimeframeM15 TimeframeUnit = "15m"
	TimeframeM30 TimeframeUnit = "30m"
	TimeframeH1  TimeframeUnit = "1h"
	TimeframeH4  TimeframeUnit = "4h"
	TimeframeD1  TimeframeUnit = "1d"
	TimeframeW1  TimeframeUnit = "1w"
	TimeframeMN1 TimeframeUnit = "1M"
)

// AllTimeframes returns all 9 canonical Cobalt intervals in ascending order
func AllTimeframes() []TimeframeUnit {
	return []TimeframeUnit{
		TimeframeM1, TimeframeM5, TimeframeM15, TimeframeM30,
		TimeframeH1, TimeframeH4, TimeframeD1, TimeframeW1, TimeframeMN1,
	}
}

// ToIBKRBarSize maps a Cobalt timeframe to the exact TWS API bar size string
func (tf TimeframeUnit) ToIBKRBarSize() string {
	switch tf {
	case TimeframeM1:
		return "1 min"
	case TimeframeM5:
		return "5 mins"
	case TimeframeM15:
		return "15 mins"
	case TimeframeM30:
		return "30 mins"
	case TimeframeH1:
		return "1 hour"
	case TimeframeH4:
		return "4 hours"
	case TimeframeD1:
		return "1 day"
	case TimeframeW1:
		return "1 week"
	case TimeframeMN1:
		return "1 month"
	default:
		return "1 day"
	}
}

// DefaultIBKRDuration returns recommended historical duration string to cover warm-up periods
func (tf TimeframeUnit) DefaultIBKRDuration() string {
	switch tf {
	case TimeframeM1:
		return "1 D"
	case TimeframeM5:
		return "5 D"
	case TimeframeM15:
		return "15 D"
	case TimeframeM30:
		return "1 M"
	case TimeframeH1:
		return "2 M"
	case TimeframeH4:
		return "6 M"
	case TimeframeD1:
		return "2 Y"
	case TimeframeW1:
		return "5 Y"
	case TimeframeMN1:
		return "10 Y"
	default:
		return "1 Y"
	}
}

// ParseTimeframe parses a string into TimeframeUnit with lenient normalization
func ParseTimeframe(s string) (TimeframeUnit, error) {
	norm := strings.TrimSpace(s)
	switch norm {
	case "1m", "1M_min", "1min":
		return TimeframeM1, nil
	case "5m", "5min":
		return TimeframeM5, nil
	case "15m", "15min":
		return TimeframeM15, nil
	case "30m", "30min":
		return TimeframeM30, nil
	case "1h", "60m":
		return TimeframeH1, nil
	case "4h", "240m":
		return TimeframeH4, nil
	case "1d", "D", "1D":
		return TimeframeD1, nil
	case "1w", "W", "1W":
		return TimeframeW1, nil
	case "1M", "M", "1MO":
		return TimeframeMN1, nil
	default:
		return TimeframeD1, fmt.Errorf("unrecognized timeframe: %s", s)
	}
}

type Indicator struct {
	Type      IndicatorType          `json:"type"`
	Length    *int                   `json:"length,omitempty"`
	Period    int                    `json:"period,omitempty"`
	Timeframe TimeframeUnit          `json:"timeframe,omitempty"`
	Source    string                 `json:"source,omitempty"`
	Symbol    *string                `json:"symbol,omitempty"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Component *string                `json:"component,omitempty"`
}

func (ind *Indicator) GetComponent() string {
	if ind == nil || ind.Component == nil || *ind.Component == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(*ind.Component))
}

func (ind *Indicator) GetSource() string {
	if ind == nil || ind.Source == "" {
		return "close"
	}
	return strings.ToLower(strings.TrimSpace(ind.Source))
}

func (ind *Indicator) GetPeriod(defaultPeriod int) int {
	if ind == nil {
		return defaultPeriod
	}
	if ind.Period > 0 {
		return ind.Period
	}
	if ind.Length != nil && *ind.Length > 0 {
		return *ind.Length
	}
	if ind.Params != nil {
		if p, ok := ind.Params["period"].(float64); ok && p > 0 {
			return int(p)
		}
		if p, ok := ind.Params["period"].(int); ok && p > 0 {
			return p
		}
		if l, ok := ind.Params["length"].(float64); ok && l > 0 {
			return int(l)
		}
		if l, ok := ind.Params["length"].(int); ok && l > 0 {
			return l
		}
	}
	return defaultPeriod
}

func (ind *Indicator) GetTimeframe(defaultTf TimeframeUnit) TimeframeUnit {
	if ind == nil || ind.Timeframe == "" {
		return defaultTf
	}
	return ind.Timeframe
}

func (ind *Indicator) CacheKey(symbol string) string {
	comp := ind.GetComponent()
	p := ind.GetPeriod(0)
	tf := ind.GetTimeframe(TimeframeD1)
	src := ind.GetSource()
	return fmt.Sprintf("%s:%s:%s:%d:%s:%s", symbol, tf, ind.Type, p, src, comp)
}

type Condition struct {
	Type            ConditionType `json:"type"`
	IndicatorA      *Indicator    `json:"indicator_a,omitempty"`
	IndicatorB      *Indicator    `json:"indicator_b,omitempty"`
	Threshold       float64       `json:"threshold,omitempty"`
	LookbackPeriods int           `json:"lookback_periods,omitempty"`
	RangeStart      string        `json:"range_start,omitempty"`
	RangeEnd        string        `json:"range_end,omitempty"`
}

type Rule struct {
	ID          string     `json:"id,omitempty"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	Scope       RuleScope  `json:"scope"`
	Condition   *Condition `json:"condition"`
	Action      ActionType `json:"action"`
	Enabled     bool       `json:"enabled"`
	Priority    int        `json:"priority,omitempty"`
}

type Strategy struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Version           string   `json:"version,omitempty"`
	Description       string   `json:"description,omitempty"`
	Tickers           []string `json:"tickers,omitempty"`
	Rules             []Rule   `json:"rules"`
	InitialCapital    float64  `json:"initial_capital,omitempty"`
	MaxPositions      int      `json:"max_positions,omitempty"`
	PositionSizeMode  string   `json:"position_size_mode,omitempty"`
	PositionSizeValue float64  `json:"position_size_value,omitempty"`
	CreatedAt         string   `json:"created_at,omitempty"`
	UpdatedAt         string   `json:"updated_at,omitempty"`
}

type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

type RuleSignal struct {
	RuleID    string     `json:"rule_id"`
	RuleName  string     `json:"rule_name"`
	Symbol    string     `json:"symbol"`
	Action    ActionType `json:"action"`
	Active    bool       `json:"active"`
	Priority  int        `json:"priority"`
	Timestamp int64      `json:"timestamp"`
	Message   string     `json:"message,omitempty"`
}
