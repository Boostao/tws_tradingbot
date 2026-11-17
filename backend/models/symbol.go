package models

import (
	"fmt"
	"strings"
)

type NormalizedSymbol struct {
	RawInput          string `json:"raw_input"`
	Symbol            string `json:"symbol"`
	Exchange          string `json:"exchange"`
	Venue             string `json:"venue"`
	InstrumentID      string `json:"instrument_id"`
	Currency          string `json:"currency"`
	IsOrderable       bool   `json:"is_orderable"`
	IsRegimeReference bool   `json:"is_regime_reference"`
}

var macroExchanges = map[string]bool{
	"TVC":        true,
	"SP":         true,
	"DJ":         true,
	"IC":         true,
	"FX":         true,
	"FX_IDC":     true,
	"CAPITALCOM": true,
	"INDEX":      true,
	"BINANCE":    true,
	"COINBASE":   true,
	"KRAKEN":     true,
	"CME_MINI":   true,
	"CME":        true,
}

var tsxIndices = map[string]bool{
	"TSX":  true,
	"TX60": true,
	"TX40": true,
	"TX20": true,
	"JX":   true,
}

var macroSymbols = map[string]bool{
	"US10Y":  true,
	"US02Y":  true,
	"US05Y":  true,
	"US20Y":  true,
	"US30Y":  true,
	"VIX":    true,
	"SPX":    true,
	"DXY":    true,
	"TNX":    true,
	"DJI":    true,
	"IXIC":   true,
	"RUT":    true,
	"BTCUSD": true,
	"ETHUSD": true,
	"SOLUSD": true,
}

// ClassifySymbol determines regime reference vs orderable equity status
func ClassifySymbol(exchange, symbol string) (isRegime bool, isOrderable bool) {
	exUpper := strings.ToUpper(strings.TrimSpace(exchange))
	symUpper := strings.ToUpper(strings.TrimSpace(symbol))

	if macroExchanges[exUpper] || (exUpper == "TSX" && tsxIndices[symUpper]) || macroSymbols[symUpper] || strings.HasSuffix(symUpper, "1!") {
		return true, false
	}
	return false, true
}

// parseDotExchange extracts an explicit exchange suffix from a dot-delimited identifier.
// Returns ok=false for dot-class tickers such as BRK.B, CTC.A, or BBD.B.
func parseDotExchange(upper string) (exchange, symbol string, ok bool) {
	switch {
	case strings.HasSuffix(upper, ".TO") || strings.HasSuffix(upper, ".TSX"):
		return "TSX", strings.TrimSuffix(strings.TrimSuffix(upper, ".TO"), ".TSX"), true
	case strings.HasSuffix(upper, ".V") || strings.HasSuffix(upper, ".TSXV"):
		return "TSXV", strings.TrimSuffix(strings.TrimSuffix(upper, ".V"), ".TSXV"), true
	case strings.HasSuffix(upper, ".CN") || strings.HasSuffix(upper, ".CSE"):
		return "CSE", strings.TrimSuffix(strings.TrimSuffix(upper, ".CN"), ".CSE"), true
	case strings.HasSuffix(upper, ".SP"):
		return "SP", strings.TrimSuffix(upper, ".SP"), true
	case strings.HasSuffix(upper, ".NASDAQ"):
		return "NASDAQ", strings.TrimSuffix(upper, ".NASDAQ"), true
	case strings.HasSuffix(upper, ".NYSE"):
		return "NYSE", strings.TrimSuffix(upper, ".NYSE"), true
	case strings.HasSuffix(upper, ".AMEX"):
		return "AMEX", strings.TrimSuffix(upper, ".AMEX"), true
	case strings.HasSuffix(upper, ".ARCA"):
		return "ARCA", strings.TrimSuffix(upper, ".ARCA"), true
	case strings.HasSuffix(upper, ".BATS"):
		return "BATS", strings.TrimSuffix(upper, ".BATS"), true
	case strings.HasSuffix(upper, ".IEX"):
		return "IEX", strings.TrimSuffix(upper, ".IEX"), true
	case strings.HasSuffix(upper, ".ISLAND"):
		return "ISLAND", strings.TrimSuffix(upper, ".ISLAND"), true
	case strings.HasSuffix(upper, ".CBOE"):
		return "CBOE", strings.TrimSuffix(upper, ".CBOE"), true
	case strings.HasSuffix(upper, ".NEO") || strings.HasSuffix(upper, ".AEQS"):
		return "NEO", strings.TrimSuffix(strings.TrimSuffix(upper, ".NEO"), ".AEQS"), true
	case strings.HasSuffix(upper, ".TVC"):
		return "TVC", strings.TrimSuffix(upper, ".TVC"), true
	case strings.HasSuffix(upper, ".SMART"):
		return "SMART", strings.TrimSuffix(upper, ".SMART"), true
	case strings.HasSuffix(upper, ".BINANCE"):
		return "BINANCE", strings.TrimSuffix(upper, ".BINANCE"), true
	case strings.HasSuffix(upper, ".COINBASE"):
		return "COINBASE", strings.TrimSuffix(upper, ".COINBASE"), true
	case strings.HasSuffix(upper, ".INDEX"):
		return "INDEX", strings.TrimSuffix(upper, ".INDEX"), true
	default:
		return "", "", false
	}
}

// HasExchangeSuffix reports whether raw already ends with a recognized dot-delimited exchange suffix.
func HasExchangeSuffix(raw string) bool {
	upper := strings.ToUpper(strings.TrimSpace(raw))
	_, _, ok := parseDotExchange(upper)
	return ok
}

// NormalizeSymbol converts any raw input ticker/identifier into a canonical NormalizedSymbol.
func NormalizeSymbol(raw string) NormalizedSymbol {
	s := strings.TrimSpace(raw)
	upper := strings.ToUpper(s)

	var exchange, symbol string

	if strings.Contains(upper, ":") {
		parts := strings.SplitN(upper, ":", 2)
		exchange = strings.TrimSpace(parts[0])
		symbol = strings.TrimSpace(parts[1])
	} else if strings.Contains(upper, ".") {
		if ex, sym, ok := parseDotExchange(upper); ok {
			exchange = ex
			symbol = sym
		} else {
			// Dot class ticker like BRK.B, CTC.A, BBD.B
			symbol = upper
			exchange = "SMART"
		}
	} else {
		symbol = upper
		if macroSymbols[symbol] {
			exchange = "TVC"
		} else {
			exchange = "SMART"
		}
	}

	// Canonicalize exchange identifiers
	switch exchange {
	case "TSE":
		exchange = "TSX"
	case "VENTURE":
		exchange = "TSXV"
	case "PURE":
		exchange = "CSE"
	case "AEQS":
		exchange = "NEO"
	case "":
		if macroSymbols[symbol] {
			exchange = "TVC"
		} else {
			exchange = "SMART"
		}
	case "SMART":
		if macroSymbols[symbol] {
			exchange = "TVC"
		}
	}

	instrumentID := fmt.Sprintf("%s.%s", symbol, exchange)

	isRegime, isOrderable := ClassifySymbol(exchange, symbol)

	venue := "SMART"
	currency := "USD"

	if isRegime {
		venue = exchange
	} else {
		switch exchange {
		case "TSX":
			venue = "TSE"
			currency = "CAD"
		case "TSXV":
			venue = "VENTURE"
			currency = "CAD"
		case "CSE":
			venue = "PURE"
			currency = "CAD"
		case "NEO":
			venue = "NEO"
			currency = "CAD"
		default:
			venue = "SMART"
			currency = "USD"
		}
	}

	return NormalizedSymbol{
		RawInput:          raw,
		Symbol:            symbol,
		Exchange:          exchange,
		Venue:             venue,
		InstrumentID:      instrumentID,
		Currency:          currency,
		IsOrderable:       isOrderable,
		IsRegimeReference: isRegime,
	}
}
