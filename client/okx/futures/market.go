package futures

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ljm2ya/quickex-go/client/okx"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// SubscribeQuotes implements core.PublicClient
func (c *OKXFuturesClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
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
			"channel": "tickers",
			"instId":  symbol,
		}
	}
	
	req := map[string]interface{}{
		"op":   "subscribe",
		"args": args,
	}
	
	// Use WebSocket connection for market data
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
func (c *OKXFuturesClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	var rules []core.MarketRule
	
	// Get instrument information for each quote
	for _, quote := range quotes {
		// Determine instrument type based on symbol format
		instType := c.determineInstType(quote)
		
		req := map[string]interface{}{
			"id": nextWSID(),
			"op": "instruments",
			"args": []map[string]string{
				{
					"instType": instType,
					"instId":   quote,
				},
			},
		}
		
		root, err := c.WsClient.SendRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch market rules for %s: %w", quote, err)
		}
		
		var response struct {
			Data []okx.OKXInstrument `json:"data"`
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

// determineInstType determines the instrument type based on symbol format
func (c *OKXFuturesClient) determineInstType(symbol string) string {
	// OKX futures symbol formats:
	// SWAP: BTC-USDT-SWAP (perpetual)
	// FUTURES: BTC-USDT-240329 (expiry date)
	
	if strings.Contains(symbol, "-SWAP") {
		return "SWAP"
	} else if strings.Contains(symbol, "-") {
		parts := strings.Split(symbol, "-")
		if len(parts) >= 3 {
			// Check if third part looks like a date (6 digits)
			if len(parts[2]) == 6 {
				return "FUTURES"
			}
		}
	}
	
	// Default to SWAP for perpetual futures
	return "SWAP"
}

// convertToMarketRule converts OKX instrument to core.MarketRule
func (c *OKXFuturesClient) convertToMarketRule(instrument okx.OKXInstrument) core.MarketRule {
	tickSize := okx.ToDecimal(instrument.TickSz)
	lotSize := okx.ToDecimal(instrument.LotSz)
	minSize := okx.ToDecimal(instrument.MinSz)
	maxLmtSize := okx.ToDecimal(instrument.MaxLmtSz)
	
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
		MaxPrice:       decimal.NewFromInt(10000000), // Set a reasonable max for futures
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

// getDefaultRateLimits returns default rate limits for OKX futures
func (c *OKXFuturesClient) getDefaultRateLimits() []core.RateLimit {
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
			Limit:    60,              // 60 orders per 2 seconds (futures may have different limits)
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