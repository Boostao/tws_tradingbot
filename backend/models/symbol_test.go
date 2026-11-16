package models

import (
	"testing"
)

func TestSymbolNormalizationAndClassification(t *testing.T) {
	tests := []struct {
		raw               string
		expectedSymbol    string
		expectedExchange  string
		expectedVenue     string
		expectedID        string
		expectedOrderable bool
		expectedRegime    bool
	}{
		{"AAPL", "AAPL", "SMART", "SMART", "AAPL.SMART", true, false},
		{"NASDAQ:AAPL", "AAPL", "NASDAQ", "SMART", "AAPL.NASDAQ", true, false},
		{"NYSE:BRK.B", "BRK.B", "NYSE", "SMART", "BRK.B.NYSE", true, false},
		{"TSX:SHOP", "SHOP", "TSX", "TSE", "SHOP.TSX", true, false},
		{"SHOP.TO", "SHOP", "TSX", "TSE", "SHOP.TSX", true, false},
		{"TSXV:TOI", "TOI", "TSXV", "VENTURE", "TOI.TSXV", true, false},
		{"CSE:QIMC", "QIMC", "CSE", "PURE", "QIMC.CSE", true, false},
		{"TVC:VIX", "VIX", "TVC", "TVC", "VIX.TVC", false, true},
		{"CME_MINI:ES1!", "ES1!", "CME_MINI", "CME_MINI", "ES1!.CME_MINI", false, true},
		{"BINANCE:BTCUSD", "BTCUSD", "BINANCE", "BINANCE", "BTCUSD.BINANCE", false, true},
		{"COINBASE:BTCUSD", "BTCUSD", "COINBASE", "COINBASE", "BTCUSD.COINBASE", false, true},
		{"INDEX:BTCUSD", "BTCUSD", "INDEX", "INDEX", "BTCUSD.INDEX", false, true},
		{"TSX:TSX", "TSX", "TSX", "TSX", "TSX.TSX", false, true},
		{"TSX:TX60", "TX60", "TSX", "TSX", "TX60.TSX", false, true},
		{"TSX:AQN", "AQN", "TSX", "TSE", "AQN.TSX", true, false},
		{"NYSE:AQN", "AQN", "NYSE", "SMART", "AQN.NYSE", true, false},
		{"US10Y", "US10Y", "TVC", "TVC", "US10Y.TVC", false, true},
		{"US02Y", "US02Y", "TVC", "TVC", "US02Y.TVC", false, true},
		{"VIX", "VIX", "TVC", "TVC", "VIX.TVC", false, true},
		{"SPX", "SPX", "TVC", "TVC", "SPX.TVC", false, true},
		{"DXY", "DXY", "TVC", "TVC", "DXY.TVC", false, true},
		{"BTCUSD", "BTCUSD", "TVC", "TVC", "BTCUSD.TVC", false, true},
		{"ETHUSD", "ETHUSD", "TVC", "TVC", "ETHUSD.TVC", false, true},
		{"SOLUSD", "SOLUSD", "TVC", "TVC", "SOLUSD.TVC", false, true},
		{"SPX.SP", "SPX", "SP", "SP", "SPX.SP", false, true},
	}

	for _, tt := range tests {
		norm := NormalizeSymbol(tt.raw)
		if norm.Symbol != tt.expectedSymbol {
			t.Errorf("raw %q: expected symbol %s, got %s", tt.raw, tt.expectedSymbol, norm.Symbol)
		}
		if norm.Exchange != tt.expectedExchange {
			t.Errorf("raw %q: expected exchange %s, got %s", tt.raw, tt.expectedExchange, norm.Exchange)
		}
		if norm.Venue != tt.expectedVenue {
			t.Errorf("raw %q: expected venue %s, got %s", tt.raw, tt.expectedVenue, norm.Venue)
		}
		if norm.InstrumentID != tt.expectedID {
			t.Errorf("raw %q: expected ID %s, got %s", tt.raw, tt.expectedID, norm.InstrumentID)
		}
		if norm.IsOrderable != tt.expectedOrderable {
			t.Errorf("raw %q: expected orderable %v, got %v", tt.raw, tt.expectedOrderable, norm.IsOrderable)
		}
		if norm.IsRegimeReference != tt.expectedRegime {
			t.Errorf("raw %q: expected regime %v, got %v", tt.raw, tt.expectedRegime, norm.IsRegimeReference)
		}
	}
}

func TestBareMacroSymbolClassification(t *testing.T) {
	norm := NormalizeSymbol("US10Y")
	if norm.IsOrderable {
		t.Errorf("expected NormalizeSymbol(\"US10Y\").IsOrderable to be false, got true")
	}
	if !norm.IsRegimeReference {
		t.Errorf("expected NormalizeSymbol(\"US10Y\").IsRegimeReference to be true, got false")
	}
	if norm.Venue != "TVC" {
		t.Errorf("expected NormalizeSymbol(\"US10Y\").Venue to be TVC, got %s", norm.Venue)
	}
	if norm.InstrumentID != "US10Y.TVC" {
		t.Errorf("expected NormalizeSymbol(\"US10Y\").InstrumentID to be US10Y.TVC, got %s", norm.InstrumentID)
	}
}

func TestBareCryptoAndSPXMacroClassification(t *testing.T) {
	for _, sym := range []string{"BTCUSD", "ETHUSD", "SOLUSD", "SPX.SP"} {
		norm := NormalizeSymbol(sym)
		if norm.IsOrderable {
			t.Errorf("expected %s to be non-orderable, got is_orderable=true", sym)
		}
		if !norm.IsRegimeReference {
			t.Errorf("expected %s to be regime reference, got is_regime_reference=false", sym)
		}
	}
}
