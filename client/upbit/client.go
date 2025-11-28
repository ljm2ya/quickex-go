package upbit

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/ljm2ya/quickex-go/core"
)

const (
	baseURL = "https://api.upbit.com"
)

type UpbitClient struct {
	apiKey     string
	secretKey  string
	client     *http.Client
	privateWS  *UpbitPrivateWS
	wsConnected bool
	wsMu       sync.Mutex

	// Subscription state
	orderEventCh    chan core.OrderEvent
	balanceEventCh  chan core.BalanceEvent
	subscriptionCtx context.Context
	subscriptionCancel context.CancelFunc
}

func NewUpbitClient(accessKey, secretKey string) *UpbitClient {
	client := &UpbitClient{
		apiKey:      accessKey,
		secretKey:   secretKey,
		client:      &http.Client{Timeout: 30 * time.Second},
		wsConnected: false,
		wsMu:        sync.Mutex{},
	}
	client.privateWS = NewUpbitPrivateWS(client)
	return client
}

// Connect implements core.PrivateClient interface
func (u *UpbitClient) Connect(ctx context.Context) (int64, error) {
	u.wsMu.Lock()
	defer u.wsMu.Unlock()

	if u.wsConnected {
		return 0, nil
	}

	// Establish private websocket connection for real-time order and balance updates
	if err := u.privateWS.Connect(ctx); err != nil {
		// If websocket connection fails, log the error but don't fail the entire connection
		// since REST API can be used as backup
		fmt.Printf("Warning: Failed to connect to private websocket: %v\n", err)
	} else {
		u.wsConnected = true
	}

	// Return 0 delta timestamp since no time sync is needed for REST API
	return 0, nil
}

// Close implements core.PrivateClient interface
func (u *UpbitClient) Close() error {
	u.wsMu.Lock()
	defer u.wsMu.Unlock()

	if u.wsConnected && u.privateWS != nil {
		err := u.privateWS.Close()
		if err != nil {
			fmt.Printf("Warning: Failed to close private websocket: %v\n", err)
		}
		u.wsConnected = false
	}

	return nil
}

// Token generates JWT token for authenticated requests
func (u *UpbitClient) Token(query map[string]string) (string, error) {
	claim := jwt.MapClaims{
		"access_key": u.apiKey,
		"nonce":      uuid.New().String(),
	}

	if query != nil && len(query) > 0 {
		urlValues := url.Values{}
		for key, value := range query {
			urlValues.Add(key, value)
		}
		rawQuery := urlValues.Encode()

		// Create SHA512 hash of query string
		hash := sha512.Sum512([]byte(rawQuery))
		claim["query_hash"] = hex.EncodeToString(hash[:])
		claim["query_hash_alg"] = "SHA512"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	tokenStr, err := token.SignedString([]byte(u.secretKey))
	if err != nil {
		return "", err
	}

	return "Bearer " + tokenStr, nil
}

// makeRequest makes authenticated HTTP request to Upbit API
func (u *UpbitClient) makeRequest(method, endpoint string, params map[string]string) ([]byte, error) {
	fullURL := baseURL + endpoint

	token, err := u.Token(params)
	if err != nil {
		return nil, err
	}

	var req *http.Request
	if (method == "GET" || method == "DELETE") && len(params) > 0 {
		// For GET and DELETE requests, add parameters to query string
		urlValues := url.Values{}
		for key, value := range params {
			urlValues.Add(key, value)
		}
		fullURL += "?" + urlValues.Encode()
		req, err = http.NewRequest(method, fullURL, nil)
	} else if method == "POST" && len(params) > 0 {
		// For POST requests, send parameters as JSON in body
		jsonBody, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, fullURL, strings.NewReader(string(jsonBody)))
		if err != nil {
			return nil, err
		}
	} else {
		req, err = http.NewRequest(method, fullURL, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ToSymbol converts asset and quote to exchange-specific symbol format
// Upbit format: USDT-BTC (quote-asset, reversed order with hyphen)
func (u *UpbitClient) ToSymbol(asset, quote string) string {
	return quote + "-" + asset
}

// ToAsset extracts the asset from a symbol (reverse of ToSymbol)
func (u *UpbitClient) ToAsset(symbol string) string {
	// Upbit uses reversed order with hyphen: USDT-BTC (quote-asset)
	// Split by hyphen and return the second part
	parts := strings.Split(symbol, "-")
	if len(parts) >= 2 {
		return parts[1] // Return the second part (asset)
	}
	
	// If no hyphen found, return the symbol as-is (shouldn't happen with valid symbols)
	return symbol
}

// SubscribeOrderEvents implements core.PrivateClient interface
func (u *UpbitClient) SubscribeOrderEvents(ctx context.Context, symbols []string, errHandler func(err error)) (<-chan core.OrderEvent, error) {
	u.wsMu.Lock()
	defer u.wsMu.Unlock()

	// Ensure websocket is connected
	if !u.wsConnected || u.privateWS == nil || !u.privateWS.IsConnected() {
		return nil, fmt.Errorf("websocket not connected")
	}

	// Create event channel if not exists
	if u.orderEventCh == nil {
		u.orderEventCh = make(chan core.OrderEvent, 100)
		u.subscriptionCtx, u.subscriptionCancel = context.WithCancel(ctx)

		// Start forwarding events from websocket to user channel
		go u.forwardOrderEvents(errHandler)
	}

	return u.orderEventCh, nil
}

// SubscribeBalanceEvents implements core.PrivateClient interface
func (u *UpbitClient) SubscribeBalanceEvents(ctx context.Context, assets []string, errHandler func(err error)) (<-chan core.BalanceEvent, error) {
	u.wsMu.Lock()
	defer u.wsMu.Unlock()

	// Ensure websocket is connected
	if !u.wsConnected || u.privateWS == nil || !u.privateWS.IsConnected() {
		return nil, fmt.Errorf("websocket not connected")
	}

	// Create event channel if not exists
	if u.balanceEventCh == nil {
		u.balanceEventCh = make(chan core.BalanceEvent, 100)
		u.subscriptionCtx, u.subscriptionCancel = context.WithCancel(ctx)

		// Start forwarding events from websocket to user channel
		go u.forwardBalanceEvents(errHandler)
	}

	return u.balanceEventCh, nil
}

// UnsubscribeOrderEvents implements core.PrivateClient interface
func (u *UpbitClient) UnsubscribeOrderEvents() error {
	u.wsMu.Lock()
	defer u.wsMu.Unlock()

	if u.subscriptionCancel != nil {
		u.subscriptionCancel()
	}

	if u.orderEventCh != nil {
		close(u.orderEventCh)
		u.orderEventCh = nil
	}

	return nil
}

// UnsubscribeBalanceEvents implements core.PrivateClient interface
func (u *UpbitClient) UnsubscribeBalanceEvents() error {
	u.wsMu.Lock()
	defer u.wsMu.Unlock()

	if u.subscriptionCancel != nil {
		u.subscriptionCancel()
	}

	if u.balanceEventCh != nil {
		close(u.balanceEventCh)
		u.balanceEventCh = nil
	}

	return nil
}

// forwardOrderEvents forwards order events from websocket to user channel
func (u *UpbitClient) forwardOrderEvents(errHandler func(err error)) {
	wsOrderCh := u.privateWS.GetOrderEventChannel()

	for {
		select {
		case orderEvent, ok := <-wsOrderCh:
			if !ok {
				if errHandler != nil {
					errHandler(fmt.Errorf("order event websocket channel closed"))
				}
				return
			}

			// Forward to user channel
			select {
			case u.orderEventCh <- orderEvent:
			case <-u.subscriptionCtx.Done():
				return
			default:
				// User channel full, drop event
				if errHandler != nil {
					errHandler(fmt.Errorf("order event channel full, dropping event for order %s", orderEvent.OrderID))
				}
			}

		case <-u.subscriptionCtx.Done():
			return
		}
	}
}

// forwardBalanceEvents forwards balance events from websocket to user channel
func (u *UpbitClient) forwardBalanceEvents(errHandler func(err error)) {
	wsBalanceCh := u.privateWS.GetBalanceEventChannel()

	for {
		select {
		case balanceEvent, ok := <-wsBalanceCh:
			if !ok {
				if errHandler != nil {
					errHandler(fmt.Errorf("balance event websocket channel closed"))
				}
				return
			}

			// Forward to user channel
			select {
			case u.balanceEventCh <- balanceEvent:
			case <-u.subscriptionCtx.Done():
				return
			default:
				// User channel full, drop event
				if errHandler != nil {
					errHandler(fmt.Errorf("balance event channel full, dropping event for asset %s", balanceEvent.Asset))
				}
			}

		case <-u.subscriptionCtx.Done():
			return
		}
	}
}
