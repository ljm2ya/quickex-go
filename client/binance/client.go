package binance

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/ed25519"
	"github.com/ljm2ya/quickex-go/core"
)

const (
	binanceWsURL        = "wss://ws-api.binance.com:443/ws-api/v3"
	binanceTestnetWsURL = "wss://ws-api.testnet.binance.vision/ws-api/v3"
	wsLifetime          = 23*time.Hour + 50*time.Minute
)

type BinanceClient struct {
	*core.WsClient
	balances   map[string]*core.Wallet
	orders     map[string]*core.OrderResponse // order ID : response
	wsReject   map[string]chan wsListStatus
	balancesMu sync.RWMutex
	ordersMu   sync.RWMutex
	wsRejectMu sync.Mutex
	// ... other Binance-specific fields ...
}

func NewClient(apiKey string, prvKey ed25519.PrivateKey) *BinanceClient {
	b := &BinanceClient{
		balances:   make(map[string]*core.Wallet),
		orders:     make(map[string]*core.OrderResponse),
		wsReject:   make(map[string]chan wsListStatus),
		balancesMu: sync.RWMutex{},
		ordersMu:   sync.RWMutex{},
		wsRejectMu: sync.Mutex{},
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
				free := decimal.RequireFromString(bal.Free)
				locked := decimal.RequireFromString(bal.Locked)
				b.balancesMu.Lock()
				b.balances[bal.Asset].Free = free
				b.balances[bal.Asset].Locked = locked
				b.balances[bal.Asset].Total = free.Add(locked)
				b.balancesMu.Unlock()
			}
		} else {
			fmt.Printf("json unmarshal error on handling user event: %v\n", err)
			panic(err)
		}
	case "balanceUpdate":
		/*
			b.logger.Infof("bal")
			var bal wsBalanceUpdate
			if err := json.Unmarshal(event, &bal); err == nil {
				delta, _ := strconv.ParseFloat(bal.BalanceDelta, 64)
				b.balancesMu.Lock()
				b.balances[bal.Asset].Free += delta
				b.balances[bal.Asset].Total += delta
				b.balancesMu.Unlock()
			} else {
				b.logger.Panicf("json unmarshal error on handling user event: %v", err)
			}*/
	case "executionReport":
		/*
			var ord wsOrderTradeUpdate
			if err := ord.UnmarshalJSON(root["event"]); err == nil {
				resp := toOrderResponse(ord)
				b.ordersMu.Lock()
				b.orders[resp.OrderID] = resp
				b.ordersMu.Unlock()
			} else {
				fmt.Printf("json unmarshal error on handling user event: %v\n", err)
				panic(err)
			}*/
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
		// 1. Subscribe user data stream
		reqSub := map[string]interface{}{
			"method": "userDataStream.subscribe",
		}
		_, err := c.SendRequest(reqSub)
		if err != nil {
			return fmt.Errorf("failed userDataStream.subscribe: %w", err)
		}

		// 2. Load initial balances
		ts := time.Now().UnixMilli()
		req := map[string]interface{}{
			"id":     nextWSID(),
			"method": "account.status",
			"params": map[string]interface{}{
				"timestamp": ts,
			},
		}
		root, err := c.SendRequest(req)
		if err != nil {
			return fmt.Errorf("failed account.status: %w", err)
		}
		var resultMap map[string]interface{}
		if err := json.Unmarshal(root["result"], &resultMap); err != nil {
			return err
		}
		bals, _ := resultMap["balances"].([]interface{})
		b.balancesMu.Lock()
		for _, bal := range bals {
			balMap, _ := bal.(map[string]interface{})
			asset := core.StringFromMap(balMap, "asset")
			free := decimal.RequireFromString(core.StringFromMap(balMap, "free"))
			locked := decimal.RequireFromString(core.StringFromMap(balMap, "locked"))
			var wal = &core.Wallet{
				Asset:  asset,
				Free:   free,
				Locked: locked,
				Total:  locked.Add(free),
			}
			b.balances[asset] = wal
		}
		b.balancesMu.Unlock()
		return nil
	}
}

// ToSymbol converts asset and quote to exchange-specific symbol format
// Binance format: BTCUSDT (no separator)
func (b *BinanceClient) ToSymbol(asset, quote string) string {
	return asset + quote
}

// ToAsset extracts the asset from a symbol (reverse of ToSymbol)
func (b *BinanceClient) ToAsset(symbol string) string {
	// Binance uses simple concatenation: BTCUSDT
	// Common quote currencies to check (ordered by likelihood)
	quotes := []string{"USDT", "BUSD", "USDC", "BTC", "ETH", "BNB", "EUR", "GBP", "AUD", "TRY"}
	
	for _, quote := range quotes {
		if len(symbol) > len(quote) && symbol[len(symbol)-len(quote):] == quote {
			return symbol[:len(symbol)-len(quote)]
		}
	}
	
	// If no match found, return the symbol as-is (shouldn't happen with valid symbols)
	return symbol
}
