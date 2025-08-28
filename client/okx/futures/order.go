package futures

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ljm2ya/quickex-go/client/okx"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// LimitBuy implements core.PrivateClient
func (c *OKXFuturesClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeFuturesOrder(symbol, "buy", "limit", quantity, price, tif)
}

// LimitSell implements core.PrivateClient
func (c *OKXFuturesClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeFuturesOrder(symbol, "sell", "limit", quantity, price, tif)
}

// MarketBuy implements core.PrivateClient
func (c *OKXFuturesClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	// For futures market orders, we use base quantity, not quote quantity
	// Convert quote quantity to base quantity using current market price if needed
	return c.placeFuturesMarketOrder(symbol, "buy", quoteQuantity)
}

// MarketSell implements core.PrivateClient  
func (c *OKXFuturesClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	return c.placeFuturesMarketOrder(symbol, "sell", quantity)
}

// placeFuturesOrder places a futures limit order
func (c *OKXFuturesClient) placeFuturesOrder(symbol, side, ordType string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	// Map time in force
	okxTif := c.mapTimeInForce(tif)
	
	// Determine trading mode and position side
	tdMode := "cross" // Default to cross margin for futures
	posSide := "net"  // Default to net position mode
	
	req := map[string]interface{}{
		"id": nextWSID(),
		"op": "order",
		"args": []map[string]interface{}{
			{
				"instId":  symbol,
				"tdMode":  tdMode,
				"side":    side,
				"posSide": posSide,
				"ordType": okxTif,
				"sz":      quantity.String(),
				"px":      price.String(),
			},
		},
	}
	
	// For limit orders, we need to set ordType to "limit" and add timeInForce
	if ordType == "limit" {
		args := req["args"].([]map[string]interface{})
		args[0]["ordType"] = "limit"
		// Only add timeInForce for limit orders if it's not GTC
		if okxTif != "limit" {
			args[0]["ordType"] = okxTif
		}
	}
	
	root, err := c.WsClient.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to place futures order: %w", err)
	}
	
	return c.parseOrderResponse(root, symbol, side, quantity, price)
}

// placeFuturesMarketOrder places a futures market order
func (c *OKXFuturesClient) placeFuturesMarketOrder(symbol, side string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	// Determine trading mode and position side
	tdMode := "cross" // Default to cross margin for futures
	posSide := "net"  // Default to net position mode
	
	req := map[string]interface{}{
		"id": nextWSID(),
		"op": "order",
		"args": []map[string]interface{}{
			{
				"instId":  symbol,
				"tdMode":  tdMode,
				"side":    side,
				"posSide": posSide,
				"ordType": "market",
				"sz":      quantity.String(),
			},
		},
	}
	
	root, err := c.WsClient.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to place futures market order: %w", err)
	}
	
	return c.parseOrderResponse(root, symbol, side, quantity, decimal.Zero)
}

// parseOrderResponse parses the order placement response
func (c *OKXFuturesClient) parseOrderResponse(root map[string]json.RawMessage, symbol, side string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	var response struct {
		Data []struct {
			OrdID string `json:"ordId"`
			SCode string `json:"sCode"`
			SMsg  string `json:"sMsg"`
		} `json:"data"`
	}
	
	if dataRaw, ok := root["data"]; ok {
		if err := json.Unmarshal(dataRaw, &response.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal order response: %w", err)
		}
	}
	
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("empty order response")
	}
	
	orderData := response.Data[0]
	
	// Check for order-specific errors
	if orderData.SCode != "" && orderData.SCode != "0" {
		return nil, okx.ParseOKXError(orderData.SCode, orderData.SMsg)
	}
	
	if orderData.OrdID == "" {
		return nil, fmt.Errorf("order ID not returned")
	}
	
	return &core.OrderResponse{
		OrderID:         orderData.OrdID,
		Symbol:          symbol,
		Side:            side,
		Status:          core.OrderStatusOpen, // New orders start as open
		Price:           price,
		Quantity:        quantity,
		IsQuoteQuantity: false, // Futures orders use base quantity
		CreateTime:      time.Now(),
	}, nil
}

// CancelOrder implements core.PrivateClient
func (c *OKXFuturesClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	req := map[string]interface{}{
		"id": nextWSID(),
		"op": "cancel-order",
		"args": []map[string]interface{}{
			{
				"instId": symbol,
				"ordId":  orderId,
			},
		},
	}
	
	root, err := c.WsClient.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel futures order: %w", err)
	}
	
	var response struct {
		Data []struct {
			OrdID string `json:"ordId"`
			SCode string `json:"sCode"`
			SMsg  string `json:"sMsg"`
		} `json:"data"`
	}
	
	if dataRaw, ok := root["data"]; ok {
		if err := json.Unmarshal(dataRaw, &response.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cancel response: %w", err)
		}
	}
	
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("empty cancel response")
	}
	
	orderData := response.Data[0]
	
	// Check for cancellation-specific errors
	if orderData.SCode != "" && orderData.SCode != "0" {
		return nil, okx.ParseOKXError(orderData.SCode, orderData.SMsg)
	}
	
	// Get the order details from our local cache or fetch from server
	c.ordersMu.RLock()
	cachedOrder, exists := c.orders[orderId]
	c.ordersMu.RUnlock()
	
	if exists {
		// Return cached order with updated status
		return &core.OrderResponse{
			OrderID:         cachedOrder.OrderID,
			Symbol:          cachedOrder.Symbol,
			Side:            cachedOrder.Side,
			Status:          core.OrderStatusCanceled,
			Price:           cachedOrder.Price,
			Quantity:        cachedOrder.Quantity,
			IsQuoteQuantity: cachedOrder.IsQuoteQuantity,
			CreateTime:      cachedOrder.CreateTime,
		}, nil
	}
	
	// If not in cache, return minimal response
	return &core.OrderResponse{
		OrderID: orderId,
		Symbol:  symbol,
		Status:  core.OrderStatusCanceled,
	}, nil
}

// CancelAll implements core.PrivateClient
func (c *OKXFuturesClient) CancelAll(symbol string) error {
	req := map[string]interface{}{
		"id": nextWSID(),
		"op": "mass-cancel",
		"args": []map[string]interface{}{
			{
				"instId": symbol,
			},
		},
	}
	
	_, err := c.WsClient.SendRequest(req)
	if err != nil {
		return fmt.Errorf("failed to cancel all futures orders: %w", err)
	}
	
	return nil
}

// mapTimeInForce maps core.TimeInForce to OKX order types
func (c *OKXFuturesClient) mapTimeInForce(tif string) string {
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