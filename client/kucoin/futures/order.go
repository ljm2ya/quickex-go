package futures

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/order"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/ljm2ya/quickex-go/client/kucoin/common"
	"github.com/shopspring/decimal"
)

func (c *KucoinFuturesClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	ctx := context.Background()
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	orderAPI := futuresService.GetOrderAPI()

	// Use the SDK to fetch order
	request := order.NewGetOrderByOrderIdReqBuilder().
		SetOrderId(orderId).
		Build()

	resp, err := orderAPI.GetOrderByOrderId(request, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch order: %w", err)
	}

	// Parse decimal values from response fields directly
	price, _ := decimal.NewFromString(resp.Price)
	quantity := decimal.NewFromInt(int64(resp.Size))
	executedQty := decimal.NewFromInt(int64(resp.DealSize))
	avgPrice := decimal.Zero
	fee := decimal.Zero // Futures doesn't return fee in this response

	if resp.AvgDealPrice != "" {
		avgPrice, _ = decimal.NewFromString(resp.AvgDealPrice)
	}

	// Convert timestamps
	createTime := time.Unix(resp.CreatedAt/1000, 0)
	updateTime := time.Unix(resp.UpdatedAt/1000, 0)

	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:    resp.Id,
			Symbol:     resp.Symbol,
			Side:       strings.ToUpper(resp.Side),
			Status:     mapKucoinFuturesOrderStatus(resp.Status),
			Price:      price,
			Quantity:   quantity,
			CreateTime: createTime,
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     executedQty,
		Commission:      fee,
		CommissionAsset: resp.SettleCurrency,
		UpdateTime:      updateTime,
	}, nil
}

func (c *KucoinFuturesClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeLimitOrder(symbol, "buy", quantity, price, tif)
}

func (c *KucoinFuturesClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeLimitOrder(symbol, "sell", quantity, price, tif)
}

func (c *KucoinFuturesClient) placeLimitOrder(symbol, side string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	// Check if private WebSocket is connected
	if c.privateWS == nil || !c.privateWS.IsConnected() {
		return nil, fmt.Errorf("private WebSocket not connected, please call Connect() first")
	}

	// Generate unique client order ID
	clientOid := fmt.Sprintf("quickex-futures-%d", time.Now().UnixNano())

	// Create WebSocket order request
	wsReq := &OrderWSRequest{
		ClientOid:   clientOid,
		Side:        side,
		Symbol:      symbol,
		Type:        "limit",
		Size:        quantity.String(),
		Price:       price.String(),
		TimeInForce: mapTifToKucoin(tif),
	}

	// Place order via WebSocket
	resp, err := c.privateWS.PlaceOrder(wsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to place %s order: %w", side, err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("order placement failed: %s", resp.Error)
	}

	return &core.OrderResponse{
		OrderID:    resp.OrderID,
		Symbol:     symbol,
		Side:       strings.ToUpper(side),
		Status:     core.OrderStatusOpen,
		Price:      price,
		Quantity:   quantity,
		CreateTime: time.Now(),
	}, nil
}

func (c *KucoinFuturesClient) MarketBuy(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	// Check if private WebSocket is connected
	if c.privateWS == nil || !c.privateWS.IsConnected() {
		return nil, fmt.Errorf("private WebSocket not connected, please call Connect() first")
	}

	clientOid := fmt.Sprintf("quickex-futures-%d", time.Now().UnixNano())

	// Create WebSocket order request for market buy
	// Note: In futures, market orders use size (quantity) for both buy and sell
	wsReq := &OrderWSRequest{
		ClientOid: clientOid,
		Side:      "buy",
		Symbol:    symbol,
		Type:      "market",
		Size:      quantity.String(),
		Leverage:  "1", // Default leverage
	}

	// Place order via WebSocket
	resp, err := c.privateWS.PlaceOrder(wsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to place market buy order: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("order placement failed: %s", resp.Error)
	}

	return &core.OrderResponse{
		OrderID:    resp.OrderID,
		Symbol:     symbol,
		Side:       "BUY",
		Status:     core.OrderStatusOpen,
		Quantity:   quantity,
		CreateTime: time.Now(),
	}, nil
}

func (c *KucoinFuturesClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	// Check if private WebSocket is connected
	if c.privateWS == nil || !c.privateWS.IsConnected() {
		return nil, fmt.Errorf("private WebSocket not connected, please call Connect() first")
	}

	clientOid := fmt.Sprintf("quickex-futures-%d", time.Now().UnixNano())

	// Create WebSocket order request for market sell
	wsReq := &OrderWSRequest{
		ClientOid: clientOid,
		Side:      "sell",
		Symbol:    symbol,
		Type:      "market",
		Size:      quantity.String(),
		Leverage:  "1", // Default leverage
	}

	// Place order via WebSocket
	resp, err := c.privateWS.PlaceOrder(wsReq)
	if err != nil {
		return nil, fmt.Errorf("failed to place market sell order: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("order placement failed: %s", resp.Error)
	}

	return &core.OrderResponse{
		OrderID:    resp.OrderID,
		Symbol:     symbol,
		Side:       "SELL",
		Status:     core.OrderStatusOpen,
		Quantity:   quantity,
		CreateTime: time.Now(),
	}, nil
}

func (c *KucoinFuturesClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	ctx := context.Background()
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	orderAPI := futuresService.GetOrderAPI()

	// Use the SDK to cancel order
	request := order.NewCancelOrderByIdReqBuilder().
		SetOrderId(orderId).
		Build()

	resp, err := orderAPI.CancelOrderById(request, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	// Futures cancel response only returns cancelled order IDs
	if len(resp.CancelledOrderIds) == 0 {
		return nil, fmt.Errorf("no order was cancelled")
	}

	// Return a basic response with the cancelled order ID
	return &core.OrderResponse{
		OrderID:    resp.CancelledOrderIds[0],
		Symbol:     symbol,
		Status:     core.OrderStatusCanceled,
		CreateTime: time.Now(),
	}, nil
}

func (c *KucoinFuturesClient) CancelAll(symbol string) error {
	ctx := context.Background()
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	orderAPI := futuresService.GetOrderAPI()

	// Build request - symbol is optional for futures
	reqBuilder := order.NewCancelAllOrdersV3ReqBuilder()
	if symbol != "" {
		reqBuilder.SetSymbol(symbol)
	}
	request := reqBuilder.Build()

	// Cancel all orders
	_, err := orderAPI.CancelAllOrdersV3(request, ctx)
	if err != nil {
		return fmt.Errorf("failed to cancel all orders: %w", err)
	}

	return nil
}

// Helper functions

func mapKucoinFuturesOrderStatus(status string) core.OrderStatus {
	switch status {
	case "open", "match":
		return core.OrderStatusOpen
	case "done":
		return core.OrderStatusFilled
	case "cancel":
		return core.OrderStatusCanceled
	default:
		return core.OrderStatusError
	}
}

// Use common utility function
var mapTifToKucoin = common.MapTifToKucoin
