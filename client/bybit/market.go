package bybit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
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

func (b *BybitClient) FetchQuotes(symbols []string) (map[string]core.Quote, error) {
	return make(map[string]core.Quote), fmt.Errorf("not implemented yet")
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

// SpotOrderbookMessage represents the structure of Bybit spot orderbook WebSocket messages
type SpotOrderbookMessage struct {
	Topic string `json:"topic"`
	TS    int64  `json:"ts"`
	Type  string `json:"type"`
	Data  struct {
		Symbol string     `json:"s"`
		Bids   [][]string `json:"b"`
		Asks   [][]string `json:"a"`
		U      int64      `json:"u"`
		Seq    int64      `json:"seq"`
	} `json:"data"`
	CTS int64 `json:"cts"`
}

// SubscribeQuotes implements core.PublicClient interface
// Uses manual WebSocket implementation due to hirokisan/bybit library limitations with spot subscriptions
func (c *BybitClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChans := make(map[string]chan core.Quote)

	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 1)
		quoteChans[symbol] = quoteChan
	}

	// Connect to Bybit spot WebSocket
	u, _ := url.Parse("wss://stream.bybit.com/v5/public/spot")
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to spot WebSocket: %w", err)
	}

	// Subscribe to orderbook for each symbol
	for _, symbol := range symbols {
		subMsg := map[string]interface{}{
			"op":   "subscribe",
			"args": []string{fmt.Sprintf("orderbook.1.%s", symbol)},
		}

		if err := conn.WriteJSON(subMsg); err != nil {
			if errHandler != nil {
				errHandler(fmt.Errorf("failed to subscribe to %s: %w", symbol, err))
			}
			continue
		}
	}

	// Start reading messages
	go func() {
		defer func() {
			conn.Close()
			for _, ch := range quoteChans {
				close(ch)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, message, err := conn.ReadMessage()
				if err != nil {
					if errHandler != nil {
						errHandler(fmt.Errorf("WebSocket read error: %w", err))
					}
					return
				}

				// Parse orderbook message
				var msg SpotOrderbookMessage
				if err := json.Unmarshal(message, &msg); err != nil {
					continue // Skip non-orderbook messages
				}

				// Process only orderbook data
				if msg.Topic != "" && len(msg.Topic) > 10 && msg.Topic[:10] == "orderbook." {
					// Extract symbol from topic (format: orderbook.1.BTCUSDT)
					if len(msg.Topic) < 13 {
						continue
					}
					symbol := msg.Topic[12:] // Skip "orderbook.1."

					// Ensure we have valid bid/ask data
					if len(msg.Data.Bids) > 0 && len(msg.Data.Asks) > 0 {
						bid := msg.Data.Bids[0]
						ask := msg.Data.Asks[0]

						if len(bid) >= 2 && len(ask) >= 2 {
							quote := core.Quote{
								Symbol:   symbol,
								BidPrice: decimal.RequireFromString(bid[0]),
								BidQty:   decimal.RequireFromString(bid[1]),
								AskPrice: decimal.RequireFromString(ask[0]),
								AskQty:   decimal.RequireFromString(ask[1]),
								Time:     time.UnixMilli(msg.TS),
							}

							if ch, exists := quoteChans[symbol]; exists {
								select {
								case ch <- quote:
								default:
									// Channel full, skip
								}
							}
						}
					}
				}
			}
		}
	}()

	return quoteChans, nil
}
