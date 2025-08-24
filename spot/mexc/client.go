package mexc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

const (
	mexcWsURL       = "wss://wbs.mexc.com/ws"
	mexcSpotBaseURL = "https://api.mexc.com"
	wsLifetime      = 23*time.Hour + 50*time.Minute
)

type MexcClient struct {
	apiKey    string
	apiSecret string
	
	// WebSocket connection for real-time data
	wsConn     *websocket.Conn
	wsMu       sync.Mutex
	connected  bool
	ctx        context.Context
	cancel     context.CancelFunc
	
	// Market data channels
	quoteChans map[string]chan core.Quote
	quoteMu    sync.RWMutex
}

func NewClient(apiKey, apiSecret string) *MexcClient {
	return &MexcClient{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		quoteChans: make(map[string]chan core.Quote),
	}
}

// Connect implements core.PrivateClient interface
func (c *MexcClient) Connect(ctx context.Context) (int64, error) {
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(mexcWsURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to MEXC WebSocket: %w", err)
	}
	
	c.wsMu.Lock()
	c.wsConn = conn
	c.connected = true
	c.wsMu.Unlock()
	
	// Start message handler
	go c.wsMessageHandler()
	
	// No time sync for MEXC WebSocket, return 0
	return 0, nil
}

// Close implements core.PrivateClient interface
func (c *MexcClient) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	
	c.wsMu.Lock()
	defer c.wsMu.Unlock()
	
	if c.wsConn != nil {
		c.wsConn.Close()
		c.wsConn = nil
	}
	c.connected = false
	
	// Close all quote channels
	c.quoteMu.Lock()
	for _, ch := range c.quoteChans {
		close(ch)
	}
	c.quoteChans = make(map[string]chan core.Quote)
	c.quoteMu.Unlock()
	
	return nil
}

// wsMessageHandler handles incoming WebSocket messages
func (c *MexcClient) wsMessageHandler() {
	defer func() {
		c.wsMu.Lock()
		if c.wsConn != nil {
			c.wsConn.Close()
			c.wsConn = nil
		}
		c.connected = false
		c.wsMu.Unlock()
	}()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.wsMu.Lock()
			if c.wsConn == nil {
				c.wsMu.Unlock()
				return
			}
			
			_, message, err := c.wsConn.ReadMessage()
			c.wsMu.Unlock()
			
			if err != nil {
				fmt.Printf("[mexc] WebSocket read error: %v\n", err)
				return
			}
			
			c.handleWebSocketMessage(message)
		}
	}
}

// handleWebSocketMessage processes incoming WebSocket messages
func (c *MexcClient) handleWebSocketMessage(message []byte) {
	var response struct {
		Channel string `json:"channel"`
		Symbol  string `json:"symbol"`
		Data    struct {
			BidPrice    string `json:"bidPrice"`
			BidQuantity string `json:"bidSize"`
			AskPrice    string `json:"askPrice"`
			AskQuantity string `json:"askSize"`
		} `json:"data"`
		Timestamp int64 `json:"ts"`
	}
	
	if err := json.Unmarshal(message, &response); err != nil {
		// Skip non-quote messages
		return
	}
	
	// Process only bookTicker messages
	if response.Channel != "" && response.Symbol != "" {
		bidPrice, err1 := decimal.NewFromString(response.Data.BidPrice)
		bidQty, err2 := decimal.NewFromString(response.Data.BidQuantity)
		askPrice, err3 := decimal.NewFromString(response.Data.AskPrice)
		askQty, err4 := decimal.NewFromString(response.Data.AskQuantity)
		
		if err1 == nil && err2 == nil && err3 == nil && err4 == nil {
			quote := core.Quote{
				Symbol:   response.Symbol,
				BidPrice: bidPrice,
				BidQty:   bidQty,
				AskPrice: askPrice,
				AskQty:   askQty,
				Time:     time.UnixMilli(response.Timestamp),
			}
			
			c.quoteMu.RLock()
			if ch, exists := c.quoteChans[response.Symbol]; exists {
				select {
				case ch <- quote:
				default:
					// Channel full, skip
				}
			}
			c.quoteMu.RUnlock()
		}
	}
}

// generateSignature generates HMAC-SHA256 signature for MEXC API
func (c *MexcClient) generateSignature(params string) string {
	h := hmac.New(sha256.New, []byte(c.apiSecret))
	h.Write([]byte(params))
	return hex.EncodeToString(h.Sum(nil))
}

// buildSignedParams builds signed query parameters for MEXC API
func (c *MexcClient) buildSignedParams(params url.Values) string {
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	queryString := params.Encode()
	signature := c.generateSignature(queryString)
	params.Set("signature", signature)
	return params.Encode()
}
