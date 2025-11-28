package upbit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

const (
	privateWsURL = "wss://api.upbit.com/websocket/v1/private"
	wsLifetime   = 23*time.Hour + 50*time.Minute
)

// UpbitPrivateWS manages private websocket connections for order and balance updates
type UpbitPrivateWS struct {
	client          *UpbitClient
	wsClient        *core.WsClient
	orderEventCh    chan core.OrderEvent     // unified order event channel
	balanceEventCh  chan core.BalanceEvent   // unified balance event channel
	isConnected     bool
	connectionMu    sync.Mutex
	subscribedSymbols []string
	subscribedAssets  []string
	subscriptionMu    sync.RWMutex
}

// NewUpbitPrivateWS creates a new private websocket instance
func NewUpbitPrivateWS(client *UpbitClient) *UpbitPrivateWS {
	return &UpbitPrivateWS{
		client:         client,
		orderEventCh:   make(chan core.OrderEvent, 100),
		balanceEventCh: make(chan core.BalanceEvent, 100),
		connectionMu:   sync.Mutex{},
		subscriptionMu: sync.RWMutex{},
	}
}

// Connect establishes the private websocket connection with JWT authentication
func (pw *UpbitPrivateWS) Connect(ctx context.Context) error {
	pw.connectionMu.Lock()
	defer pw.connectionMu.Unlock()

	if pw.isConnected {
		return nil
	}

	// Generate JWT token for websocket authentication
	token, err := pw.client.Token(nil)
	if err != nil {
		return fmt.Errorf("failed to generate JWT token: %v", err)
	}

	// Create headers with authentication
	headers := http.Header{}
	headers.Set("Authorization", token)

	// Create websocket client with custom headers
	pw.wsClient = core.NewWsClientWithHeaders(
		privateWsURL,
		wsLifetime,
		headers,
		nil, // no auth function needed since we use headers
		pw.userDataHandler,
		nil, // no request ID function needed for Upbit
		nil, // no extract ID function needed
		nil, // no extract error function needed
		pw.afterConnect,
	)

	_, err = pw.wsClient.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to private websocket: %v", err)
	}

	pw.isConnected = true
	return nil
}

// Close closes the private websocket connection
func (pw *UpbitPrivateWS) Close() error {
	pw.connectionMu.Lock()
	defer pw.connectionMu.Unlock()

	if !pw.isConnected {
		return nil
	}

	if pw.wsClient != nil {
		err := pw.wsClient.Close()
		if err != nil {
			return err
		}
	}

	// Close channels
	close(pw.orderEventCh)
	close(pw.balanceEventCh)

	// Reset subscription state
	pw.subscriptionMu.Lock()
	pw.subscribedSymbols = nil
	pw.subscribedAssets = nil
	pw.subscriptionMu.Unlock()

	pw.isConnected = false
	return nil
}

// afterConnect handles post-connection setup and subscriptions
func (pw *UpbitPrivateWS) afterConnect(c *core.WsClient) error {
	// Subscribe to both order and balance events in a single subscription
	// Upbit private WebSocket expects multiple types in one subscription message
	subscription := []map[string]interface{}{
		{"ticket": fmt.Sprintf("private-%d", time.Now().UnixNano())},
		{"type": "myOrder"},
		{"type": "myAsset"},
		{"format": "DEFAULT"},
	}

	// Send subscription message
	if err := c.SendMessage(subscription); err != nil {
		return fmt.Errorf("failed to subscribe to private events: %v", err)
	}

	return nil
}

// userDataHandler processes incoming websocket messages
func (pw *UpbitPrivateWS) userDataHandler(msg []byte) {
	var baseMsg map[string]interface{}
	if err := json.Unmarshal(msg, &baseMsg); err != nil {
		fmt.Printf("Failed to unmarshal base message: %v\n", err)
		return
	}

	msgType, ok := baseMsg["type"].(string)
	if !ok {
		fmt.Printf("Message type not found or invalid\n")
		return
	}

	switch msgType {
	case "myOrder":
		pw.handleOrderUpdate(msg)
	case "myAsset":
		pw.handleBalanceUpdate(msg)
	default:
		fmt.Printf("Unknown message type: %s\n", msgType)
	}
}

// handleOrderUpdate processes order update messages
func (pw *UpbitPrivateWS) handleOrderUpdate(msg []byte) {
	var wsOrder WsOrder
	if err := json.Unmarshal(msg, &wsOrder); err != nil {
		fmt.Printf("Failed to unmarshal order update: %v\n", err)
		return
	}

	// Convert websocket order to core.OrderEvent
	orderEvent := pw.convertWsOrderToEvent(wsOrder)

	// Send to unified order event channel
	select {
	case pw.orderEventCh <- orderEvent:
	default:
		// Channel is full, skip this update
		fmt.Printf("Order event channel full, dropping update for order %s\n", wsOrder.UUID)
	}
}

// handleBalanceUpdate processes balance update messages
func (pw *UpbitPrivateWS) handleBalanceUpdate(msg []byte) {
	var wsAsset WsAsset
	if err := json.Unmarshal(msg, &wsAsset); err != nil {
		fmt.Printf("Failed to unmarshal balance update: %v\n", err)
		return
	}

	// Process each asset in the update
	for _, asset := range wsAsset.Assets {
		balanceEvent := core.BalanceEvent{
			Asset:      asset.Currency,
			Free:       decimal.NewFromFloat(asset.Balance),
			Locked:     decimal.NewFromFloat(asset.Locked),
			Total:      decimal.NewFromFloat(asset.Balance).Add(decimal.NewFromFloat(asset.Locked)),
			UpdateTime: time.Unix(0, wsAsset.Timestamp*int64(time.Millisecond)),
		}

		// Send to unified balance event channel
		select {
		case pw.balanceEventCh <- balanceEvent:
		default:
			// Channel is full, skip this update
			fmt.Printf("Balance event channel full, dropping update for asset %s\n", asset.Currency)
		}
	}
}

// convertWsOrderToEvent converts a websocket order to core.OrderEvent
func (pw *UpbitPrivateWS) convertWsOrderToEvent(wsOrder WsOrder) core.OrderEvent {
	var side string
	if wsOrder.AskBid == "bid" {
		side = "BUY"
	} else {
		side = "SELL"
	}

	return core.OrderEvent{
		OrderID:         wsOrder.UUID,
		Symbol:          wsOrder.Code,
		Side:            side,
		OrderType:       wsOrder.OrderType,
		Status:          parseOrderStatus(wsOrder.State),
		Price:           decimal.NewFromFloat(wsOrder.Price),
		Quantity:        decimal.NewFromFloat(wsOrder.Volume),
		ExecutedQty:     decimal.NewFromFloat(wsOrder.ExecutedVolume),
		AvgPrice:        decimal.NewFromFloat(wsOrder.AvgPrice),
		Commission:      decimal.NewFromFloat(wsOrder.PaidFee),
		CommissionAsset: "KRW", // Upbit doesn't specify commission asset
		UpdateTime:      time.Unix(0, wsOrder.Timestamp*int64(time.Millisecond)),
		TradeID:         wsOrder.TradeUUID,
		IsMaker:         wsOrder.IsMaker,
	}
}

// GetOrderEventChannel returns the order event channel
func (pw *UpbitPrivateWS) GetOrderEventChannel() <-chan core.OrderEvent {
	return pw.orderEventCh
}

// GetBalanceEventChannel returns the balance event channel
func (pw *UpbitPrivateWS) GetBalanceEventChannel() <-chan core.BalanceEvent {
	return pw.balanceEventCh
}

// IsConnected returns whether the websocket is currently connected
func (pw *UpbitPrivateWS) IsConnected() bool {
	pw.connectionMu.Lock()
	defer pw.connectionMu.Unlock()
	return pw.isConnected
}