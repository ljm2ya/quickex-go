package upbit

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/ljm2ya/quickex-go/core"
)

const (
	baseURL = "https://api.upbit.com"
)

type UpbitClient struct {
	apiKey    string
	secretKey string
	client    *http.Client
}

func NewUpbitClient(accessKey, secretKey string) *UpbitClient {
	return &UpbitClient{
		apiKey:    accessKey,
		secretKey: secretKey,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Connect implements core.PrivateClient interface
func (u *UpbitClient) Connect(ctx context.Context) (int64, error) {
	// Upbit doesn't use persistent connections like WebSocket
	// Return 0 delta timestamp since no time sync is needed
	return 0, nil
}

// Close implements core.PrivateClient interface
func (u *UpbitClient) Close() error {
	// Upbit doesn't maintain persistent connections
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
	if method == "GET" && len(params) > 0 {
		urlValues := url.Values{}
		for key, value := range params {
			urlValues.Add(key, value)
		}
		fullURL += "?" + urlValues.Encode()
		req, err = http.NewRequest(method, fullURL, nil)
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
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}
	
	return body, nil
}

// FetchBalance implements core.PrivateClient interface
func (u *UpbitClient) FetchBalance(asset string, includeLocked bool) (decimal.Decimal, error) {
	body, err := u.makeRequest("GET", "/v1/accounts", nil)
	if err != nil {
		return decimal.Zero, err
	}
	
	var balances []UpbitBalance
	if err := json.Unmarshal(body, &balances); err != nil {
		return decimal.Zero, err
	}
	
	for _, balance := range balances {
		if balance.Currency == asset {
			free, err := decimal.NewFromString(balance.Balance)
			if err != nil {
				return decimal.Zero, err
			}
			
			if includeLocked {
				locked, err := decimal.NewFromString(balance.Locked)
				if err != nil {
					return decimal.Zero, err
				}
				return free.Add(locked), nil
			}
			return free, nil
		}
	}
	
	return decimal.Zero, errors.New("asset not found")
}

// FetchOrder implements core.PrivateClient interface
func (u *UpbitClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	params := make(map[string]string)
	if len(orderId) <= 10 { // identifier
		params["identifier"] = orderId
	} else { // uuid
		params["uuid"] = orderId
	}
	
	body, err := u.makeRequest("GET", "/v1/order", params)
	if err != nil {
		return nil, err
	}
	
	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	
	// Convert to core types
	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}
	
	var side string
	if order.Side == "bid" {
		side = "BUY"
	} else {
		side = "SELL"
	}
	
	price, _ := decimal.NewFromString(order.Price)
	quantity, _ := decimal.NewFromString(order.Volume)
	executedQty, _ := decimal.NewFromString(order.ExecutedVolume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)
	
	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:    order.UUID,
			Symbol:     order.Market,
			Side:       side,
			Tif:        core.TimeInForceGTC,
			Status:     status,
			Price:      price,
			Quantity:   quantity,
			CreateTime: createdAt,
		},
		AvgPrice:    price, // Upbit doesn't provide avg price
		ExecutedQty: executedQty,
		UpdateTime:  createdAt, // Upbit doesn't provide update time
	}, nil
}

// LimitBuy implements core.PrivateClient interface
func (u *UpbitClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "bid",
		"ord_type":   "limit",
		"price":      price.String(),
		"volume":     quantity.String(),
		"identifier": uuid.New().String(),
	}
	
	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}
	
	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	
	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}
	
	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)
	
	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "BUY",
		Tif:        core.TimeInForce(tif),
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// LimitSell implements core.PrivateClient interface
func (u *UpbitClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "ask",
		"ord_type":   "limit",
		"price":      price.String(),
		"volume":     quantity.String(),
		"identifier": uuid.New().String(),
	}
	
	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}
	
	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	
	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}
	
	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)
	
	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "SELL",
		Tif:        core.TimeInForce(tif),
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// MarketBuy implements core.PrivateClient interface
func (u *UpbitClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "bid",
		"ord_type":   "price",
		"price":      quoteQuantity.String(),
		"identifier": uuid.New().String(),
	}
	
	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}
	
	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	
	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}
	
	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)
	
	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "BUY",
		Tif:        core.TimeInForceGTC,
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// MarketSell implements core.PrivateClient interface
func (u *UpbitClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "ask",
		"ord_type":   "market",
		"volume":     quantity.String(),
		"identifier": uuid.New().String(),
	}
	
	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}
	
	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	
	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}
	
	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)
	
	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "SELL",
		Tif:        core.TimeInForceGTC,
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// StopLossSell implements core.PrivateClient interface
func (u *UpbitClient) StopLossSell(symbol string, quantity, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	return nil, errors.New("upbit does not support stop-loss orders")
}

// TakeProfitSell implements core.PrivateClient interface
func (u *UpbitClient) TakeProfitSell(symbol string, quantity, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	return nil, errors.New("upbit does not support take-profit orders")
}

// CancelOrder implements core.PrivateClient interface
func (u *UpbitClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	params := make(map[string]string)
	if len(orderId) <= 10 { // identifier
		params["identifier"] = orderId
	} else { // uuid
		params["uuid"] = orderId
	}
	
	body, err := u.makeRequest("DELETE", "/v1/order", params)
	if err != nil {
		return nil, err
	}
	
	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}
	
	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}
	
	var side string
	if order.Side == "bid" {
		side = "BUY"
	} else {
		side = "SELL"
	}
	
	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)
	
	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       side,
		Tif:        core.TimeInForceGTC,
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// CancelAll implements core.PrivateClient interface
func (u *UpbitClient) CancelAll(symbol string) error {
	// Get open orders first
	params := map[string]string{"market": symbol}
	body, err := u.makeRequest("GET", "/v1/orders", params)
	if err != nil {
		return err
	}
	
	var orders []UpbitOrder
	if err := json.Unmarshal(body, &orders); err != nil {
		return err
	}
	
	// Cancel each order
	for _, order := range orders {
		_, err := u.CancelOrder(symbol, order.UUID)
		if err != nil {
			fmt.Printf("Failed to cancel order %s: %v\n", order.UUID, err)
		}
	}
	
	return nil
}

// GetTicker gets ticker data for a symbol
func (u *UpbitClient) GetTicker(symbol string) (*UpbitTickerOfMarket, error) {
	// This is a public endpoint, no authentication needed
	resp, err := http.Get(baseURL + "/v1/ticker?markets=" + symbol)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var tickers []UpbitTickerOfMarket
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, err
	}
	
	if len(tickers) == 0 {
		return nil, errors.New("ticker not found")
	}
	
	return &tickers[0], nil
}

// GetMarketRules gets market rules for a quote currency
func (u *UpbitClient) GetMarketRules(quote string) ([]UpbitMarket, error) {
	// This is a public endpoint, no authentication needed
	resp, err := http.Get(baseURL + "/v1/market/all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var markets []UpbitMarket
	if err := json.Unmarshal(body, &markets); err != nil {
		return nil, err
	}
	
	// Filter by quote currency
	var filtered []UpbitMarket
	for _, market := range markets {
		if strings.HasPrefix(market.Market, quote+"-") {
			filtered = append(filtered, market)
		}
	}
	
	return filtered, nil
}

// SubscribeQuotes implements core.PublicClient interface
func (u *UpbitClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]<-chan core.Quote, error) {
	quoteChans := make(map[string]<-chan core.Quote)
	
	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 1)
		quoteChans[symbol] = quoteChan
		
		// Poll ticker data in a goroutine (Upbit doesn't have quote-specific WebSocket)
		go func(sym string, ch chan core.Quote) {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			defer close(ch)
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					tickerData, err := u.GetTicker(sym)
					if err != nil {
						if errHandler != nil {
							errHandler(err)
						}
						continue
					}
					
					select {
					case ch <- core.Quote{
						Symbol:   tickerData.Market,
						BidPrice: decimal.NewFromFloat(tickerData.TradePrice),
						BidQty:   decimal.Zero, // Upbit ticker doesn't provide bid/ask quantities
						AskPrice: decimal.NewFromFloat(tickerData.TradePrice),
						AskQty:   decimal.Zero,
						Time:     time.Now(),
					}:
					default:
						// Channel full, skip
					}
				}
			}
		}(symbol, quoteChan)
	}
	
	return quoteChans, nil
}

// FetchMarketRules implements core.PublicClient interface
func (u *UpbitClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	var allRules []core.MarketRule
	
	for _, quote := range quotes {
		rules, err := u.GetMarketRules(quote)
		if err != nil {
			return nil, err
		}
		
		// Convert to core types
		for _, rule := range rules {
			allRules = append(allRules, core.MarketRule{
				Symbol:         rule.Market,
				BaseAsset:      rule.English_name,
				QuoteAsset:     quote,
				PricePrecision: 8, // Upbit default
				QtyPrecision:   8, // Upbit default
				MinPrice:       decimal.Zero,
				MaxPrice:       decimal.NewFromInt(1000000),
				MinQty:         decimal.NewFromFloat(0.00000001),
				MaxQty:         decimal.NewFromInt(1000000),
				TickSize:       decimal.NewFromFloat(0.00000001),
				StepSize:       decimal.NewFromFloat(0.00000001),
				RateLimits:     []core.RateLimit{},
			})
		}
	}
	
	return allRules, nil
}