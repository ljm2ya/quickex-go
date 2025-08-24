package kucoin

import (
	"context"
	"fmt"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/futurespublic"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// SubscribeQuotes implements PublicClient interface
func (c *KucoinFuturesClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChans := make(map[string]chan core.Quote)

	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 1)
		quoteChans[symbol] = quoteChan
	}

	// Initialize public WebSocket if not already done
	if c.wsPublicClient == nil {
		wsService := c.client.WsService()
		c.wsPublicClient = wsService.NewFuturesPublicWS()
		if err := c.wsPublicClient.Start(); err != nil {
			return nil, fmt.Errorf("failed to start public WebSocket: %w", err)
		}
	}

	// Subscribe to ticker for each symbol separately
	for _, symbol := range symbols {
		tickercallback := func(topic string, subject string, data *futurespublic.TickerV2Event) error {
			quote := core.Quote{
				Symbol:   data.Symbol,
				BidPrice: decimal.RequireFromString(data.BestBidPrice),
				BidQty:   decimal.NewFromInt(int64(data.BestBidSize)),
				AskPrice: decimal.RequireFromString(data.BestAskPrice),
				AskQty:   decimal.NewFromInt(int64(data.BestAskSize)),
				Time:     time.UnixMilli(data.Ts),
			}

			if ch, exists := quoteChans[data.Symbol]; exists {
				select {
				case ch <- quote:
				default:
					// Channel full, skip
				}
			}
			return nil
		}

		_, err := c.wsPublicClient.TickerV2(symbol, tickercallback)
		if err != nil {
			if errHandler != nil {
				errHandler(fmt.Errorf("failed to subscribe to %s: %w", symbol, err))
			}
		}
	}

	return quoteChans, nil
}

// FetchMarketRules implements PublicClient interface
func (c *KucoinFuturesClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	futuresService := c.client.RestService().GetFuturesService()
	marketAPI := futuresService.GetMarketAPI()
	
	// For now, implement as a placeholder - the exact API structure needs verification
	_ = marketAPI // Keep variable used
	_ = quotes     // Keep variable used
	
	return nil, fmt.Errorf("market rules fetching for futures not yet implemented")
}

