package bot

import (
	"testing"
	"tws_traderbot/backend/models"
)

func TestDotClassConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BRK.B", "BRK B"},
		{"BRK/B", "BRK B"},
		{"CTC.A", "CTC A"},
		{"BBD.B", "BBD B"},
		{"BF.B", "BF B"},
		{"BF.A", "BF A"},
		{"AAPL", "AAPL"},
		{"MSFT", "MSFT"},
		{"BRK.B.SMART", "BRK B"},
		{"CTC.A.TSE", "CTC A"},
		{"CTC.A.TO", "CTC A"},
		{"CTC.A.V", "CTC A"},
		{"ZEN.V", "ZEN"},
		{"SHOP.TO", "SHOP"},
		{"BBD.PR.B", "BBD PR B"},
		{"", ""},
	}

	for _, tc := range tests {
		actual := ConvertDotClass(tc.input)
		if actual != tc.expected {
			t.Errorf("ConvertDotClass(%q) = %q, expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestResolveContract_USEquities(t *testing.T) {
	tests := []struct {
		item             *models.WatchlistItem
		expectedSymbol   string
		expectedExchange string
		expectedCurrency string
		expectedPrimary  string
	}{
		{
			item: &models.WatchlistItem{
				Symbol:      "AAPL",
				Exchange:    "SMART",
				Venue:       "SMART",
				IsOrderable: true,
			},
			expectedSymbol:   "AAPL",
			expectedExchange: "SMART",
			expectedCurrency: "USD",
			expectedPrimary:  "",
		},
		{
			item: &models.WatchlistItem{
				Symbol:      "BRK.B",
				Exchange:    "NYSE",
				Venue:       "SMART",
				IsOrderable: true,
			},
			expectedSymbol:   "BRK B",
			expectedExchange: "SMART",
			expectedCurrency: "USD",
			expectedPrimary:  "",
		},
		{
			item: &models.WatchlistItem{
				Symbol:      "TSLA",
				Exchange:    "NASDAQ",
				Venue:       "SMART",
				IsOrderable: true,
			},
			expectedSymbol:   "TSLA",
			expectedExchange: "SMART",
			expectedCurrency: "USD",
			expectedPrimary:  "",
		},
	}

	for _, tc := range tests {
		c, err := ResolveContract(tc.item)
		if err != nil {
			t.Fatalf("unexpected error resolving US contract: %v", err)
		}
		if c.Symbol != tc.expectedSymbol {
			t.Errorf("Symbol: expected %q, got %q", tc.expectedSymbol, c.Symbol)
		}
		if c.Exchange != tc.expectedExchange {
			t.Errorf("Exchange: expected %q, got %q", tc.expectedExchange, c.Exchange)
		}
		if c.Currency != tc.expectedCurrency {
			t.Errorf("Currency: expected %q, got %q", tc.expectedCurrency, c.Currency)
		}
		if c.PrimaryExchange != tc.expectedPrimary {
			t.Errorf("PrimaryExchange: expected %q, got %q", tc.expectedPrimary, c.PrimaryExchange)
		}
		if c.SecType != "STK" {
			t.Errorf("SecType: expected 'STK', got %q", c.SecType)
		}
	}
}

func TestResolveContract_CanadianVenues(t *testing.T) {
	tests := []struct {
		name             string
		item             *models.WatchlistItem
		expectedSymbol   string
		expectedExchange string
		expectedCurrency string
		expectedPrimary  string
	}{
		{
			name: "TSX Senior Equity",
			item: &models.WatchlistItem{
				Symbol:      "SHOP",
				Exchange:    "TSX",
				Venue:       "TSE",
				IsOrderable: true,
			},
			expectedSymbol:   "SHOP",
			expectedExchange: "SMART",
			expectedCurrency: "CAD",
			expectedPrimary:  "TSE",
		},
		{
			name: "TSX Dot-Class Equity",
			item: &models.WatchlistItem{
				Symbol:      "CTC.A",
				Exchange:    "TSX",
				Venue:       "TSE",
				IsOrderable: true,
			},
			expectedSymbol:   "CTC A",
			expectedExchange: "SMART",
			expectedCurrency: "CAD",
			expectedPrimary:  "TSE",
		},
		{
			name: "TSXV Venture Equity",
			item: &models.WatchlistItem{
				Symbol:      "TOI",
				Exchange:    "TSXV",
				Venue:       "VENTURE",
				IsOrderable: true,
			},
			expectedSymbol:   "TOI",
			expectedExchange: "SMART",
			expectedCurrency: "CAD",
			expectedPrimary:  "VENTURE",
		},
		{
			name: "CSE Pure Equity",
			item: &models.WatchlistItem{
				Symbol:      "QIMC",
				Exchange:    "CSE",
				Venue:       "PURE",
				IsOrderable: true,
			},
			expectedSymbol:   "QIMC",
			expectedExchange: "SMART",
			expectedCurrency: "CAD",
			expectedPrimary:  "PURE",
		},
		{
			name: "NEO Aequitas Equity",
			item: &models.WatchlistItem{
				Symbol:      "NEO_SYM",
				Exchange:    "NEO",
				Venue:       "NEO",
				IsOrderable: true,
			},
			expectedSymbol:   "NEO_SYM",
			expectedExchange: "SMART",
			expectedCurrency: "CAD",
			expectedPrimary:  "NEO",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ResolveContract(tc.item)
			if err != nil {
				t.Fatalf("unexpected error resolving Canadian contract: %v", err)
			}
			if c.Symbol != tc.expectedSymbol {
				t.Errorf("Symbol: expected %q, got %q", tc.expectedSymbol, c.Symbol)
			}
			if c.Exchange != tc.expectedExchange {
				t.Errorf("Exchange: expected %q, got %q", tc.expectedExchange, c.Exchange)
			}
			if c.Currency != tc.expectedCurrency {
				t.Errorf("Currency: expected %q, got %q", tc.expectedCurrency, c.Currency)
			}
			if c.PrimaryExchange != tc.expectedPrimary {
				t.Errorf("PrimaryExchange: expected %q, got %q", tc.expectedPrimary, c.PrimaryExchange)
			}
			if c.SecType != "STK" {
				t.Errorf("SecType: expected 'STK', got %q", c.SecType)
			}
		})
	}
}

func TestResolveContract_RejectsNonOrderableAndRegimeReferences(t *testing.T) {
	nonOrderable := []*models.WatchlistItem{
		{
			Symbol:            "VIX",
			Exchange:          "TVC",
			Venue:             "TVC",
			IsOrderable:       false,
			IsRegimeReference: true,
		},
		{
			Symbol:            "US10Y",
			Exchange:          "TVC",
			Venue:             "TVC",
			IsOrderable:       false,
			IsRegimeReference: true,
		},
		{
			Symbol:            "SPX",
			Exchange:          "SP",
			Venue:             "SP",
			IsOrderable:       false,
			IsRegimeReference: true,
		},
		{
			Symbol:            "BTCUSD",
			Exchange:          "BINANCE",
			Venue:             "BINANCE",
			IsOrderable:       false,
			IsRegimeReference: true,
		},
		{
			Symbol:            "SPY",
			IsOrderable:       false,
			IsRegimeReference: false,
			InstrumentID:      "",
		},
		{
			Symbol:      "BNP",
			Exchange:    "PARIS",
			Venue:       "PARIS",
			IsOrderable: true,
		},
		{
			Symbol:      "BARC",
			Venue:       "LSE",
			IsOrderable: true,
		},
	}

	for _, item := range nonOrderable {
		c, err := ResolveContract(item)
		if err == nil {
			t.Errorf("expected error resolving non-orderable symbol %q, got contract: %v", item.Symbol, c)
		}
	}

	// Nil item test
	_, err := ResolveContract(nil)
	if err == nil {
		t.Errorf("expected error resolving nil item")
	}
}

func TestResolveContractFromSymbol(t *testing.T) {
	c, err := ResolveContractFromSymbol("SHOP.TO")
	if err != nil {
		t.Fatalf("unexpected error resolving from symbol SHOP.TO: %v", err)
	}
	if c.Symbol != "SHOP" || c.Currency != "CAD" || c.PrimaryExchange != "TSE" {
		t.Errorf("unexpected contract for SHOP.TO: %+v", c)
	}

	c, err = ResolveContractFromSymbol("BRK.B")
	if err != nil {
		t.Fatalf("unexpected error resolving from symbol BRK.B: %v", err)
	}
	if c.Symbol != "BRK B" || c.Currency != "USD" {
		t.Errorf("unexpected contract for BRK.B: %+v", c)
	}

	_, err = ResolveContractFromSymbol("VIX.TVC")
	if err == nil {
		t.Errorf("expected error for VIX.TVC")
	}

	// Regime benchmarks must fail-closed on bare and dot suffixes
	for _, rawRegime := range []string{"BTCUSD", "ETHUSD", "SOLUSD", "SPX.SP", "VIX", "SPX", "US10Y", "DXY"} {
		_, err := ResolveContractFromSymbol(rawRegime)
		if err == nil {
			t.Errorf("expected fail-closed error resolving regime benchmark %q", rawRegime)
		}
	}
}
