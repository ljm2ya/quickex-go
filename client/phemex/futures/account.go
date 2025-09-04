package futures

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/ljm2ya/quickex-go/client/phemex"
	"github.com/ljm2ya/quickex-go/core"
)

func (c *PhemexFuturesClient) Connect(ctx context.Context) (int64, error) {
	return c.WsClient.Connect(ctx)
}

func (c *PhemexFuturesClient) Close() error {
	return c.WsClient.Close()
}

func (c *PhemexFuturesClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		return c.getFuturesPositionSize(asset)
	}
	
	c.balancesMu.RLock()
	defer c.balancesMu.RUnlock()
	
	if wallet, exists := c.balances[asset]; exists {
		if includeLocked {
			return wallet.Total, nil
		}
		return wallet.Free, nil
	}
	
	// If not found in cache, try to fetch from server
	return c.fetchBalanceFromServer(asset, includeLocked)
}

func (c *PhemexFuturesClient) getFuturesPositionSize(asset string) (decimal.Decimal, error) {
	c.positionsMu.RLock()
	defer c.positionsMu.RUnlock()
	
	totalValue := decimal.Zero
	
	// Sum all position values for the given asset
	for _, position := range c.positions {
		if position.Currency == asset && position.PositionStatus != "Closed" {
			positionValue := phemex.FromEp(position.ValueEv, 8)
			totalValue = totalValue.Add(positionValue.Abs()) // Use absolute value
		}
	}
	
	if totalValue.IsZero() {
		// If no positions found, try to fetch from server
		return c.fetchPositionFromServer(asset)
	}
	
	return totalValue, nil
}

func (c *PhemexFuturesClient) fetchBalanceFromServer(asset string, includeLocked bool) (decimal.Decimal, error) {
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "account.query",
		"params": map[string]interface{}{
			"currency": asset,
		},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to query account balance: %w", err)
	}
	
	var result struct {
		Accounts []phemex.PhemexAccount `json:"accounts"`
	}
	
	if err := json.Unmarshal(response, &result); err != nil {
		return decimal.Zero, fmt.Errorf("failed to unmarshal account response: %w", err)
	}
	
	for _, account := range result.Accounts {
		if account.Currency == asset {
			balance := phemex.FromEp(account.BalanceEv, 8)
			usedBalance := phemex.FromEp(account.TotalUsedBalanceEv, 8)
			
			// Update cache
			c.balancesMu.Lock()
			c.balances[asset] = &core.Wallet{
				Asset:  asset,
				Free:   balance.Sub(usedBalance),
				Locked: usedBalance,
				Total:  balance,
			}
			c.balancesMu.Unlock()
			
			if includeLocked {
				return balance, nil
			}
			return balance.Sub(usedBalance), nil
		}
	}
	
	return decimal.Zero, fmt.Errorf("asset %s not found", asset)
}

func (c *PhemexFuturesClient) fetchPositionFromServer(asset string) (decimal.Decimal, error) {
	msg := phemex.PhemexWSMessage{
		ID:     nextWSID(),
		Method: "position.query",
		Params: map[string]interface{}{
			"currency": asset,
		},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to query positions: %w", err)
	}
	
	var result struct {
		Positions []phemex.PhemexPosition `json:"positions"`
	}
	
	if err := json.Unmarshal(response, &result); err != nil {
		return decimal.Zero, fmt.Errorf("failed to unmarshal position response: %w", err)
	}
	
	totalValue := decimal.Zero
	
	c.positionsMu.Lock()
	for _, position := range result.Positions {
		if position.Currency == asset && position.PositionStatus != "Closed" {
			c.positions[position.Symbol] = &position
			positionValue := phemex.FromEp(position.ValueEv, 8)
			totalValue = totalValue.Add(positionValue.Abs())
		}
	}
	c.positionsMu.Unlock()
	
	return totalValue, nil
}

func (c *PhemexFuturesClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	c.ordersMu.RLock()
	if order, exists := c.orders[orderId]; exists {
		c.ordersMu.RUnlock()
		
		// Convert to full response (for now, just copy basic fields)
		return &core.OrderResponseFull{
			OrderResponse: *order,
			AvgPrice:      order.Price, // Simplified - would need actual average price
			ExecutedQty:   decimal.Zero, // Would need actual executed quantity
			Commission:    decimal.Zero, // Would need commission data
			CommissionAsset: "",
			UpdateTime:    order.CreateTime,
		}, nil
	}
	c.ordersMu.RUnlock()
	
	// If not found in cache, query from server
	return c.fetchOrderFromServer(symbol, orderId)
}

func (c *PhemexFuturesClient) fetchOrderFromServer(symbol, orderId string) (*core.OrderResponseFull, error) {
	msg := phemex.PhemexWSMessage{
		ID:     nextWSID(),
		Method: "order.query",
		Params: map[string]interface{}{
			"symbol":  symbol,
			"orderID": orderId,
		},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to query order: %w", err)
	}
	
	var result struct {
		Orders []phemex.PhemexOrder `json:"orders"`
	}
	
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order response: %w", err)
	}
	
	if len(result.Orders) == 0 {
		return nil, fmt.Errorf("order %s not found", orderId)
	}
	
	order := result.Orders[0]
	status := c.mapOrderStatus(order.OrderStatus)
	
	priceScale := c.getPriceScale(order.Symbol)
	avgPrice := phemex.FromEp(order.AvgPriceEp, priceScale)
	if avgPrice.IsZero() {
		avgPrice = phemex.FromEp(order.PriceEp, priceScale)
	}
	
	orderResp := &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:         order.OrderID,
			Symbol:          order.Symbol,
			Side:            order.Side,
			Status:          status,
			Price:           phemex.FromEp(order.PriceEp, priceScale),
			Quantity:        c.convertQuantity(order.OrderQty, order.Symbol),
			IsQuoteQuantity: false,
			CreateTime:      phemex.ToTimeNs(order.ActionTimeNs),
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     c.convertQuantity(order.CumQty, order.Symbol),
		Commission:      phemex.FromEp(order.CumFeeEv, 8),
		CommissionAsset: c.getBaseCurrency(order.Symbol),
		UpdateTime:      phemex.ToTimeNs(order.TransactTimeNs),
	}
	
	// Update cache
	c.ordersMu.Lock()
	c.orders[orderId] = &orderResp.OrderResponse
	c.ordersMu.Unlock()
	
	return orderResp, nil
}