package models

type WatchlistItem struct {
	ID                int64   `json:"id,omitempty"`
	WatchlistID       string  `json:"watchlist_id,omitempty"`
	GroupID           string  `json:"group_id,omitempty"`
	GroupName         string  `json:"group_name,omitempty"`
	Symbol            string  `json:"symbol"`
	Exchange          string  `json:"exchange,omitempty"`
	Venue             string  `json:"venue,omitempty"`
	InstrumentID      string  `json:"instrument_id,omitempty"`
	Name              string  `json:"name,omitempty"`
	Enabled           bool    `json:"enabled"`
	Tier              string  `json:"tier,omitempty"`
	IsOrderable       bool    `json:"is_orderable,omitempty"`
	IsRegimeReference bool    `json:"is_regime_reference,omitempty"`
	IsHolding         bool    `json:"is_holding,omitempty"`
	SizeMode          string  `json:"size_mode,omitempty"`
	SizeValue         float64 `json:"size_value,omitempty"`
	CreatedAt         string  `json:"created_at,omitempty"`
	UpdatedAt         string  `json:"updated_at,omitempty"`
}

type WatchlistGroup struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Source string          `json:"source,omitempty"`
	Items  []WatchlistItem `json:"items"`
}

type WatchlistFeed struct {
	Provider        string `json:"provider"`
	URL             string `json:"url"`
	Title           string `json:"title,omitempty"`
	ExternalID      string `json:"external_id,omitempty"`
	LastRefreshedAt string `json:"last_refreshed_at,omitempty"`
}

type WatchlistResponse struct {
	ID          string           `json:"id,omitempty"`
	Name        string           `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	IsDefault   bool             `json:"is_default,omitempty"`
	Symbols     []string         `json:"symbols"`
	Groups      []WatchlistGroup `json:"groups"`
	Feed        *WatchlistFeed   `json:"feed,omitempty"`
	UpdatedAt   string           `json:"updated_at,omitempty"`
}

type WatchlistSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsDefault   bool   `json:"is_default"`
	Source      string `json:"source"`
	FeedURL     string `json:"feed_url,omitempty"`
	ItemCount   int    `json:"item_count"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

type ReplaceWatchlistRequest struct {
	WatchlistID string           `json:"watchlist_id,omitempty"`
	Name        string           `json:"name,omitempty"`
	Source      string           `json:"source,omitempty"`
	Symbols     []string         `json:"symbols,omitempty"`
	Groups      []WatchlistGroup `json:"groups,omitempty"`
	Items       []WatchlistItem  `json:"items,omitempty"`
	Feed        *WatchlistFeed   `json:"feed,omitempty"`
}

type SymbolRecord struct {
	Symbol   string `json:"symbol"`
	Name     string `json:"name,omitempty"`
	Exchange string `json:"exchange,omitempty"`
	Type     string `json:"type,omitempty"`
}

type SymbolQuery struct {
	Q        string `json:"q,omitempty"`
	Type     string `json:"type,omitempty"`
	Exchange string `json:"exchange,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Refresh  bool   `json:"refresh,omitempty"`
}

type SymbolsResponse struct {
	Symbols   []SymbolRecord `json:"symbols"`
	Source    string         `json:"source,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	Warning   string         `json:"warning,omitempty"`
}
