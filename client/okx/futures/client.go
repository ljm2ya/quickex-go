package futures

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/client/okx"
	"github.com/ljm2ya/quickex-go/core"
)

const (
	okxWSURLPublic  = "wss://ws.okx.com:8443/ws/v5/public"
	okxWSURLPrivate = "wss://ws.okx.com:8443/ws/v5/private"
	wsLifetime      = 23*time.Hour + 50*time.Minute
)

type OKXFuturesClient struct {
	*core.WsClient
	apiKey     string
	secretKey  string
	passphrase string
	
	balances     map[string]*core.Wallet
	positions    map[string]*core.Position
	orders       map[string]*core.OrderResponse
	quoteChans   map[string]chan core.Quote
	balancesMu   sync.RWMutex
	positionsMu  sync.RWMutex
	ordersMu     sync.RWMutex
	quoteChansMu sync.RWMutex
}

func NewClient(apiKey, secretKey, passphrase string) *OKXFuturesClient {
	client := &OKXFuturesClient{
		apiKey:     apiKey,
		secretKey:  secretKey,
		passphrase: passphrase,
		balances:   make(map[string]*core.Wallet),
		positions:  make(map[string]*core.Position),
		orders:     make(map[string]*core.OrderResponse),
		quoteChans: make(map[string]chan core.Quote),
	}
	
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

func (c *OKXFuturesClient) authFn() core.WsAuthFunc {
	return func(ws *websocket.Conn) (int64, error) {
		if err := okx.ValidateCredentials(c.apiKey, c.secretKey, c.passphrase); err != nil {
			return 0, err
		}
		
		timestamp := okx.GetTimestampSeconds()
		signature := okx.GenerateWSSignature(timestamp, c.secretKey)
		
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
		
		var resp okx.OKXWSMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			return 0, fmt.Errorf("failed to unmarshal login response: %w", err)
		}
		
		if resp.Code != "0" {
			return 0, fmt.Errorf("login failed: %s - %s", resp.Code, resp.Msg)
		}
		
		return 0, nil
	}
}

func (c *OKXFuturesClient) userDataHandlerFn() core.WsUserEventHandler {
	return func(msg []byte) {
		var wsMsg okx.OKXWSMessage
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
		case "positions":
			c.handlePositionUpdate(wsMsg.Data)
		case "orders":
			c.handleOrderUpdate(wsMsg.Data)
		case "tickers":
			c.handleTickerUpdate(wsMsg.Data)
		default:
			// Handle other channels if needed
		}
	}
}

func (c *OKXFuturesClient) handleAccountUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var accounts []okx.OKXAccount
	if err := json.Unmarshal(dataBytes, &accounts); err != nil {
		return
	}
	
	c.balancesMu.Lock()
	defer c.balancesMu.Unlock()
	
	for _, account := range accounts {
		for _, detail := range account.Details {
			bal := okx.ToDecimal(detail.Bal)
			availBal := okx.ToDecimal(detail.AvailBal)
			frozenBal := okx.ToDecimal(detail.FrozenBal)
			
			c.balances[detail.Ccy] = &core.Wallet{
				Asset:  detail.Ccy,
				Free:   availBal,
				Locked: frozenBal,
				Total:  bal,
			}
		}
	}
}

func (c *OKXFuturesClient) handlePositionUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var positions []okx.OKXPosition
	if err := json.Unmarshal(dataBytes, &positions); err != nil {
		return
	}
	
	c.positionsMu.Lock()
	defer c.positionsMu.Unlock()
	
	for _, pos := range positions {
		positionSide := c.mapPositionSide(pos.PosSide)
		
		c.positions[pos.InstID] = &core.Position{
			Symbol:         pos.InstID,
			Side:           positionSide,
			Amount:         okx.ToFloat64(pos.Pos),
			UrlProfit:      okx.ToFloat64(pos.UPL),
			IsolatedMargin: 0, // OKX doesn't directly provide this
			Notional:       okx.ToFloat64(pos.NotionalUsd),
			IsolatedWallet: 0, // OKX doesn't directly provide this
			InitialMargin:  0, // Would need to calculate based on leverage
			MaintMargin:    0, // Would need to calculate
			UpdatedTime:    okx.ToTime(pos.UTime),
		}
	}
}

func (c *OKXFuturesClient) handleOrderUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var orders []okx.OKXOrder
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
			Price:           okx.ToDecimal(order.Px),
			Quantity:        okx.ToDecimal(order.Sz),
			IsQuoteQuantity: false,
			CreateTime:      okx.ToTime(order.CTime),
		}
	}
}

func (c *OKXFuturesClient) handleTickerUpdate(data interface{}) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	
	var tickers []okx.OKXTicker
	if err := json.Unmarshal(dataBytes, &tickers); err != nil {
		return
	}
	
	c.quoteChansMu.RLock()
	defer c.quoteChansMu.RUnlock()
	
	for _, ticker := range tickers {
		if quoteChan, exists := c.quoteChans[ticker.InstID]; exists {
			quote := core.Quote{
				Symbol:   ticker.InstID,
				BidPrice: okx.ToDecimal(ticker.BidPx),
				BidQty:   okx.ToDecimal(ticker.BidSz),
				AskPrice: okx.ToDecimal(ticker.AskPx),
				AskQty:   okx.ToDecimal(ticker.AskSz),
				Time:     okx.ToTime(ticker.Ts),
			}
			
			select {
			case quoteChan <- quote:
			default:
				// Channel full, skip this update
			}
		}
	}
}

func (c *OKXFuturesClient) mapPositionSide(posSide string) core.PositionSide {
	switch posSide {
	case "long":
		return core.LONG
	case "short":
		return core.SHORT
	case "net":
		return core.BOTH
	default:
		return core.BOTH
	}
}

func (c *OKXFuturesClient) mapOrderStatus(state string) core.OrderStatus {
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

func (c *OKXFuturesClient) afterConnect() core.WsAfterConnectFunc {
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
		
		// Subscribe to positions updates
		positionsMsg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]interface{}{
				{
					"channel":  "positions",
					"instType": "SWAP", // Perpetual futures
				},
			},
		}
		
		_, err = wsClient.SendRequest(positionsMsg)
		if err != nil {
			return fmt.Errorf("failed to subscribe to positions channel: %w", err)
		}
		
		// Subscribe to orders updates for futures
		ordersMsg := map[string]interface{}{
			"op": "subscribe",
			"args": []map[string]interface{}{
				{
					"channel":  "orders",
					"instType": "SWAP",
				},
			},
		}
		
		_, err = wsClient.SendRequest(ordersMsg)
		if err != nil {
			return fmt.Errorf("failed to subscribe to orders channel: %w", err)
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
		// Check for 'code' field
		if codeRaw, ok := root["code"]; ok {
			var code string
			if err := json.Unmarshal(codeRaw, &code); err == nil && code != "0" {
				var msg string
				if msgRaw, ok := root["msg"]; ok {
					_ = json.Unmarshal(msgRaw, &msg)
				}
				return okx.ParseOKXError(code, msg)
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
				return okx.ParseOKXError(code, msg)
			}
		}
		
		return nil
	}
}