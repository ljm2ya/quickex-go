package kucoin

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/spot/market"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *KucoinSpotClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	// Create a new WebSocket connection for this subscription
	// Each SubscribeQuotes call gets its own independent connection
	return c.NewWebSocketConnection(ctx, symbols, errHandler)
}


func (c *KucoinSpotClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	marketAPI := spotService.GetMarketAPI()

	// First, get all available symbols
	allSymbolsReq := market.NewGetAllSymbolsReqBuilder().
		SetMarket(""). // Empty string for all markets
		Build()

	allSymbolsResp, err := marketAPI.GetAllSymbols(allSymbolsReq, context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get all symbols: %w", err)
	}

	// Create a map for quick lookup of requested quote currencies
	quotesMap := make(map[string]bool)
	for _, quote := range quotes {
		quotesMap[quote] = true
	}

	var rules []core.MarketRule

	// Filter symbols that have the requested quote currencies
	for _, symbolData := range allSymbolsResp.Data {
		// Skip if this symbol doesn't have one of the requested quote currencies
		if !quotesMap[symbolData.QuoteCurrency] {
			continue
		}

		// Skip if trading is not enabled
		if !symbolData.EnableTrading {
			continue
		}

		// Parse decimal values
		minPrice, _ := decimal.NewFromString(symbolData.PriceIncrement)
		minQty, _ := decimal.NewFromString(symbolData.BaseIncrement)
		minQtyValue, _ := decimal.NewFromString(symbolData.BaseMinSize)
		maxQty, _ := decimal.NewFromString(symbolData.BaseMaxSize)

		// Calculate precision from increment values
		pricePrecision := int64(calculatePrecision(symbolData.PriceIncrement))
		qtyPrecision := int64(calculatePrecision(symbolData.BaseIncrement))

		rule := core.MarketRule{
			Symbol:         symbolData.Symbol,
			BaseAsset:      symbolData.BaseCurrency,
			QuoteAsset:     symbolData.QuoteCurrency,
			PricePrecision: pricePrecision,
			QtyPrecision:   qtyPrecision,
			MinPrice:       minPrice,
			MaxPrice:       decimal.NewFromInt(math.MaxInt64), // KuCoin doesn't provide max price in symbol list
			MinQty:         minQtyValue,                       // Use BaseMinSize for minimum quantity
			MaxQty:         maxQty,
			TickSize:       minPrice, // PriceIncrement is the tick size
			StepSize:       minQty,   // BaseIncrement is the step size
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
