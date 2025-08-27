package futures

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/market"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *KucoinFuturesClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	// Create a new WebSocket connection for this subscription
	// Each SubscribeQuotes call gets its own independent connection
	return c.NewWebSocketConnection(ctx, symbols, errHandler)
}


func (c *KucoinFuturesClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	marketAPI := futuresService.GetMarketAPI()
	
	var rules []core.MarketRule
	
	for _, symbol := range quotes {
		// Get specific symbol info
		req := market.NewGetSymbolReqBuilder().
			SetSymbol(symbol).
			Build()
		
		resp, err := marketAPI.GetSymbol(req, context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get contract info for %s: %w", symbol, err)
		}
		
		// Parse decimal values directly from response
		tickSize := decimal.NewFromFloat(resp.TickSize)
		lotSize := decimal.NewFromInt(int64(resp.LotSize))
		maxOrderQty := decimal.NewFromInt(int64(resp.MaxOrderQty))
		
		// Calculate precision from tick size
		pricePrecision := int64(6) // Default precision for futures
		qtyPrecision := int64(0) // Futures quantities are usually integers
		
		rule := core.MarketRule{
			Symbol:         symbol,
			BaseAsset:      resp.BaseCurrency,
			QuoteAsset:     resp.QuoteCurrency,
			PricePrecision: pricePrecision,
			QtyPrecision:   qtyPrecision,
			TickSize:       tickSize,
			StepSize:       lotSize,
			MinQty:         lotSize,
			MaxQty:         maxOrderQty,
		}
		
		rules = append(rules, rule)
	}
	
	return rules, nil
}

// Helper function to calculate precision from increment string
func calculatePrecision(increment string) int {
	if !strings.Contains(increment, ".") {
		return 0
	}
	parts := strings.Split(increment, ".")
	if len(parts) != 2 {
		return 0
	}
	return len(parts[1])
}