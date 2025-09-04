package futures

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/futurespublic"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// WebSocketConnection represents a single WebSocket connection with its subscriptions
type WebSocketConnection struct {
	ws            futurespublic.FuturesPublicWS
	symbols       []string
	channels      map[string]chan core.Quote
	channelsMu    sync.RWMutex
	subscriptions []string
	ctx           context.Context
	cancel        context.CancelFunc
	loaded        bool
	loadingMu     sync.RWMutex
}

// NewWebSocketConnection creates a new WebSocket connection for the given symbols
func (c *KucoinFuturesClient) NewWebSocketConnection(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	// Create a new WebSocket instance for this subscription
	ws := c.wsService.NewFuturesPublicWS()

	// Start the WebSocket connection
	err := ws.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start futures WebSocket: %w", err)
	}

	// Create a cancelable context for this connection
	connCtx, connCancel := context.WithCancel(ctx)

	// Create the connection struct
	conn := &WebSocketConnection{
		ws:       ws,
		symbols:  symbols,
		channels: make(map[string]chan core.Quote),
		ctx:      connCtx,
		cancel:   connCancel,
		loaded:   false,
	}

	// Create channels for each symbol
	returnChannels := make(map[string]chan core.Quote)
	for _, symbol := range symbols {
		ch := make(chan core.Quote, 100)
		conn.channels[symbol] = ch
		returnChannels[symbol] = ch
	}

	// Subscribe to each symbol individually using TickerV2 (futures requires individual subscriptions)
	for _, symbol := range symbols {
		tickerCallback := func(topic string, subject string, data *futurespublic.TickerV2Event) error {
			return conn.handleFuturesTickerEvent(topic, subject, data, errHandler)
		}

		subId, err := ws.TickerV2(symbol, tickerCallback)
		if err != nil {
			// Clean up on error
			for _, id := range conn.subscriptions {
				ws.UnSubscribe(id)
			}
			ws.Stop()
			connCancel()
			return nil, fmt.Errorf("failed to subscribe to futures ticker for %s: %w", symbol, err)
		}

		conn.subscriptions = append(conn.subscriptions, subId)
		time.Sleep(time.Second / 10)
	}
	conn.loadingMu.Lock()
	conn.loaded = true
	conn.loadingMu.Unlock()

	// Monitor context cancellation
	go func() {
		<-connCtx.Done()

		// Unsubscribe all
		for _, subId := range conn.subscriptions {
			ws.UnSubscribe(subId)
		}
		ws.Stop()

		// Close channels after marking as closed
		conn.channelsMu.Lock()
		for _, ch := range conn.channels {
			close(ch)
		}
		conn.channelsMu.Unlock()
	}()

	return returnChannels, nil
}

// handleFuturesTickerEvent processes ticker events for this specific connection
func (conn *WebSocketConnection) handleFuturesTickerEvent(topic string, subject string, data *futurespublic.TickerV2Event, errHandler func(err error)) error {
	// Parse symbol from topic
	// For futures, the symbol is in the data
	symbol := data.Symbol

	// Parse futures ticker data
	bestBid, err := decimal.NewFromString(data.BestBidPrice)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse futures best bid price %s: %w", data.BestBidPrice, err))
		}
		return nil
	}

	bestAsk, err := decimal.NewFromString(data.BestAskPrice)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse futures best ask price %s: %w", data.BestAskPrice, err))
		}
		return nil
	}

	// Futures sizes are int32, need to convert
	bestBidSizeStr := fmt.Sprintf("%d", data.BestBidSize)
	bestBidSize, err := decimal.NewFromString(bestBidSizeStr)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse futures best bid size %d: %w", data.BestBidSize, err))
		}
		return nil
	}

	bestAskSizeStr := fmt.Sprintf("%d", data.BestAskSize)
	bestAskSize, err := decimal.NewFromString(bestAskSizeStr)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse futures best ask size %d: %w", data.BestAskSize, err))
		}
		return nil
	}

	// Convert timestamp - futures uses 'Ts' field
	var timestamp time.Time
	if data.Ts > 0 {
		timestamp = time.Unix(0, data.Ts*int64(time.Millisecond))
	} else {
		timestamp = time.Now()
	}

	quote := core.Quote{
		Symbol:   symbol,
		BidPrice: bestBid,
		BidQty:   bestBidSize,
		AskPrice: bestAsk,
		AskQty:   bestAskSize,
		Time:     timestamp,
	}

	// Send to appropriate channel
	conn.channelsMu.RLock()
	ch, exists := conn.channels[symbol]
	conn.channelsMu.RUnlock()

	conn.loadingMu.RLock()
	defer conn.loadingMu.RUnlock()
	if exists && conn.loaded {
		select {
		case ch <- quote:
		default:
			// Channel is full, drop the quote
			if errHandler != nil {
				errHandler(fmt.Errorf("quote channel for %s is full, dropping quote", symbol))
			}
		}
	}

	return nil
}
