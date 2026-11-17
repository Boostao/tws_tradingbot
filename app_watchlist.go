package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"tws_traderbot/backend/models"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var tvPayloadRegex = regexp.MustCompile(`(?s)<script type="application/prs\.init-data\+json">\s*(\{.*?\})\s*</script>`)

// GetWatchlists returns list of all watchlists for Cockpit and Sidebar dropdowns
func (a *App) GetWatchlists() ([]models.WatchlistSummary, error) {
	return a.db.GetWatchlists()
}

// GetWatchlistById returns full details for a specific watchlist
func (a *App) GetWatchlistById(id string) (*models.WatchlistResponse, error) {
	return a.db.GetWatchlistByID(id)
}

// SaveWatchlist writes the watchlist state relationally with smart diff/merge
func (a *App) SaveWatchlist(wl *models.WatchlistResponse) error {
	return a.db.SaveWatchlistState(wl)
}

// ReplaceWatchlist updates or replaces watchlist groups with smart diff/merge while preserving existing metadata.
func (a *App) ReplaceWatchlist(req *models.ReplaceWatchlistRequest) (*models.WatchlistResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	id := strings.TrimSpace(req.WatchlistID)
	if id == "" {
		if defWl, err := a.db.LoadWatchlistState(); err == nil && defWl != nil && defWl.ID != "" {
			id = defWl.ID
		} else {
			id = "cobalt-telegram"
		}
	}

	name := strings.TrimSpace(req.Name)
	desc := ""
	feed := req.Feed

	if existing, err := a.db.GetWatchlistByID(id); err == nil && existing != nil {
		if name == "" {
			name = existing.Name
		}
		desc = existing.Description
		if feed == nil {
			feed = existing.Feed
		}
	}
	if name == "" {
		name = id
	}

	groups := req.Groups
	if len(groups) == 0 {
		if len(req.Items) > 0 {
			groups = []models.WatchlistGroup{
				{
					ID:     "manual",
					Name:   "Manual",
					Source: "manual",
					Items:  req.Items,
				},
			}
		} else if len(req.Symbols) > 0 {
			items := make([]models.WatchlistItem, 0, len(req.Symbols))
			for _, rawSym := range req.Symbols {
				rawSym = strings.TrimSpace(rawSym)
				if rawSym == "" {
					continue
				}
				norm := models.NormalizeSymbol(rawSym)
				items = append(items, models.WatchlistItem{
					Symbol:            norm.Symbol,
					Exchange:          norm.Exchange,
					Venue:             norm.Venue,
					InstrumentID:      norm.InstrumentID,
					Name:              norm.Symbol,
					Enabled:           true,
					Tier:              "2-Croissance",
					IsOrderable:       norm.IsOrderable,
					IsRegimeReference: norm.IsRegimeReference,
				})
			}
			groups = []models.WatchlistGroup{
				{
					ID:     "manual",
					Name:   "Manual",
					Source: "manual",
					Items:  items,
				},
			}
		}
	}

	return a.db.MergeAndSaveWatchlist(id, name, desc, feed, groups, false)
}

// RefreshWatchlistFeed re-fetches the TradingView feed and merges relationally
func (a *App) RefreshWatchlistFeed(watchlistID string) (*models.WatchlistResponse, error) {
	if watchlistID == "" {
		watchlistID = "cobalt-telegram"
	}
	wl, err := a.db.GetWatchlistByID(watchlistID)
	if err != nil {
		return nil, err
	}
	if wl.Feed == nil || wl.Feed.URL == "" {
		return nil, fmt.Errorf("watchlist %s has no feed URL", watchlistID)
	}
	return a.ImportTradingViewWatchlist(wl.Feed.URL, watchlistID)
}

// GetSymbols returns matching symbols from the watchlist universe
func (a *App) GetSymbols(query models.SymbolQuery) (*models.SymbolsResponse, error) {
	return a.db.GetSymbols(query)
}

// ImportTradingViewWatchlist fetches and parses a public TradingView watchlist
func (a *App) ImportTradingViewWatchlist(url string, targetWatchlistID ...string) (*models.WatchlistResponse, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("bad request: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("tradingview returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read failed: %v", err)
	}

	matches := tvPayloadRegex.FindSubmatch(body)
	if len(matches) < 2 {
		return nil, fmt.Errorf("payload not found in tradingview response")
	}

	rawJSON := html.UnescapeString(string(matches[1]))

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse tradingview json: %v", err)
	}

	sharedWatchlist, ok := payload["sharedWatchlist"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("sharedWatchlist key missing")
	}

	list, ok := sharedWatchlist["list"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("list key missing")
	}

	name := "TradingView Import"
	if n, ok := list["name"].(string); ok && n != "" {
		name = n
	}

	rawSymbols, ok := list["symbols"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("symbols missing")
	}

	var symbols []string
	var groups []models.WatchlistGroup
	fallbackName := name
	currentGroupIndex := -1

	startGroup := func(gName string) {
		id := strings.ToLower(strings.ReplaceAll(gName, " ", "-"))
		if id == "" {
			id = fmt.Sprintf("feed-group-%d", len(groups)+1)
		}
		groups = append(groups, models.WatchlistGroup{
			ID:     id,
			Name:   gName,
			Source: "tradingview",
			Items:  []models.WatchlistItem{},
		})
		currentGroupIndex = len(groups) - 1
	}

	for _, rawSym := range rawSymbols {
		if s, ok := rawSym.(string); ok {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}

			if strings.HasPrefix(s, "###") {
				groupName := strings.TrimSpace(strings.TrimPrefix(s, "###"))
				if groupName == "" {
					groupName = fallbackName
				}
				startGroup(groupName)
				continue
			}

			if currentGroupIndex == -1 {
				startGroup(fallbackName)
			}

			norm := models.NormalizeSymbol(s)
			if norm.Symbol == "" {
				continue
			}

			item := models.WatchlistItem{
				Symbol:            norm.Symbol,
				Exchange:          norm.Exchange,
				Venue:             norm.Venue,
				InstrumentID:      norm.InstrumentID,
				Name:              norm.Symbol,
				Enabled:           true,
				Tier:              "2-Croissance",
				IsOrderable:       norm.IsOrderable,
				IsRegimeReference: norm.IsRegimeReference,
			}
			groups[currentGroupIndex].Items = append(groups[currentGroupIndex].Items, item)
			symbols = append(symbols, item.InstrumentID)
		}
	}

	var finalGroups []models.WatchlistGroup
	for _, g := range groups {
		if len(g.Items) > 0 {
			finalGroups = append(finalGroups, g)
		}
	}
	if len(finalGroups) == 0 {
		finalGroups = []models.WatchlistGroup{
			{
				ID:     "tradingview-import",
				Name:   name,
				Source: "tradingview",
				Items:  []models.WatchlistItem{},
			},
		}
	}

	externalID := ""
	if strID, ok := list["id"].(string); ok {
		externalID = strID
	} else if numID, ok := list["id"].(float64); ok {
		externalID = fmt.Sprintf("%.0f", numID)
	}

	id := externalID
	if len(targetWatchlistID) > 0 && targetWatchlistID[0] != "" {
		id = targetWatchlistID[0]
		if existing, err := a.db.GetWatchlistByID(id); err == nil && existing != nil && existing.Name != "" {
			if name == "TradingView Import" || name == "" {
				name = existing.Name
			}
		}
	}
	if id == "" {
		id = "tradingview-import"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	feed := &models.WatchlistFeed{
		Provider:        "tradingview",
		URL:             url,
		Title:           name,
		ExternalID:      externalID,
		LastRefreshedAt: now,
	}

	res, err := a.db.MergeAndSaveWatchlist(id, name, "", feed, finalGroups, true)
	if err != nil {
		fmt.Printf("Warning: failed to save imported watchlist state: %v\n", err)
		return nil, err
	}

	return res, nil
}

// ExportWatchlistFile exports the current watchlist as a JSON file chosen via native save dialog.
func (a *App) ExportWatchlistFile() error {
	state := a.GetWatchlist()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialise watchlist: %v", err)
	}

	selectedPath, err := a.app.Dialog.SaveFileWithOptions(&application.SaveFileDialogOptions{
		Title:    "Export Watchlist",
		Filename: "watchlist.json",
		Filters:  []application.FileFilter{{DisplayName: "JSON Files", Pattern: "*.json"}},
	}).PromptForSingleSelection()
	if err != nil || selectedPath == "" {
		return err
	}

	return os.WriteFile(selectedPath, data, 0644)
}

// ImportWatchlistFile reads a watchlist JSON file chosen via native open dialog and replaces the current watchlist.
func (a *App) ImportWatchlistFile() (*models.WatchlistResponse, error) {
	selectedPath, err := a.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:   "Import Watchlist",
		Filters: []application.FileFilter{{DisplayName: "JSON Files", Pattern: "*.json"}},
	}).PromptForSingleSelection()
	if err != nil || selectedPath == "" {
		return nil, err
	}

	data, err := os.ReadFile(selectedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read watchlist file: %v", err)
	}

	var watchlist models.WatchlistResponse
	if err := json.Unmarshal(data, &watchlist); err != nil {
		return nil, fmt.Errorf("invalid watchlist JSON: %v", err)
	}

	if err := a.db.SaveWatchlistState(&watchlist); err != nil {
		return nil, fmt.Errorf("failed to save imported watchlist: %v", err)
	}

	return &watchlist, nil
}

// AddWatchlistSymbol adds a new symbol to the manual group
func (a *App) AddWatchlistSymbol(symbol string) (*models.WatchlistResponse, error) {
	state, err := a.db.LoadWatchlistState()
	if err != nil || state == nil {
		state = a.GetWatchlist()
	}

	norm := models.NormalizeSymbol(symbol)

	for _, s := range state.Symbols {
		if s == norm.InstrumentID || s == symbol {
			return state, nil
		}
	}

	state.Symbols = append(state.Symbols, norm.InstrumentID)

	manualFound := false
	for i, g := range state.Groups {
		if g.ID == "manual" {
			state.Groups[i].Items = append(state.Groups[i].Items, models.WatchlistItem{
				Symbol:            norm.Symbol,
				Exchange:          norm.Exchange,
				Venue:             norm.Venue,
				InstrumentID:      norm.InstrumentID,
				Name:              norm.Symbol,
				Enabled:           true,
				Tier:              "2-Croissance",
				IsOrderable:       norm.IsOrderable,
				IsRegimeReference: norm.IsRegimeReference,
			})
			manualFound = true
			break
		}
	}

	if !manualFound {
		state.Groups = append(state.Groups, models.WatchlistGroup{
			ID:     "manual",
			Name:   "Manual",
			Source: "manual",
			Items: []models.WatchlistItem{
				{
					Symbol:            norm.Symbol,
					Exchange:          norm.Exchange,
					Venue:             norm.Venue,
					InstrumentID:      norm.InstrumentID,
					Name:              norm.Symbol,
					Enabled:           true,
					Tier:              "2-Croissance",
					IsOrderable:       norm.IsOrderable,
					IsRegimeReference: norm.IsRegimeReference,
				},
			},
		})
	}

	if err := a.db.SaveWatchlistState(state); err != nil {
		return nil, fmt.Errorf("failed to save watchlist: %v", err)
	}

	return state, nil
}

// RemoveWatchlistSymbol removes a symbol from all groups
func (a *App) RemoveWatchlistSymbol(symbol string) (*models.WatchlistResponse, error) {
	state, err := a.db.LoadWatchlistState()
	if err != nil || state == nil {
		return nil, fmt.Errorf("watchlist state not found")
	}

	norm := models.NormalizeSymbol(symbol)

	var newSymbols []string
	for _, s := range state.Symbols {
		if s != symbol && s != norm.InstrumentID {
			newSymbols = append(newSymbols, s)
		}
	}
	state.Symbols = newSymbols

	for i, g := range state.Groups {
		var newItems []models.WatchlistItem
		for _, item := range g.Items {
			if item.Symbol != symbol && item.InstrumentID != norm.InstrumentID {
				newItems = append(newItems, item)
			}
		}
		state.Groups[i].Items = newItems
	}

	if err := a.db.SaveWatchlistState(state); err != nil {
		return nil, fmt.Errorf("failed to save watchlist: %v", err)
	}

	return state, nil
}
