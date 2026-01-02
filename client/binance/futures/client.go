package binance

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/ed25519"
)

const (
	binanceWsURL        = "wss://ws-fapi.binance.com/ws-fapi/v1"
	binanceTestnetWsURL = "wss://testnet.binancefuture.com/ws-fapi/v1"
	wsLifetime          = 23*time.Hour + 50*time.Minute
	commisionRate       = 0.0005
)

type BinanceClient struct {
	*core.WsClient
	balances   map[string]*core.Wallet
	orders     map[string]*core.OrderResponse // order ID : response
	wsReject   map[string]chan wsListStatus
	balancesMu sync.RWMutex
	ordersMu   sync.RWMutex
	wsRejectMu sync.Mutex
	apiKey     string
	privateKey ed25519.PrivateKey
	baseURL    string
	hedgeMode  bool
	isTestnet  bool

	// User data stream for private events
	userDataStream *BinanceUserDataStream

	// Real-time event subscription channels
	orderEventCh       chan core.OrderEvent
	balanceEventCh     chan core.BalanceEvent
	orderEventSymbols  []string // Filter for order events
	balanceEventAssets []string // Filter for balance events
	subscriptionCtx    context.Context
	subscriptionCancel context.CancelFunc
	subscriptionMu     sync.Mutex
}

func NewClient(apiKey string, prvKey ed25519.PrivateKey) *BinanceClient {
	b := &BinanceClient{
		balances:   make(map[string]*core.Wallet),
		orders:     make(map[string]*core.OrderResponse),
		wsReject:   make(map[string]chan wsListStatus),
		balancesMu: sync.RWMutex{},
		ordersMu:   sync.RWMutex{},
		wsRejectMu: sync.Mutex{},
		apiKey:     apiKey,
		privateKey: prvKey,
		baseURL:    "https://fapi.binance.com",
		isTestnet:  false,
	}
	b.WsClient = core.NewWsClient(
		binanceWsURL,
		wsLifetime,
		authFn(apiKey, prvKey),
		userDataHandlerFn(b),
		requestIDFn(nextWSID),
		extractIDFn(),
		extractErrFn(),
		afterConnect(b),
	)
	// Initialize user data stream
	b.userDataStream = NewBinanceUserDataStream(b, apiKey, prvKey, false)
	return b
}

func NewTestClient(apiKey string, prvKey ed25519.PrivateKey) *BinanceClient {
	b := &BinanceClient{
		balances:   make(map[string]*core.Wallet),
		orders:     make(map[string]*core.OrderResponse),
		wsReject:   make(map[string]chan wsListStatus),
		balancesMu: sync.RWMutex{},
		ordersMu:   sync.RWMutex{},
		wsRejectMu: sync.Mutex{},
		apiKey:     apiKey,
		privateKey: prvKey,
		baseURL:    "https://testnet.binancefuture.com",
		isTestnet:  true,
	}
	b.WsClient = core.NewWsClient(
		binanceTestnetWsURL,
		wsLifetime,
		authFn(apiKey, prvKey),
		userDataHandlerFn(b),
		requestIDFn(nextWSID),
		extractIDFn(),
		extractErrFn(),
		afterConnect(b),
	)
	// Initialize user data stream for testnet
	b.userDataStream = NewBinanceUserDataStream(b, apiKey, prvKey, true)
	return b
}

func (b *BinanceClient) handleUserDataEvent(msg []byte) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(msg, &root); err != nil {
		fmt.Printf("Failed to unmarshal user data event: %v\n", err)
		return
	}

	var eventRaw map[string]json.RawMessage
	if err := json.Unmarshal(root["event"], &eventRaw); err != nil {
		fmt.Printf("Failed to unmarshal user data event: %v\n", err)
		return
	}
	switch core.StringFromRawMap(eventRaw, "e") {
	case "outboundAccountPosition":
		var acct wsAccountUpdate
		if err := acct.UnmarshalJSON(root["event"]); err == nil {
			for _, bal := range acct.Balances {
				free := core.ParseStringFloat(bal.Free)
				locked := core.ParseStringFloat(bal.Locked)
				total := free + locked

				b.balancesMu.Lock()
				if b.balances[bal.Asset] == nil {
					b.balances[bal.Asset] = &core.Wallet{Asset: bal.Asset}
				}
				b.balances[bal.Asset].Free = decimal.NewFromFloat(free)
				b.balances[bal.Asset].Locked = decimal.NewFromFloat(locked)
				b.balances[bal.Asset].Total = decimal.NewFromFloat(total)
				b.balancesMu.Unlock()

				// Balance events are now handled by the separate user data stream
			}
		} else {
			fmt.Printf("json unmarshal error on handling user event: %v\n", err)
			panic(err)
		}
	case "balanceUpdate":
		/*
			b.Logger.Infof("bal")
			var bal wsBalanceUpdate
			if err := json.Unmarshal(event, &bal); err == nil {
				delta, _ := strconv.ParseFloat(bal.BalanceDelta, 64)
				b.balancesMu.Lock()
				b.balances[bal.Asset].Free += delta
				b.balances[bal.Asset].Total += delta
				b.balancesMu.Unlock()
			} else {
				fmt.Printf("json unmarshal error on handling user event: %v\n", err)
			panic(err)
			}*/
	case "executionReport":
		var ord wsOrderTradeUpdate
		if err := ord.UnmarshalJSON(root["event"]); err == nil {
			// Update internal order tracking
			resp := toOrderResponse(ord)
			b.ordersMu.Lock()
			b.orders[resp.OrderID] = resp
			b.ordersMu.Unlock()

			// Order events are now handled by the separate user data stream
		} else {
			fmt.Printf("json unmarshal error on handling user event: %v\n", err)
			panic(err)
		}
	case "listStatusOrder":
		var lso wsListStatus
		if err := lso.UnmarshalJSON(root["event"]); err == nil {
			sCh := make(chan wsListStatus, 1)
			b.wsRejectMu.Lock()
			b.wsReject[lso.Symbol] = sCh
			sCh <- lso
			b.wsRejectMu.Unlock()
		} else {
			fmt.Printf("json unmarshal error on handling user event: %v\n", err)
			panic(err)
		}
	case "eventStreamTerminated":
		fmt.Println("[binance] Event stream terminated, sleeping before re-auth...")
		time.Sleep(5 * time.Second)
		b.Reconnect()
	case "externalLockUpdate":
		/*
			var lock wsExternalLockUpdate
			if err := json.Unmarshal(event, &lock); err == nil {
				locked, _ := strconv.ParseFloat(lock.BalanceDelta, 64)
				b.balancesMu.Lock()
				b.balances[lock.Asset].Free -= locked
				b.balances[lock.Asset].Locked += locked
				b.balancesMu.Unlock()
			}*/
	}
}

func nextWSID() string {
	return time.Now().Format("20060102150405") + "-" + fmt.Sprint(rand.Int63())
}

func authFn(apiKey string, privKey ed25519.PrivateKey) core.WsAuthFunc {
	return func(ws *websocket.Conn) (int64, error) {
		ts := time.Now().UnixMilli()
		payload := fmt.Sprintf("apiKey=%s&timestamp=%d", apiKey, ts)
		sig := ed25519.Sign(privKey, []byte(payload))
		signature := base64.StdEncoding.EncodeToString(sig)
		authMsg := map[string]interface{}{
			"id":     nextWSID(),
			"method": "session.logon",
			"params": map[string]interface{}{
				"apiKey":    apiKey,
				"signature": signature,
				"timestamp": ts,
			},
		}
		if err := ws.WriteJSON(authMsg); err != nil {
			return 0, err
		}

		_, msg, err := ws.ReadMessage()
		if err != nil {
			return 0, err
		}
		var resp struct {
			Result struct {
				ServerTime int64 `json:"serverTime"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return 0, err
		}
		return ts - resp.Result.ServerTime, nil
	}
}

func userDataHandlerFn(b *BinanceClient) core.WsUserEventHandler {
	return func(msg []byte) {
		b.handleUserDataEvent(msg)
	}
}

func requestIDFn(nextWSID func() string) core.WsRequestIDFunc {
	return func(req map[string]interface{}) (interface{}, bool) {
		// Check if already present
		if id, ok := req["id"].(string); ok && id != "" {
			return id, true
		}
		// Generate new
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
		// Try float (some APIs send numeric id)
		var idNum float64
		if err := json.Unmarshal(idRaw, &idNum); err == nil {
			return strconv.FormatInt(int64(idNum), 10), true
		}
		return "", false
	}
}

func wrapStatusCode(status int64, msg string) error {
	switch status {
	case 200:
		return nil
	case 429, 418:
		return fmt.Errorf("rate limited or banned (status %d): %s", status, msg)
	case 500, 502, 503, 504:
		return fmt.Errorf("binance server error (status %d): %s", status, msg)
	default:
		if status >= 400 && status < 500 {
			return fmt.Errorf("binance client error (status %d): %s", status, msg)
		}
		if status >= 500 {
			return fmt.Errorf("binance server error (status %d): %s", status, msg)
		}
	}
	return nil
}

func extractErrFn() core.WsExtractErrFunc {
	return func(root map[string]json.RawMessage) error {
		if errRaw, ok := root["error"]; ok {
			var errMsgObj map[string]interface{}
			if err := json.Unmarshal(errRaw, &errMsgObj); err == nil {
				// Optionally inject status handler/callback here too
				status := core.IntFromRawMap(root, "status")
				errMsg := core.StringFromMap(errMsgObj, "msg")
				return wrapStatusCode(status, errMsg)
			}
		}
		return nil
	}
}

func afterConnect(b *BinanceClient) core.WsAfterConnectFunc {
	return func(c *core.WsClient) error {
		err := b.userDataStream.Connect(b.Ctx)
		if err != nil {
			return fmt.Errorf("User data stream connect: %v", err)
		}
		acct, err := b.GetAccount()
		if err != nil {
			return fmt.Errorf("Get Account: %v", err)
		}
		b.balancesMu.Lock()
		for symbol, wal := range acct.Assets {
			b.balances[symbol] = &core.Wallet{
				Asset:  symbol,
				Free:   wal.Free,
				Locked: wal.Locked,
				Total:  wal.Total,
			}
		}
		b.balancesMu.Unlock()
		go func() {
			for {
				select {
				case <-c.Ctx.Done():
					return
				default:
				}
				acct, err := b.GetAccount()
				if err != nil {
					time.Sleep(time.Second * 5)
					continue
				}
				b.balancesMu.Lock()
				for symbol, wal := range acct.Assets {
					b.balances[symbol] = &core.Wallet{
						Asset:  symbol,
						Free:   wal.Free,
						Locked: wal.Locked,
						Total:  wal.Total,
					}
				}
				b.balancesMu.Unlock()
				time.Sleep(time.Second * 5)
			}
		}()
		return nil
	}
}

// ToSymbol converts asset and quote to exchange-specific symbol format
// Binance Futures format: BTCUSDT (no separator)
func (b *BinanceClient) ToSymbol(asset, quote string) string {
	return asset + quote
}

// ToAsset extracts the asset from a symbol (reverse of ToSymbol)
func (b *BinanceClient) ToAsset(symbol string) string {
	// Binance Futures uses simple concatenation: BTCUSDT
	// Common quote currencies to check (ordered by likelihood)
	quotes := []string{"USDT", "BUSD", "USDC", "BTC", "ETH", "BNB"}

	for _, quote := range quotes {
		if len(symbol) > len(quote) && symbol[len(symbol)-len(quote):] == quote {
			return symbol[:len(symbol)-len(quote)]
		}
	}

	// If no match found, return the symbol as-is (shouldn't happen with valid symbols)
	return symbol
}

// SubscribeOrderEvents implements core.PrivateClient interface
func (b *BinanceClient) SubscribeOrderEvents(ctx context.Context, symbols []string, errHandler func(err error)) (<-chan core.OrderEvent, error) {
	b.subscriptionMu.Lock()
	defer b.subscriptionMu.Unlock()

	// Ensure user data stream is already connected (should be connected at program init)
	if !b.userDataStream.IsConnected() {
		return nil, fmt.Errorf("user data stream not connected")
	}

	if b.orderEventCh == nil {
		b.orderEventCh = make(chan core.OrderEvent, 100)
		b.orderEventSymbols = symbols // Store filter symbols
		b.subscriptionCtx, b.subscriptionCancel = context.WithCancel(ctx)

		// Start forwarding events from user data stream
		go b.forwardOrderEvents(errHandler)
	}

	return b.orderEventCh, nil
}

// SubscribeBalanceEvents implements core.PrivateClient interface
func (b *BinanceClient) SubscribeBalanceEvents(ctx context.Context, assets []string, errHandler func(err error)) (<-chan core.BalanceEvent, error) {
	b.subscriptionMu.Lock()
	defer b.subscriptionMu.Unlock()

	// Ensure user data stream is already connected (should be connected at program init)
	if !b.userDataStream.IsConnected() {
		return nil, fmt.Errorf("user data stream not connected")
	}

	if b.balanceEventCh == nil {
		b.balanceEventCh = make(chan core.BalanceEvent, 100)
		b.balanceEventAssets = assets // Store filter assets
		b.subscriptionCtx, b.subscriptionCancel = context.WithCancel(ctx)

		// Start forwarding events from user data stream
		go b.forwardBalanceEvents(errHandler)
	}

	return b.balanceEventCh, nil
}

// UnsubscribeOrderEvents implements core.PrivateClient interface
func (b *BinanceClient) UnsubscribeOrderEvents() error {
	b.subscriptionMu.Lock()
	defer b.subscriptionMu.Unlock()

	if b.subscriptionCancel != nil {
		b.subscriptionCancel()
	}

	if b.orderEventCh != nil {
		close(b.orderEventCh)
		b.orderEventCh = nil
	}

	// Close user data stream if no more subscriptions
	if b.balanceEventCh == nil {
		b.userDataStream.Close()
	}

	return nil
}

// UnsubscribeBalanceEvents implements core.PrivateClient interface
func (b *BinanceClient) UnsubscribeBalanceEvents() error {
	b.subscriptionMu.Lock()
	defer b.subscriptionMu.Unlock()

	if b.subscriptionCancel != nil {
		b.subscriptionCancel()
	}

	if b.balanceEventCh != nil {
		close(b.balanceEventCh)
		b.balanceEventCh = nil
	}

	// Close user data stream if no more subscriptions
	if b.orderEventCh == nil {
		b.userDataStream.Close()
	}

	return nil
}

// forwardOrderEvents forwards order events from user data stream to user channel
func (b *BinanceClient) forwardOrderEvents(errHandler func(err error)) {
	wsOrderCh := b.userDataStream.GetOrderEventChannel()

	for {
		select {
		case orderEvent, ok := <-wsOrderCh:
			if !ok {
				return
			}

			// Filter by symbols if specified
			if len(b.orderEventSymbols) > 0 {
				if !contains(b.orderEventSymbols, orderEvent.Symbol) {
					continue // Skip events not matching filter
				}
			}

			// Forward to user channel
			select {
			case b.orderEventCh <- orderEvent:
			case <-b.subscriptionCtx.Done():
				return
			default:
				// User channel full, drop event
				if errHandler != nil {
					errHandler(fmt.Errorf("order event channel full, dropping event for order %s", orderEvent.OrderID))
				}
			}

		case <-b.subscriptionCtx.Done():
			return
		}
	}
}

// forwardBalanceEvents forwards balance events from user data stream to user channel
func (b *BinanceClient) forwardBalanceEvents(errHandler func(err error)) {
	wsBalanceCh := b.userDataStream.GetBalanceEventChannel()

	for {
		select {
		case balanceEvent, ok := <-wsBalanceCh:
			if !ok {
				return
			}

			// Filter by assets if specified
			if len(b.balanceEventAssets) > 0 {
				if !contains(b.balanceEventAssets, balanceEvent.Asset) {
					continue // Skip events not matching filter
				}
			}

			// Forward to user channel
			select {
			case b.balanceEventCh <- balanceEvent:
			case <-b.subscriptionCtx.Done():
				return
			default:
				// User channel full, drop event
				if errHandler != nil {
					errHandler(fmt.Errorf("balance event channel full, dropping event for asset %s", balanceEvent.Asset))
				}
			}

		case <-b.subscriptionCtx.Done():
			return
		}
	}
}

// convertToOrderEvent converts wsOrderTradeUpdate to core.OrderEvent
func (b *BinanceClient) convertToOrderEvent(ord wsOrderTradeUpdate) core.OrderEvent {
	var status core.OrderStatus
	switch ord.Status {
	case "NEW":
		status = core.OrderStatusOpen
	case "FILLED":
		status = core.OrderStatusFilled
	case "CANCELED":
		status = core.OrderStatusCanceled
	case "REJECTED", "EXPIRED":
		status = core.OrderStatusError
	case "PARTIALLY_FILLED":
		status = core.OrderStatusOpen // Treat as open since it's still active
	default:
		status = core.OrderStatusOpen
	}

	price, _ := decimal.NewFromString(ord.Price)
	quantity, _ := decimal.NewFromString(ord.OrigQty)
	executedQty, _ := decimal.NewFromString(ord.ExecutedQty)
	lastFilledPrice, _ := decimal.NewFromString(ord.LastFilledPrice)

	// Convert string side to core.OrderSide
	var side core.OrderSide
	if ord.Side == "BUY" {
		side = core.OrderSideBuy
	} else if ord.Side == "SELL" {
		side = core.OrderSideSell
	}

	return core.OrderEvent{
		OrderID:     strconv.FormatInt(ord.OrderID, 10),
		Symbol:      ord.Symbol,
		Side:        side,
		OrderType:   ord.OrderType,
		Status:      status,
		Price:       price,
		Quantity:    quantity,
		ExecutedQty: executedQty,
		AvgPrice:    lastFilledPrice, // Approximate - Binance doesn't provide true average
		UpdateTime:  time.Unix(0, ord.LastFilledTime*int64(time.Millisecond)),
		TradeID:     strconv.FormatInt(ord.TradeID, 10),
	}
}

// contains checks if a string slice contains a specific value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
