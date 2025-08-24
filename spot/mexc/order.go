package mexc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// LimitBuy implements core.PrivateClient interface
func (c *MexcClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "BUY", "LIMIT", quantity, price, decimal.Zero, tif)
}

// LimitSell implements core.PrivateClient interface
func (c *MexcClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "SELL", "LIMIT", quantity, price, decimal.Zero, tif)
}

// MarketBuy implements core.PrivateClient interface
func (c *MexcClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "BUY", "MARKET", decimal.Zero, decimal.Zero, quoteQuantity, "")
}

// MarketSell implements core.PrivateClient interface
func (c *MexcClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "SELL", "MARKET", quantity, decimal.Zero, decimal.Zero, "")
}

// placeOrder is the internal method for placing orders via REST API
func (c *MexcClient) placeOrder(symbol, side, orderType string, quantity, price, quoteOrderQty decimal.Decimal, timeInForce string) (*core.OrderResponse, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side)
	params.Set("type", orderType)
	
	// Set quantity based on order type
	if orderType == "MARKET" && side == "BUY" && !quoteOrderQty.IsZero() {
		params.Set("quoteOrderQty", quoteOrderQty.String())
	} else if !quantity.IsZero() {
		params.Set("quantity", quantity.String())
	}
	
	// Set price for limit orders
	if orderType == "LIMIT" && !price.IsZero() {
		params.Set("price", price.String())
	}
	
	// Set time in force for limit orders
	if orderType == "LIMIT" && timeInForce != "" {
		params.Set("timeInForce", timeInForce)
	}
	
	signedParams := c.buildSignedParams(params)
	
	// Use query string in URL for GET-like behavior but with POST
	reqURL := fmt.Sprintf("%s/api/v3/order?%s", mexcSpotBaseURL, signedParams)
	
	req, err := http.NewRequest("POST", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create order request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read order response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("order request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var orderResp struct {
		Symbol              string      `json:"symbol"`
		OrderId             string      `json:"orderId"`
		OrderListId         json.Number `json:"orderListId"`
		ClientOrderId       string      `json:"clientOrderId"`
		TransactTime        int64       `json:"transactTime"`
		Price               string      `json:"price"`
		OrigQty             string      `json:"origQty"`
		ExecutedQty         string      `json:"executedQty"`
		CumulativeQuoteQty  string      `json:"cummulativeQuoteQty"`
		Status              string      `json:"status"`
		TimeInForce         string      `json:"timeInForce"`
		Type                string      `json:"type"`
		Side                string      `json:"side"`
	}
	
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order response: %w", err)
	}
	
	// Parse quantities
	var qty decimal.Decimal
	var isQuoteQty bool
	
	if orderResp.OrigQty != "" {
		qty = decimal.RequireFromString(orderResp.OrigQty)
		isQuoteQty = false
	} else if orderResp.CumulativeQuoteQty != "" {
		qty = decimal.RequireFromString(orderResp.CumulativeQuoteQty)
		isQuoteQty = true
	}
	
	// Parse price
	orderPrice := decimal.Zero
	if orderResp.Price != "" {
		orderPrice = decimal.RequireFromString(orderResp.Price)
	}
	
	return &core.OrderResponse{
		OrderID:         orderResp.OrderId,
		Symbol:          orderResp.Symbol,
		Side:            orderResp.Side,
		Tif:             core.TimeInForce(orderResp.TimeInForce),
		Status:          parseMexcOrderStatus(orderResp.Status),
		Price:           orderPrice,
		Quantity:        qty,
		IsQuoteQuantity: isQuoteQty,
		CreateTime:      time.UnixMilli(orderResp.TransactTime),
	}, nil
}

// CancelOrder implements core.PrivateClient interface
func (c *MexcClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", orderId)
	
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v3/order", mexcSpotBaseURL)
	
	req, err := http.NewRequest("DELETE", reqURL, bytes.NewBufferString(signedParams))
	if err != nil {
		return nil, fmt.Errorf("failed to create cancel request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read cancel response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cancel request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var cancelResp struct {
		Symbol              string `json:"symbol"`
		OrderId             string `json:"orderId"`
		ClientOrderId       string `json:"clientOrderId"`
		Price               string `json:"price"`
		OrigQty             string `json:"origQty"`
		ExecutedQty         string `json:"executedQty"`
		CumulativeQuoteQty  string `json:"cummulativeQuoteQty"`
		Status              string `json:"status"`
		TimeInForce         string `json:"timeInForce"`
		Type                string `json:"type"`
		Side                string `json:"side"`
	}
	
	if err := json.Unmarshal(body, &cancelResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cancel response: %w", err)
	}
	
	qty := decimal.RequireFromString(cancelResp.OrigQty)
	orderPrice := decimal.RequireFromString(cancelResp.Price)
	
	return &core.OrderResponse{
		OrderID:         cancelResp.OrderId,
		Symbol:          cancelResp.Symbol,
		Side:            cancelResp.Side,
		Tif:             core.TimeInForce(cancelResp.TimeInForce),
		Status:          core.OrderStatusCanceled,
		Price:           orderPrice,
		Quantity:        qty,
		IsQuoteQuantity: false,
		CreateTime:      time.Now(), // MEXC doesn't return original creation time on cancel
	}, nil
}

// CancelAll implements core.PrivateClient interface
func (c *MexcClient) CancelAll(symbol string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v3/openOrders", mexcSpotBaseURL)
	
	req, err := http.NewRequest("DELETE", reqURL, bytes.NewBufferString(signedParams))
	if err != nil {
		return fmt.Errorf("failed to create cancel all request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("cancel all request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	return nil
}

// FetchOrder implements core.PrivateClient interface
func (c *MexcClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", orderId)
	
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v3/order?%s", mexcSpotBaseURL, signedParams)
	
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create order query request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order: %w", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read order query response: %w", err)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("order query request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	var orderInfo struct {
		Symbol              string `json:"symbol"`
		OrderId             string `json:"orderId"`
		ClientOrderId       string `json:"clientOrderId"`
		Price               string `json:"price"`
		OrigQty             string `json:"origQty"`
		ExecutedQty         string `json:"executedQty"`
		CumulativeQuoteQty  string `json:"cummulativeQuoteQty"`
		Status              string `json:"status"`
		TimeInForce         string `json:"timeInForce"`
		Type                string `json:"type"`
		Side                string `json:"side"`
		Time                int64  `json:"time"`
		UpdateTime          int64  `json:"updateTime"`
	}
	
	if err := json.Unmarshal(body, &orderInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order query response: %w", err)
	}
	
	qty := decimal.RequireFromString(orderInfo.OrigQty)
	orderPrice := decimal.RequireFromString(orderInfo.Price)
	executedQty := decimal.RequireFromString(orderInfo.ExecutedQty)
	
	// For average price calculation, we need to use executed qty and cumulative quote qty
	avgPrice := orderPrice
	if !executedQty.IsZero() && orderInfo.CumulativeQuoteQty != "" {
		cumulativeQuote := decimal.RequireFromString(orderInfo.CumulativeQuoteQty)
		if !cumulativeQuote.IsZero() {
			avgPrice = cumulativeQuote.Div(executedQty)
		}
	}
	
	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:         orderInfo.OrderId,
			Symbol:          orderInfo.Symbol,
			Side:            orderInfo.Side,
			Tif:             core.TimeInForce(orderInfo.TimeInForce),
			Status:          parseMexcOrderStatus(orderInfo.Status),
			Price:           orderPrice,
			Quantity:        qty,
			IsQuoteQuantity: false,
			CreateTime:      time.UnixMilli(orderInfo.Time),
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     executedQty,
		Commission:      decimal.Zero, // MEXC doesn't provide commission in order info
		CommissionAsset: "",
		UpdateTime:      time.UnixMilli(orderInfo.UpdateTime),
	}, nil
}

// parseMexcOrderStatus converts MEXC order status to core.OrderStatus
func parseMexcOrderStatus(status string) core.OrderStatus {
	switch status {
	case "NEW":
		return core.OrderStatusOpen
	case "PARTIALLY_FILLED":
		return core.OrderStatusOpen
	case "FILLED":
		return core.OrderStatusFilled
	case "CANCELED":
		return core.OrderStatusCanceled
	case "REJECTED":
		return core.OrderStatusError
	case "EXPIRED":
		return core.OrderStatusError
	case "": // MEXC doesn't return status for new orders, assume NEW
		return core.OrderStatusOpen
	default:
		return core.OrderStatusError
	}
}
