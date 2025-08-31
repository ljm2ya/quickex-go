package futures

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/order"
	"github.com/ljm2ya/quickex-go/client/kucoin/common"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *KucoinFuturesClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	orderAPI := futuresService.GetOrderAPI()

	// Use the SDK to fetch order
	req := order.NewGetOrderByOrderIdReqBuilder().
		SetOrderId(orderId).
		Build()

	var resp *order.GetOrderByOrderIdResp
	var err error
	for {
		resp, err = orderAPI.GetOrderByOrderId(req, context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get order: %w", err)
		}
		if resp != nil {
			if resp.Id != "" {
				break
			}
		}
		time.Sleep(time.Millisecond * 150)
	}

	mul, ok := c.multiplierMap[symbol]
	if !ok {
		return nil, fmt.Errorf("no matching symbol or not connected")
	}
	// Parse decimal values from response fields directly
	price, _ := decimal.NewFromString(resp.Price)
	quantity := decimal.NewFromInt(int64(resp.Size))
	executedQty := decimal.NewFromInt(int64(resp.DealSize)).Mul(mul)
	avgPrice, _ := decimal.NewFromString(resp.AvgDealPrice)
	fee := decimal.Zero // Futures doesn't return fee in this response

	// Convert timestamps
	createTime := time.Unix(resp.CreatedAt/1000, 0)
	updateTime := time.Unix(resp.UpdatedAt/1000, 0)

	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:    resp.Id,
			Symbol:     resp.Symbol,
			Side:       strings.ToUpper(resp.Side),
			Status:     mapOrderStatus(resp),
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

	mul, on := c.multiplierMap[symbol]
	if !on {
		return nil, fmt.Errorf("failed to get lot of order symbol: check initial connection")
	}
	lotQty := quantity.DivRound(mul, 0)
	if lotQty.IsZero() {
		return nil, fmt.Errorf("order failed: quantity too small: %s", quantity.String())
	}

	// Create WebSocket order request
	wsReq := &OrderWSRequest{
		ClientOid:   clientOid,
		Side:        strings.ToLower(side),
		Symbol:      symbol,
		Type:        "limit",
		Size:        lotQty.String(),
		Price:       price.String(),
		TimeInForce: mapTifToKucoin(tif),
		MarginMode:  "CROSS",
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

func (c *KucoinFuturesClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	// Check if private WebSocket is connected
	if c.privateWS == nil || !c.privateWS.IsConnected() {
		return nil, fmt.Errorf("private WebSocket not connected, please call Connect() first")
	}

	clientOid := fmt.Sprintf("quickex-futures-%d", time.Now().UnixNano())

	lotQty := quoteQuantity.RoundDown(0).String()
	// Create WebSocket order request for market buy
	// Note: In futures, market orders use size (quantity) for both buy and sell
	wsReq := &OrderWSRequest{
		ClientOid:  clientOid,
		Side:       "buy",
		Symbol:     symbol,
		Type:       "market",
		ValueQty:   lotQty,
		MarginMode: "CROSS",
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
		OrderID:         resp.OrderID,
		Symbol:          symbol,
		Side:            "BUY",
		Status:          core.OrderStatusOpen,
		Quantity:        quoteQuantity,
		IsQuoteQuantity: true,
		CreateTime:      time.Now(),
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
		ClientOid:  clientOid,
		Side:       "sell",
		Symbol:     symbol,
		Type:       "market",
		Qty:        quantity.String(),
		MarginMode: "CROSS",
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

func mapOrderStatus(order *order.GetOrderByOrderIdResp) core.OrderStatus {
	if order.Status == "open" {
		return core.OrderStatusOpen
	} else {
		if order.CancelExist {
			return core.OrderStatusCanceled
		} else {
			if order.DealSize != 0 {
				return core.OrderStatusFilled
			}
			return core.OrderStatusError
		}
	}
}

// Use common utility function
var mapTifToKucoin = common.MapTifToKucoin
