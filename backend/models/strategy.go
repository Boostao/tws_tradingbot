package models

// Strategy representations ported from Python models.py

type RuleScope string

const (
	ScopeGlobal     RuleScope = "global"
	ScopeSymbol     RuleScope = "symbol"
	ScopeInstrument RuleScope = "instrument"
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
	ConditionCrossAbove ConditionType = "cross_above"
	ConditionCrossBelow ConditionType = "cross_below"
	ConditionGreater    ConditionType = "greater_than"
	ConditionLess       ConditionType = "less_than"
	ConditionSlopeAbove ConditionType = "slope_above"
	ConditionSlopeBelow ConditionType = "slope_below"
	ConditionWithin     ConditionType = "within_range"
)

type IndicatorType string

const (
	IndicatorPrice IndicatorType = "price"
	IndicatorEMA   IndicatorType = "ema"
	IndicatorSMA   IndicatorType = "sma"
	IndicatorRSI   IndicatorType = "rsi"
	IndicatorMACD  IndicatorType = "macd"
	IndicatorVIX   IndicatorType = "vix"
	IndicatorTime  IndicatorType = "time"
)

type TimeframeUnit string

const (
	TimeframeM1  TimeframeUnit = "1m"
	TimeframeM5  TimeframeUnit = "5m"
	TimeframeM15 TimeframeUnit = "15m"
	TimeframeH1  TimeframeUnit = "1h"
	TimeframeD1  TimeframeUnit = "1d"
)

type Indicator struct {
	Type      IndicatorType  `json:"type"`
	Timeframe TimeframeUnit  `json:"timeframe,omitempty"`
	Period    int            `json:"period,omitempty"`
	Source    string         `json:"source,omitempty"`
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
	Name      string     `json:"name"`
	Scope     RuleScope  `json:"scope"`
	Condition *Condition `json:"condition"`
	Action    ActionType `json:"action"`
	Enabled   bool       `json:"enabled"`
}

type Strategy struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Rules       []Rule   `json:"rules"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}
