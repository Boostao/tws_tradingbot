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
		switch {
		case strings.HasSuffix(upper, ".TO") || strings.HasSuffix(upper, ".TSX"):
			exchange = "TSX"
			symbol = strings.TrimSuffix(strings.TrimSuffix(upper, ".TO"), ".TSX")
		case strings.HasSuffix(upper, ".V") || strings.HasSuffix(upper, ".TSXV"):
			exchange = "TSXV"
			symbol = strings.TrimSuffix(strings.TrimSuffix(upper, ".V"), ".TSXV")
		case strings.HasSuffix(upper, ".CN") || strings.HasSuffix(upper, ".CSE"):
			exchange = "CSE"
			symbol = strings.TrimSuffix(strings.TrimSuffix(upper, ".CN"), ".CSE")
		case strings.HasSuffix(upper, ".SP"):
			exchange = "SP"
			symbol = strings.TrimSuffix(upper, ".SP")
		case strings.HasSuffix(upper, ".NASDAQ"):
			exchange = "NASDAQ"
			symbol = strings.TrimSuffix(upper, ".NASDAQ")
		case strings.HasSuffix(upper, ".NYSE"):
			exchange = "NYSE"
			symbol = strings.TrimSuffix(upper, ".NYSE")
		case strings.HasSuffix(upper, ".AMEX"):
			exchange = "AMEX"
			symbol = strings.TrimSuffix(upper, ".AMEX")
		case strings.HasSuffix(upper, ".CBOE"):
			exchange = "CBOE"
			symbol = strings.TrimSuffix(upper, ".CBOE")
		case strings.HasSuffix(upper, ".TVC"):
			exchange = "TVC"
			symbol = strings.TrimSuffix(upper, ".TVC")
		case strings.HasSuffix(upper, ".SMART"):
			exchange = "SMART"
			symbol = strings.TrimSuffix(upper, ".SMART")
		case strings.HasSuffix(upper, ".BINANCE"):
			exchange = "BINANCE"
			symbol = strings.TrimSuffix(upper, ".BINANCE")
		case strings.HasSuffix(upper, ".COINBASE"):
			exchange = "COINBASE"
			symbol = strings.TrimSuffix(upper, ".COINBASE")
		case strings.HasSuffix(upper, ".INDEX"):
			exchange = "INDEX"
			symbol = strings.TrimSuffix(upper, ".INDEX")
		default:
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
