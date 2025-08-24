package mexc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// SubscribeQuotes implements core.PublicClient interface
// Uses manual WebSocket implementation similar to Bybit approach
func (c *MexcClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	if !c.connected {
		return nil, fmt.Errorf("client not connected, call Connect() first")
	}
	
	quoteChans := make(map[string]chan core.Quote)
	
	// Create channels for each symbol
	c.quoteMu.Lock()
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 100)
		quoteChans[symbol] = quoteChan
		c.quoteChans[symbol] = quoteChan
	}
	c.quoteMu.Unlock()
	
	// Try different WebSocket endpoints and formats
	go c.startWebSocketSubscription(ctx, symbols, errHandler)
	
	return quoteChans, nil
}

// pollQuotes polls MEXC REST API for book ticker data
func (c *MexcClient) pollQuotes(ctx context.Context, symbol string, errHandler func(err error)) {
	ticker := time.NewTicker(2 * time.Second) // Poll every 2 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			quote, err := c.fetchSingleQuote(symbol)
			if err != nil {
				if errHandler != nil {
					errHandler(fmt.Errorf("failed to fetch quote for %s: %w", symbol, err))
				}
				continue
			}
			
			// Send to channel if it exists
			c.quoteMu.Lock()
			if quoteChan, exists := c.quoteChans[symbol]; exists {
				select {
				case quoteChan <- *quote:
					// Quote sent successfully
				default:
					// Channel full, skip this quote
				}
			}
			c.quoteMu.Unlock()
		}
	}
}

// fetchSingleQuote fetches book ticker data for a single symbol via REST API
func (c *MexcClient) fetchSingleQuote(symbol string) (*core.Quote, error) {
	url := fmt.Sprintf("%s/api/v3/ticker/bookTicker?symbol=%s", mexcSpotBaseURL, symbol)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quote: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("quote request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read quote response: %w", err)
	}
	
	var ticker struct {
		Symbol   string `json:"symbol"`
		BidPrice string `json:"bidPrice"`
		BidQty   string `json:"bidQty"`
		AskPrice string `json:"askPrice"`
		AskQty   string `json:"askQty"`
	}
	
	if err := json.Unmarshal(body, &ticker); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quote response: %w", err)
	}
	
	bidPrice := decimal.RequireFromString(ticker.BidPrice)
	bidQty := decimal.RequireFromString(ticker.BidQty)
	askPrice := decimal.RequireFromString(ticker.AskPrice)
	askQty := decimal.RequireFromString(ticker.AskQty)
	
	return &core.Quote{
		Symbol:   ticker.Symbol,
		BidPrice: bidPrice,
		BidQty:   bidQty,
		AskPrice: askPrice,
		AskQty:   askQty,
		Time:     time.Now(),
	}, nil
}

// startWebSocketSubscription starts WebSocket connection with multiple endpoint attempts
func (c *MexcClient) startWebSocketSubscription(ctx context.Context, symbols []string, errHandler func(err error)) {
	// WebSocket endpoints to try (based on official MEXC API documentation)
	endpoints := []struct {
		url    string
		method string
		format string
	}{
		// Official MEXC API v3 endpoints (from documentation)
		{"wss://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.aggre.bookTicker.v3.api.pb@100ms@ADAUSDT"},
		{"wss://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.aggre.bookTicker.v3.api.pb@10ms@ADAUSDT"},
		{"wss://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.bookTicker.v3.api@ADAUSDT"},
		{"wss://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.deals.v3.api@ADAUSDT"},
		{"wss://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.kline.v3.api@ADAUSDT@1m"},
		{"wss://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.increase.depth.v3.api@ADAUSDT"},
		{"wss://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.limit.depth.v3.api@ADAUSDT@5"},
		
		// Alternative official endpoint
		{"ws://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.aggre.bookTicker.v3.api.pb@100ms@ADAUSDT"},
		{"ws://wbs-api.mexc.com/ws", "SUBSCRIPTION", "spot@public.bookTicker.v3.api@ADAUSDT"},
		
		// Fallback to old endpoints (likely blocked)
		{"wss://wbs.mexc.com/ws", "SUBSCRIPTION", "ADAUSDT@bookTicker"},
		{"wss://wbs.mexc.com/ws", "SUBSCRIPTION", "adausdt@bookTicker"},
	}
	
	for _, endpoint := range endpoints {
		if c.tryWebSocketEndpoint(ctx, endpoint.url, endpoint.method, endpoint.format, symbols, errHandler) {
			return // Success
		}
		
		// Wait between attempts
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
	
	// All WebSocket attempts failed, fallback to REST API polling
	if errHandler != nil {
		errHandler(fmt.Errorf("all WebSocket endpoints failed, falling back to REST API polling"))
	}
	
	for _, symbol := range symbols {
		go c.pollQuotes(ctx, symbol, errHandler)
	}
}

// tryWebSocketEndpoint attempts to connect to a specific WebSocket endpoint
func (c *MexcClient) tryWebSocketEndpoint(ctx context.Context, wsURL, method, formatTemplate string, symbols []string, errHandler func(err error)) bool {
	fmt.Printf("[mexc] Trying WebSocket: %s with method: %s\n", wsURL, method)
	
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Printf("[mexc] WebSocket connection failed: %v\n", err)
		return false
	}
	defer conn.Close()
	
	// Subscribe to symbols
	for _, symbol := range symbols {
		var subMsg map[string]interface{}
		
		// Generate subscription message based on format
		channel := strings.ReplaceAll(formatTemplate, "ADAUSDT", strings.ToUpper(symbol))
		channel = strings.ReplaceAll(channel, "adausdt", strings.ToLower(symbol))
		
		if method == "SUBSCRIPTION" {
			// Official MEXC API v3 protocol
			subMsg = map[string]interface{}{
				"method": "SUBSCRIPTION",
				"params": []string{channel},
			}
		} else if method == "SUBSCRIBE" {
			subMsg = map[string]interface{}{
				"method": method,
				"params": []string{channel},
				"id":     1,
			}
		} else {
			subMsg = map[string]interface{}{
				"method": method,
				"params": []string{channel},
			}
		}
		
		if err := conn.WriteJSON(subMsg); err != nil {
			fmt.Printf("[mexc] Failed to send subscription: %v\n", err)
			return false
		}
		
		fmt.Printf("[mexc] Sent subscription: %+v\n", subMsg)
	}
	
	// Test for 5 seconds to see if we get valid data
	timeout := time.After(5 * time.Second)
	messageCount := 0
	validData := false
	
	for {
		select {
		case <-timeout:
			fmt.Printf("[mexc] Endpoint test timeout. Messages: %d, Valid data: %t\n", messageCount, validData)
			if validData {
				// This endpoint works, start the real subscription
				go c.maintainWebSocketConnection(ctx, wsURL, method, formatTemplate, symbols, errHandler)
				return true
			}
			return false
			
		case <-ctx.Done():
			return false
			
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, message, err := conn.ReadMessage()
			if err != nil {
				if strings.Contains(err.Error(), "timeout") {
					continue
				}
				if strings.Contains(err.Error(), "repeated read") {
					fmt.Printf("[mexc] Connection closed by server\n")
					return false
				}
				fmt.Printf("[mexc] Read error: %v\n", err)
				return false
			}
			
			messageCount++
			
			// Try to parse as JSON
			var response map[string]interface{}
			if err := json.Unmarshal(message, &response); err == nil {
				fmt.Printf("[mexc] Received JSON: %+v\n", response)
				
				// Check for subscription confirmation
				if code, ok := response["code"]; ok {
					if codeFloat, ok := code.(float64); ok && codeFloat == 0 {
						if msg, exists := response["msg"]; exists {
							msgStr := fmt.Sprintf("%v", msg)
							if strings.Contains(msgStr, "successfully") && !strings.Contains(msgStr, "Not") {
								validData = true
								fmt.Printf("[mexc] Subscription successful!\n")
							} else if strings.Contains(msgStr, "Blocked") {
								fmt.Printf("[mexc] Subscription blocked: %s\n", msgStr)
								return false
							}
						}
					}
				}
				
				// Check for actual market data
				if _, hasSymbol := response["s"]; hasSymbol {
					validData = true
					fmt.Printf("[mexc] Market data received!\n")
				}
				if _, hasData := response["data"]; hasData {
					validData = true
					fmt.Printf("[mexc] Data stream received!\n")
				}
			} else {
				fmt.Printf("[mexc] Received non-JSON: %s\n", string(message[:min(100, len(message))]))
			}
			
			if messageCount >= 3 {
				break
			}
		}
	}
	
	return validData
}

// maintainWebSocketConnection maintains a working WebSocket connection
func (c *MexcClient) maintainWebSocketConnection(ctx context.Context, wsURL, method, formatTemplate string, symbols []string, errHandler func(err error)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		fmt.Printf("[mexc] Starting WebSocket connection to %s\n", wsURL)
		
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			if errHandler != nil {
				errHandler(fmt.Errorf("WebSocket connection failed: %w", err))
			}
			time.Sleep(5 * time.Second)
			continue
		}
		
		// Subscribe to symbols
		for _, symbol := range symbols {
			channel := strings.ReplaceAll(formatTemplate, "ADAUSDT", strings.ToUpper(symbol))
			channel = strings.ReplaceAll(channel, "adausdt", strings.ToLower(symbol))
			
			var subMsg map[string]interface{}
			if method == "SUBSCRIPTION" {
				// Official MEXC API v3 protocol
				subMsg = map[string]interface{}{
					"method": "SUBSCRIPTION",
					"params": []string{channel},
				}
			} else if method == "SUBSCRIBE" {
				subMsg = map[string]interface{}{
					"method": method,
					"params": []string{channel},
					"id":     1,
				}
			} else {
				subMsg = map[string]interface{}{
					"method": method,
					"params": []string{channel},
				}
			}
			
			if err := conn.WriteJSON(subMsg); err != nil {
				if errHandler != nil {
					errHandler(fmt.Errorf("subscription failed: %w", err))
				}
				continue
			}
		}
		
		// Start reading messages
		c.readWebSocketMessages(ctx, conn, symbols, errHandler)
		
		conn.Close()
		
		// Reconnect after delay
		time.Sleep(3 * time.Second)
	}
}

// readWebSocketMessages reads and processes WebSocket messages
func (c *MexcClient) readWebSocketMessages(ctx context.Context, conn *websocket.Conn, symbols []string, errHandler func(err error)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("[mexc] WebSocket read error: %v\n", err)
			return
		}
		
		// Try to parse as JSON
		var response map[string]interface{}
		if err := json.Unmarshal(message, &response); err != nil {
			continue // Skip non-JSON messages
		}
		
		// Process market data
		if c.processMarketData(response, symbols) {
			// Successfully processed market data
		}
	}
}

// processMarketData processes incoming market data and sends to appropriate channels
func (c *MexcClient) processMarketData(data map[string]interface{}, symbols []string) bool {
	// Try different data formats
	
	// Format 1: Direct symbol data
	if symbol, ok := data["s"].(string); ok {
		if bidPrice, hasBid := data["b"].(string); hasBid {
			if askPrice, hasAsk := data["a"].(string); hasAsk {
				return c.sendQuoteToChannel(symbol, bidPrice, askPrice, data)
			}
		}
	}
	
	// Format 2: Data field with nested info
	if dataField, ok := data["data"].(map[string]interface{}); ok {
		if symbol, ok := dataField["s"].(string); ok {
			if bidPrice, hasBid := dataField["b"].(string); hasBid {
				if askPrice, hasAsk := dataField["a"].(string); hasAsk {
					return c.sendQuoteToChannel(symbol, bidPrice, askPrice, dataField)
				}
			}
		}
	}
	
	return false
}

// sendQuoteToChannel sends quote data to the appropriate channel
func (c *MexcClient) sendQuoteToChannel(symbol, bidPriceStr, askPriceStr string, data map[string]interface{}) bool {
	bidPrice, err := decimal.NewFromString(bidPriceStr)
	if err != nil {
		return false
	}
	
	askPrice, err := decimal.NewFromString(askPriceStr)
	if err != nil {
		return false
	}
	
	// Default quantities if not provided
	bidQty := decimal.NewFromFloat(1.0)
	askQty := decimal.NewFromFloat(1.0)
	
	// Try to get quantities
	if bidQtyStr, ok := data["B"].(string); ok {
		if qty, err := decimal.NewFromString(bidQtyStr); err == nil {
			bidQty = qty
		}
	}
	if askQtyStr, ok := data["A"].(string); ok {
		if qty, err := decimal.NewFromString(askQtyStr); err == nil {
			askQty = qty
		}
	}
	
	quote := core.Quote{
		Symbol:   symbol,
		BidPrice: bidPrice,
		BidQty:   bidQty,
		AskPrice: askPrice,
		AskQty:   askQty,
		Time:     time.Now(),
	}
	
	c.quoteMu.RLock()
	if quoteChan, exists := c.quoteChans[symbol]; exists {
		select {
		case quoteChan <- quote:
			// Quote sent successfully
		default:
			// Channel full, skip this quote
		}
	}
	c.quoteMu.RUnlock()
	
	return true
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// FetchMarketRules implements core.PublicClient interface
func (c *MexcClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	// MEXC exchange info endpoint
	url := mexcSpotBaseURL + "/api/v3/exchangeInfo"
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exchange info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exchange info request failed with status: %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var exchangeInfo struct {
		Symbols []struct {
			Symbol                string `json:"symbol"`
			BaseAsset             string `json:"baseAsset"`
			QuoteAsset            string `json:"quoteAsset"`
			Status                string `json:"status"`
			BaseAssetPrecision    int    `json:"baseAssetPrecision"`
			QuoteAssetPrecision   int    `json:"quoteAssetPrecision"`
			Filters               []struct {
				FilterType       string `json:"filterType"`
				MinPrice         string `json:"minPrice,omitempty"`
				MaxPrice         string `json:"maxPrice,omitempty"`
				TickSize         string `json:"tickSize,omitempty"`
				MinQty           string `json:"minQty,omitempty"`
				MaxQty           string `json:"maxQty,omitempty"`
				StepSize         string `json:"stepSize,omitempty"`
				MinNotional      string `json:"minNotional,omitempty"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	
	if err := json.Unmarshal(body, &exchangeInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal exchange info: %w", err)
	}
	
	// Create set of requested quotes for filtering
	quoteSet := make(map[string]struct{})
	for _, quote := range quotes {
		quoteSet[quote] = struct{}{}
	}
	
	var rules []core.MarketRule
	for _, symbol := range exchangeInfo.Symbols {
		// Filter by quote asset if specified
		if len(quotes) > 0 {
			if _, exists := quoteSet[symbol.QuoteAsset]; !exists {
				continue
			}
		}
		
		// Only include active symbols (MEXC uses "1" instead of "ENABLED")
		if symbol.Status != "1" {
			continue
		}
		
		rule := core.MarketRule{
			Symbol:         symbol.Symbol,
			BaseAsset:      symbol.BaseAsset,
			QuoteAsset:     symbol.QuoteAsset,
			PricePrecision: int64(symbol.QuoteAssetPrecision),
			QtyPrecision:   int64(symbol.BaseAssetPrecision),
			MinPrice:       decimal.Zero,
			MaxPrice:       decimal.NewFromInt(999999999),
			MinQty:         decimal.Zero,
			MaxQty:         decimal.NewFromInt(999999999),
			TickSize:       decimal.Zero,
			StepSize:       decimal.Zero,
		}
		
		// Parse filters for price and quantity constraints
		for _, filter := range symbol.Filters {
			switch filter.FilterType {
			case "PRICE_FILTER":
				if filter.MinPrice != "" {
					if minPrice, err := decimal.NewFromString(filter.MinPrice); err == nil {
						rule.MinPrice = minPrice
					}
				}
				if filter.MaxPrice != "" {
					if maxPrice, err := decimal.NewFromString(filter.MaxPrice); err == nil {
						rule.MaxPrice = maxPrice
					}
				}
				if filter.TickSize != "" {
					if tickSize, err := decimal.NewFromString(filter.TickSize); err == nil {
						rule.TickSize = tickSize
					}
				}
			case "LOT_SIZE":
				if filter.MinQty != "" {
					if minQty, err := decimal.NewFromString(filter.MinQty); err == nil {
						rule.MinQty = minQty
					}
				}
				if filter.MaxQty != "" {
					if maxQty, err := decimal.NewFromString(filter.MaxQty); err == nil {
						rule.MaxQty = maxQty
					}
				}
				if filter.StepSize != "" {
					if stepSize, err := decimal.NewFromString(filter.StepSize); err == nil {
						rule.StepSize = stepSize
					}
				}
			}
		}
		
		rules = append(rules, rule)
	}
	
	if len(rules) == 0 {
		return nil, fmt.Errorf("no market rules found for quotes: %v", quotes)
	}
	
	return rules, nil
}

// GetTicker fetches current ticker for a symbol
func (c *MexcClient) GetTicker(symbol string) (*core.Ticker, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	
	reqURL := fmt.Sprintf("%s/api/v3/ticker/24hr?%s", mexcSpotBaseURL, params.Encode())
	
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticker: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticker request failed with status: %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ticker response: %w", err)
	}
	
	var mexcTicker struct {
		Symbol      string  `json:"symbol"`
		LastPrice   string  `json:"lastPrice"`
		BidPrice    string  `json:"bidPrice"`
		BidQty      string  `json:"bidQty"`
		AskPrice    string  `json:"askPrice"`
		AskQty      string  `json:"askQty"`
		Volume      string  `json:"volume"`
		CloseTime   int64   `json:"closeTime"`
	}
	
	if err := json.Unmarshal(body, &mexcTicker); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ticker: %w", err)
	}
	
	lastPrice, _ := strconv.ParseFloat(mexcTicker.LastPrice, 64)
	bidPrice, _ := strconv.ParseFloat(mexcTicker.BidPrice, 64)
	bidQty, _ := strconv.ParseFloat(mexcTicker.BidQty, 64)
	askPrice, _ := strconv.ParseFloat(mexcTicker.AskPrice, 64)
	askQty, _ := strconv.ParseFloat(mexcTicker.AskQty, 64)
	volume, _ := strconv.ParseFloat(mexcTicker.Volume, 64)
	
	return &core.Ticker{
		Symbol:    mexcTicker.Symbol,
		LastPrice: lastPrice,
		BidPrice:  bidPrice,
		BidQty:    bidQty,
		AskPrice:  askPrice,
		AskQty:    askQty,
		Volume:    volume,
		Time:      time.UnixMilli(mexcTicker.CloseTime),
	}, nil
}
