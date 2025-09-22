package futures

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/ljm2ya/quickex-go/client/phemex"
	"github.com/ljm2ya/quickex-go/core"
)

func (c *PhemexFuturesClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Buy", "Limit", quantity, price, tif)
}

func (c *PhemexFuturesClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Sell", "Limit", quantity, price, tif)
}

func (c *PhemexFuturesClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Buy", "Market", quoteQuantity, decimal.Zero, "IOC")
}

func (c *PhemexFuturesClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Sell", "Market", quantity, decimal.Zero, "IOC")
}

func (c *PhemexFuturesClient) placeOrder(symbol, side, orderType string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	c.authMu.RLock()
	if !c.authenticated {
		c.authMu.RUnlock()
		return nil, fmt.Errorf("client not authenticated")
	}
	c.authMu.RUnlock()
	
	// Get symbol info for scaling
	priceScale := c.getPriceScale(symbol)
	ratioScale := c.getRatioScale(symbol)
	
	// Prepare order request
	orderReq := phemex.PhemexOrderRequest{
		Symbol:      symbol,
		Side:        side,
		OrderQty:    phemex.ToEp(quantity, ratioScale),
		OrdType:     orderType,
		TimeInForce: c.mapTimeInForce(tif),
	}
	
	// Set price for limit orders
	if orderType == "Limit" {
		orderReq.PriceEp = phemex.ToEp(price, priceScale)
	}
	
	msg := phemex.PhemexWSMessage{
		ID:     nextWSID(),
		Method: "order.place",
		Params: orderReq,
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}
	
	return c.parseOrderResponse(response, symbol, side, quantity, price)
}

func (c *PhemexFuturesClient) parseOrderResponse(responseBytes []byte, symbol, side string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	var result struct {
		OrderID       string `json:"orderID"`
		ClOrderID     string `json:"clOrderID"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		OrderStatus   string `json:"orderStatus"`
		ActionTimeNs  int64  `json:"actionTimeNs"`
		PriceEp       int64  `json:"priceEp"`
		OrderQty      int64  `json:"orderQty"`
		TimeInForce   string `json:"timeInForce"`
	}
	
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order response: %w", err)
	}
	
	if result.OrderID == "" {
		return nil, fmt.Errorf("order placement failed: no order ID received")
	}
	
	status := c.mapOrderStatus(result.OrderStatus)
	priceScale := c.getPriceScale(symbol)
	
	orderResp := &core.OrderResponse{
		OrderID:         result.OrderID,
		Symbol:          symbol,
		Side:            side,
		Status:          status,
		Price:           phemex.FromEp(result.PriceEp, priceScale),
		Quantity:        c.convertQuantity(result.OrderQty, symbol),
		IsQuoteQuantity: false,
		CreateTime:      phemex.ToTimeNs(result.ActionTimeNs),
	}
	
	// Update cache
	c.ordersMu.Lock()
	c.orders[result.OrderID] = orderResp
	c.ordersMu.Unlock()
	
	return orderResp, nil
}

func (c *PhemexFuturesClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	c.authMu.RLock()
	if !c.authenticated {
		c.authMu.RUnlock()
		return nil, fmt.Errorf("client not authenticated")
	}
	c.authMu.RUnlock()
	
	msg := phemex.PhemexWSMessage{
		ID:     nextWSID(),
		Method: "order.cancel",
		Params: map[string]interface{}{
			"symbol":  symbol,
			"orderID": orderId,
		},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}
	
	return c.parseCancelResponse(response, symbol, orderId)
}

func (c *PhemexFuturesClient) parseCancelResponse(responseBytes []byte, symbol, orderId string) (*core.OrderResponse, error) {
	var result struct {
		OrderID     string `json:"orderID"`
		Symbol      string `json:"symbol"`
		OrderStatus string `json:"orderStatus"`
	}
	
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cancel response: %w", err)
	}
	
	if result.OrderID != orderId {
		return nil, fmt.Errorf("cancel response order ID mismatch: expected %s, got %s", orderId, result.OrderID)
	}
	
	// Get order from cache and update status
	c.ordersMu.Lock()
	defer c.ordersMu.Unlock()
	
	if order, exists := c.orders[orderId]; exists {
		order.Status = core.OrderStatusCanceled
		return order, nil
	}
	
	// If not in cache, create a minimal response
	return &core.OrderResponse{
		OrderID: orderId,
		Symbol:  symbol,
		Status:  core.OrderStatusCanceled,
	}, nil
}

func (c *PhemexFuturesClient) CancelAll(symbol string) error {
	c.authMu.RLock()
	if !c.authenticated {
		c.authMu.RUnlock()
		return fmt.Errorf("client not authenticated")
	}
	c.authMu.RUnlock()
	
	msg := phemex.PhemexWSMessage{
		ID:     nextWSID(),
		Method: "order.cancelAll",
		Params: map[string]interface{}{
			"symbol": symbol,
		},
	}
	
	_, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}
	
	// Update cache - mark all orders for this symbol as canceled
	c.ordersMu.Lock()
	defer c.ordersMu.Unlock()
	
	for _, order := range c.orders {
		if order.Symbol == symbol && order.Status == core.OrderStatusOpen {
			order.Status = core.OrderStatusCanceled
		}
	}
	
	return nil
}

// mapTimeInForce maps core TimeInForce to Phemex order types
func (c *PhemexFuturesClient) mapTimeInForce(tif string) string {
	switch tif {
	case "GTC":
		return "GoodTillCancel"
	case "IOC":
		return "ImmediateOrCancel"
	case "FOK":
		return "FillOrKill"
	default:
		return "GoodTillCancel" // Default to GTC
	}
}