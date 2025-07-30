package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
)

func (b *BinanceClient) GetTicker(symbol string) (*core.Ticker, error) {
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "ticker.book",
		"params": map[string]interface{}{"symbol": symbol},
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}
	result := root["result"]
	var obj wsTicker
	if err := obj.UnmarshalJSON(result); err != nil {
		return nil, err
	}
	return &core.Ticker{
		Symbol:   obj.Symbol,
		BidPrice: core.ParseStringFloat(obj.BidPrice),
		BidQty:   core.ParseStringFloat(obj.BidQty),
		AskPrice: core.ParseStringFloat(obj.AskPrice),
		AskQty:   core.ParseStringFloat(obj.AskQty),
		Time:     time.Now(),
	}, nil
}

func (b *BinanceClient) GetTickers(symbols []string) ([]core.Ticker, error) {
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "ticker.book",
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}
	var tickers []core.Ticker
	result := root["result"]
	var arr wsTickerArr
	if err := arr.UnmarshalJSON(result); err != nil {
		return nil, err
	}
	for _, obj := range arr {
		if funk.ContainsString(symbols, obj.Symbol) {
			tickers = append(tickers, core.Ticker{
				Symbol:   obj.Symbol,
				BidPrice: core.ParseStringFloat(obj.BidPrice),
				BidQty:   core.ParseStringFloat(obj.BidQty),
				AskPrice: core.ParseStringFloat(obj.AskPrice),
				AskQty:   core.ParseStringFloat(obj.AskQty),
				Time:     time.Now(),
			})
		}
	}
	return tickers, nil
}

func (b *BinanceClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	resp, err := http.Get("https://fapi.binance.com/fapi/v1/exchangeInfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data WsfapiExchangeInfo
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Create set of allowed quote suffixes
	quoteSet := make(map[string]struct{}, len(quotes))
	for _, q := range quotes {
		quoteSet[q] = struct{}{}
	}

	// Build rate limits
	var rateLimits []core.RateLimit
	for _, rl := range data.RateLimits {
		var cat core.RateLimitCategory
		switch rl.RateLimitType {
		case "REQUEST_WEIGHT":
			cat = core.RateLimitRequest
		case "ORDERS":
			cat = core.RateLimitOrder
		case "RAW_REQUEST", "CONNECTIONS":
			cat = core.RateLimitConnection
		}
		var interval time.Duration
		switch rl.Interval {
		case "SECOND":
			interval = time.Second * time.Duration(rl.IntervalNum)
		case "MINUTE":
			interval = time.Minute * time.Duration(rl.IntervalNum)
		case "DAY":
			interval = 24 * time.Hour * time.Duration(rl.IntervalNum)
		}
		rateLimits = append(rateLimits, core.RateLimit{
			Category: cat,
			Interval: interval,
			Limit:    int64(rl.Limit),
			Count:    0,
		})
	}

	// Filter symbols ending with allowed quotes
	var mktRules []core.MarketRule
	for _, s := range data.Symbols {
		if s.Status != "TRADING" {
			continue
		}

		keep := false
		for q := range quoteSet {
			if s.QuoteAsset == q && s.Symbol == s.BaseAsset+q {
				keep = true
				break
			}
		}
		if !keep {
			continue
		}

		var (
			minPrice, maxPrice float64
			minQty, maxQty     float64
			tickSize, stepSize decimal.Decimal
		)
		for _, f := range s.Filters {
			switch f.FilterType {
			case "PRICE_FILTER":
				minPrice = core.ParseStringFloat(f.MinPrice)
				maxPrice = core.ParseStringFloat(f.MaxPrice)
				tickSize = decimal.RequireFromString(f.TickSize)
			case "LOT_SIZE":
				minQty = core.ParseStringFloat(f.MinQty)
				maxQty = core.ParseStringFloat(f.MaxQty)
				stepSize = decimal.RequireFromString(f.StepSize)
			}
		}

		mktRules = append(mktRules, core.MarketRule{
			Symbol:         s.Symbol,
			BaseAsset:      s.BaseAsset,
			QuoteAsset:     s.QuoteAsset,
			PricePrecision: int64(s.PricePrecision),
			QtyPrecision:   int64(s.QuantityPrecision),
			MinPrice:       decimal.NewFromFloat(minPrice),
			MaxPrice:       decimal.NewFromFloat(maxPrice),
			MinQty:         decimal.NewFromFloat(minQty),
			MaxQty:         decimal.NewFromFloat(maxQty),
			TickSize:       tickSize,
			StepSize:       stepSize,
			RateLimits:     rateLimits,
		})
	}

	return mktRules, nil
}

func (b *BinanceClient) GetOrderbook(symbol string, depth int64) (*core.Orderbook, error) {
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "depth",
		"params": map[string]interface{}{
			"symbol": symbol,
			"limit":  depth,
		},
	}

	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}

	// 1. status 체크
	var status int
	if err := json.Unmarshal(root["status"], &status); err != nil || status != 200 {
		return nil, fmt.Errorf("unexpected status: %v", status)
	}

	// 2. result 파싱
	var result struct {
		LastUpdateID int        `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}
	if err := json.Unmarshal(root["result"], &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	ob := &core.Orderbook{Symbol: symbol}

	// 3. Bids
	for _, arr := range result.Bids {
		if len(arr) < 2 {
			continue
		}
		price, err1 := strconv.ParseFloat(arr[0], 64)
		qty, err2 := strconv.ParseFloat(arr[1], 64)
		if err1 == nil && err2 == nil {
			ob.Bids = append(ob.Bids, core.OrderbookEntry{Price: decimal.NewFromFloat(price), Quantity: decimal.NewFromFloat(qty)})
		}
	}
	// 4. Asks
	for _, arr := range result.Asks {
		if len(arr) < 2 {
			continue
		}
		price, err1 := strconv.ParseFloat(arr[0], 64)
		qty, err2 := strconv.ParseFloat(arr[1], 64)
		if err1 == nil && err2 == nil {
			ob.Asks = append(ob.Asks, core.OrderbookEntry{Price: decimal.NewFromFloat(price), Quantity: decimal.NewFromFloat(qty)})
		}
	}

	return ob, nil
}

func SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChMap := make(map[string]chan core.Quote)
	ws, _, err := websocket.DefaultDialer.Dial("wss://fstream.binance.com", nil)
	if err != nil {
		return quoteChMap, err
	}

	ws.SetPingHandler(func(pingData string) error {
		return ws.WriteControl(
			websocket.PongMessage,
			[]byte(pingData),
			time.Now().Add(10*time.Second),
		)
	})

	params := make([]string, len(symbols))
	for i, sym := range symbols {
		params[i] = strings.ToLower(sym) + "@bookTicker"
	}

	req := wsSubscribeRequest{
		Method: "SUBSCRIBE",
		Params: params,
		ID:     1,
	}
	err = ws.WriteJSON(req)
	if err != nil {
		return quoteChMap, err
	}
	var res struct {
		Result interface{} `json:"result"`
		Id     int64       `json:"id"`
	}
	_, msg, err := ws.ReadMessage()
	if err != nil {
		return quoteChMap, fmt.Errorf("[wsclient] WS read error: %v", err)
	}
	if err := json.Unmarshal(msg, &res); err != nil {
		return quoteChMap, fmt.Errorf("[wsclient] Unmarshal error: %v %s", err, string(msg))
	}
	for _, symbol := range symbols {
		quoteChMap[symbol] = make(chan core.Quote, 1)
	}

	go func() {
		defer func() {
			// Close all channels when context is cancelled
			for _, ch := range quoteChMap {
				close(ch)
			}
			ws.Close()
		}()
		for {
			select {
			case <-ctx.Done():
				ws.Close()
				return
			default:
			}
			_, msg, err := ws.ReadMessage()
			if err != nil && errHandler != nil {
				errHandler(fmt.Errorf("WebSocket service error: %w", err))
			}
			var res wsTickerStream
			if err := res.UnmarshalJSON(msg); err != nil {
				if errHandler != nil {
					errHandler(fmt.Errorf("WebSocket unmarshal error: %w", err))
				}
			}

			quote := core.Quote{
				Symbol:   res.Symbol,
				BidPrice: decimal.RequireFromString(res.BestBidPrice),
				BidQty:   decimal.RequireFromString(res.BestBidQty),
				AskPrice: decimal.RequireFromString(res.BestAskPrice),
				AskQty:   decimal.RequireFromString(res.BestAskQty),
				Time:     time.UnixMilli(res.EventTime),
			}
			// Send to the appropriate channel
			if ch, exists := quoteChMap[res.Symbol]; exists {
				select {
				case ch <- quote:
				default:
					// Channel full, skip
				}
			}
		}
	}()
	return quoteChMap, nil
}
