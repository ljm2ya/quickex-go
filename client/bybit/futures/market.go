package bybit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *BybitFuturesClient) GetTicker(symbol string) (*core.Ticker, error) {
	if symbol == "" {
		return nil, errors.New("symbol cannot be empty")
	}
	resp, err := c.client.V5().Market().GetTickers(bybit.V5GetTickersParam{
		Category: "linear", Symbol: &symbol,
	})
	if err != nil || len(resp.Result.LinearInverse.List) == 0 {
		return nil, err
	}
	t := resp.Result.LinearInverse.List[0]
	return &core.Ticker{
		Symbol:   t.Symbol,
		BidPrice: core.ToFloat(t.Bid1Price),
		BidQty:   core.ToFloat(t.Bid1Size),
		AskPrice: core.ToFloat(t.Ask1Price),
		AskQty:   core.ToFloat(t.Ask1Size),
		Time:     time.Now(),
	}, nil
}

func (c *BybitFuturesClient) FetchQuotes(symbols []string) (map[string]core.Quote, error) {
	var out = make(map[string]core.Quote)
	for _, s := range symbols {
		resp, err := c.client.V5().Market().GetTickers(bybit.V5GetTickersParam{
			Category: "linear", Symbol: &s,
		})
		if err != nil || len(resp.Result.LinearInverse.List) == 0 {
			return out, err
		}
		t := resp.Result.LinearInverse.List[0]
		out[s] = core.Quote{
			Symbol:   t.Symbol,
			BidPrice: decimal.RequireFromString(t.Bid1Price),
			BidQty:   decimal.RequireFromString(t.Bid1Size),
			AskPrice: decimal.RequireFromString(t.Ask1Price),
			AskQty:   decimal.RequireFromString(t.Ask1Size),
			Time:     time.Now(),
		}
		time.Sleep(time.Second / 100)
	}
	return out, nil
}

func (c *BybitFuturesClient) GetTickers(symbols []string) ([]core.Ticker, error) {
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

func (c *BybitFuturesClient) GetOrderbook(symbol string, depth int64) (*core.Orderbook, error) {
	resp, err := c.client.V5().Market().GetOrderbook(bybit.V5GetOrderbookParam{
		Category: "linear", Symbol: symbol,
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

func (c *BybitFuturesClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	if len(quotes) == 0 {
		return nil, errors.New("empty quotes")
	}
	limit := 1000
	resp, err := c.client.V5().Market().GetInstrumentsInfo(bybit.V5GetInstrumentsInfoParam{
		Category: "linear",
		Limit:    &limit,
	})
	if err != nil {
		return nil, err
	}

	quoteSet := make(map[string]struct{})
	for _, q := range quotes {
		quoteSet[q] = struct{}{}
	}
	var rules []core.MarketRule
	for _, r := range resp.Result.LinearInverse.List {
		if _, ok := quoteSet[r.QuoteCoin]; !ok {
			continue
		}
		rules = append(rules, core.MarketRule{
			Symbol:         r.Symbol,
			BaseAsset:      r.BaseCoin,
			QuoteAsset:     r.QuoteCoin,
			PricePrecision: toInt(r.PriceScale),
			QtyPrecision:   toInt(r.LotSizeFilter.MinNotionalValue),
			MinPrice:       decimal.NewFromFloat(core.ToFloat(r.PriceFilter.MinPrice)),
			MaxPrice:       decimal.NewFromFloat(core.ToFloat(r.PriceFilter.MaxPrice)),
			MinQty:         decimal.NewFromFloat(core.ToFloat(r.LotSizeFilter.MinOrderQty)),
			MaxQty:         decimal.NewFromFloat(core.ToFloat(r.LotSizeFilter.MaxOrderQty)),
			TickSize:       decimal.RequireFromString(r.PriceFilter.TickSize),
			StepSize:       decimal.RequireFromString(r.LotSizeFilter.QtyStep),
		})
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("no market rules found for quotes: %v", quotes)
	}
	return rules, nil
}

// FuturesOrderbookMessage represents the structure of Bybit futures orderbook WebSocket messages
type FuturesOrderbookMessage struct {
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
// Uses manual WebSocket implementation for better reliability
func (c *BybitFuturesClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChans := make(map[string]chan core.Quote)

	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 1)
		quoteChans[symbol] = quoteChan
	}

	// Connect to Bybit linear/futures WebSocket
	u, _ := url.Parse("wss://stream.bybit.com/v5/public/linear")
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to futures WebSocket: %w", err)
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
				var msg FuturesOrderbookMessage
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
