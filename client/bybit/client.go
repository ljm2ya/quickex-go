package bybit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

const (
	bybitWsURLPrivate = "wss://stream.bybit.com/v5/trade"
	wsLifetime        = 23*time.Hour + 50*time.Minute
)

type BybitClient struct {
	*core.WsClient
	client    *bybit.Client
	apiKey    string
	apiSecret string

	balances   map[string]*core.Wallet
	orders     map[string]*core.OrderResponse
	balancesMu sync.RWMutex
	ordersMu   sync.RWMutex
}

func NewClient(apiKey, apiSecret string) *BybitClient {
	restCli := bybit.NewClient().WithAuth(apiKey, apiSecret)
	client := &BybitClient{
		client:    restCli,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		balances:  make(map[string]*core.Wallet),
		orders:    make(map[string]*core.OrderResponse),
	}
	client.WsClient = core.NewWsClient(
		bybitWsURLPrivate,
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

// --- 인증 메시지 작성 ---
func (c *BybitClient) authFn() core.WsAuthFunc {
	return func(ws *websocket.Conn) (int64, error) {
		expires := time.Now().UnixMilli() + 1000
		expiresStr := strconv.FormatInt(expires, 10)
		sign := hmacSHA256("GET/realtime"+expiresStr, c.apiSecret)
		authMsg := map[string]interface{}{
			"op":   "auth",
			"args": []interface{}{c.apiKey, expiresStr, sign},
		}
		if err := ws.WriteJSON(authMsg); err != nil {
			return 0, err
		}
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return 0, err
		}
		// parse auth result (ret_code==0)
		var resp struct {
			RetCode int64  `json:retCode`
			RetMsg  string `json:"retMsg"`
			Op      string `json:"op"`
			ConnId  string `json:connId`
		}
		if err := json.Unmarshal(msg, &resp); err != nil {
			return 0, err
		}
		if resp.RetCode != 0 {
			switch resp.RetCode {
			case 20001:
				return 0, nil // repeat auth, just go on
			case 10004:
				return 0, fmt.Errorf("Bybit ws auth invalid sign: %s", resp.RetMsg)
			case 10001:
				return 0, fmt.Errorf("Bybit ws auth param error: %s", resp.RetMsg)
			default:
				return 0, fmt.Errorf("bybit ws auth unknown error: %s", resp.RetMsg)
			}
		}
		return 0, nil
	}
}

// --- 유저 데이터 핸들러 ---
func (c *BybitClient) userDataHandlerFn() core.WsUserEventHandler {
	return func(msg []byte) {
		// 체결, 잔고, 포지션 등 이벤트 핸들링 필요시
		// 예시: 잔고 topic
		var root map[string]json.RawMessage
		if err := json.Unmarshal(msg, &root); err != nil {
			fmt.Printf("Bybit user data unmarshal err: %v\n", err)
			return
		}
		if topicRaw, ok := root["topic"]; ok {
			var topic string
			_ = json.Unmarshal(topicRaw, &topic)
			switch topic {
			case "wallet":
				// TODO: 잔고 핸들링 구현
			case "execution":
				// TODO: 체결 핸들링 구현
			default:
				fmt.Printf("Unhandled ws topic: %s\n", topic)
			}
		}
	}
}

// --- 요청 id 추출 등 공통 util ---
func requestIDFn(nextWSID func() string) core.WsRequestIDFunc {
	return func(req map[string]interface{}) (interface{}, bool) {
		if id, ok := req["reqId"].(string); ok && id != "" {
			return id, true
		}
		id := nextWSID()
		req["reqId"] = id
		return id, true
	}
}

func extractIDFn() core.WsExtractIDFunc {
	return func(root map[string]json.RawMessage) (string, bool) {
		idRaw, ok := root["reqId"]
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

// --- Bybit 에러 메시지 파싱 ---
func extractErrFn() core.WsExtractErrFunc {
	return func(root map[string]json.RawMessage) error {
		if errRaw, ok := root["retCode"]; ok {
			var retCode int
			_ = json.Unmarshal(errRaw, &retCode)
			if retCode != 0 {
				var retMsg string
				if msgRaw, ok := root["retMsg"]; ok {
					_ = json.Unmarshal(msgRaw, &retMsg)
				}
				return fmt.Errorf("bybit ws error %d: %s", retCode, retMsg)
			}
		}
		return nil
	}
}

// --- 연결 직후 잔고 폴링 goroutine (선택적) ---
func (c *BybitClient) afterConnect() core.WsAfterConnectFunc {
	return func(ws *core.WsClient) error {
		resp, err := c.client.V5().Account().GetWalletBalance(bybit.AccountTypeV5UNIFIED, nil)
		if err != nil {
			return fmt.Errorf("Get Wallet Balance: %v", err)
		}
		for _, coin := range resp.Result.List[0].Coin {
			avail := core.ToFloat(coin.WalletBalance)
			total := core.ToFloat(coin.Equity)
			c.balancesMu.Lock()
			c.balances[coin.Coin] = &core.Wallet{
				Asset:  coin.Coin,
				Free:   decimal.NewFromFloat(avail),
				Locked: decimal.NewFromFloat(total - avail),
				Total:  decimal.NewFromFloat(total),
			}
			c.balancesMu.Unlock()
		}
		// Set deposit account to UNIFIED after successful connection
		if err := c.SetDepositAccount("UNIFIED"); err != nil {
			// Log the error but don't fail the connection
			// This is not critical for the client to function
			fmt.Printf("Warning: Failed to set deposit account to UNIFIED: %v\n", err)
		}
		ticker := time.NewTicker(3 * time.Second)
		pingTicker := time.NewTicker(20 * time.Second)
		go func() {
			defer ticker.Stop()
			for {
				select {
				case <-c.Ctx.Done():
					return
				case <-pingTicker.C:
					msg := map[string]interface{}{
						"op": "ping",
					}
					ws.SendRequest(msg)
				case <-ticker.C:
					resp, err := c.client.V5().Account().GetWalletBalance(bybit.AccountTypeV5UNIFIED, nil)
					if err != nil {
						fmt.Printf("REST balance poll error: %v\n", err)
						continue
					}
					for _, coin := range resp.Result.List[0].Coin {
						avail := core.ToFloat(coin.WalletBalance)
						total := core.ToFloat(coin.Equity)
						c.balancesMu.Lock()
						c.balances[coin.Coin] = &core.Wallet{
							Asset:  coin.Coin,
							Free:   decimal.NewFromFloat(avail),
							Locked: decimal.NewFromFloat(total - avail),
							Total:  decimal.NewFromFloat(total),
						}
						c.balancesMu.Unlock()
					}
				}
			}
		}()
		return nil
	}
}

// ToSymbol converts asset and quote to exchange-specific symbol format
// Bybit format: BTCUSDT (no separator)
func (c *BybitClient) ToSymbol(asset, quote string) string {
	return asset + quote
}

// --- HMAC-SHA256 서명 ---
func hmacSHA256(msg, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}
