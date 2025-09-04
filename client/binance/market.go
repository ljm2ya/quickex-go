package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (b *BinanceClient) GetTicker(symbol string) (*core.Ticker, error) {
	tckSlc, err := b.GetTickers([]string{symbol})
	if err != nil {
		return nil, err
	}
	return &tckSlc[0], nil
}

func (b *BinanceClient) FetchQuotes(symbols []string) (map[string]core.Quote, error) {
	return make(map[string]core.Quote), fmt.Errorf("not implemented yet")
}

func (b *BinanceClient) GetTickers(symbols []string) ([]core.Ticker, error) {
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "ticker.book",
		"params": map[string]interface{}{"symbols": symbols},
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
		tickers = append(tickers, core.Ticker{
			Symbol:   obj.Symbol,
			BidPrice: core.ParseStringFloat(obj.BidPrice),
			BidQty:   core.ParseStringFloat(obj.BidQty),
			AskPrice: core.ParseStringFloat(obj.AskPrice),
			AskQty:   core.ParseStringFloat(obj.AskQty),
			Time:     time.Now(),
		})
	}
	return tickers, nil
}

func (b *BinanceClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "exchangeInfo",
		"params": map[string]interface{}{
			"permissions":  []string{"SPOT"},
			"symbolStatus": "TRADING",
		},
		// No "params" here; get all symbols
	}

	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}
	var mktRules []core.MarketRule
	result := root["result"]
	var wsRes wsExchangeInfoResult
	if err := wsRes.UnmarshalJSON(result); err != nil {
		return nil, err
	}
	var rateLimit []core.RateLimit
	for _, rlObj := range wsRes.RateLimits {
		var category core.RateLimitCategory
		var interval time.Duration
		switch rlObj.RateLimitType {
		case "REQUEST_WEIGHT":
			category = core.RateLimitRequest
		case "ORDERS":
			category = core.RateLimitOrder
		case "CONNECTIONS":
			category = core.RateLimitConnection
		}
		switch rlObj.Interval {
		case "SECOND":
			interval = time.Second * time.Duration(rlObj.IntervalNum)
		case "MINUTE":
			interval = time.Minute * time.Duration(rlObj.IntervalNum)
		case "DAY":
			interval = time.Hour * 24 * time.Duration(rlObj.IntervalNum)
		}
		rateLimit = append(rateLimit, core.RateLimit{
			Category: category,
			Interval: interval,
			Limit:    int64(rlObj.Limit),
			Count:    0,
		})
	}

	// Only keep symbols ending with any quote in quotes
	for _, obj := range wsRes.Symbols {
		for _, quote := range quotes {
			if obj.QuoteAsset == quote {
				mktRules = append(mktRules, core.MarketRule{
					Symbol:         obj.Symbol,
					BaseAsset:      obj.BaseAsset,
					QuoteAsset:     obj.QuoteAsset,
					PricePrecision: int64(obj.QuoteAssetPrecision),
					QtyPrecision:   int64(obj.BaseAssetPrecision),
					MinPrice:       decimal.RequireFromString(obj.Filters[0].MinPrice),
					MaxPrice:       decimal.RequireFromString(obj.Filters[0].MaxPrice),
					MinQty:         decimal.RequireFromString(obj.Filters[1].MinQty),
					MaxQty:         decimal.RequireFromString(obj.Filters[1].MaxQty),
					TickSize:       decimal.RequireFromString(obj.Filters[0].TickSize),
					StepSize:       decimal.RequireFromString(obj.Filters[1].StepSize),
					RateLimits:     rateLimit,
				})
				break // Found a matching quote, skip to next symbol
			}
		}
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
		price := decimal.RequireFromString(arr[0])
		qty := decimal.RequireFromString(arr[1])
		ob.Bids = append(ob.Bids, core.OrderbookEntry{Price: price, Quantity: qty})
	}
	// 4. Asks
	for _, arr := range result.Asks {
		if len(arr) < 2 {
			continue
		}
		price := decimal.RequireFromString(arr[0])
		qty := decimal.RequireFromString(arr[1])
		ob.Asks = append(ob.Asks, core.OrderbookEntry{Price: price, Quantity: qty})
	}

	return ob, nil
}

// SubscribeQuotes subscribes to real-time ticker updates via WebSocket for spot markets
func (b *BinanceClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	quoteChMap := make(map[string]chan core.Quote)
	ws, _, err := websocket.DefaultDialer.Dial("wss://stream.binance.com:9443/ws/", nil)
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
			if ch, exists := quoteChMap[res.Symbol]; exists {
				select {
				case ch <- core.Quote{
					Symbol:   res.Symbol,
					BidPrice: decimal.RequireFromString(res.BestBidPrice),
					BidQty:   decimal.RequireFromString(res.BestBidQty),
					AskPrice: decimal.RequireFromString(res.BestAskPrice),
					AskQty:   decimal.RequireFromString(res.BestAskQty),
					Time:     time.UnixMilli(res.EventTime),
				}:
				default:
					// Channel is full, skip this update
				}
			}
		}
	}()
	return quoteChMap, nil
}
