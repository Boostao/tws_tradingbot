package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"tws_traderbot_go/backend/models"
)

var tvPayloadRegex = regexp.MustCompile(`(?s)<script type="application/prs\.init-data\+json">\s*(\{.*?\})\s*</script>`)

func (a *App) ImportTradingViewWatchlist(url string) (*models.WatchlistResponse, error) {
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

			var exchange, symbol string
			parts := strings.SplitN(s, ":", 2)
			if len(parts) == 2 {
				exchange = strings.TrimSpace(parts[0])
				symbol = strings.TrimSpace(parts[1])
			} else {
				symbol = strings.TrimSpace(s)
				exchange = ""
			}

			if symbol == "" {
				continue
			}

			item := models.WatchlistItem{
				Symbol:   strings.ToUpper(symbol),
				Exchange: strings.ToUpper(exchange),
				Name:     "",
				Enabled:  true,
			}
			groups[currentGroupIndex].Items = append(groups[currentGroupIndex].Items, item)
			symbols = append(symbols, item.Symbol)
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

	id := ""
	if strID, ok := list["id"].(string); ok {
		id = strID
	} else if numID, ok := list["id"].(float64); ok {
		id = fmt.Sprintf("%.0f", numID)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	feed := &models.WatchlistFeed{
		Provider:        "tradingview",
		URL:             url,
		Title:           name,
		ExternalID:      id,
		LastRefreshedAt: now,
	}

	res := &models.WatchlistResponse{
		Symbols: symbols,
		Groups:  finalGroups,
		Feed:    feed,
	}

	if err := a.db.SaveWatchlistState(res); err != nil {
		fmt.Printf("Warning: failed to save imported watchlist state: %v\n", err)
	}

	return res, nil
}

// AddWatchlistSymbol adds a new symbol to the manual group
func (a *App) AddWatchlistSymbol(symbol string) (*models.WatchlistResponse, error) {
	state, err := a.db.LoadWatchlistState()
	if err != nil || state == nil {
		state = a.GetWatchlist() // load default
	}

	// Check if already exists
	for _, s := range state.Symbols {
		if s == symbol {
			return state, nil // Already there
		}
	}

	state.Symbols = append(state.Symbols, symbol)
	
	// Find or create manual group
	manualFound := false
	for i, g := range state.Groups {
		if g.ID == "manual" {
			state.Groups[i].Items = append(state.Groups[i].Items, models.WatchlistItem{
				Symbol:   symbol,
				Exchange: "SMART",
				Name:     symbol,
				Enabled:  true,
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
				{Symbol: symbol, Exchange: "SMART", Name: symbol, Enabled: true},
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

	// Filter symbols array
	var newSymbols []string
	for _, s := range state.Symbols {
		if s != symbol {
			newSymbols = append(newSymbols, s)
		}
	}
	state.Symbols = newSymbols

	// Filter items inside groups
	for i, g := range state.Groups {
		var newItems []models.WatchlistItem
		for _, item := range g.Items {
			if item.Symbol != symbol {
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
