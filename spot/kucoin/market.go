package kucoin

import (
	"context"
	"fmt"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/spot/spotpublic"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// SubscribeQuotes implements PublicClient interface
func (c *KucoinSpotClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChans := make(map[string]chan core.Quote)

	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 1)
		quoteChans[symbol] = quoteChan
	}

	// Initialize public WebSocket if not already done
	if c.wsPublicClient == nil {
		wsService := c.client.WsService()
		c.wsPublicClient = wsService.NewSpotPublicWS()
		if err := c.wsPublicClient.Start(); err != nil {
			return nil, fmt.Errorf("failed to start public WebSocket: %w", err)
		}
	}

	// Subscribe to ticker callback that handles all symbols
	tickercallback := func(topic string, subject string, data *spotpublic.TickerEvent) error {
		// Extract symbol from topic (format: /market/ticker:BTC-USDT)
		if len(topic) > 16 && topic[:16] == "/market/ticker:" {
			symbol := topic[16:]

			quote := core.Quote{
				Symbol:   symbol,
				BidPrice: decimal.RequireFromString(data.BestBid),
				BidQty:   decimal.RequireFromString(data.BestBidSize),
				AskPrice: decimal.RequireFromString(data.BestAsk),
				AskQty:   decimal.RequireFromString(data.BestAskSize),
				Time:     time.UnixMilli(data.Time),
			}

			if ch, exists := quoteChans[symbol]; exists {
				select {
				case ch <- quote:
				default:
					// Channel full, skip
				}
			}
		}
		return nil
	}

	// Subscribe to ticker for all symbols at once
	_, err := c.wsPublicClient.Ticker(symbols, tickercallback)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to subscribe to tickers: %w", err))
		}
		return nil, err
	}

	return quoteChans, nil
}

// FetchMarketRules implements PublicClient interface
func (c *KucoinSpotClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	marketService := c.client.RestService().GetSpotService()
	marketAPI := marketService.GetMarketAPI()

	// For now, implement as placeholder - exact API structure needs verification
	_ = marketAPI
	_ = quotes
	
	return nil, fmt.Errorf("spot market rules fetching not yet implemented")
}

