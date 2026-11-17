package db

import (
	"fmt"
	"tws_traderbot/backend/models"
)

type SeedWatchlistDef struct {
	ID          string
	Name        string
	Description string
	IsDefault   bool
	Source      string
	FeedURL     string
	Groups      []SeedGroupDef
}

type SeedGroupDef struct {
	ID      string
	Name    string
	Symbols []string
	Tier    string
	Holding bool
}

// GetPreSeededWatchlists returns the pre-seeded Cobalt watchlist definitions.
func GetPreSeededWatchlists() []SeedWatchlistDef {
	return cobaltWatchlistSeeds
}

var cobaltWatchlistSeeds = []SeedWatchlistDef{
	{
		ID:          "cobalt-telegram",
		Name:        "✝️@ Cobalt- TELEGRAM",
		Description: "Master Cobalt Telegram distribution universe (TradingView 209453826)",
		IsDefault:   true,
		Source:      "tradingview",
		FeedURL:     "https://www.tradingview.com/watchlists/209453826/",
		Groups: []SeedGroupDef{
			{
				ID:      "regime",
				Name:    "Macro & Indices",
				Symbols: []string{"TVC:VIX", "CME_MINI:ES1!", "CME_MINI:NQ1!"},
				Tier:    "2-Croissance",
			},
			{
				ID:      "btc",
				Name:    "BTC Spot",
				Symbols: []string{"BINANCE:BTCUSD", "COINBASE:BTCUSD", "INDEX:BTCUSD"},
				Tier:    "3-Spéculatif",
			},
			{
				ID:      "crypto-spot",
				Name:    "Crypto Spot",
				Symbols: []string{"COINBASE:ETHUSD", "INDEX:ETHUSD", "COINBASE:SOLUSD", "COINBASE:XRPUSD", "COINBASE:ADAUSD"},
				Tier:    "3-Spéculatif",
			},
			{
				ID:      "crypto-etf",
				Name:    "Crypto ETFs",
				Symbols: []string{"NASDAQ:IBIT", "CBOE:FBTC", "TSX:FETH", "CBOE:FETH", "CBOE:BITX", "CBOE:ETHU", "TSX:LMAX", "TSX:MSBT", "TSX:YBTC"},
				Tier:    "2-Croissance",
			},
			{
				ID:   "titres",
				Name: "Titres",
				Symbols: []string{
					"NASDAQ:AAPL", "NASDAQ:MSFT", "NASDAQ:NVDA", "NASDAQ:AMZN", "NASDAQ:GOOGL",
					"NASDAQ:META", "NASDAQ:TSLA", "NASDAQ:AMD", "NASDAQ:AVGO", "NASDAQ:NFLX",
					"NYSE:BRK.B", "NYSE:ANET", "NYSE:DIS", "NYSE:CAVA", "NYSE:SG", "NYSE:TOST", "NYSE:GNRC",
					"TSX:SHOP", "TSX:RY", "TSX:TD", "TSX:BBD.B", "TSX:CTC.A", "TSX:AQN", "NYSE:AQN",
					"TSX:CNR", "NYSE:CNR", "TSX:NFLX", "TSX:MDA", "NYSE:MDA", "TSXV:TOI", "TSXV:LMN", "CSE:QIMC",
				},
				Tier: "1-Blue Chip",
			},
		},
	},
	{
		ID:          "cobalt-home-run",
		Name:        "⚾️@ Cobalt- Home Run",
		Description: "Curated high-momentum growth universe (TradingView 111426240)",
		IsDefault:   false,
		Source:      "tradingview",
		FeedURL:     "https://www.tradingview.com/watchlists/111426240/",
		Groups: []SeedGroupDef{
			{
				ID:      "anchors",
				Name:    "Top ETFs & Anchors",
				Symbols: []string{"AMEX:ARKF", "COINBASE:BTCUSD", "CBOE:ARKB", "NASDAQ:IBIT", "CBOE:FBTC", "TSX:QBTC", "NYSE:BRK.B", "AMEX:EEM", "AMEX:KWEB", "AMEX:PBW"},
				Tier:    "2-Croissance",
			},
			{
				ID:   "titres-croissance",
				Name: "Titres Croissance / Home Run",
				Symbols: []string{
					"NASDAQ:SHOP", "NYSE:ANET", "NASDAQ:FSLY", "NYSE:CAVA", "NYSE:SG", "NASDAQ:QNTM",
					"NYSE:RBRK", "NYSE:DIS", "NYSE:TOST", "NYSE:GNRC", "TSXV:TOI", "NASDAQ:DASH",
					"TSXV:QNC", "TSX:NPI", "TSXV:LMN",
				},
				Tier: "2-Croissance",
			},
		},
	},
	{
		ID:          "cblt-marches",
		Name:        "CBLT - MARCHÉS",
		Description: "54-symbol macro regime & cross-asset benchmark universe (TradingView 40576813)",
		IsDefault:   false,
		Source:      "tradingview",
		FeedURL:     "https://www.tradingview.com/watchlists/40576813/",
		Groups: []SeedGroupDef{
			{
				ID:   "indices-us",
				Name: "Indices US & Globaux",
				Symbols: []string{
					"TVC:DJI", "AMEX:DIA", "TVC:IXIC", "NASDAQ:NDX", "NASDAQ:ONEQ", "NASDAQ:QQQ",
					"TVC:SPX", "AMEX:SPY", "AMEX:RSP", "IC:STOXX50", "CBOE:MTUM", "AMEX:IWF",
					"TVC:RUT", "AMEX:IJR", "AMEX:IWM", "DJ:DJT", "AMEX:EEM", "TVC:VIX",
				},
				Tier: "2-Croissance",
			},
			{
				ID:      "indices-ca",
				Name:    "Indices Canadiens (TSX)",
				Symbols: []string{"TSX:TSX", "TSX:TX60", "TSX:TX40", "TSX:TX20", "TSX:JX"},
				Tier:    "2-Croissance",
			},
			{
				ID:   "commodities",
				Name: "Matières Premières & Énergie",
				Symbols: []string{
					"TVC:USOIL", "CAPITALCOM:NATURALGAS", "TVC:GOLD", "AMEX:GLD", "TVC:SILVER", "AMEX:SLV",
					"AMEX:CPER", "AMEX:SLX", "NASDAQ:NQUSB55102010", "AMEX:LIT", "CAPITALCOM:COCOA", "AMEX:URA", "AMEX:URNM",
				},
				Tier: "3-Spéculatif",
			},
			{
				ID:      "yields",
				Name:    "Taux du Trésor US",
				Symbols: []string{"TVC:US02Y", "TVC:US05Y", "TVC:US10Y", "TVC:US20Y", "TVC:US30Y"},
				Tier:    "2-Croissance",
			},
			{
				ID:      "crypto-spot",
				Name:    "Crypto Spot",
				Symbols: []string{"COINBASE:BTCUSD", "KRAKEN:ETHUSD", "COINBASE:SOLUSD", "COINBASE:ADAUSD", "COINBASE:XRPUSD"},
				Tier:    "3-Spéculatif",
			},
			{
				ID:   "fx",
				Name: "Devises & Dollar US",
				Symbols: []string{
					"TVC:DXY", "FX_IDC:CADUSD", "FX:USDCAD", "FX_IDC:USDCNY", "FX:EURUSD",
					"FX:GBPUSD", "FX:AUDUSD", "FX:USDJPY",
				},
				Tier: "2-Croissance",
			},
		},
	},
	{
		ID:          "premarkets",
		Name:        "Prémarkets – À regarder",
		Description: "Daily morning focus tickers from REGARD MATINAL briefs",
		IsDefault:   false,
		Source:      "manual",
		Groups: []SeedGroupDef{
			{
				ID:      "focus",
				Name:    "Focus Matinal",
				Symbols: []string{"NYSE:ANET", "NASDAQ:ISRG", "NASDAQ:DASH", "NASDAQ:AMZN", "NYSE:APH", "NYSE:GNRC", "NASDAQ:AVGO", "NYSE:SPOT"},
				Tier:    "1-Blue Chip",
			},
		},
	},
	{
		ID:          "positions-detenues",
		Name:        "Positions détenues ($$$$)",
		Description: "Active held portfolio positions tracked according to Méthode 8.8 Step E",
		IsDefault:   false,
		Source:      "manual",
		Groups: []SeedGroupDef{
			{
				ID:      "holding",
				Name:    "Positions Détenues",
				Symbols: []string{},
				Tier:    "1-Blue Chip",
				Holding: true,
			},
		},
	},
}

// SeedCobaltWatchlists inserts the 5 Cobalt watchlists inside a single atomic transaction.
func (db *Database) SeedCobaltWatchlists() error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	insertWl, err := tx.Prepare(`
		INSERT INTO watchlists (id, name, description, is_default, source, feed_url, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			is_default = excluded.is_default,
			source = excluded.source,
			feed_url = excluded.feed_url
	`)
	if err != nil {
		return err
	}
	defer insertWl.Close()

	insertItem, err := tx.Prepare(`
		INSERT INTO watchlist_items (
			watchlist_id, group_id, group_name, symbol, exchange, venue, instrument_id,
			name, enabled, tier, is_orderable, is_regime_reference, is_holding,
			size_mode, size_value, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, TRUE, ?, ?, ?, ?, 'shares', 100.0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(watchlist_id, instrument_id) DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer insertItem.Close()

	for _, wl := range cobaltWatchlistSeeds {
		if _, err := insertWl.Exec(wl.ID, wl.Name, wl.Description, wl.IsDefault, wl.Source, wl.FeedURL); err != nil {
			return fmt.Errorf("failed to insert watchlist %s: %w", wl.ID, err)
		}

		for _, grp := range wl.Groups {
			for _, rawSym := range grp.Symbols {
				norm := models.NormalizeSymbol(rawSym)
				tier := grp.Tier
				if tier == "" {
					tier = "2-Croissance"
				}
				if _, err := insertItem.Exec(
					wl.ID, grp.ID, grp.Name, norm.Symbol, norm.Exchange, norm.Venue, norm.InstrumentID,
					norm.Symbol, tier, norm.IsOrderable, norm.IsRegimeReference, grp.Holding,
				); err != nil {
					return fmt.Errorf("failed to insert seed item %s into %s: %w", norm.InstrumentID, wl.ID, err)
				}
			}
		}
	}

	return tx.Commit()
}
