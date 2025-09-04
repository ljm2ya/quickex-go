package okx

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// LimitBuy implements core.PrivateClient
func (c *OKXClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrderViaWebSocket(symbol, "buy", "limit", quantity.String(), price.String())
}

// LimitSell implements core.PrivateClient
func (c *OKXClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeOrderViaWebSocket(symbol, "sell", "limit", quantity.String(), price.String())
}

// MarketBuy implements core.PrivateClient
func (c *OKXClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	// For market buy orders, use quote quantity (USDT amount) with tgtCcy
	return c.placeMarketOrderViaWebSocket(symbol, "buy", quoteQuantity.String(), true)
}

// MarketSell implements core.PrivateClient  
func (c *OKXClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	// For market sell orders, use base quantity (BTC amount)
	return c.placeMarketOrderViaWebSocket(symbol, "sell", quantity.String(), false)
}


// CancelOrder implements core.PrivateClient
func (c *OKXClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	if err := c.cancelOrderViaWebSocket(symbol, orderId); err != nil {
		return nil, err
	}
	
	// Return minimal response for successful cancellation
	return &core.OrderResponse{
		OrderID: orderId,
		Symbol:  symbol,
		Status:  core.OrderStatusCanceled,
	}, nil
}

// CancelAll implements core.PrivateClient
func (c *OKXClient) CancelAll(symbol string) error {
	if !c.persistentWS.IsConnected() {
		if err := c.persistentWS.Connect(); err != nil {
			return fmt.Errorf("failed to connect persistent WebSocket: %w", err)
		}
	}
	
	req := map[string]interface{}{
		"op": "mass-cancel",
		"args": []map[string]interface{}{
			{
				"instId": symbol,
			},
		},
	}
	
	_, err := c.persistentWS.SendRequest(req)
	if err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}
	
	return nil
}

// placeOrderViaWebSocket places a limit order via persistent WebSocket
func (c *OKXClient) placeOrderViaWebSocket(symbol, side, orderType, size, price string) (*core.OrderResponse, error) {
	if !c.persistentWS.IsConnected() {
		if err := c.persistentWS.Connect(); err != nil {
			return nil, fmt.Errorf("failed to connect persistent WebSocket: %w", err)
		}
	}
	
	orderMsg := map[string]interface{}{
		"op": "order",
		"args": []map[string]interface{}{
			{
				"instId":  symbol,
				"tdMode":  "cash", // Spot trading mode
				"side":    side,
				"ordType": orderType,
				"sz":      size,
			},
		},
	}
	
	// Add price for limit orders
	if orderType == "limit" && price != "" {
		orderMsg["args"].([]map[string]interface{})[0]["px"] = price
	}
	
	response, err := c.persistentWS.SendRequest(orderMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}
	
	return c.parseWSOrderResponse(response)
}

// placeMarketOrderViaWebSocket places a market order via persistent WebSocket
func (c *OKXClient) placeMarketOrderViaWebSocket(symbol, side, size string, isQuoteQuantity bool) (*core.OrderResponse, error) {
	if !c.persistentWS.IsConnected() {
		if err := c.persistentWS.Connect(); err != nil {
			return nil, fmt.Errorf("failed to connect persistent WebSocket: %w", err)
		}
	}
	
	args := map[string]interface{}{
		"instId":  symbol,
		"tdMode":  "cash", // Spot trading mode
		"side":    side,
		"ordType": "market",
		"sz":      size,
	}
	
	// For market buy with quote quantity, set tgtCcy to quote_ccy
	if side == "buy" && isQuoteQuantity {
		args["tgtCcy"] = "quote_ccy"
	}
	
	orderMsg := map[string]interface{}{
		"op":   "order",
		"args": []map[string]interface{}{args},
	}
	
	response, err := c.persistentWS.SendRequest(orderMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to place market order: %w", err)
	}
	
	return c.parseWSOrderResponse(response)
}

// cancelOrderViaWebSocket cancels an order via persistent WebSocket
func (c *OKXClient) cancelOrderViaWebSocket(symbol, orderID string) error {
	if !c.persistentWS.IsConnected() {
		if err := c.persistentWS.Connect(); err != nil {
			return fmt.Errorf("failed to connect persistent WebSocket: %w", err)
		}
	}
	
	cancelMsg := map[string]interface{}{
		"op": "cancel-order",
		"args": []map[string]interface{}{
			{
				"instId": symbol,
				"ordId":  orderID,
			},
		},
	}
	
	_, err := c.persistentWS.SendRequest(cancelMsg)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}
	
	return nil
}

// parseWSOrderResponse parses OKX WebSocket order response into core.OrderResponse
func (c *OKXClient) parseWSOrderResponse(responseBytes []byte) (*core.OrderResponse, error) {
	var response map[string]interface{}
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	// Check for error
	if code, ok := response["code"].(string); ok && code != "0" {
		msg := ""
		if msgVal, ok := response["msg"].(string); ok {
			msg = msgVal
		}
		return nil, fmt.Errorf("order failed: %s - %s", code, msg)
	}
	
	// Extract order data
	data, ok := response["data"].([]interface{})
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("invalid response data")
	}
	
	orderData, ok := data[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid order data")
	}
	
	// Check order status
	if sCode, ok := orderData["sCode"].(string); ok && sCode != "0" {
		sMsg := ""
		if sMsgVal, ok := orderData["sMsg"].(string); ok {
			sMsg = sMsgVal
		}
		return nil, fmt.Errorf("order failed: %s - %s", sCode, sMsg)
	}
	
	// Extract order ID
	orderID, ok := orderData["ordId"].(string)
	if !ok {
		return nil, fmt.Errorf("missing order ID")
	}
	
	return &core.OrderResponse{
		OrderID:    orderID,
		Symbol:     "", // Will be filled by caller if needed
		Status:     core.OrderStatusOpen, // OKX returns order as created
		CreateTime: time.Now(),
	}, nil
}

// mapTimeInForce maps core.TimeInForce to OKX order types
func (c *OKXClient) mapTimeInForce(tif string) string {
	switch tif {
	case "GTC":
		return "limit" // GTC is default for limit orders
	case "IOC":
		return "ioc"
	case "FOK":
		return "fok"
	case "PO", "POST_ONLY":
		return "post_only"
	default:
		return "limit" // Default to limit/GTC
	}
}