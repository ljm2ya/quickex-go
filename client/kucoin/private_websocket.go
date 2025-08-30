package kucoin

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/client/kucoin/common"
)

// PrivateWebSocket handles authenticated WebSocket connection for order placement
type PrivateWebSocket struct {
	apiKey          string
	apiSecret       string
	apiPassphrase   string
	serverTimeDelta int64             // Add server time delta
	marketType      common.MarketType // Market type (spot or futures)

	conn   *websocket.Conn
	connMu sync.Mutex

	url          string
	token        string
	pingInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	// Channel for order responses
	orderResponses chan *common.OrderWSResponse

	// Request tracking
	requestID     int64
	requestIDMu   sync.Mutex
	pendingOrders map[string]chan *common.OrderWSResponse
	pendingMu     sync.RWMutex

	// Connection error handling
	connErr   chan error
	connReady chan struct{}
}

// Type aliases for backward compatibility
type OrderWSRequest = common.OrderWSRequest
type OrderWSResponse = common.OrderWSResponse

// NewPrivateWebSocket creates a new private WebSocket connection
func NewPrivateWebSocket(apiKey, apiSecret, apiPassphrase string, serverTimeDelta int64) *PrivateWebSocket {
	return NewPrivateWebSocketWithMarket(apiKey, apiSecret, apiPassphrase, serverTimeDelta, common.MarketTypeSpot)
}

// NewPrivateWebSocketWithMarket creates a new private WebSocket connection for specific market
func NewPrivateWebSocketWithMarket(apiKey, apiSecret, apiPassphrase string, serverTimeDelta int64, marketType common.MarketType) *PrivateWebSocket {
	ctx, cancel := context.WithCancel(context.Background())

	return &PrivateWebSocket{
		apiKey:          apiKey,
		apiSecret:       apiSecret,
		apiPassphrase:   apiPassphrase,
		serverTimeDelta: serverTimeDelta,
		marketType:      marketType,
		ctx:             ctx,
		cancel:          cancel,
		orderResponses:  make(chan *common.OrderWSResponse, 100),
		pendingOrders:   make(map[string]chan *common.OrderWSResponse),
		connErr:         make(chan error, 1),
		connReady:       make(chan struct{}, 1),
	}
}

// Connect establishes the private WebSocket connection with authentication
func (ws *PrivateWebSocket) Connect() error {
	// Generate timestamp adjusted with server time delta
	timestamp := strconv.FormatInt(time.Now().UnixNano()/1e6-ws.serverTimeDelta, 10)

	// Create signature for WebSocket authentication
	// The signature string is "apikey+timestamp"
	signatureString := ws.apiKey + timestamp
	h := hmac.New(sha256.New, []byte(ws.apiSecret))
	h.Write([]byte(signatureString))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Create passphrase signature
	ph := hmac.New(sha256.New, []byte(ws.apiSecret))
	ph.Write([]byte(ws.apiPassphrase))
	passphraseSignature := base64.StdEncoding.EncodeToString(ph.Sum(nil))

	// Build WebSocket URL with authentication parameters (URL encoded)
	ws.url = fmt.Sprintf("wss://wsapi.kucoin.com/v1/private?apikey=%s&sign=%s&passphrase=%s&timestamp=%s",
		url.QueryEscape(ws.apiKey),
		url.QueryEscape(signature),
		url.QueryEscape(passphraseSignature),
		timestamp)
	ws.pingInterval = 20 * time.Second // Default ping interval

	// Connect to WebSocket
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(ws.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	ws.conn = conn
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			// Log error or handle reconnection
		}
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	if sessionId, hasSession := msg["sessionId"].(string); hasSession {
		if timestamp, hasTimestamp := msg["timestamp"].(float64); hasTimestamp {
			// Send authentication response
			ws.authenticateSession(sessionId, int64(timestamp))
		}
	} else {
		msgBytes, _ := json.Marshal(msg)
		return fmt.Errorf("Unknown WebSocket Response: %s", string(msgBytes))
	}

	if err := conn.ReadJSON(&msg); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			// Log error or handle reconnection
		}
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	if _, hasCode := msg["code"]; hasCode {
		// Error message - log it and send to error channel
		msgBytes, _ := json.Marshal(msg)
		return fmt.Errorf("KuCoin WebSocket error: %s", string(msgBytes))
	}
	// Start message reader
	go ws.readMessages()

	// Start ping sender
	go ws.pingLoop()
	return nil
}

// pingLoop sends periodic ping messages to keep the connection alive
func (ws *PrivateWebSocket) pingLoop() {
	ticker := time.NewTicker(ws.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-ticker.C:
			ws.sendPing()
		}
	}
}

// sendPing sends a ping message
func (ws *PrivateWebSocket) sendPing() error {
	ws.connMu.Lock()
	defer ws.connMu.Unlock()

	if ws.conn == nil {
		return fmt.Errorf("connection is closed")
	}

	pingMsg := common.PingMessage{
		Id:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Op:        "ping",
		Timestamp: time.Now().Unix(),
	}

	return ws.conn.WriteJSON(pingMsg)
}

// readMessages reads messages from the WebSocket connection
func (ws *PrivateWebSocket) readMessages() {
	defer func() {
		ws.Close()
	}()

	for {
		select {
		case <-ws.ctx.Done():
			return
		default:
			var msg map[string]interface{}
			if err := ws.conn.ReadJSON(&msg); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					// Log error or handle reconnection
				}
				return
			}

			/*
				// Filter and log important messages
				if data, ok := msg["data"].(string); ok && data == "welcome" {
					// Connection established successfully - signal ready
					select {
					case ws.connReady <- struct{}{}:
					default:
						// Channel already signaled
					}
				} else if code, hasCode := msg["code"]; hasCode {
					// Error message - log it and send to error channel
					msgBytes, _ := json.Marshal(msg)
					fmt.Printf("[ERROR] KuCoin WebSocket error: %s\n", string(msgBytes))

					// Extract error details
					var errMsg string
					if m, ok := msg["msg"].(string); ok {
						errMsg = m
					} else if m, ok := msg["message"].(string); ok {
						errMsg = m
					} else {
						errMsg = fmt.Sprintf("error code: %v", code)
					}

					// Send error to connection error channel (non-blocking)
					select {
					case ws.connErr <- fmt.Errorf("KuCoin WebSocket error: %s", errMsg):
					default:
						// Error channel full or already used
					}
				}
			*/

			// Handle different message types based on the raw message
			msgType, _ := msg["type"].(string)

			switch msgType {
			case "pong":
				// Pong received, connection is healthy
			case "welcome":
				// Connection authenticated successfully - signal ready
				select {
				case ws.connReady <- struct{}{}:
				default:
					// Channel already signaled
				}
			default:
				// Check if it's a session message
				if sessionId, hasSession := msg["sessionId"].(string); hasSession {
					if timestamp, hasTimestamp := msg["timestamp"].(float64); hasTimestamp {
						// Send authentication response
						ws.authenticateSession(sessionId, int64(timestamp))
					}
				} else if _, hasID := msg["id"].(string); hasID {
					// Check if this is an order response
					// Check if it's an error response
					if code, hasCode := msg["code"].(float64); hasCode && code != 0 {
						ws.handleErrorRaw(msg)
					} else {
						// It's likely an order response
						ws.handleOrderResponseRaw(msg)
					}
				}
			}
		}
	}
}

// PlaceOrder places an order via WebSocket
func (ws *PrivateWebSocket) PlaceOrder(req *OrderWSRequest) (*OrderWSResponse, error) {
	// Generate request ID
	ws.requestIDMu.Lock()
	ws.requestID++
	requestID := fmt.Sprintf("%d", ws.requestID)
	ws.requestIDMu.Unlock()

	// Create response channel
	respChan := make(chan *OrderWSResponse, 1)
	ws.pendingMu.Lock()
	ws.pendingOrders[requestID] = respChan
	ws.pendingMu.Unlock()

	// Create order message using the unified trading API format
	args := map[string]interface{}{
		"symbol":      req.Symbol,
		"side":        strings.ToLower(req.Side),
		"type":        strings.ToLower(req.Type),
		"timeInForce": strings.ToUpper(req.TimeInForce),
	}

	// Add price and quantity for limit orders
	if req.Type == "limit" {
		args["price"] = req.Price
		args["size"] = req.Size
	} else if req.Type == "market" {
		// For market orders
		if ws.marketType == common.MarketTypeSpot && req.Side == "buy" {
			// Spot market buy uses funds
			args["funds"] = req.Funds
		} else {
			// Spot market sell and all futures market orders use size
			args["size"] = req.Size
		}
	}

	// Add futures-specific fields if applicable
	if ws.marketType == common.MarketTypeFutures {
		if req.Leverage != "" {
			args["leverage"] = req.Leverage
		}
		if req.StopPrice != "" {
			args["stopPrice"] = req.StopPrice
		}
	}

	// Determine operation based on market type
	operation := "spot.order"
	if ws.marketType == common.MarketTypeFutures {
		operation = "futures.order"
	}

	// Use the unified trading API message format
	msg := map[string]interface{}{
		"id":   requestID,
		"op":   operation,
		"args": args,
	}

	// Log the message being sent for debugging
	//msgBytes, _ := json.Marshal(msg)
	json.Marshal(msg)

	// Send order
	ws.connMu.Lock()
	err := ws.conn.WriteJSON(msg)
	ws.connMu.Unlock()

	if err != nil {
		// Clean up on error
		ws.pendingMu.Lock()
		delete(ws.pendingOrders, requestID)
		ws.pendingMu.Unlock()
		close(respChan)
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		return resp, nil
	case <-time.After(5 * time.Second):
		// Clean up on timeout
		ws.pendingMu.Lock()
		delete(ws.pendingOrders, requestID)
		ws.pendingMu.Unlock()
		close(respChan)
		return nil, fmt.Errorf("order placement timeout")
	case <-ws.ctx.Done():
		return nil, fmt.Errorf("connection closed")
	}
}

// Close closes the WebSocket connection
func (ws *PrivateWebSocket) Close() error {
	ws.cancel()

	ws.connMu.Lock()
	defer ws.connMu.Unlock()

	if ws.conn != nil {
		err := ws.conn.Close()
		ws.conn = nil
		return err
	}

	return nil
}

// IsConnected checks if the WebSocket is connected
func (ws *PrivateWebSocket) IsConnected() bool {
	ws.connMu.Lock()
	defer ws.connMu.Unlock()

	return ws.conn != nil
}

// authenticateSession sends authentication response for session
func (ws *PrivateWebSocket) authenticateSession(sessionId string, timestamp int64) error {
	// Create the session JSON to encrypt
	sessionData := map[string]interface{}{
		"sessionId": sessionId,
		"timestamp": timestamp,
	}

	// Convert to JSON
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	// Encrypt with HMAC SHA256
	h := hmac.New(sha256.New, []byte(ws.apiSecret))
	h.Write(sessionJSON)
	authToken := base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Send authentication message - try different formats
	// First try just sending the token as a string
	ws.connMu.Lock()
	defer ws.connMu.Unlock()

	if ws.conn == nil {
		return fmt.Errorf("connection is closed")
	}

	// Try sending just the auth token as text
	return ws.conn.WriteMessage(websocket.TextMessage, []byte(authToken))
}

// handleOrderResponseRaw processes raw order placement responses
func (ws *PrivateWebSocket) handleOrderResponseRaw(msg map[string]interface{}) {
	id, _ := msg["id"].(string)

	// Extract data from the response
	data, _ := msg["data"].(map[string]interface{})
	if data == nil {
		// If no data field, check result field
		data, _ = msg["result"].(map[string]interface{})
	}
	if data == nil {
		// If still no data, the whole message might be the response
		data = msg
	}

	response := &OrderWSResponse{
		Success: true,
	}

	// Check for order ID in response (could be orderId or order_id)
	if orderId, ok := data["orderId"].(string); ok {
		response.OrderID = orderId
	} else if orderId, ok := data["order_id"].(string); ok {
		response.OrderID = orderId
	}

	// Check for client OID (could be clientOid or client_oid)
	if clientOid, ok := data["clientOid"].(string); ok {
		response.ClientOid = clientOid
	} else if clientOid, ok := data["client_oid"].(string); ok {
		response.ClientOid = clientOid
	}

	// Send to pending order channel if exists
	ws.pendingMu.RLock()
	if ch, ok := ws.pendingOrders[id]; ok {
		ws.pendingMu.RUnlock()
		ch <- response

		// Clean up
		ws.pendingMu.Lock()
		delete(ws.pendingOrders, id)
		close(ch)
		ws.pendingMu.Unlock()
	} else {
		ws.pendingMu.RUnlock()
	}
}

// handleErrorRaw processes raw error messages
func (ws *PrivateWebSocket) handleErrorRaw(msg map[string]interface{}) {
	id, _ := msg["id"].(string)

	response := &OrderWSResponse{
		Success: false,
	}

	// Extract error message from various possible fields
	if errMsg, ok := msg["msg"].(string); ok {
		response.Error = errMsg
	} else if errMsg, ok := msg["message"].(string); ok {
		response.Error = errMsg
	} else if data, ok := msg["data"].(map[string]interface{}); ok {
		if errMsg, ok := data["msg"].(string); ok {
			response.Error = errMsg
		} else if errMsg, ok := data["message"].(string); ok {
			response.Error = errMsg
		}
	} else {
		// If no specific error message, create one from code if available
		if code, ok := msg["code"].(float64); ok {
			response.Error = fmt.Sprintf("Error code: %v", code)
		}
	}

	// Send to pending order channel if exists
	ws.pendingMu.RLock()
	if ch, ok := ws.pendingOrders[id]; ok {
		ws.pendingMu.RUnlock()
		ch <- response

		// Clean up
		ws.pendingMu.Lock()
		delete(ws.pendingOrders, id)
		close(ch)
		ws.pendingMu.Unlock()
	} else {
		ws.pendingMu.RUnlock()
	}
}
