package futures

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/client/phemex"
	"github.com/ljm2ya/quickex-go/core"
)

const (
	phemexFuturesWSURL = "wss://ws.phemex.com"
	wsLifetime         = 23*time.Hour + 50*time.Minute
	pingInterval       = 5 * time.Second
)

type PhemexFuturesClient struct {
	*core.WsClient
	apiKey    string
	apiSecret string
	
	// Data storage
	balances     map[string]*core.Wallet
	positions    map[string]*phemex.PhemexPosition
	orders       map[string]*core.OrderResponse
	quoteChans   map[string]chan core.Quote
	symbols      map[string]*phemex.PhemexSymbol
	
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

func NewClient(apiKey, apiSecret string) *PhemexFuturesClient {
	client := &PhemexFuturesClient{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		balances:  make(map[string]*core.Wallet),
		positions: make(map[string]*phemex.PhemexPosition),
		orders:    make(map[string]*core.OrderResponse),
		quoteChans: make(map[string]chan core.Quote),
		symbols:   make(map[string]*phemex.PhemexSymbol),
	}
	
	client.WsClient = core.NewWsClient(
		phemexFuturesWSURL,
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

func (c *PhemexFuturesClient) authFn() core.WsAuthFunc {
	return func(ws *websocket.Conn) (int64, error) {
		if err := phemex.ValidateCredentials(c.apiKey, c.apiSecret); err != nil {
			return 0, err
		}
		
		expiry := phemex.GetExpiryTime()
		signature := phemex.GenerateSignature(c.apiSecret, c.apiKey, expiry)
		
		authMsg := phemex.PhemexWSMessage{
			ID:     nextWSID(),
			Method: "user.auth",
			Params: phemex.PhemexAuthParams{
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
		
		var resp phemex.PhemexWSMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			return 0, fmt.Errorf("failed to unmarshal auth response: %w", err)
		}
		
		if resp.Error != nil {
			return 0, phemex.ParsePhemexError(resp.Error.Code, resp.Error.Message)
		}
		
		c.authMu.Lock()
		c.authenticated = true
		c.authMu.Unlock()
		
		// Start ping routine for heartbeat
		go c.startPingRoutine(ws)
		
		return 0, nil
	}
}

func (c *PhemexFuturesClient) startPingRoutine(ws *websocket.Conn) {
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

func (c *PhemexFuturesClient) userDataHandlerFn() core.WsUserEventHandler {
	return func(msg []byte) {
		var wsMsg phemex.PhemexWSMessage
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

func (c *PhemexFuturesClient) handleAccountUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var accounts []phemex.PhemexAccount
	if err := json.Unmarshal(dataBytes, &accounts); err != nil {
		return
	}
	
	c.balancesMu.Lock()
	defer c.balancesMu.Unlock()
	
	for _, account := range accounts {
		balance := phemex.FromEp(account.BalanceEv, 8)
		usedBalance := phemex.FromEp(account.TotalUsedBalanceEv, 8)
		
		c.balances[account.Currency] = &core.Wallet{
			Asset:  account.Currency,
			Free:   balance.Sub(usedBalance),
			Locked: usedBalance,
			Total:  balance,
		}
	}
}

func (c *PhemexFuturesClient) handlePositionUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var positions []phemex.PhemexPosition
	if err := json.Unmarshal(dataBytes, &positions); err != nil {
		return
	}
	
	c.positionsMu.Lock()
	defer c.positionsMu.Unlock()
	
	for _, position := range positions {
		c.positions[position.Symbol] = &position
	}
}

func (c *PhemexFuturesClient) handleOrderUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var orders []phemex.PhemexOrder
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
			Price:           phemex.FromEp(order.PriceEp, c.getPriceScale(order.Symbol)),
			Quantity:        c.convertQuantity(order.OrderQty, order.Symbol),
			IsQuoteQuantity: false,
			CreateTime:      phemex.ToTimeNs(order.ActionTimeNs),
		}
	}
}

func (c *PhemexFuturesClient) handleTickerUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var tickers []phemex.PhemexTicker
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
				BidPrice: phemex.FromEp(ticker.BidEp, priceScale),
				BidQty:   c.convertQuantity(ticker.BidSizeEp, ticker.Symbol),
				AskPrice: phemex.FromEp(ticker.AskEp, priceScale),
				AskQty:   c.convertQuantity(ticker.AskSizeEp, ticker.Symbol),
				Time:     phemex.ToTime(ticker.Timestamp),
			}
			
			select {
			case quoteChan <- quote:
			default:
				// Channel full, skip this update
			}
		}
	}
}

func (c *PhemexFuturesClient) handleSymbolUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var symbols []phemex.PhemexSymbol
	if err := json.Unmarshal(dataBytes, &symbols); err != nil {
		return
	}
	
	c.symbolsMu.Lock()
	defer c.symbolsMu.Unlock()
	
	for _, symbol := range symbols {
		c.symbols[symbol.Symbol] = &symbol
	}
}

func (c *PhemexFuturesClient) mapOrderStatus(status string) core.OrderStatus {
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

func (c *PhemexFuturesClient) getPriceScale(symbol string) int {
	c.symbolsMu.RLock()
	defer c.symbolsMu.RUnlock()
	
	if symbolInfo, exists := c.symbols[symbol]; exists {
		return symbolInfo.PriceScale
	}
	return 4 // Default scale
}

func (c *PhemexFuturesClient) getRatioScale(symbol string) int {
	c.symbolsMu.RLock()
	defer c.symbolsMu.RUnlock()
	
	if symbolInfo, exists := c.symbols[symbol]; exists {
		return symbolInfo.RatioScale
	}
	return 8 // Default scale
}

func (c *PhemexFuturesClient) convertQuantity(qty int64, symbol string) decimal.Decimal {
	return phemex.FromEp(qty, c.getRatioScale(symbol))
}

func (c *PhemexFuturesClient) getBaseCurrency(symbol string) string {
	c.symbolsMu.RLock()
	defer c.symbolsMu.RUnlock()
	
	if symbolInfo, exists := c.symbols[symbol]; exists {
		return symbolInfo.BaseCurrency
	}
	return "USDT" // Default fallback
}

func (c *PhemexFuturesClient) afterConnect() core.WsAfterConnectFunc {
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
			var phemexErr phemex.PhemexError
			if err := json.Unmarshal(errorRaw, &phemexErr); err == nil {
				return phemex.ParsePhemexError(phemexErr.Code, phemexErr.Message)
			}
		}
		return nil
	}
}