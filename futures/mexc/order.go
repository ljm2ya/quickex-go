package mexc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// LimitBuy implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, 1, 1, quantity, price, tif) // 1 = Buy, 1 = Limit
}

// LimitSell implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, 2, 1, quantity, price, tif) // 2 = Sell, 1 = Limit
}

// MarketBuy implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, 1, 2, quoteQuantity, decimal.Zero, "") // 1 = Buy, 2 = Market
}

// MarketSell implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, 2, 2, quantity, decimal.Zero, "") // 2 = Sell, 2 = Market
}

// placeOrder is the internal method for placing futures orders via REST API
func (c *MexcFuturesClient) placeOrder(symbol string, side, orderType int, quantity, price decimal.Decimal, timeInForce string) (*core.OrderResponse, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", strconv.Itoa(side))
	params.Set("type", strconv.Itoa(orderType))
	params.Set("vol", quantity.String())
	
	// Set price for limit orders
	if orderType == 1 && !price.IsZero() { // Limit order
		params.Set("price", price.String())
	}
	
	// MEXC futures doesn't use timeInForce in the same way
	// Default behavior is GTC for limit orders
	
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v1/private/order/submit", mexcFuturesBaseURL)
	
	req, err := http.NewRequest("POST", reqURL, bytes.NewBufferString(signedParams))
	if err != nil {
		return nil, fmt.Errorf("failed to create futures order request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to place futures order: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read futures order response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("futures order request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var orderResp struct {
		Success bool   `json:"success"`
		Code    int    `json:"code"`
		Data    string `json:"data"` // Order ID as string
	}
	
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal futures order response: %w", err)
	}
	
	if !orderResp.Success {
		return nil, fmt.Errorf("futures order failed with code: %d", orderResp.Code)
	}
	
	// Convert side back to string for response
	sideStr := "BUY"
	if side == 2 {
		sideStr = "SELL"
	}
	
	// Convert timeInForce
	tif := core.TimeInForceGTC
	if timeInForce == "IOC" {
		tif = core.TimeInForceIOC
	} else if timeInForce == "FOK" {
		tif = core.TimeInForceFOK
	}
	
	return &core.OrderResponse{
		OrderID:         orderResp.Data,
		Symbol:          symbol,
		Side:            sideStr,
		Tif:             tif,
		Status:          core.OrderStatusOpen,
		Price:           price,
		Quantity:        quantity,
		IsQuoteQuantity: false,
		CreateTime:      time.Now(),
	}, nil
}

// CancelOrder implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("order_id", orderId)
	
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v1/private/order/cancel", mexcFuturesBaseURL)
	
	req, err := http.NewRequest("POST", reqURL, bytes.NewBufferString(signedParams))
	if err != nil {
		return nil, fmt.Errorf("failed to create futures cancel request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel futures order: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read futures cancel response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("futures cancel request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var cancelResp struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
	}
	
	if err := json.Unmarshal(body, &cancelResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal futures cancel response: %w", err)
	}
	
	if !cancelResp.Success {
		return nil, fmt.Errorf("futures cancel failed with code: %d", cancelResp.Code)
	}
	
	return &core.OrderResponse{
		OrderID:    orderId,
		Symbol:     symbol,
		Status:     core.OrderStatusCanceled,
		CreateTime: time.Now(),
	}, nil
}

// CancelAll implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) CancelAll(symbol string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v1/private/order/cancel_all", mexcFuturesBaseURL)
	
	req, err := http.NewRequest("POST", reqURL, bytes.NewBufferString(signedParams))
	if err != nil {
		return fmt.Errorf("failed to create futures cancel all request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to cancel all futures orders: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("futures cancel all request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	return nil
}

// FetchOrder implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("order_id", orderId)
	
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v1/private/order/get?%s", mexcFuturesBaseURL, signedParams)
	
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create futures order query request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch futures order: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read futures order query response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("futures order query request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var orderInfo struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    struct {
			OrderId       string  `json:"orderId"`
			Symbol        string  `json:"symbol"`
			PositionId    string  `json:"positionId"`
			Price         float64 `json:"price"`
			Vol           float64 `json:"vol"`
			Leverage      int     `json:"leverage"`
			Side          int     `json:"side"`
			Category      int     `json:"category"`
			OrderType     int     `json:"orderType"`
			DealAvgPrice  float64 `json:"dealAvgPrice"`
			DealVol       float64 `json:"dealVol"`
			OrderMargin   float64 `json:"orderMargin"`
			UsedMargin    float64 `json:"usedMargin"`
			TakerFeeRate  float64 `json:"takerFeeRate"`
			MakerFeeRate  float64 `json:"makerFeeRate"`
			Profit        float64 `json:"profit"`
			FeeCurrency   string  `json:"feeCurrency"`
			OpenType      int     `json:"openType"`
			State         int     `json:"state"`
			ExternalOid   string  `json:"externalOid"`
			ErrorCode     int     `json:"errorCode"`
			UsedFee       float64 `json:"usedFee"`
			CreateTime    int64   `json:"createTime"`
			UpdateTime    int64   `json:"updateTime"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &orderInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal futures order query response: %w", err)
	}
	
	if !orderInfo.Success {
		return nil, fmt.Errorf("futures order query failed with code: %d", orderInfo.Code)
	}
	
	order := orderInfo.Data
	
	// Convert side
	sideStr := "BUY"
	if order.Side == 2 {
		sideStr = "SELL"
	}
	
	// Convert state to order status
	status := parseMexcFuturesOrderState(order.State)
	
	qty := decimal.NewFromFloat(order.Vol)
	orderPrice := decimal.NewFromFloat(order.Price)
	executedQty := decimal.NewFromFloat(order.DealVol)
	avgPrice := decimal.NewFromFloat(order.DealAvgPrice)
	commission := decimal.NewFromFloat(order.UsedFee)
	
	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:         order.OrderId,
			Symbol:          order.Symbol,
			Side:            sideStr,
			Tif:             core.TimeInForceGTC, // MEXC futures default
			Status:          status,
			Price:           orderPrice,
			Quantity:        qty,
			IsQuoteQuantity: false,
			CreateTime:      time.UnixMilli(order.CreateTime),
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     executedQty,
		Commission:      commission,
		CommissionAsset: order.FeeCurrency,
		UpdateTime:      time.UnixMilli(order.UpdateTime),
	}, nil
}

// parseMexcFuturesOrderState converts MEXC futures order state to core.OrderStatus
func parseMexcFuturesOrderState(state int) core.OrderStatus {
	switch state {
	case 1: // New
		return core.OrderStatusOpen
	case 2: // Partially filled
		return core.OrderStatusOpen
	case 3: // Filled
		return core.OrderStatusFilled
	case 4: // Canceled
		return core.OrderStatusCanceled
	case 5: // Rejected
		return core.OrderStatusError
	default:
		return core.OrderStatusError
	}
}
