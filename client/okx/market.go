package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// SubscribeQuotes implements core.PublicClient
func (c *OKXClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	result := make(map[string]chan core.Quote)
	
	c.quoteChansMu.Lock()
	defer c.quoteChansMu.Unlock()
	
	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 100) // Buffer to prevent blocking
		result[symbol] = quoteChan
		c.quoteChans[symbol] = quoteChan
	}
	
	// Subscribe to tickers for all symbols
	args := make([]map[string]string, len(symbols))
	for i, symbol := range symbols {
		args[i] = map[string]string{
			"channel":  "tickers",
			"instId":   symbol,
		}
	}
	
	req := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}
	
	// Use a separate public WebSocket connection for market data
	// For now, we'll use the private connection, but in production you might want separate connections
	_, err := c.WsClient.SendRequest(req)
	if err != nil {
		// Clean up channels on error
		for _, ch := range result {
			close(ch)
		}
		for _, symbol := range symbols {
			delete(c.quoteChans, symbol)
		}
		return nil, fmt.Errorf("failed to subscribe to quotes: %w", err)
	}
	
	return result, nil
}

// FetchMarketRules implements core.PublicClient
func (c *OKXClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	var rules []core.MarketRule
	
	// Get instrument information for each quote
	for _, quote := range quotes {
		req := map[string]interface{}{
			"id": nextWSID(),
			"op": "instruments",
			"args": []map[string]string{
				{
					"instType": "SPOT",
					"instId":   quote,
				},
			},
		}
		
		root, err := c.WsClient.SendRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch market rules for %s: %w", quote, err)
		}
		
		var response struct {
			Data []OKXInstrument `json:"data"`
		}
		
		if dataRaw, ok := root["data"]; ok {
			if err := json.Unmarshal(dataRaw, &response.Data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal instruments for %s: %w", quote, err)
			}
		}
		
		for _, instrument := range response.Data {
			if instrument.InstID == quote {
				rule := c.convertToMarketRule(instrument)
				rules = append(rules, rule)
				break
			}
		}
	}
	
	return rules, nil
}

// convertToMarketRule converts OKX instrument to core.MarketRule
func (c *OKXClient) convertToMarketRule(instrument OKXInstrument) core.MarketRule {
	tickSize := ToDecimal(instrument.TickSz)
	lotSize := ToDecimal(instrument.LotSz)
	minSize := ToDecimal(instrument.MinSz)
	maxLmtSize := ToDecimal(instrument.MaxLmtSz)
	_ = ToDecimal(instrument.MaxMktSz) // maxMktSize not used currently
	
	// Calculate precision from tick size and lot size
	pricePrecision := calculatePrecision(instrument.TickSz)
	qtyPrecision := calculatePrecision(instrument.LotSz)
	
	return core.MarketRule{
		Symbol:         instrument.InstID,
		BaseAsset:      instrument.BaseCcy,
		QuoteAsset:     instrument.QuoteCcy,
		PricePrecision: pricePrecision,
		QtyPrecision:   qtyPrecision,
		MinPrice:       tickSize, // Minimum price is typically the tick size
		MaxPrice:       decimal.NewFromInt(1000000), // Set a reasonable max, OKX doesn't provide this directly
		MinQty:         minSize,
		MaxQty:         maxLmtSize,
		TickSize:       tickSize,
		StepSize:       lotSize,
		RateLimits:     c.getDefaultRateLimits(),
	}
}

// calculatePrecision calculates decimal precision from a string value
func calculatePrecision(value string) int64 {
	if value == "" || value == "0" {
		return 0
	}
	
	// Remove leading zeros and find decimal point
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return 0
	}
	
	// Count decimal places
	decimalPart := strings.TrimRight(parts[1], "0")
	return int64(len(decimalPart))
}

// getDefaultRateLimits returns default rate limits for OKX
func (c *OKXClient) getDefaultRateLimits() []core.RateLimit {
	return []core.RateLimit{
		{
			Category: core.RateLimitRequest,
			Interval: time.Second, // 1 second
			Limit:    20,          // 20 requests per second
			Count:    0,
		},
		{
			Category: core.RateLimitOrder,
			Interval: 2 * time.Second, // 2 seconds  
			Limit:    60,              // 60 orders per 2 seconds
			Count:    0,
		},
		{
			Category: core.RateLimitConnection,
			Interval: time.Hour, // 1 hour
			Limit:    480,       // 480 requests per hour per connection
			Count:    0,
		},
	}
}