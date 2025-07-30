package bybit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *BybitClient) GetTicker(symbol string) (*core.Ticker, error) {
	if symbol == "" {
		return nil, errors.New("symbol cannot be empty")
	}
	resp, err := c.client.V5().Market().GetTickers(bybit.V5GetTickersParam{
		Category: "spot", Symbol: &symbol,
	})
	if err != nil || len(resp.Result.Spot.List) == 0 {
		return nil, err
	}
	t := resp.Result.Spot.List[0]
	return &core.Ticker{
		Symbol:   t.Symbol,
		BidPrice: core.ToFloat(t.Bid1Price),
		BidQty:   core.ToFloat(t.Bid1Size),
		AskPrice: core.ToFloat(t.Ask1Price),
		AskQty:   core.ToFloat(t.Ask1Size),
		Time:     time.Now(),
	}, nil
}

func (c *BybitClient) GetTickers(symbols []string) ([]core.Ticker, error) {
	var out []core.Ticker
	for _, s := range symbols {
		t, err := c.GetTicker(s)
		if err == nil {
			out = append(out, *t)
		}
		time.Sleep(time.Millisecond * 10)
	}
	return out, nil
}

func (c *BybitClient) GetOrderbook(symbol string, depth int64) (*core.Orderbook, error) {
	resp, err := c.client.V5().Market().GetOrderbook(bybit.V5GetOrderbookParam{
		Category: "spot", Symbol: symbol,
	})
	if err != nil {
		return nil, err
	}
	ob := &core.Orderbook{
		Symbol: symbol,
	}
	for _, e := range resp.Result.Bids {
		ob.Bids = append(ob.Bids, core.OrderbookEntry{
			Price:    decimal.RequireFromString(e.Price),
			Quantity: decimal.RequireFromString(e.Quantity),
		})
	}
	for _, e := range resp.Result.Asks {
		ob.Asks = append(ob.Asks, core.OrderbookEntry{
			Price:    decimal.RequireFromString(e.Price),
			Quantity: decimal.RequireFromString(e.Quantity),
		})
	}
	return ob, nil
}

func (c *BybitClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	if len(quotes) == 0 {
		return nil, errors.New("empty quotes")
	}
	resp, err := c.client.V5().Market().GetInstrumentsInfo(bybit.V5GetInstrumentsInfoParam{
		Category: "spot",
	})
	if err != nil {
		return nil, err
	}

	quoteSet := make(map[string]struct{})
	for _, q := range quotes {
		quoteSet[q] = struct{}{}
	}
	var rules []core.MarketRule
	for _, r := range resp.Result.Spot.List {
		if _, ok := quoteSet[r.QuoteCoin]; !ok {
			continue
		}

		tickSize := decimal.RequireFromString(r.LotSizeFilter.QuotePrecision)
		stepSize := decimal.RequireFromString(r.LotSizeFilter.BasePrecision)
		pricePrecision := -tickSize.Exponent()
		qtyPrecision := -stepSize.Exponent()
		rules = append(rules, core.MarketRule{
			Symbol:         r.Symbol,
			BaseAsset:      r.BaseCoin,
			QuoteAsset:     r.QuoteCoin,
			PricePrecision: int64(pricePrecision),
			QtyPrecision:   int64(qtyPrecision),
			MinPrice:       decimal.Zero,
			MaxPrice:       decimal.NewFromInt(math.MaxInt64),
			MinQty:         decimal.RequireFromString(r.LotSizeFilter.MinOrderQty),
			MaxQty:         decimal.RequireFromString(r.LotSizeFilter.MaxOrderQty),
			TickSize:       decimal.RequireFromString(r.PriceFilter.TickSize),
			StepSize:       stepSize,
		})
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("no market rules found for quotes: %v", quotes)
	}
	return rules, nil
}

// SubscribeQuotes implements core.PublicClient interface
func (c *BybitClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChans := make(map[string]chan core.Quote)

	// Create WebSocket client for public market data
	wsClient := bybit.NewWebsocketClient()
	wsPublic, err := wsClient.V5().Public(bybit.CategoryV5Spot)
	if err != nil {
		return nil, fmt.Errorf("failed to create WebSocket public service: %w", err)
	}

	// Create channels and subscribe to orderbook for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 1)
		quoteChans[symbol] = quoteChan

		// Subscribe to orderbook with depth 1 to get best bid/ask
		key := bybit.V5WebsocketPublicOrderBookParamKey{
			Depth:  1,
			Symbol: bybit.SymbolV5(symbol),
		}

		// Capture symbol for closure
		currentSymbol := symbol
		_, err := wsPublic.SubscribeOrderBook(key, func(response bybit.V5WebsocketPublicOrderBookResponse) error {
			// Convert orderbook data to quote
			if len(response.Data.Bids) > 0 && len(response.Data.Asks) > 0 {
				bid := response.Data.Bids[0]
				ask := response.Data.Asks[0]

				quote := core.Quote{
					Symbol:   string(response.Data.Symbol),
					BidPrice: decimal.RequireFromString(bid.Price),
					BidQty:   decimal.RequireFromString(bid.Size),
					AskPrice: decimal.RequireFromString(ask.Price),
					AskQty:   decimal.RequireFromString(ask.Size),
					Time:     time.UnixMilli(response.TimeStamp),
				}

				// Send to the appropriate channel
				if ch, exists := quoteChans[currentSymbol]; exists {
					select {
					case ch <- quote:
					default:
						// Channel full, skip
					}
				}
			}
			return nil
		})

		if err != nil {
			if errHandler != nil {
				errHandler(fmt.Errorf("failed to subscribe to orderbook for %s: %w", symbol, err))
			}
			continue
		}
	}

	// Start the WebSocket service
	go func() {
		defer func() {
			// Close all channels when context is cancelled
			for _, ch := range quoteChans {
				close(ch)
			}
			wsPublic.Close()
		}()

		// Start WebSocket service
		err := wsPublic.Run()
		if err != nil && errHandler != nil {
			errHandler(fmt.Errorf("WebSocket service error: %w", err))
		}

		// Wait for context cancellation
		<-ctx.Done()
	}()

	return quoteChans, nil
}
