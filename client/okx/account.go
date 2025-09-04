package okx

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// Connect implements core.PrivateClient
func (c *OKXClient) Connect(ctx context.Context) (int64, error) {
	return c.WsClient.Connect(ctx)
}

// Close implements core.PrivateClient - closes both regular and persistent WebSocket connections
func (c *OKXClient) Close() error {
	var err error
	
	// Close persistent WebSocket
	if c.persistentWS != nil {
		if wsErr := c.persistentWS.Close(); wsErr != nil {
			err = wsErr
		}
	}
	
	// Close regular WebSocket client
	if c.WsClient != nil {
		if wsErr := c.WsClient.Close(); wsErr != nil {
			if err == nil {
				err = wsErr
			}
		}
	}
	
	return err
}

// FetchBalance implements core.PrivateClient
func (c *OKXClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		// For futures positions, we need to get position size instead of balance
		return c.getFuturesPositionSize(asset)
	}
	
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
func (c *OKXClient) getFuturesPositionSize(asset string) (decimal.Decimal, error) {
	// Get positions via WebSocket request
	req := map[string]interface{}{
		"id":     nextWSID(),
		"op":     "batch-positions",
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
		Data []OKXPosition `json:"data"`
	}
	
	if dataRaw, ok := root["data"]; ok {
		if err := json.Unmarshal(dataRaw, &response.Data); err != nil {
			return decimal.Zero, fmt.Errorf("failed to unmarshal positions: %w", err)
		}
	}
	
	totalPosition := decimal.Zero
	for _, pos := range response.Data {
		if pos.InstID == asset || extractBaseCurrency(pos.InstID) == asset {
			posSize := ToDecimal(pos.Pos)
			if posSize.IsPositive() {
				totalPosition = totalPosition.Add(posSize)
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
func (c *OKXClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	req := map[string]interface{}{
		"id":     nextWSID(),
		"op":     "batch-orders",
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
		Data []OKXOrder `json:"data"`
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
			Price:           ToDecimal(order.Px),
			Quantity:        ToDecimal(order.Sz),
			IsQuoteQuantity: false,
			CreateTime:      ToTime(order.CTime),
		},
		AvgPrice:        ToDecimal(order.AvgPx),
		ExecutedQty:     ToDecimal(order.AccFillSz),
		Commission:      ToDecimal(order.Fee),
		CommissionAsset: order.FeeCcy,
		UpdateTime:      ToTime(order.UTime),
	}, nil
}