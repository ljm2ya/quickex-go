package futures

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ljm2ya/quickex-go/client/okx"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// Connect implements core.PrivateClient
func (c *OKXFuturesClient) Connect(ctx context.Context) (int64, error) {
	return c.WsClient.Connect(ctx)
}

// Close implements core.PrivateClient
func (c *OKXFuturesClient) Close() error {
	return c.WsClient.Close()
}

// FetchBalance implements core.PrivateClient
func (c *OKXFuturesClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		// For futures, return total position size
		return c.getFuturesPositionSize(asset)
	}
	
	// Return account balance (margin available for trading)
	c.balancesMu.RLock()
	defer c.balancesMu.RUnlock()
	
	wallet, exists := c.balances[asset]
	if !exists {
		return decimal.Zero, fmt.Errorf("balance not found for asset: %s", asset)
	}
	
	if includeLocked {
		return wallet.Total, nil
	}
	return wallet.Free, nil
}

// getFuturesPositionSize gets the total position size for futures trading
func (c *OKXFuturesClient) getFuturesPositionSize(asset string) (decimal.Decimal, error) {
	c.positionsMu.RLock()
	defer c.positionsMu.RUnlock()
	
	totalPosition := decimal.Zero
	
	// Sum up all position sizes for the given asset
	for instID, position := range c.positions {
		if extractBaseCurrency(instID) == asset {
			posAmount := decimal.NewFromFloat(position.Amount)
			if posAmount.IsPositive() {
				totalPosition = totalPosition.Add(posAmount.Abs())
			}
		}
	}
	
	// If no positions found in cache, fetch from server
	if totalPosition.IsZero() {
		return c.fetchPositionFromServer(asset)
	}
	
	return totalPosition, nil
}

// fetchPositionFromServer fetches position data from server via WebSocket
func (c *OKXFuturesClient) fetchPositionFromServer(asset string) (decimal.Decimal, error) {
	req := map[string]interface{}{
		"id": nextWSID(),
		"op": "batch-positions",
		"args": []map[string]string{
			{
				"instType": "SWAP", // Perpetual futures
			},
		},
	}
	
	root, err := c.WsClient.SendRequest(req)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get positions: %w", err)
	}
	
	var response struct {
		Data []okx.OKXPosition `json:"data"`
	}
	
	if dataRaw, ok := root["data"]; ok {
		if err := json.Unmarshal(dataRaw, &response.Data); err != nil {
			return decimal.Zero, fmt.Errorf("failed to unmarshal positions: %w", err)
		}
	}
	
	totalPosition := decimal.Zero
	for _, pos := range response.Data {
		if extractBaseCurrency(pos.InstID) == asset {
			posSize := okx.ToDecimal(pos.Pos)
			if posSize.IsPositive() {
				totalPosition = totalPosition.Add(posSize.Abs())
			}
		}
	}
	
	return totalPosition, nil
}

// extractBaseCurrency extracts base currency from instrument ID (e.g., BTC-USDT-SWAP -> BTC)
func extractBaseCurrency(instID string) string {
	for i, char := range instID {
		if char == '-' {
			return instID[:i]
		}
	}
	return instID
}

// FetchOrder implements core.PrivateClient
func (c *OKXFuturesClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	req := map[string]interface{}{
		"id": nextWSID(),
		"op": "batch-orders",
		"args": []map[string]string{
			{
				"instId": symbol,
				"ordId":  orderId,
			},
		},
	}
	
	root, err := c.WsClient.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order: %w", err)
	}
	
	var response struct {
		Data []okx.OKXOrder `json:"data"`
	}
	
	if dataRaw, ok := root["data"]; ok {
		if err := json.Unmarshal(dataRaw, &response.Data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal order: %w", err)
		}
	}
	
	if len(response.Data) == 0 {
		return nil, fmt.Errorf("order not found: %s", orderId)
	}
	
	order := response.Data[0]
	status := c.mapOrderStatus(order.State)
	
	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:         order.OrdID,
			Symbol:          order.InstID,
			Side:            order.Side,
			Status:          status,
			Price:           okx.ToDecimal(order.Px),
			Quantity:        okx.ToDecimal(order.Sz),
			IsQuoteQuantity: false,
			CreateTime:      okx.ToTime(order.CTime),
		},
		AvgPrice:        okx.ToDecimal(order.AvgPx),
		ExecutedQty:     okx.ToDecimal(order.AccFillSz),
		Commission:      okx.ToDecimal(order.Fee),
		CommissionAsset: order.FeeCcy,
		UpdateTime:      okx.ToTime(order.UTime),
	}, nil
}