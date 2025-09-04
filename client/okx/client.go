package okx

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
)

const (
	okxWSURLPublic  = "wss://ws.okx.com:8443/ws/v5/public"
	okxWSURLPrivate = "wss://ws.okx.com:8443/ws/v5/private"
	wsLifetime      = 23*time.Hour + 50*time.Minute
)

type OKXClient struct {
	*core.WsClient
	apiKey     string
	secretKey  string
	passphrase string
	
	// Persistent WebSocket connection
	persistentWS *PersistentWebSocket
	
	balances     map[string]*core.Wallet
	orders       map[string]*core.OrderResponse
	quoteChans   map[string]chan core.Quote
	balancesMu   sync.RWMutex
	ordersMu     sync.RWMutex
	quoteChansMu sync.RWMutex
}

func NewClient(apiKey, secretKey, passphrase string) *OKXClient {
	client := &OKXClient{
		apiKey:     apiKey,
		secretKey:  secretKey,
		passphrase: passphrase,
		balances:   make(map[string]*core.Wallet),
		orders:     make(map[string]*core.OrderResponse),
		quoteChans: make(map[string]chan core.Quote),
	}
	
	// Initialize persistent WebSocket for private operations
	client.persistentWS = NewPersistentWebSocket(apiKey, secretKey, passphrase, true)
	
	client.WsClient = core.NewWsClient(
		okxWSURLPrivate,
		wsLifetime,
		client.authFn(),
		client.userDataHandlerFn(),
		requestIDFn(nextWSID),
		extractIDFn(),
		extractErrFn(),
		client.afterConnect(),
	)
	
	return client
}

func nextWSID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(10000))
}

func (c *OKXClient) authFn() core.WsAuthFunc {
	return func(ws *websocket.Conn) (int64, error) {
		if err := ValidateCredentials(c.apiKey, c.secretKey, c.passphrase); err != nil {
			return 0, err
		}
		
		timestamp := GetTimestampSeconds()
		signature := GenerateWSSignature(timestamp, c.secretKey)
		
		loginMsg := map[string]interface{}{
			"op": "login",
			"args": []map[string]interface{}{
				{
					"apiKey":     c.apiKey,
					"passphrase": c.passphrase,
					"timestamp":  timestamp,
					"sign":       signature,
				},
			},
		}
		
		if err := ws.WriteJSON(loginMsg); err != nil {
			return 0, fmt.Errorf("failed to send login message: %w", err)
		}
		
		// Read login response
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return 0, fmt.Errorf("failed to read login response: %w", err)
		}
		
		var resp OKXWSMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			return 0, fmt.Errorf("failed to unmarshal login response: %w", err)
		}
		
		if resp.Code != "0" {
			return 0, fmt.Errorf("login failed: %s - %s", resp.Code, resp.Msg)
		}
		
		// For OKX, we don't get server time in login response, so return 0
		return 0, nil
	}
}

func (c *OKXClient) userDataHandlerFn() core.WsUserEventHandler {
	return func(msg []byte) {
		var wsMsg OKXWSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			fmt.Printf("Failed to unmarshal WebSocket message: %v\n", err)
			return
		}
		
		if wsMsg.Args == nil || len(wsMsg.Args) == 0 {
			return
		}
		
		arg := wsMsg.Args[0]
		switch arg.Channel {
		case "account":
			c.handleAccountUpdate(wsMsg.Data)
		case "orders":
			c.handleOrderUpdate(wsMsg.Data)
		case "tickers":
			c.handleTickerUpdate(wsMsg.Data)
		default:
			// Handle other channels if needed
		}
	}
}

func (c *OKXClient) handleAccountUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var accounts []OKXAccount
	if err := json.Unmarshal(dataBytes, &accounts); err != nil {
		return
	}
	
	c.balancesMu.Lock()
	defer c.balancesMu.Unlock()
	
	for _, account := range accounts {
		for _, detail := range account.Details {
			bal := ToDecimal(detail.Bal)
			availBal := ToDecimal(detail.AvailBal)
			frozenBal := ToDecimal(detail.FrozenBal)
			
			c.balances[detail.Ccy] = &core.Wallet{
				Asset:  detail.Ccy,
				Free:   availBal,
				Locked: frozenBal,
				Total:  bal,
			}
		}
	}
}

func (c *OKXClient) handleOrderUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var orders []OKXOrder
	if err := json.Unmarshal(dataBytes, &orders); err != nil {
		return
	}
	
	c.ordersMu.Lock()
	defer c.ordersMu.Unlock()
	
	for _, order := range orders {
		status := c.mapOrderStatus(order.State)
		
		c.orders[order.OrdID] = &core.OrderResponse{
			OrderID:         order.OrdID,
			Symbol:          order.InstID,
			Side:            order.Side,
			Status:          status,
			Price:           ToDecimal(order.Px),
			Quantity:        ToDecimal(order.Sz),
			IsQuoteQuantity: false,
			CreateTime:      ToTime(order.CTime),
		}
	}
}

func (c *OKXClient) handleTickerUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var tickers []OKXTicker
	if err := json.Unmarshal(dataBytes, &tickers); err != nil {
		return
	}
	
	c.quoteChansMu.RLock()
	defer c.quoteChansMu.RUnlock()
	
	for _, ticker := range tickers {
		if quoteChan, exists := c.quoteChans[ticker.InstID]; exists {
			quote := core.Quote{
				Symbol:   ticker.InstID,
				BidPrice: ToDecimal(ticker.BidPx),
				BidQty:   ToDecimal(ticker.BidSz),
				AskPrice: ToDecimal(ticker.AskPx),
				AskQty:   ToDecimal(ticker.AskSz),
				Time:     ToTime(ticker.Ts),
			}
			
			select {
			case quoteChan <- quote:
			default:
				// Channel full, skip this update
			}
		}
	}
}

func (c *OKXClient) mapOrderStatus(state string) core.OrderStatus {
	switch state {
	case "live", "partially_filled":
		return core.OrderStatusOpen
	case "filled":
		return core.OrderStatusFilled
	case "canceled":
		return core.OrderStatusCanceled
	default:
		return core.OrderStatusError
	}
}

func (c *OKXClient) afterConnect() core.WsAfterConnectFunc {
	return func(wsClient *core.WsClient) error {
		// Subscribe to account updates
		accountMsg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]interface{}{
				{
					"channel": "account",
				},
			},
		}
		
		_, err := wsClient.SendRequest(accountMsg)
		if err != nil {
			return fmt.Errorf("failed to subscribe to account channel: %w", err)
		}
		
		// Subscribe to orders updates
		ordersMsg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]interface{}{
				{
					"channel":  "orders",
					"instType": "SPOT",
				},
			},
		}
		
		_, err = wsClient.SendRequest(ordersMsg)
		if err != nil {
			return fmt.Errorf("failed to subscribe to orders channel: %w", err)
		}
		
		// Load initial account balance
		if err := c.loadInitialBalance(); err != nil {
			fmt.Printf("Warning: Failed to load initial balance: %v\n", err)
		}
		
		return nil
	}
}

func (c *OKXClient) loadInitialBalance() error {
	// This would typically be done via REST API call
	// For now, we'll wait for WebSocket updates
	return nil
}

func requestIDFn(nextWSID func() string) core.WsRequestIDFunc {
	return func(req map[string]interface{}) (interface{}, bool) {
		if id, ok := req["id"].(string); ok && id != "" {
			return id, true
		}
		id := nextWSID()
		req["id"] = id
		return id, true
	}
}

func extractIDFn() core.WsExtractIDFunc {
	return func(root map[string]json.RawMessage) (string, bool) {
		idRaw, ok := root["id"]
		if !ok {
			return "", false
		}
		
		var id string
		if err := json.Unmarshal(idRaw, &id); err == nil && id != "" {
			return id, true
		}
		
		var idNum float64
		if err := json.Unmarshal(idRaw, &idNum); err == nil {
			return strconv.FormatInt(int64(idNum), 10), true
		}
		
		return "", false
	}
}

func extractErrFn() core.WsExtractErrFunc {
	return func(root map[string]json.RawMessage) error {
		// Check for 'code' field
		if codeRaw, ok := root["code"]; ok {
			var code string
			if err := json.Unmarshal(codeRaw, &code); err == nil && code != "0" {
				var msg string
				if msgRaw, ok := root["msg"]; ok {
					_ = json.Unmarshal(msgRaw, &msg)
				}
				return ParseOKXError(code, msg)
			}
		}
		
		// Check for 'event' field with 'error' value
		if eventRaw, ok := root["event"]; ok {
			var event string
			if err := json.Unmarshal(eventRaw, &event); err == nil && event == "error" {
				var code, msg string
				if codeRaw, ok := root["code"]; ok {
					_ = json.Unmarshal(codeRaw, &code)
				}
				if msgRaw, ok := root["msg"]; ok {
					_ = json.Unmarshal(msgRaw, &msg)
				}
				return ParseOKXError(code, msg)
			}
		}
		
		return nil
	}
}

