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
		apiKey:     apiKey,
		privateKey: prvKey,
		baseURL:    "https://testnet.binancefuture.com",
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
				free := core.ParseStringFloat(bal.Free)
				locked := core.ParseStringFloat(bal.Locked)
				b.balancesMu.Lock()
				b.balances[bal.Asset].Free = decimal.NewFromFloat(free)
				b.balances[bal.Asset].Locked = decimal.NewFromFloat(locked)
				b.balances[bal.Asset].Total = decimal.NewFromFloat(free + locked)
				b.balancesMu.Unlock()
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
			resp := toOrderResponse(ord)
			b.ordersMu.Lock()
			b.orders[resp.OrderID] = resp
			b.ordersMu.Unlock()
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
