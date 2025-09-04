package kucoin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/spot/spotpublic"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// WebSocketConnection represents a single WebSocket connection with its subscriptions
type WebSocketConnection struct {
	ws           spotpublic.SpotPublicWS
	symbols      []string
	channels     map[string]chan core.Quote
	channelsMu   sync.RWMutex
	subscriptions []string
	ctx          context.Context
	cancel       context.CancelFunc
	closed       bool
	closedMu     sync.RWMutex
}

// NewWebSocketConnection creates a new WebSocket connection for the given symbols
func (c *KucoinSpotClient) NewWebSocketConnection(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	// Create a new WebSocket instance for this subscription
	ws := c.wsService.NewSpotPublicWS()
	
	// Start the WebSocket connection
	err := ws.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start WebSocket: %w", err)
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
	}
	
	// Create channels for each symbol
	returnChannels := make(map[string]chan core.Quote)
	for _, symbol := range symbols {
		ch := make(chan core.Quote, 100)
		conn.channels[symbol] = ch
		returnChannels[symbol] = ch
	}
	
	// Create ticker callback that uses this connection's channels
	tickerCallback := func(topic string, subject string, data *spotpublic.TickerEvent) error {
		return conn.handleTickerEvent(topic, subject, data, errHandler)
	}
	
	// Subscribe to ticker for all symbols
	subId, err := ws.Ticker(symbols, tickerCallback)
	if err != nil {
		ws.Stop()
		connCancel()
		return nil, fmt.Errorf("failed to subscribe to ticker: %w", err)
	}
	
	conn.subscriptions = append(conn.subscriptions, subId)
	
	// Monitor context cancellation
	go func() {
		<-connCtx.Done()
		
		// Mark connection as closed
		conn.closedMu.Lock()
		conn.closed = true
		conn.closedMu.Unlock()
		
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

// handleTickerEvent processes ticker events for this specific connection
func (conn *WebSocketConnection) handleTickerEvent(topic string, subject string, data *spotpublic.TickerEvent, errHandler func(err error)) error {
	// Parse symbol from topic
	symbol := strings.Split(topic, ":")[1]
	
	// Parse ticker data
	bestBid, err := decimal.NewFromString(data.BestBid)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse best bid %s: %w", data.BestBid, err))
		}
		return nil
	}
	
	bestAsk, err := decimal.NewFromString(data.BestAsk)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse best ask %s: %w", data.BestAsk, err))
		}
		return nil
	}
	
	bestBidSize, err := decimal.NewFromString(data.BestBidSize)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse best bid size %s: %w", data.BestBidSize, err))
		}
		return nil
	}
	
	bestAskSize, err := decimal.NewFromString(data.BestAskSize)
	if err != nil {
		if errHandler != nil {
			errHandler(fmt.Errorf("failed to parse best ask size %s: %w", data.BestAskSize, err))
		}
		return nil
	}
	
	// Convert timestamp
	var timestamp time.Time
	if data.Time > 0 {
		timestamp = time.Unix(0, data.Time*int64(time.Millisecond))
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
	
	// Check if connection is closed before sending
	conn.closedMu.RLock()
	if conn.closed {
		conn.closedMu.RUnlock()
		return nil // Connection is closed, ignore the event
	}
	conn.closedMu.RUnlock()
	
	// Send to appropriate channel
	conn.channelsMu.RLock()
	ch, exists := conn.channels[symbol]
	conn.channelsMu.RUnlock()
	
	if exists {
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