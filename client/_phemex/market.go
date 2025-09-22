package phemex

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ljm2ya/quickex-go/core"
)

func (c *PhemexClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	// Create channels for each symbol
	quoteChans := make(map[string]chan core.Quote)
	
	c.quoteChansMu.Lock()
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 100)
		quoteChans[symbol] = quoteChan
		c.quoteChans[symbol] = quoteChan
	}
	c.quoteChansMu.Unlock()
	
	// Subscribe to tickers for all symbols
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "tick.subscribe",
		"params": symbols,
	}
	
	_, err := c.WsClient.SendRequest(msg)
	if err != nil {
		// Clean up channels if subscription fails
		c.quoteChansMu.Lock()
		for _, symbol := range symbols {
			delete(c.quoteChans, symbol)
			if ch, exists := quoteChans[symbol]; exists {
				close(ch)
				delete(quoteChans, symbol)
			}
		}
		c.quoteChansMu.Unlock()
		return nil, fmt.Errorf("failed to subscribe to quotes: %w", err)
	}
	
	// Start cleanup routine
	go func() {
		<-ctx.Done()
		c.quoteChansMu.Lock()
		for _, symbol := range symbols {
			if ch, exists := c.quoteChans[symbol]; exists {
				close(ch)
				delete(c.quoteChans, symbol)
			}
		}
		c.quoteChansMu.Unlock()
	}()
	
	return quoteChans, nil
}

func (c *PhemexClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	// First, try to get symbols from cache
	c.symbolsMu.RLock()
	var cachedRules []core.MarketRule
	var missingSymbols []string
	
	for _, symbol := range quotes {
		if symbolInfo, exists := c.symbols[symbol]; exists {
			rule := c.convertToMarketRule(*symbolInfo)
			cachedRules = append(cachedRules, rule)
		} else {
			missingSymbols = append(missingSymbols, symbol)
		}
	}
	c.symbolsMu.RUnlock()
	
	// If all symbols are cached, return cached rules
	if len(missingSymbols) == 0 {
		return cachedRules, nil
	}
	
	// Fetch missing symbols from server
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "symbol.list",
		"params": map[string]interface{}{},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch market rules: %w", err)
	}
	
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	
	var result struct {
		Symbols []PhemexSymbol `json:"symbols"`
	}
	
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal symbol response: %w", err)
	}
	
	// Update symbol cache and build rules
	c.symbolsMu.Lock()
	symbolMap := make(map[string]*PhemexSymbol)
	for i, symbol := range result.Symbols {
		c.symbols[symbol.Symbol] = &result.Symbols[i]
		symbolMap[symbol.Symbol] = &result.Symbols[i]
	}
	c.symbolsMu.Unlock()
	
	// Build all requested rules
	var allRules []core.MarketRule
	for _, symbol := range quotes {
		if symbolInfo, exists := symbolMap[symbol]; exists {
			rule := c.convertToMarketRule(*symbolInfo)
			allRules = append(allRules, rule)
		} else {
			// Return error if symbol not found
			return nil, fmt.Errorf("symbol %s not found", symbol)
		}
	}
	
	return allRules, nil
}

// convertToMarketRule converts Phemex symbol to core.MarketRule
func (c *PhemexClient) convertToMarketRule(symbol PhemexSymbol) core.MarketRule {
	minPrice := FromEp(symbol.MinPriceEp, symbol.PriceScale)
	maxPrice := FromEp(symbol.MaxPriceEp, symbol.PriceScale)
	tickSize := FromEp(symbol.TickSize, symbol.PriceScale)
	
	minQty := FromEp(symbol.LotSize, symbol.RatioScale)
	maxQty := FromEp(symbol.MaxOrderQty, symbol.RatioScale)
	stepSize := FromEp(symbol.LotSize, symbol.RatioScale) // Use lotSize as stepSize
	
	return core.MarketRule{
		Symbol:         symbol.Symbol,
		BaseAsset:      symbol.BaseCurrency,
		QuoteAsset:     symbol.QuoteCurrency,
		PricePrecision: int64(symbol.PricePrecision),
		QtyPrecision:   int64(8), // Default precision for quantity
		MinPrice:       minPrice,
		MaxPrice:       maxPrice,
		MinQty:         minQty,
		MaxQty:         maxQty,
		TickSize:       tickSize,
		StepSize:       stepSize,
		RateLimits:     c.getDefaultRateLimits(),
	}
}

// getDefaultRateLimits returns default rate limits for Phemex
func (c *PhemexClient) getDefaultRateLimits() []core.RateLimit {
	return []core.RateLimit{
		{
			Category: core.RateLimitRequest,
			Interval: time.Second,
			Limit:    20, // 20 requests per second
			Count:    0,
		},
		{
			Category: core.RateLimitOrder,
			Interval: time.Second,
			Limit:    10, // 10 orders per second
			Count:    0,
		},
		{
			Category: core.RateLimitConnection,
			Interval: time.Hour,
			Limit:    5, // 5 connections per hour
			Count:    0,
		},
	}
}