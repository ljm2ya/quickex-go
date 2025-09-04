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

// tryWebSocketOrder tries to place order via WebSocket first (currently not supported by Phemex spot)
func (c *PhemexClient) tryWebSocketOrder(symbol, side, orderType string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	// WebSocket 주문 시도 - Phemex spot API에서 지원하지 않지만 표준 패턴 유지
	c.authMu.RLock()
	if !c.authenticated {
		c.authMu.RUnlock()
		return nil, fmt.Errorf("not authenticated for websocket order")
	}
	c.authMu.RUnlock()
	
	// Phemex spot에는 order.place websocket 메소드가 없어서 항상 실패
	// 하지만 websocket 우선 시도 패턴을 유지하기 위해 구현
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "order.place", // Phemex spot에서 지원하지 않는 메소드
		"params": map[string]interface{}{
			"symbol":      symbol,
			"side":        side,
			"ordType":     orderType,
			"orderQty":    ToEp(quantity, c.getRatioScale(symbol)),
			"timeInForce": c.mapTimeInForce(tif),
		},
	}
	
	if orderType == "Limit" {
		priceScale := c.getPriceScale(symbol)
		msg["params"].(map[string]interface{})["priceEp"] = ToEp(price, priceScale)
	}
	
	// 이 요청은 Phemex spot에서 지원하지 않으므로 항상 에러 반환
	_, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("websocket order not supported in Phemex spot: %w", err)
	}
	
	// 여기까지 오는 경우는 없지만 안전을 위해
	return nil, fmt.Errorf("websocket order placement not implemented in Phemex spot")
}

// tryWebSocketCancel tries to cancel order via WebSocket first (currently not supported by Phemex spot)
func (c *PhemexClient) tryWebSocketCancel(symbol, orderId string) (*core.OrderResponse, error) {
	c.authMu.RLock()
	if !c.authenticated {
		c.authMu.RUnlock()
		return nil, fmt.Errorf("not authenticated for websocket cancel")
	}
	c.authMu.RUnlock()
	
	// Phemex spot에는 order.cancel websocket 메소드가 없어서 항상 실패
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "order.cancel", // Phemex spot에서 지원하지 않는 메소드
		"params": map[string]interface{}{
			"symbol":  symbol,
			"orderID": orderId,
		},
	}
	
	_, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("websocket cancel not supported in Phemex spot: %w", err)
	}
	
	return nil, fmt.Errorf("websocket order cancellation not implemented in Phemex spot")
}

// tryWebSocketCancelAll tries to cancel all orders via WebSocket first (currently not supported by Phemex spot)
func (c *PhemexClient) tryWebSocketCancelAll(symbol string) error {
	c.authMu.RLock()
	if !c.authenticated {
		c.authMu.RUnlock()
		return fmt.Errorf("not authenticated for websocket cancel all")
	}
	c.authMu.RUnlock()
	
	// Phemex spot에는 order.cancelAll websocket 메소드가 없어서 항상 실패
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "order.cancelAll", // Phemex spot에서 지원하지 않는 메소드
		"params": map[string]interface{}{
			"symbol": symbol,
		},
	}
	
	_, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return fmt.Errorf("websocket cancel all not supported in Phemex spot: %w", err)
	}
	
	return fmt.Errorf("websocket cancel all not implemented in Phemex spot")
}

func (c *PhemexClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeOrder(symbol, "Sell", "Market", quantity, decimal.Zero, "IOC")
}

func (c *PhemexClient) placeOrder(symbol, side, orderType string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	// 웹소켓 우선 시도 - Phemex spot에서는 지원하지 않지만 시도해본다
	if wsResp, err := c.tryWebSocketOrder(symbol, side, orderType, quantity, price, tif); err == nil {
		return wsResp, nil
	}
	
	// 웹소켓 실패시 REST API fallback - Phemex spot에서는 websocket 주문이 지원되지 않아 REST 사용
	return c.placeRestOrder(symbol, side, orderType, quantity, price, tif)
}

// placeRestOrder implements correct Phemex spot trading format with baseQtyEv/quoteQtyEv and qtyType
func (c *PhemexClient) placeRestOrder(symbol, side, orderType string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	// Phemex spot uses baseQtyEv/quoteQtyEv with qtyType instead of orderQty
	orderReq := map[string]interface{}{
		"symbol":  symbol,
		"clOrdID": fmt.Sprintf("qx_%d", time.Now().UnixNano()),
		"side":    side,
		"ordType": orderType,
	}
	
	// For Market Buy orders, use ByQuote (buy with USDT amount)
	// For Market Sell and all Limit orders, use ByBase (sell/buy specific crypto amount)
	isMarketBuy := (orderType == "Market" && side == "Buy")
	
	if isMarketBuy {
		// Market Buy: Use quote quantity (USDT amount)
		// quantity represents the USDT amount to spend
		orderReq["qtyType"] = "ByQuote"
		orderReq["quoteQtyEv"] = quantity.Mul(decimal.NewFromInt(100000000)).String() // Scale by 1e8
		orderReq["baseQtyEv"] = "0"
	} else {
		// Market Sell, Limit Buy/Sell: Use base quantity (crypto amount)
		// quantity represents the crypto amount
		orderReq["qtyType"] = "ByBase"
		orderReq["baseQtyEv"] = quantity.Mul(decimal.NewFromInt(100000000)).String() // Scale by 1e8
		orderReq["quoteQtyEv"] = "0"
	}
	
	// Set price for limit orders (scaled by 1e8)
	if orderType == "Limit" {
		orderReq["priceEp"] = price.Mul(decimal.NewFromInt(100000000)).String()
	}
	
	// Add time in force
	orderReq["timeInForce"] = c.mapTimeInForce(tif)
	
	orderBody, err := json.Marshal(orderReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}
	
	// Make REST API call using correct spot endpoint
	resp, err := c.makeRestRequest("POST", "/spot/orders", orderBody)
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
	// WebSocket 우선 시도
	if wsResp, err := c.tryWebSocketCancel(symbol, orderId); err == nil {
		return wsResp, nil
	}
	
	// REST API fallback - Phemex spot에서 WebSocket cancel이 지원되지 않아 REST 사용
	cancelReq := map[string]interface{}{
		"symbol":  symbol,
		"orderID": orderId,
	}
	
	cancelBody, err := json.Marshal(cancelReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cancel request: %w", err)
	}
	
	resp, err := c.makeRestRequest("DELETE", "/spot/orders", cancelBody)
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
	// WebSocket 우선 시도
	if err := c.tryWebSocketCancelAll(symbol); err == nil {
		return nil
	}
	
	// REST API fallback - Phemex spot에서 WebSocket cancelAll이 지원되지 않아 REST 사용
	cancelAllReq := map[string]interface{}{
		"symbol": symbol,
	}
	
	cancelAllBody, err := json.Marshal(cancelAllReq)
	if err != nil {
		return fmt.Errorf("failed to marshal cancel all request: %w", err)
	}
	
	resp, err := c.makeRestRequest("DELETE", "/spot/orders/all", cancelAllBody)
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