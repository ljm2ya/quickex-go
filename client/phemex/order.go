package phemex

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/ljm2ya/quickex-go/core"
)

func (c *PhemexClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Buy", "Limit", quantity, price, tif)
}

func (c *PhemexClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Sell", "Limit", quantity, price, tif)
}

func (c *PhemexClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Buy", "Market", quoteQuantity, decimal.Zero, "IOC")
}

func (c *PhemexClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Sell", "Market", quantity, decimal.Zero, "IOC")
}

func (c *PhemexClient) placeOrder(symbol, side, orderType string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	// Use REST API for order placement as WebSocket doesn't support it
	
	// Get symbol info for scaling
	priceScale := c.getPriceScale(symbol)
	ratioScale := c.getRatioScale(symbol)
	
	// Prepare REST order request
	orderReq := map[string]interface{}{
		"symbol":      symbol,
		"clOrdID":     fmt.Sprintf("qx_%d", time.Now().UnixNano()),
		"side":        side,
		"orderQty":    ToEp(quantity, ratioScale),
		"ordType":     orderType,
		"timeInForce": c.mapTimeInForce(tif),
	}
	
	// Set price for limit orders
	if orderType == "Limit" {
		orderReq["priceEp"] = ToEp(price, priceScale)
	}
	
	orderBody, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}
	
	// Make REST API call
	resp, err := c.makeRestRequest("POST", "/orders", orderBody)
	if err != nil {
		return nil, fmt.Errorf("failed to place order via REST: %w", err)
	}
	
	if resp.Code != 0 {
		return nil, fmt.Errorf("order placement failed: Code=%d, Msg=%s", resp.Code, resp.Msg)
	}
	
	return c.parseRestOrderResponse(resp.Data, symbol, side, quantity, price)
}

func (c *PhemexClient) parseRestOrderResponse(data interface{}, symbol, side string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid order response format")
	}
	
	orderID, exists := dataMap["orderID"]
	if !exists {
		return nil, fmt.Errorf("order placement failed: no order ID received")
	}
	
	orderIDStr := fmt.Sprintf("%v", orderID)
	if orderIDStr == "" {
		return nil, fmt.Errorf("order placement failed: empty order ID")
	}
	
	priceScale := c.getPriceScale(symbol)
	
	orderResp := &core.OrderResponse{
		OrderID:         orderIDStr,
		Symbol:          symbol,
		Side:            side,
		Status:          core.OrderStatusOpen,
		Price:           price,
		Quantity:        quantity,
		IsQuoteQuantity: false,
		CreateTime:      time.Now(),
	}
	
	// Extract additional fields if available
	if priceEp, exists := dataMap["priceEp"]; exists {
		if priceEpFloat, ok := priceEp.(float64); ok {
			orderResp.Price = FromEp(int64(priceEpFloat), priceScale)
		}
	}
	
	if orderStatus, exists := dataMap["orderStatus"]; exists {
		if statusStr, ok := orderStatus.(string); ok {
			orderResp.Status = c.mapOrderStatus(statusStr)
		}
	}
	
	// Update cache
	c.ordersMu.Lock()
	c.orders[orderIDStr] = orderResp
	c.ordersMu.Unlock()
	
	return orderResp, nil
}

func (c *PhemexClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	// Use REST API for order cancellation
	cancelPath := fmt.Sprintf("/orders/cancel?symbol=%s&orderID=%s", symbol, orderId)
	
	resp, err := c.makeRestRequest("DELETE", cancelPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order via REST: %w", err)
	}
	
	if resp.Code != 0 {
		return nil, fmt.Errorf("order cancellation failed: Code=%d, Msg=%s", resp.Code, resp.Msg)
	}
	
	return c.parseRestCancelResponse(resp.Data, symbol, orderId)
}

func (c *PhemexClient) parseRestCancelResponse(data interface{}, symbol, orderId string) (*core.OrderResponse, error) {
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

func (c *PhemexClient) CancelAll(symbol string) error {
	// Use REST API for cancel all orders
	cancelPath := fmt.Sprintf("/orders/all?symbol=%s", symbol)
	
	resp, err := c.makeRestRequest("DELETE", cancelPath, nil)
	if err != nil {
		return fmt.Errorf("failed to cancel all orders via REST: %w", err)
	}
	
	if resp.Code != 0 {
		return fmt.Errorf("cancel all orders failed: Code=%d, Msg=%s", resp.Code, resp.Msg)
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
func (c *PhemexClient) mapTimeInForce(tif string) string {
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

func (c *PhemexClient) getRatioScale(symbol string) int {
	c.symbolsMu.RLock()
	defer c.symbolsMu.RUnlock()
	
	if symbolInfo, exists := c.symbols[symbol]; exists {
		return symbolInfo.RatioScale
	}
	return 8 // Default scale
}