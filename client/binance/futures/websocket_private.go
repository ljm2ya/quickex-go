package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/ed25519"
)

const (
	userDataStreamURL     = "wss://fstream.binance.com"
	userDataStreamTestURL = "wss://testnet.binancefuture.com"
	listenKeyLifetime     = 24*time.Hour - 5*time.Minute // 24h minus 5min buffer
	keepaliveInterval     = 30 * time.Minute             // Extend listenKey every 30 minutes
)

// BinanceUserDataStream manages private websocket connections for user data events
type BinanceUserDataStream struct {
	client         *BinanceClient
	apiKey         string
	privateKey     ed25519.PrivateKey
	baseURL        string
	wsConn         *websocket.Conn
	listenKey      string
	orderEventCh   chan core.OrderEvent
	balanceEventCh chan core.BalanceEvent
	isConnected    bool
	connectionMu   sync.Mutex
	stopCh         chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewBinanceUserDataStream creates a new user data stream instance
func NewBinanceUserDataStream(client *BinanceClient, apiKey string, privateKey ed25519.PrivateKey, isTestnet bool) *BinanceUserDataStream {
	baseURL := userDataStreamURL
	if isTestnet {
		baseURL = userDataStreamTestURL
	}

	return &BinanceUserDataStream{
		client:         client,
		apiKey:         apiKey,
		privateKey:     privateKey,
		baseURL:        baseURL,
		orderEventCh:   make(chan core.OrderEvent, 100),
		balanceEventCh: make(chan core.BalanceEvent, 100),
		connectionMu:   sync.Mutex{},
		stopCh:         make(chan struct{}),
	}
}

// Connect establishes the user data stream websocket connection
func (uds *BinanceUserDataStream) Connect(ctx context.Context) error {
	uds.connectionMu.Lock()
	defer uds.connectionMu.Unlock()

	if uds.isConnected {
		return nil
	}

	uds.ctx, uds.cancel = context.WithCancel(ctx)

	// First get a listenKey via WebSocket
	if err := uds.obtainListenKey(); err != nil {
		return fmt.Errorf("failed to obtain listen key: %v", err)
	}

	// Now connect to the user data stream with the listenKey
	if err := uds.connectWithListenKey(); err != nil {
		return fmt.Errorf("failed to connect with listen key: %v", err)
	}

	// Start message handling goroutine
	go uds.handleMessages()

	// Start keepalive goroutine
	go uds.keepaliveLoop()

	uds.isConnected = true
	return nil
}

// Close closes the user data stream connection
func (uds *BinanceUserDataStream) Close() error {
	uds.connectionMu.Lock()
	defer uds.connectionMu.Unlock()

	if !uds.isConnected {
		return nil
	}

	// Stop keepalive and message handling
	if uds.cancel != nil {
		uds.cancel()
	}

	close(uds.stopCh)

	// Stop listen key if we have connection
	if uds.wsConn != nil && uds.listenKey != "" {
		uds.stopListenKey()
	}

	// Close websocket connection
	if uds.wsConn != nil {
		uds.wsConn.Close()
	}

	// Close channels
	close(uds.orderEventCh)
	close(uds.balanceEventCh)

	uds.isConnected = false
	uds.listenKey = ""
	return nil
}

// obtainListenKey gets initial listenKey via REST API
func (uds *BinanceUserDataStream) obtainListenKey() error {
	// Use REST API to start listen key
	restURL := "https://fapi.binance.com"
	if uds.baseURL == userDataStreamTestURL {
		restURL = "https://testnet.binancefuture.com"
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", restURL+"/fapi/v1/listenKey", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Add API key header
	req.Header.Set("X-MBX-APIKEY", uds.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("REST API request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var response struct {
		ListenKey string `json:"listenKey"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	uds.listenKey = response.ListenKey
	fmt.Printf("[binance] User data stream listen key obtained via REST: %s\n", uds.listenKey)
	return nil
}

// connectWithListenKey connects to user data stream with the obtained listenKey
func (uds *BinanceUserDataStream) connectWithListenKey() error {
	// Connect to user data stream URL with listenKey
	// Based on official docs: wss://fstream.binance.com/ws/<listenKey>
	streamURL := fmt.Sprintf("%s/ws/%s", uds.baseURL, uds.listenKey)

	dialer := websocket.DefaultDialer
	conn, resp, err := dialer.Dial(streamURL, http.Header{})
	if err != nil {
		if resp != nil {
			fmt.Printf("[binance] WebSocket dial failed with status: %d\n", resp.StatusCode)
		}
		return fmt.Errorf("failed to connect to user data stream with listenKey: %v", err)
	}

	uds.wsConn = conn
	fmt.Printf("[binance] Connected to user data stream: %s\n", streamURL)
	return nil
}

// keepaliveListenKey extends listenKey validity via REST API
func (uds *BinanceUserDataStream) keepaliveListenKey() error {
	// Use REST API to extend listen key
	restURL := "https://fapi.binance.com"
	if uds.baseURL == userDataStreamTestURL {
		restURL = "https://testnet.binancefuture.com"
	}

	// Create HTTP request with listen key
	req, err := http.NewRequest("PUT", restURL+"/fapi/v1/listenKey", nil)
	if err != nil {
		return fmt.Errorf("failed to create keepalive request: %v", err)
	}

	// Add API key header
	req.Header.Set("X-MBX-APIKEY", uds.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send keepalive request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("keepalive request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("[binance] User data stream listen key extended via REST\n")
	return nil
}

// stopListenKey stops the current listenKey via REST API
func (uds *BinanceUserDataStream) stopListenKey() error {
	// Use REST API to close listen key
	restURL := "https://fapi.binance.com"
	if uds.baseURL == userDataStreamTestURL {
		restURL = "https://testnet.binancefuture.com"
	}

	// Create HTTP request
	req, err := http.NewRequest("DELETE", restURL+"/fapi/v1/listenKey", nil)
	if err != nil {
		return fmt.Errorf("failed to create stop request: %v", err)
	}

	// Add API key header
	req.Header.Set("X-MBX-APIKEY", uds.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send stop request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("stop request failed with status %d", resp.StatusCode)
	}

	fmt.Printf("[binance] User data stream listen key stopped via REST\n")
	return nil
}

// keepaliveLoop runs the keepalive timer to extend listenKey validity
func (uds *BinanceUserDataStream) keepaliveLoop() {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := uds.keepaliveListenKey(); err != nil {
				fmt.Printf("[binance] Failed to keepalive listen key: %v\n", err)
				// Try to reconnect if keepalive fails
				go uds.reconnect()
				return
			}
		case <-uds.ctx.Done():
			return
		case <-uds.stopCh:
			return
		}
	}
}

// handleMessages processes incoming websocket messages
func (uds *BinanceUserDataStream) handleMessages() {
	for {
		select {
		case <-uds.ctx.Done():
			return
		case <-uds.stopCh:
			return
		default:
		}

		_, msg, err := uds.wsConn.ReadMessage()
		if err != nil {
			fmt.Printf("[binance] WebSocket read error: %v\n", err)
			// Try to reconnect on read error
			go uds.reconnect()
			return
		}

		uds.handleUserDataEvent(msg)
	}
}

// handleUserDataEvent processes user data stream events
func (uds *BinanceUserDataStream) handleUserDataEvent(msg []byte) {
	var baseEvent map[string]interface{}
	if err := json.Unmarshal(msg, &baseEvent); err != nil {
		fmt.Printf("[binance] Failed to unmarshal user data event: %v\n", err)
		return
	}

	eventType, ok := baseEvent["e"].(string)
	if !ok {
		// Not an event message, might be a response to our requests
		return
	}

	switch eventType {
	case "ACCOUNT_UPDATE":
		uds.handleAccountUpdate(msg)
	case "ORDER_TRADE_UPDATE":
		uds.handleOrderUpdate(msg)
	case "listenKeyExpired":
		uds.handleListenKeyExpired()
	case "TRADE_LITE":
		// TRADE_LITE events are lightweight trade updates, safe to ignore for user data stream
		// These are typically aggregated trade information
	default:
		fmt.Printf("[binance] Unknown user data event type: %s\n", eventType)
	}
}

// handleAccountUpdate processes ACCOUNT_UPDATE events (balance and position updates)
func (uds *BinanceUserDataStream) handleAccountUpdate(msg []byte) {
	var accountUpdate struct {
		EventType      string `json:"e"`
		EventTime      int64  `json:"E"`
		TransactionTime int64 `json:"T"`
		Account        struct {
			EventReasonType string `json:"m"`
			Balances        []struct {
				Asset            string `json:"a"`
				WalletBalance    string `json:"wb"`
				CrossWalletBalance string `json:"cw"`
			} `json:"B"`
			Positions []struct {
				Symbol                    string `json:"s"`
				PositionAmount           string `json:"pa"`
				EntryPrice               string `json:"ep"`
				AccumulatedRealized      string `json:"cr"`
				UnrealizedPnl           string `json:"up"`
				MarginType              string `json:"mt"`
				IsolatedWallet          string `json:"iw"`
				PositionSide            string `json:"ps"`
			} `json:"P"`
		} `json:"a"`
	}

	if err := json.Unmarshal(msg, &accountUpdate); err != nil {
		fmt.Printf("[binance] Failed to unmarshal ACCOUNT_UPDATE: %v\n", err)
		return
	}

	// Forward balance updates
	for _, balance := range accountUpdate.Account.Balances {
		walletBalance, _ := decimal.NewFromString(balance.WalletBalance)
		crossBalance, _ := decimal.NewFromString(balance.CrossWalletBalance)

		balanceEvent := core.BalanceEvent{
			Asset:      balance.Asset,
			Free:       crossBalance,
			Locked:     walletBalance.Sub(crossBalance),
			Total:      walletBalance,
			UpdateTime: time.Unix(0, accountUpdate.EventTime*int64(time.Millisecond)),
		}

		select {
		case uds.balanceEventCh <- balanceEvent:
		default:
			fmt.Printf("[binance] Balance event channel full, dropping update for asset %s\n", balance.Asset)
		}
	}
}

// handleOrderUpdate processes ORDER_TRADE_UPDATE events
func (uds *BinanceUserDataStream) handleOrderUpdate(msg []byte) {
	var orderUpdate struct {
		EventType       string `json:"e"`
		EventTime       int64  `json:"E"`
		TransactionTime int64  `json:"T"`
		Order           struct {
			Symbol              string `json:"s"`
			ClientOrderID       string `json:"c"`
			Side                string `json:"S"`
			OrderType           string `json:"o"`
			TimeInForce         string `json:"f"`
			OriginalQty         string `json:"q"`
			OriginalPrice       string `json:"p"`
			AveragePrice        string `json:"ap"`
			StopPrice           string `json:"sp"`
			ExecutionType       string `json:"x"`
			OrderStatus         string `json:"X"`
			OrderID             int64  `json:"i"`
			LastFilledQty       string `json:"l"`
			FilledAccumulatedQty string `json:"z"`
			LastFilledPrice     string `json:"L"`
			CommissionAsset     string `json:"N"`
			CommissionAmount    string `json:"n"`
			OrderTradeTime      int64  `json:"T"`
			TradeID             int64  `json:"t"`
			BidsNotional        string `json:"b"`
			AsksNotional        string `json:"a"`
			IsMaker             bool   `json:"m"`
			IsReduceOnly        bool   `json:"R"`
			WorkingType         string `json:"wt"`
			OriginalOrderType   string `json:"ot"`
			PositionSide        string `json:"ps"`
			ClosePosition       bool   `json:"cp"`
			ActivationPrice     string `json:"AP"`
			CallbackRate        string `json:"cr"`
			RealizedProfit      string `json:"rp"`
		} `json:"o"`
	}

	if err := json.Unmarshal(msg, &orderUpdate); err != nil {
		fmt.Printf("[binance] Failed to unmarshal ORDER_TRADE_UPDATE: %v\n", err)
		return
	}

	order := orderUpdate.Order

	// Convert status
	var status core.OrderStatus
	switch order.OrderStatus {
	case "NEW":
		status = core.OrderStatusOpen
	case "FILLED":
		status = core.OrderStatusFilled
	case "CANCELED":
		status = core.OrderStatusCanceled
	case "REJECTED", "EXPIRED":
		status = core.OrderStatusError
	case "PARTIALLY_FILLED":
		status = core.OrderStatusOpen
	default:
		status = core.OrderStatusOpen
	}

	// Parse decimal values
	price, _ := decimal.NewFromString(order.OriginalPrice)
	quantity, _ := decimal.NewFromString(order.OriginalQty)
	executedQty, _ := decimal.NewFromString(order.FilledAccumulatedQty)
	avgPrice, _ := decimal.NewFromString(order.AveragePrice)
	commission, _ := decimal.NewFromString(order.CommissionAmount)

	orderEvent := core.OrderEvent{
		OrderID:         fmt.Sprintf("%d", order.OrderID),
		Symbol:          order.Symbol,
		Side:            order.Side,
		OrderType:       order.OrderType,
		Status:          status,
		Price:           price,
		Quantity:        quantity,
		ExecutedQty:     executedQty,
		AvgPrice:        avgPrice,
		Commission:      commission,
		CommissionAsset: order.CommissionAsset,
		UpdateTime:      time.Unix(0, orderUpdate.EventTime*int64(time.Millisecond)),
		TradeID:         fmt.Sprintf("%d", order.TradeID),
		IsMaker:         order.IsMaker,
	}

	select {
	case uds.orderEventCh <- orderEvent:
	default:
		fmt.Printf("[binance] Order event channel full, dropping update for order %d\n", order.OrderID)
	}
}

// handleListenKeyExpired handles listen key expiration events
func (uds *BinanceUserDataStream) handleListenKeyExpired() {
	fmt.Printf("[binance] Listen key expired, reconnecting...\n")
	go uds.reconnect()
}

// reconnect attempts to reconnect the user data stream
func (uds *BinanceUserDataStream) reconnect() {
	uds.connectionMu.Lock()
	defer uds.connectionMu.Unlock()

	if !uds.isConnected {
		return
	}

	fmt.Printf("[binance] Reconnecting user data stream...\n")

	// Close current connection
	if uds.wsConn != nil {
		uds.wsConn.Close()
	}

	// Wait a bit before reconnecting
	time.Sleep(5 * time.Second)

	// Try to reconnect
	if err := uds.connectInternal(); err != nil {
		fmt.Printf("[binance] Failed to reconnect user data stream: %v\n", err)
		// Try again after delay
		time.Sleep(10 * time.Second)
		go uds.reconnect()
	}
}

// connectInternal is the internal connect logic without mutex
func (uds *BinanceUserDataStream) connectInternal() error {
	// Get new listenKey
	if err := uds.obtainListenKey(); err != nil {
		return fmt.Errorf("failed to obtain listen key: %v", err)
	}

	// Connect with new listenKey
	if err := uds.connectWithListenKey(); err != nil {
		return fmt.Errorf("failed to connect with listen key: %v", err)
	}

	// Restart message handling and keepalive
	go uds.handleMessages()
	go uds.keepaliveLoop()

	return nil
}

// GetOrderEventChannel returns the order event channel
func (uds *BinanceUserDataStream) GetOrderEventChannel() <-chan core.OrderEvent {
	return uds.orderEventCh
}

// GetBalanceEventChannel returns the balance event channel
func (uds *BinanceUserDataStream) GetBalanceEventChannel() <-chan core.BalanceEvent {
	return uds.balanceEventCh
}

// IsConnected returns whether the stream is currently connected
func (uds *BinanceUserDataStream) IsConnected() bool {
	uds.connectionMu.Lock()
	defer uds.connectionMu.Unlock()
	return uds.isConnected
}