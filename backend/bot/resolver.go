package bot

import (
	"fmt"
	"strings"

	"tws_traderbot/backend/models"

	"github.com/scmhub/ibapi"
)

var knownVenues = map[string]bool{
	"SMART":    true,
	"NASDAQ":   true,
	"NYSE":     true,
	"AMEX":     true,
	"ARCA":     true,
	"BATS":     true,
	"TSE":      true,
	"TSX":      true,
	"TO":       true,
	"VENTURE":  true,
	"TSXV":     true,
	"V":        true,
	"PURE":     true,
	"CSE":      true,
	"NEO":      true,
	"AEQS":     true,
	"TVC":      true,
	"BINANCE":  true,
	"COINBASE": true,
	"KRAKEN":   true,
	"INDEX":    true,
	"CBOE":     true,
	"CME":      true,
	"SP":       true,
}

// ConvertDotClass converts dot-class or slash-class tickers to TWS space-separated format.
// Examples:
//
//	"BRK.B" -> "BRK B"
//	"CTC.A" -> "CTC A"
//	"BBD.B" -> "BBD B"
//	"BF.B"  -> "BF B"
//	"BRK/B" -> "BRK B"
//	"BRK.B.SMART" -> "BRK B"
func ConvertDotClass(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}

	// Normalize slashes to dots, uppercase, and trim leading/trailing dots
	s = strings.ToUpper(strings.ReplaceAll(s, "/", "."))
	s = strings.Trim(s, ".")
	if s == "" {
		return ""
	}

	// If symbol has a venue suffix (e.g. BRK.B.SMART), strip venue first
	parts := strings.Split(s, ".")
	if len(parts) > 1 {
		last := strings.ToUpper(parts[len(parts)-1])
		if knownVenues[last] {
			s = strings.Trim(strings.Join(parts[:len(parts)-1], "."), ".")
		}
	}

	parts = strings.Split(s, ".")
	var cleanParts []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleanParts = append(cleanParts, p)
		}
	}
	if len(cleanParts) <= 1 {
		if len(cleanParts) == 1 {
			return cleanParts[0]
		}
		return s
	}

	return strings.Join(cleanParts, " ")
}

// ResolveContract resolves a WatchlistItem into a fully configured IBKR Contract.
// Supports US equities (SMART/USD) and Canadian equities (SMART/CAD with TSE, VENTURE, PURE primary exchange).
// Enforces that non-orderable regime references are rejected fail-closed.
func ResolveContract(item *models.WatchlistItem) (*ibapi.Contract, error) {
	if item == nil {
		return nil, fmt.Errorf("cannot resolve contract: item is nil")
	}

	// Gate 7 Invariant: Non-orderable regime references cannot be resolved
	if item.IsRegimeReference {
		return nil, fmt.Errorf("cannot resolve contract for %q: non-orderable regime reference", item.Symbol)
	}
	if !item.IsOrderable {
		return nil, fmt.Errorf("cannot resolve contract for %q: non-orderable regime reference", item.Symbol)
	}

	rawSym := strings.TrimSpace(item.Symbol)
	if rawSym == "" && item.InstrumentID != "" {
		rawSym = strings.TrimSpace(item.InstrumentID)
	}
	if rawSym == "" {
		return nil, fmt.Errorf("cannot resolve contract: symbol is empty")
	}

	// Fallback to symbol normalization if exchange/venue are missing
	norm := models.NormalizeSymbol(rawSym)
	if !norm.IsOrderable || norm.IsRegimeReference {
		return nil, fmt.Errorf("cannot resolve contract for %q: classified as non-orderable regime reference", rawSym)
	}

	exchange := item.Exchange
	if exchange == "" {
		exchange = norm.Exchange
	}
	venue := item.Venue
	if venue == "" {
		venue = norm.Venue
	}

	exchUpper := strings.ToUpper(strings.TrimSpace(exchange))
	venueUpper := strings.ToUpper(strings.TrimSpace(venue))

	// Verify supported venue/exchange
	validVenues := map[string]bool{
		"SMART": true, "NASDAQ": true, "NYSE": true, "AMEX": true, "ARCA": true, "BATS": true, "CBOE": true, "IEX": true, "ISLAND": true,
		"TSX": true, "TSE": true, "TO": true, "TSXV": true, "VENTURE": true, "V": true,
		"CSE": true, "PURE": true, "NE": true, "CN": true, "NEO": true, "AEQS": true,
	}
	if exchUpper != "" && !validVenues[exchUpper] {
		return nil, fmt.Errorf("unsupported exchange or venue: %s", exchange)
	}
	if venueUpper != "" && !validVenues[venueUpper] {
		return nil, fmt.Errorf("unsupported exchange or venue: %s", venue)
	}

	// Clean base symbol and apply dot-class conversion for TWS
	baseSym := norm.Symbol
	if item.Symbol != "" && !strings.Contains(item.Symbol, ":") {
		// If item.Symbol already contains dot class, use it
		baseSym = item.Symbol
	}
	twsSymbol := ConvertDotClass(baseSym)

	contract := &ibapi.Contract{
		Symbol:   twsSymbol,
		SecType:  "STK",
		Exchange: "SMART",
	}

	switch {
	case exchUpper == "TSX" || exchUpper == "TSE" || exchUpper == "TO" || venueUpper == "TSE" || venueUpper == "TSX" || venueUpper == "TO":
		contract.Currency = "CAD"
		contract.PrimaryExchange = "TSE"
	case exchUpper == "TSXV" || exchUpper == "VENTURE" || exchUpper == "V" || venueUpper == "VENTURE" || venueUpper == "TSXV" || venueUpper == "V":
		contract.Currency = "CAD"
		contract.PrimaryExchange = "VENTURE"
	case exchUpper == "CSE" || exchUpper == "PURE" || exchUpper == "NE" || exchUpper == "CN" || venueUpper == "PURE" || venueUpper == "CSE":
		contract.Currency = "CAD"
		contract.PrimaryExchange = "PURE"
	case exchUpper == "NEO" || exchUpper == "AEQS" || venueUpper == "NEO":
		contract.Currency = "CAD"
		contract.PrimaryExchange = "NEO"
	default:
		// Default to US equity routed through SMART in USD
		contract.Currency = "USD"
		contract.PrimaryExchange = ""
	}

	return contract, nil
}

// ResolveContractFromSymbol creates a contract directly from a raw ticker or instrument ID string.
func ResolveContractFromSymbol(raw string) (*ibapi.Contract, error) {
	norm := models.NormalizeSymbol(raw)
	if !norm.IsOrderable || norm.IsRegimeReference {
		return nil, fmt.Errorf("cannot resolve contract for %q: non-orderable regime reference", raw)
	}

	item := &models.WatchlistItem{
		Symbol:            norm.Symbol,
		Exchange:          norm.Exchange,
		Venue:             norm.Venue,
		InstrumentID:      norm.InstrumentID,
		IsOrderable:       norm.IsOrderable,
		IsRegimeReference: norm.IsRegimeReference,
	}

	return ResolveContract(item)
}
