package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// PersistentWebSocket manages a long-lived WebSocket connection with proper heartbeat
type PersistentWebSocket struct {
	conn           *websocket.Conn
	apiKey         string
	secretKey      string
	passphrase     string
	url            string
	
	// Connection management
	mu             sync.RWMutex
	isConnected    bool
	isAuthenticated bool
	reconnecting   bool
	
	// Heartbeat management
	heartbeatTicker *time.Ticker
	heartbeatStop   chan bool
	lastPong        time.Time
	
	// Message handling
	messageQueue    chan []byte
	responseChans   map[string]chan []byte
	responseMu      sync.RWMutex
	
	// Lifecycle management
	ctx            context.Context
	cancel         context.CancelFunc
	stopChan       chan bool
}

// NewPersistentWebSocket creates a new persistent WebSocket connection
func NewPersistentWebSocket(apiKey, secretKey, passphrase string, isPrivate bool) *PersistentWebSocket {
	ctx, cancel := context.WithCancel(context.Background())
	
	url := "wss://ws.okx.com:8443/ws/v5/public"
	if isPrivate {
		url = "wss://ws.okx.com:8443/ws/v5/private"
	}
	
	pws := &PersistentWebSocket{
		apiKey:        apiKey,
		secretKey:     secretKey,
		passphrase:    passphrase,
		url:           url,
		messageQueue:  make(chan []byte, 100),
		responseChans: make(map[string]chan []byte),
		ctx:           ctx,
		cancel:        cancel,
		stopChan:      make(chan bool),
		heartbeatStop: make(chan bool),
	}
	
	return pws
}

// Connect establishes the WebSocket connection with authentication
func (pws *PersistentWebSocket) Connect() error {
	pws.mu.Lock()
	defer pws.mu.Unlock()
	
	if pws.isConnected {
		return nil // Already connected
	}
	
	// Establish WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(pws.url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	
	pws.conn = conn
	pws.isConnected = true
	pws.lastPong = time.Now()
	
	// Authenticate for private endpoints
	if pws.apiKey != "" {
		if err := pws.authenticate(); err != nil {
			pws.conn.Close()
			pws.isConnected = false
			return fmt.Errorf("authentication failed: %w", err)
		}
		pws.isAuthenticated = true
	}
	
	// Start message handling goroutines
	go pws.messageReader()
	go pws.messageWriter()
	go pws.startHeartbeat()
	
	fmt.Println("✅ WebSocket connected and authenticated successfully")
	return nil
}

// authenticate handles the login process for private WebSocket
func (pws *PersistentWebSocket) authenticate() error {
	timestamp := GetTimestampSeconds()
	signature := GenerateWSSignature(timestamp, pws.secretKey)
	
	loginMsg := map[string]interface{}{
		"op": "login",
		"args": []map[string]interface{}{
			{
				"apiKey":     pws.apiKey,
				"passphrase": pws.passphrase,
				"timestamp":  timestamp,
				"sign":       signature,
			},
		},
	}
	
	// Send login message
	if err := pws.conn.WriteJSON(loginMsg); err != nil {
		return err
	}
	
	// Wait for login response
	pws.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := pws.conn.ReadMessage()
	if err != nil {
		return err
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		return err
	}
	
	if code, ok := response["code"].(string); ok && code != "0" {
		return fmt.Errorf("login failed: %s", response["msg"])
	}
	
	return nil
}

// startHeartbeat starts the heartbeat mechanism (ping every 25 seconds)
func (pws *PersistentWebSocket) startHeartbeat() {
	pws.heartbeatTicker = time.NewTicker(25 * time.Second)
	defer pws.heartbeatTicker.Stop()
	
	for {
		select {
		case <-pws.heartbeatTicker.C:
			pws.sendPing()
		case <-pws.heartbeatStop:
			return
		case <-pws.ctx.Done():
			return
		}
	}
}

// sendPing sends a ping message to keep the connection alive
func (pws *PersistentWebSocket) sendPing() {
	pws.mu.RLock()
	if !pws.isConnected || pws.conn == nil {
		pws.mu.RUnlock()
		return
	}
	conn := pws.conn
	pws.mu.RUnlock()
	
	pingMsg := "ping"
	if err := conn.WriteMessage(websocket.TextMessage, []byte(pingMsg)); err != nil {
		fmt.Printf("⚠️ Failed to send ping: %v\n", err)
		go pws.reconnect()
		return
	}
	
	fmt.Println("💓 Heartbeat ping sent")
}

// messageReader continuously reads messages from WebSocket
func (pws *PersistentWebSocket) messageReader() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("⚠️ Message reader panic: %v\n", r)
			go pws.reconnect()
		}
	}()
	
	for {
		select {
		case <-pws.ctx.Done():
			return
		default:
			pws.mu.RLock()
			if !pws.isConnected || pws.conn == nil {
				pws.mu.RUnlock()
				return
			}
			conn := pws.conn
			pws.mu.RUnlock()
			
			// Set read deadline
			conn.SetReadDeadline(time.Now().Add(35 * time.Second)) // 35 seconds timeout
			
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("⚠️ WebSocket read error: %v\n", err)
				go pws.reconnect()
				return
			}
			
			if messageType == websocket.TextMessage {
				// Handle pong messages
				if string(message) == "pong" {
					pws.lastPong = time.Now()
					fmt.Println("💚 Heartbeat pong received")
					continue
				}
				
				// Route message to appropriate handler
				pws.handleMessage(message)
			}
		}
	}
}

// messageWriter handles outgoing messages
func (pws *PersistentWebSocket) messageWriter() {
	for {
		select {
		case message := <-pws.messageQueue:
			pws.mu.RLock()
			if !pws.isConnected || pws.conn == nil {
				pws.mu.RUnlock()
				continue
			}
			conn := pws.conn
			pws.mu.RUnlock()
			
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				fmt.Printf("⚠️ WebSocket write error: %v\n", err)
				go pws.reconnect()
				return
			}
		case <-pws.ctx.Done():
			return
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (pws *PersistentWebSocket) handleMessage(message []byte) {
	var response map[string]interface{}
	if err := json.Unmarshal(message, &response); err != nil {
		fmt.Printf("⚠️ Failed to unmarshal WebSocket message: %v\n", err)
		return
	}
	
	// Check for message ID to route response (try multiple ID field names)
	var id string
	var found bool
	
	// Try "id" field first
	if idVal, ok := response["id"].(string); ok && idVal != "" {
		id = idVal
		found = true
	}
	
	// Try numeric ID
	if !found {
		if idVal, ok := response["id"].(float64); ok {
			id = fmt.Sprintf("%.0f", idVal)
			found = true
		}
	}
	
	if found {
		pws.responseMu.RLock()
		if responseChan, exists := pws.responseChans[id]; exists {
			select {
			case responseChan <- message:
			default:
				// Channel full, skip
			}
		}
		pws.responseMu.RUnlock()
	}
}

// SendRequest sends a request and waits for response
func (pws *PersistentWebSocket) SendRequest(request map[string]interface{}) ([]byte, error) {
	// Ensure connection
	if err := pws.ensureConnected(); err != nil {
		return nil, err
	}
	
	// Generate request ID if not present
	requestID, ok := request["id"].(string)
	if !ok {
		// Use simple numeric ID format like successful tests
		requestID = fmt.Sprintf("r%d", time.Now().UnixNano()%10000)
		request["id"] = requestID
	}
	
	// Create response channel
	responseChan := make(chan []byte, 1)
	pws.responseMu.Lock()
	pws.responseChans[requestID] = responseChan
	pws.responseMu.Unlock()
	
	// Clean up response channel after request
	defer func() {
		pws.responseMu.Lock()
		delete(pws.responseChans, requestID)
		close(responseChan)
		pws.responseMu.Unlock()
	}()
	
	// Marshal and send request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	
	select {
	case pws.messageQueue <- requestBytes:
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("request queue timeout")
	}
	
	// Wait for response
	select {
	case response := <-responseChan:
		return response, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	case <-pws.ctx.Done():
		return nil, fmt.Errorf("context cancelled")
	}
}

// ensureConnected ensures the WebSocket is connected and authenticated
func (pws *PersistentWebSocket) ensureConnected() error {
	pws.mu.RLock()
	if pws.isConnected && pws.isAuthenticated {
		pws.mu.RUnlock()
		return nil
	}
	pws.mu.RUnlock()
	
	// Try to reconnect
	return pws.Connect()
}

// reconnect handles reconnection logic
func (pws *PersistentWebSocket) reconnect() {
	pws.mu.Lock()
	if pws.reconnecting {
		pws.mu.Unlock()
		return // Already reconnecting
	}
	pws.reconnecting = true
	pws.mu.Unlock()
	
	defer func() {
		pws.mu.Lock()
		pws.reconnecting = false
		pws.mu.Unlock()
	}()
	
	fmt.Println("🔄 Reconnecting WebSocket...")
	
	// Close existing connection
	pws.Close()
	
	// Retry connection with exponential backoff
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		if err := pws.Connect(); err != nil {
			waitTime := time.Duration(1<<i) * time.Second
			fmt.Printf("❌ Reconnection attempt %d failed: %v, retrying in %v\n", i+1, err, waitTime)
			time.Sleep(waitTime)
			continue
		}
		
		fmt.Println("✅ WebSocket reconnected successfully")
		return
	}
	
	fmt.Println("❌ Failed to reconnect after maximum retries")
}

// IsConnected returns whether the WebSocket is currently connected
func (pws *PersistentWebSocket) IsConnected() bool {
	pws.mu.RLock()
	defer pws.mu.RUnlock()
	return pws.isConnected && pws.isAuthenticated
}

// Close closes the WebSocket connection
func (pws *PersistentWebSocket) Close() error {
	pws.mu.Lock()
	defer pws.mu.Unlock()
	
	// Stop heartbeat
	close(pws.heartbeatStop)
	pws.heartbeatStop = make(chan bool)
	
	// Cancel context
	pws.cancel()
	
	// Close connection
	if pws.conn != nil {
		pws.conn.Close()
	}
	
	pws.isConnected = false
	pws.isAuthenticated = false
	
	fmt.Println("🔌 WebSocket connection closed")
	return nil
}