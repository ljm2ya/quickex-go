package phemex

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
)

const (
	phemexWSURL = "wss://ws.phemex.com"
	wsLifetime  = 23*time.Hour + 50*time.Minute
	pingInterval = 5 * time.Second
)

type PhemexClient struct {
	*core.WsClient
	apiKey    string
	apiSecret string
	
	// Data storage
	balances     map[string]*core.Wallet
	positions    map[string]*PhemexPosition
	orders       map[string]*core.OrderResponse
	quoteChans   map[string]chan core.Quote
	symbols      map[string]*PhemexSymbol
	
	// Mutexes
	balancesMu   sync.RWMutex
	positionsMu  sync.RWMutex
	ordersMu     sync.RWMutex
	quoteChansMu sync.RWMutex
	symbolsMu    sync.RWMutex
	
	// Connection management
	authenticated bool
	authMu        sync.RWMutex
}

func NewClient(apiKey, apiSecret string) *PhemexClient {
	client := &PhemexClient{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		balances:  make(map[string]*core.Wallet),
		positions: make(map[string]*PhemexPosition),
		orders:    make(map[string]*core.OrderResponse),
		quoteChans: make(map[string]chan core.Quote),
		symbols:   make(map[string]*PhemexSymbol),
	}
	
	client.WsClient = core.NewWsClient(
		phemexWSURL,
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
	return fmt.Sprintf("%d", time.Now().UnixNano()/1000000+int64(rand.Intn(10000)))
}

func (c *PhemexClient) authFn() core.WsAuthFunc {
	return func(ws *websocket.Conn) (int64, error) {
		if err := ValidateCredentials(c.apiKey, c.apiSecret); err != nil {
			return 0, err
		}
		
		expiry := GetExpiryTime()
		signature := GenerateSignature(c.apiSecret, c.apiKey, expiry)
		
		authMsg := PhemexWSMessage{
			ID:     nextWSID(),
			Method: "user.auth",
			Params: PhemexAuthParams{
				APIToken:  c.apiKey,
				Signature: signature,
				Expiry:    expiry,
			},
		}
		
		if err := ws.WriteJSON(authMsg); err != nil {
			return 0, fmt.Errorf("failed to send auth message: %w", err)
		}
		
		// Read auth response
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return 0, fmt.Errorf("failed to read auth response: %w", err)
		}
		
		var resp PhemexWSMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			return 0, fmt.Errorf("failed to unmarshal auth response: %w", err)
		}
		
		if resp.Error != nil {
			return 0, ParsePhemexError(resp.Error.Code, resp.Error.Message)
		}
		
		c.authMu.Lock()
		c.authenticated = true
		c.authMu.Unlock()
		
		// Start ping routine for heartbeat
		go c.startPingRoutine(ws)
		
		return 0, nil
	}
}

func (c *PhemexClient) startPingRoutine(ws *websocket.Conn) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				fmt.Printf("Failed to send ping: %v\n", err)
				return
			}
		}
	}
}

func (c *PhemexClient) userDataHandlerFn() core.WsUserEventHandler {
	return func(msg []byte) {
		var wsMsg PhemexWSMessage
		if err := json.Unmarshal(msg, &wsMsg); err != nil {
			fmt.Printf("Failed to unmarshal WebSocket message: %v\n", err)
			return
		}
		
		// Handle different message types based on method
		switch wsMsg.Method {
		case "account.update":
			c.handleAccountUpdate(wsMsg.Params)
		case "position.update":
			c.handlePositionUpdate(wsMsg.Params)
		case "order.update":
			c.handleOrderUpdate(wsMsg.Params)
		case "tick.update":
			c.handleTickerUpdate(wsMsg.Params)
		case "symbol.update":
			c.handleSymbolUpdate(wsMsg.Params)
		}
	}
}

func (c *PhemexClient) handleAccountUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var accounts []PhemexAccount
	if err := json.Unmarshal(dataBytes, &accounts); err != nil {
		return
	}
	
	c.balancesMu.Lock()
	defer c.balancesMu.Unlock()
	
	for _, account := range accounts {
		balance := FromEp(account.BalanceEv, 8) // Phemex uses 8 decimal places for most currencies
		usedBalance := FromEp(account.TotalUsedBalanceEv, 8)
		
		c.balances[account.Currency] = &core.Wallet{
			Asset:  account.Currency,
			Free:   balance.Sub(usedBalance),
			Locked: usedBalance,
			Total:  balance,
		}
	}
}

func (c *PhemexClient) handlePositionUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var positions []PhemexPosition
	if err := json.Unmarshal(dataBytes, &positions); err != nil {
		return
	}
	
	c.positionsMu.Lock()
	defer c.positionsMu.Unlock()
	
	for _, position := range positions {
		c.positions[position.Symbol] = &position
	}
}

func (c *PhemexClient) handleOrderUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var orders []PhemexOrder
	if err := json.Unmarshal(dataBytes, &orders); err != nil {
		return
	}
	
	c.ordersMu.Lock()
	defer c.ordersMu.Unlock()
	
	for _, order := range orders {
		status := c.mapOrderStatus(order.OrderStatus)
		
		c.orders[order.OrderID] = &core.OrderResponse{
			OrderID:         order.OrderID,
			Symbol:          order.Symbol,
			Side:            order.Side,
			Status:          status,
			Price:           FromEp(order.PriceEp, 4), // Default price scale
			Quantity:        c.convertQuantity(order.OrderQty, order.Symbol),
			IsQuoteQuantity: false,
			CreateTime:      ToTimeNs(order.ActionTimeNs),
		}
	}
}

func (c *PhemexClient) handleTickerUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var tickers []PhemexTicker
	if err := json.Unmarshal(dataBytes, &tickers); err != nil {
		return
	}
	
	c.quoteChansMu.RLock()
	defer c.quoteChansMu.RUnlock()
	
	for _, ticker := range tickers {
		if quoteChan, exists := c.quoteChans[ticker.Symbol]; exists {
			priceScale := c.getPriceScale(ticker.Symbol)
			
			quote := core.Quote{
				Symbol:   ticker.Symbol,
				BidPrice: FromEp(ticker.BidEp, priceScale),
				BidQty:   c.convertQuantity(ticker.BidSizeEp, ticker.Symbol),
				AskPrice: FromEp(ticker.AskEp, priceScale),
				AskQty:   c.convertQuantity(ticker.AskSizeEp, ticker.Symbol),
				Time:     ToTime(ticker.Timestamp),
			}
			
			select {
			case quoteChan <- quote:
			default:
				// Channel full, skip this update
			}
		}
	}
}

func (c *PhemexClient) handleSymbolUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var symbols []PhemexSymbol
	if err := json.Unmarshal(dataBytes, &symbols); err != nil {
		return
	}
	
	c.symbolsMu.Lock()
	defer c.symbolsMu.Unlock()
	
	for _, symbol := range symbols {
		c.symbols[symbol.Symbol] = &symbol
	}
}

func (c *PhemexClient) mapOrderStatus(status string) core.OrderStatus {
	switch status {
	case "New", "PartiallyFilled":
		return core.OrderStatusOpen
	case "Filled":
		return core.OrderStatusFilled
	case "Canceled", "Rejected":
		return core.OrderStatusCanceled
	default:
		return core.OrderStatusError
	}
}

func (c *PhemexClient) getPriceScale(symbol string) int {
	c.symbolsMu.RLock()
	defer c.symbolsMu.RUnlock()
	
	if symbolInfo, exists := c.symbols[symbol]; exists {
		return symbolInfo.PriceScale
	}
	return 4 // Default scale
}

func (c *PhemexClient) convertQuantity(qty int64, symbol string) decimal.Decimal {
	c.symbolsMu.RLock()
	defer c.symbolsMu.RUnlock()
	
	if symbolInfo, exists := c.symbols[symbol]; exists {
		return FromEp(qty, symbolInfo.RatioScale)
	}
	return FromEp(qty, 8) // Default scale
}

func (c *PhemexClient) afterConnect() core.WsAfterConnectFunc {
	return func(wsClient *core.WsClient) error {
		// Subscribe to account updates
		accountMsg := map[string]interface{}{
			"id":     nextWSID(),
			"method": "account.subscribe",
			"params": []string{},
		}
		
		_, err := wsClient.SendRequest(accountMsg)
		if err != nil {
			return fmt.Errorf("failed to subscribe to account updates: %w", err)
		}
		
		// Subscribe to position updates
		positionMsg := map[string]interface{}{
			"id":     nextWSID(),
			"method": "position.subscribe",
			"params": []string{},
		}
		
		_, err = wsClient.SendRequest(positionMsg)
		if err != nil {
			return fmt.Errorf("failed to subscribe to position updates: %w", err)
		}
		
		// Subscribe to order updates
		orderMsg := map[string]interface{}{
			"id":     nextWSID(),
			"method": "order.subscribe",
			"params": []string{},
		}
		
		_, err = wsClient.SendRequest(orderMsg)
		if err != nil {
			return fmt.Errorf("failed to subscribe to order updates: %w", err)
		}
		
		return nil
	}
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
		if errorRaw, ok := root["error"]; ok {
			var phemexErr PhemexError
			if err := json.Unmarshal(errorRaw, &phemexErr); err == nil {
				return ParsePhemexError(phemexErr.Code, phemexErr.Message)
			}
		}
		return nil
	}
}

// REST API helper methods for hybrid approach
func (c *PhemexClient) makeRestRequest(method, path string, body []byte) (*PhemexRestResponse, error) {
	baseURL := "https://api.phemex.com"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	
	pathOnly := path
	queryString := ""
	if idx := strings.Index(path, "?"); idx != -1 {
		pathOnly = path[:idx]
		queryString = path[idx+1:]
	}
	
	signature := c.generateRestSignature(pathOnly, queryString, body, timestamp)
	
	var req *http.Request
	var err error
	
	if body != nil {
		req, err = http.NewRequest(method, baseURL+path, bytes.NewBuffer(body))
	} else {
		req, err = http.NewRequest(method, baseURL+path, nil)
	}
	
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("x-phemex-access-token", c.apiKey)
	req.Header.Set("x-phemex-request-expiry", timestamp)
	req.Header.Set("x-phemex-request-signature", signature)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var phemexResp PhemexRestResponse
	if err := json.Unmarshal(respBody, &phemexResp); err != nil {
		return nil, err
	}
	
	return &phemexResp, nil
}

func (c *PhemexClient) generateRestSignature(path, queryString string, body []byte, timestamp string) string {
	message := path + queryString + timestamp + string(body)
	h := hmac.New(sha256.New, []byte(c.apiSecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}