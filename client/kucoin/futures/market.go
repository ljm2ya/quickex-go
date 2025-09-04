package futures

import (
	"context"
	"fmt"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/market"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *KucoinFuturesClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	// Create a new WebSocket connection for this subscription
	// Each SubscribeQuotes call gets its own independent connection
	return c.NewWebSocketConnection(ctx, symbols, errHandler)
}

func (c *KucoinFuturesClient) FetchQuotes(symbols []string) (map[string]core.Quote, error) {
	out := make(map[string]core.Quote)

	restService := c.client.RestService()
	spotService := restService.GetFuturesService()
	marketAPI := spotService.GetMarketAPI()

	allTickersResp, err := marketAPI.GetAllTickers(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get quotes: %w", err)
	}
	
	// Create a set of requested symbols for quick lookup
	requestedSymbols := make(map[string]bool)
	for _, symbol := range symbols {
		requestedSymbols[symbol] = true
	}
	
	for _, ticker := range allTickersResp.Data {
		// Only include tickers for requested symbols
		// If no symbols specified, return all
		if len(symbols) > 0 && !requestedSymbols[ticker.Symbol] {
			continue
		}
		
		mult, ok := c.multiplierMap[ticker.Symbol]
		if !ok {
			continue
		}
		out[ticker.Symbol] = core.Quote{
			Symbol:   ticker.Symbol,
			BidPrice: decimal.RequireFromString(ticker.BestBidPrice),
			BidQty:   decimal.NewFromInt32(ticker.BestBidSize).Mul(mult),
			AskPrice: decimal.RequireFromString(ticker.BestAskPrice),
			AskQty:   decimal.NewFromInt32(ticker.BestAskSize).Mul(mult),
			Time:     time.Now(),
		}
	}
	return out, nil
}

func (c *KucoinFuturesClient) GetAllSymbols() (*market.GetAllSymbolsResp, error) {
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	marketAPI := futuresService.GetMarketAPI()

	resp, err := marketAPI.GetAllSymbols(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get symbol info: %w", err)
	}
	return resp, err
}

func (c *KucoinFuturesClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	resp, err := c.GetAllSymbols()
	if err != nil {
		return nil, err
	}
	var rules []core.MarketRule
	for _, info := range resp.Data {
		if info.Status == "Open" {
			for _, quote := range quotes {
				if quote == info.QuoteCurrency {
					// Parse decimal values directly from response
					tickSize := decimal.NewFromFloat(info.TickSize)
					stepSize := decimal.NewFromFloat(float64(info.LotSize) * info.Multiplier)
					maxOrderQty := decimal.NewFromInt(int64(info.MaxOrderQty))
					maxPrice := decimal.NewFromFloat(info.BuyLimit)
					minPrice := decimal.NewFromFloat(info.SellLimit)

					// Calculate precision from tick size
					pricePrecision := int64(6) // Default precision for futures
					qtyPrecision := int64(0)   // Futures quantities are usually integers

					rule := core.MarketRule{
						Symbol:         info.Symbol,
						BaseAsset:      info.BaseCurrency,
						QuoteAsset:     info.QuoteCurrency,
						PricePrecision: pricePrecision,
						QtyPrecision:   qtyPrecision,
						MaxPrice:       maxPrice,
						MinPrice:       minPrice,
						TickSize:       tickSize,
						StepSize:       stepSize,
						MinQty:         stepSize,
						MaxQty:         maxOrderQty,
					}
					rules = append(rules, rule)
				}
			}

		}
	}

	return rules, nil
}
