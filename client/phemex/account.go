package phemex

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
	"github.com/ljm2ya/quickex-go/core"
)

func (c *PhemexClient) Connect(ctx context.Context) (int64, error) {
	return c.WsClient.Connect(ctx)
}

func (c *PhemexClient) Close() error {
	return c.WsClient.Close()
}

func (c *PhemexClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
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

func (c *PhemexClient) getFuturesPositionSize(asset string) (decimal.Decimal, error) {
	c.positionsMu.RLock()
	defer c.positionsMu.RUnlock()
	
	totalValue := decimal.Zero
	
	// Sum all position values for the given asset
	for _, position := range c.positions {
		if position.Currency == asset && position.PositionStatus != "Closed" {
			positionValue := FromEp(position.ValueEv, 8)
			totalValue = totalValue.Add(positionValue.Abs()) // Use absolute value
		}
	}
	
	if totalValue.IsZero() {
		// If no positions found, try to fetch from server
		return c.fetchPositionFromServer(asset)
	}
	
	return totalValue, nil
}

func (c *PhemexClient) fetchBalanceFromServer(asset string, includeLocked bool) (decimal.Decimal, error) {
	// Try WebSocket first, then fallback to REST if WebSocket fails
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "account.query",
		"params": map[string]interface{}{
			"currency": asset,
		},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		// Fallback to REST API
		return c.fetchBalanceFromRest(asset, includeLocked)
	}
	
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to marshal response: %w", err)
	}
	
	var result struct {
		Accounts []PhemexAccount `json:"accounts"`
	}
	
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return decimal.Zero, fmt.Errorf("failed to unmarshal account response: %w", err)
	}
	
	for _, account := range result.Accounts {
		if account.Currency == asset {
			balance := FromEp(account.BalanceEv, 8)
			usedBalance := FromEp(account.TotalUsedBalanceEv, 8)
			
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

func (c *PhemexClient) fetchPositionFromServer(asset string) (decimal.Decimal, error) {
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "position.query",
		"params": map[string]interface{}{
			"currency": asset,
		},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to query positions: %w", err)
	}
	
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to marshal response: %w", err)
	}
	
	var result struct {
		Positions []PhemexPosition `json:"positions"`
	}
	
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return decimal.Zero, fmt.Errorf("failed to unmarshal position response: %w", err)
	}
	
	totalValue := decimal.Zero
	
	c.positionsMu.Lock()
	for _, position := range result.Positions {
		if position.Currency == asset && position.PositionStatus != "Closed" {
			c.positions[position.Symbol] = &position
			positionValue := FromEp(position.ValueEv, 8)
			totalValue = totalValue.Add(positionValue.Abs())
		}
	}
	c.positionsMu.Unlock()
	
	return totalValue, nil
}

func (c *PhemexClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
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

func (c *PhemexClient) fetchOrderFromServer(symbol, orderId string) (*core.OrderResponseFull, error) {
	msg := map[string]interface{}{
		"id":     nextWSID(),
		"method": "order.query",
		"params": map[string]interface{}{
			"symbol":  symbol,
			"orderID": orderId,
		},
	}
	
	response, err := c.WsClient.SendRequest(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to query order: %w", err)
	}
	
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	
	var result struct {
		Orders []PhemexOrder `json:"orders"`
	}
	
	if err := json.Unmarshal(responseBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal order response: %w", err)
	}
	
	if len(result.Orders) == 0 {
		return nil, fmt.Errorf("order %s not found", orderId)
	}
	
	order := result.Orders[0]
	status := c.mapOrderStatus(order.OrderStatus)
	
	priceScale := c.getPriceScale(order.Symbol)
	avgPrice := FromEp(order.AvgPriceEp, priceScale)
	if avgPrice.IsZero() {
		avgPrice = FromEp(order.PriceEp, priceScale)
	}
	
	orderResp := &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:         order.OrderID,
			Symbol:          order.Symbol,
			Side:            order.Side,
			Status:          status,
			Price:           FromEp(order.PriceEp, priceScale),
			Quantity:        c.convertQuantity(order.OrderQty, order.Symbol),
			IsQuoteQuantity: false,
			CreateTime:      ToTimeNs(order.ActionTimeNs),
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     c.convertQuantity(order.CumQty, order.Symbol),
		Commission:      FromEp(order.CumFeeEv, 8),
		CommissionAsset: c.getBaseCurrency(order.Symbol),
		UpdateTime:      ToTimeNs(order.TransactTimeNs),
	}
	
	// Update cache
	c.ordersMu.Lock()
	c.orders[orderId] = &orderResp.OrderResponse
	c.ordersMu.Unlock()
	
	return orderResp, nil
}

func (c *PhemexClient) getBaseCurrency(symbol string) string {
	c.symbolsMu.RLock()
	defer c.symbolsMu.RUnlock()
	
	if symbolInfo, exists := c.symbols[symbol]; exists {
		return symbolInfo.BaseCurrency
	}
	return "USDT" // Default fallback
}

func (c *PhemexClient) fetchBalanceFromRest(asset string, includeLocked bool) (decimal.Decimal, error) {
	path := fmt.Sprintf("/accounts/accountPositions?currency=%s", asset)
	
	resp, err := c.makeRestRequest("GET", path, nil)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to fetch balance via REST: %w", err)
	}
	
	if resp.Code != 0 {
		return decimal.Zero, fmt.Errorf("balance request failed: Code=%d, Msg=%s", resp.Code, resp.Msg)
	}
	
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		return decimal.Zero, fmt.Errorf("invalid balance response format")
	}
	
	account, exists := dataMap["account"]
	if !exists {
		return decimal.Zero, fmt.Errorf("account data not found")
	}
	
	accountMap, ok := account.(map[string]interface{})
	if !ok {
		return decimal.Zero, fmt.Errorf("invalid account format")
	}
	
	balanceEv, exists := accountMap["accountBalanceEv"]
	if !exists {
		return decimal.Zero, fmt.Errorf("balance not found")
	}
	
	balanceEvFloat, ok := balanceEv.(float64)
	if !ok {
		return decimal.Zero, fmt.Errorf("invalid balance format")
	}
	
	balance := FromEp(int64(balanceEvFloat), 8)
	
	// For simplicity, assume all balance is free (could parse totalUsedBalanceEv for locked amount)
	c.balancesMu.Lock()
	c.balances[asset] = &core.Wallet{
		Asset: asset,
		Free:  balance,
		Locked: decimal.Zero,
		Total: balance,
	}
	c.balancesMu.Unlock()
	
	return balance, nil
}