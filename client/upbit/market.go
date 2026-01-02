package upbit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// GetTicker gets ticker data for a symbol
func (u *UpbitClient) GetTicker(symbol string) (*UpbitTickerOfMarket, error) {
	// This is a public endpoint, no authentication needed
	resp, err := http.Get(baseURL + "/v1/ticker?markets=" + symbol)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tickers []UpbitTickerOfMarket
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, err
	}

	if len(tickers) == 0 {
		return nil, errors.New("ticker not found")
	}

	return &tickers[0], nil
}

func (u *UpbitClient) GetTickers(quote string) (*[]UpbitTickerOfMarket, error) {
	// This is a public endpoint, no authentication needed
	resp, err := http.Get(baseURL + "/v1/ticker/all?quote_currencies=" + quote)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tickers []UpbitTickerOfMarket
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, err
	}

	if len(tickers) == 0 {
		return nil, errors.New("ticker not found")
	}

	return &tickers, nil
}

// GetMarketRules gets market rules for a quote currency
func (u *UpbitClient) GetMarketRules(quote string) ([]UpbitMarket, error) {
	// This is a public endpoint, no authentication needed
	resp, err := http.Get(baseURL + "/v1/market/all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var markets []UpbitMarket
	if err := json.Unmarshal(body, &markets); err != nil {
		return nil, err
	}

	// Filter by quote currency
	var filtered []UpbitMarket
	for _, market := range markets {
		if strings.HasPrefix(market.Market, quote+"-") {
			filtered = append(filtered, market)
		}
	}

	return filtered, nil
}

// SubscribeQuotes implements core.PublicClient interface
func (u *UpbitClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChans := make(map[string]chan core.Quote)

	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChans[symbol] = make(chan core.Quote, 100)
	}

	// Connect to Upbit WebSocket
	ws, _, err := websocket.DefaultDialer.Dial("wss://api.upbit.com/websocket/v1", nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial error: %w", err)
	}

	// Set ping handler
	ws.SetPingHandler(func(pingData string) error {
		return ws.WriteControl(
			websocket.PongMessage,
			[]byte(pingData),
			time.Now().Add(10*time.Second),
		)
	})

	// Create subscription request for orderbook with 1 unit
	// Append .1 to each symbol to request only 1 orderbook unit
	codes := make([]string, len(symbols))
	for i, symbol := range symbols {
		codes[i] = symbol + ".1"
	}

	// Upbit WebSocket subscription format
	subscribeMsg := []map[string]interface{}{
		{
			"ticket": fmt.Sprintf("quickex-%d", time.Now().UnixNano()),
		},
		{
			"type":  "orderbook",
			"codes": codes,
		},
		{
			"format": "SIMPLE", // Use SIMPLE format for easier parsing
		},
	}

	// Send subscription message
	if err := ws.WriteJSON(subscribeMsg); err != nil {
		ws.Close()
		return nil, fmt.Errorf("websocket write error: %w", err)
	}

	// Start goroutine to handle WebSocket messages
	go func() {
		defer func() {
			ws.Close()
			// Close all channels when WebSocket closes
			for _, ch := range quoteChans {
				close(ch)
			}
		}()

		closed := false

		for {
			select {
			case <-ctx.Done():
				closed = true
				return
			default:
				// Set read deadline
				ws.SetReadDeadline(time.Now().Add(30 * time.Second))

				_, message, err := ws.ReadMessage()
				if err != nil {
					if !closed && errHandler != nil {
						errHandler(fmt.Errorf("websocket read error: %w", err))
					}
					return
				}

				// Parse orderbook message
				var orderbook WsOrderbook
				if err := json.Unmarshal(message, &orderbook); err != nil {
					if errHandler != nil {
						errHandler(fmt.Errorf("unmarshal error: %w", err))
					}
					continue
				}

				// Check if we have orderbook data
				if orderbook.Type == "orderbook" && len(orderbook.OrderbookUnits) > 0 {
					bestUnit := orderbook.OrderbookUnits[0]

					// Create quote from orderbook data
					quote := core.Quote{
						Symbol:   orderbook.Code,
						BidPrice: decimal.NewFromFloat(bestUnit.BidPrice),
						BidQty:   decimal.NewFromFloat(bestUnit.BidSize),
						AskPrice: decimal.NewFromFloat(bestUnit.AskPrice),
						AskQty:   decimal.NewFromFloat(bestUnit.AskSize),
						Time:     time.Unix(orderbook.Timestamp/1000, (orderbook.Timestamp%1000)*1000000),
					}

					// Send to appropriate channel
					if ch, ok := quoteChans[orderbook.Code]; ok {
						select {
						case ch <- quote:
							// Successfully sent
						default:
							// Channel full, drop oldest and send new
							select {
							case <-ch:
								// Dropped oldest
							default:
								// Channel was empty
							}
							// Try to send again
							select {
							case ch <- quote:
								// Successfully sent
							default:
								if errHandler != nil {
									errHandler(fmt.Errorf("channel full for symbol %s", orderbook.Code))
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

// FetchMarketRules implements core.PublicClient interface
func (u *UpbitClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	var allRules []core.MarketRule

	for _, quote := range quotes {
		rules, err := u.GetMarketRules(quote)
		if err != nil {
			return nil, err
		}
		tickers, err := u.GetTickers(quote)
		if err != nil {
			return nil, err
		}

		// Convert to core types
		for _, rule := range rules {
			var tickSize decimal.Decimal

			// For KRW markets, fetch current price to determine tick size
			if quote == "KRW" {
				// Try to get current price
				var price float64
				for _, t := range *tickers {
					if t.Market == rule.Market {
						price = t.TradePrice
					}
				}
				if price != 0.0 {
					// Use dynamic tick size based on current price
					tickSize = getTickSizeByPrice(price)
				} else {
					panic("ticker error")
				}
			} else {
				// Crypto markets (BTC, USDT) use fixed decimal precision
				tickSize = decimal.NewFromFloat(0.00000001)
			}

			// Set step size and precisions as requested
			stepSize := decimal.NewFromFloat(0.00000001)
			pricePrecision := int64(8) // 0.00000001
			qtyPrecision := int64(8)   // 0.00000001

			baseAsset := strings.TrimPrefix(rule.Market, "KRW-")

			allRules = append(allRules, core.MarketRule{
				Symbol:         rule.Market,
				BaseAsset:      baseAsset,
				QuoteAsset:     quote,
				PricePrecision: pricePrecision,
				QtyPrecision:   qtyPrecision,
				MinPrice:       decimal.NewFromFloat(0.00000001),
				MaxPrice:       decimal.NewFromFloat(math.MaxInt64), // max of int
				MinQty:         decimal.NewFromFloat(0.00000001),
				MaxQty:         decimal.NewFromFloat(math.MaxInt64),
				TickSize:       tickSize,
				StepSize:       stepSize,
				RateLimits:     []core.RateLimit{},
			})
		}
		time.Sleep(time.Millisecond * 100) // for ip limit
	}

	return allRules, nil
}

func (u *UpbitClient) FetchQuotes(symbols []string) (map[string]core.Quote, error) {
	quotes := make(map[string]core.Quote)

	// Join symbols for batch request
	markets := strings.Join(symbols, ",")

	// Use orderbook API directly with count=1 to get best bid/ask
	resp, err := http.Get(baseURL + "/v1/orderbook?markets=" + markets + "&count=1")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var orderbooks []UpbitOrderbook
	if err := json.Unmarshal(body, &orderbooks); err != nil {
		return nil, err
	}

	// Convert orderbook data to quotes
	for _, orderbook := range orderbooks {
		if len(orderbook.OrderbookUnits) > 0 {
			// Use best bid/ask from orderbook
			bestUnit := orderbook.OrderbookUnits[0]
			quotes[orderbook.Market] = core.Quote{
				Symbol:   orderbook.Market,
				BidPrice: decimal.NewFromFloat(bestUnit.BidPrice),
				BidQty:   decimal.NewFromFloat(bestUnit.BidSize),
				AskPrice: decimal.NewFromFloat(bestUnit.AskPrice),
				AskQty:   decimal.NewFromFloat(bestUnit.AskSize),
				Time:     time.Unix(int64(orderbook.Timestamp/1000), int64((orderbook.Timestamp%1000)*1000000)),
			}
		}
	}

	return quotes, nil
}

// getTickSizeByPrice returns the tick size based on the current price according to Upbit rules
// Based on https://docs.upbit.com/kr/docs/krw-market-info
func getTickSizeByPrice(price float64) decimal.Decimal {
	if price >= 1000000 {
		return decimal.NewFromInt(1000)
	} else if price >= 500000 {
		return decimal.NewFromInt(500)
	} else if price >= 100000 {
		return decimal.NewFromInt(100)
	} else if price >= 50000 {
		return decimal.NewFromInt(50)
	} else if price >= 10000 {
		return decimal.NewFromInt(10)
	} else if price >= 5000 {
		return decimal.NewFromInt(5)
	} else if price >= 100 {
		return decimal.NewFromInt(1)
	} else if price >= 10 {
		return decimal.NewFromFloat(0.1)
	} else if price >= 1 {
		return decimal.NewFromFloat(0.01)
	} else if price >= 0.1 {
		return decimal.NewFromFloat(0.001)
	} else if price >= 0.01 {
		return decimal.NewFromFloat(0.0001)
	} else if price >= 0.001 {
		return decimal.NewFromFloat(0.00001)
	} else if price >= 0.0001 {
		return decimal.NewFromFloat(0.000001)
	} else if price >= 0.00001 {
		return decimal.NewFromFloat(0.0000001)
	} else {
		return decimal.NewFromFloat(0.00000001)
	}
}
