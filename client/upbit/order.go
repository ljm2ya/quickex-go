package upbit

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// parseOrderStatus converts Upbit order state to core.OrderStatus
func parseOrderStatus(state string) core.OrderStatus {
	switch state {
	case "wait", "watch":
		return core.OrderStatusOpen
	case "done":
		return core.OrderStatusFilled
	case "cancel":
		return core.OrderStatusCanceled
	default:
		return core.OrderStatusOpen
	}
}

// createOrderParams creates common order parameters
func createOrderParams(symbol, side, ordType string, identifier string) map[string]string {
	params := map[string]string{
		"market":     symbol,
		"side":       side,
		"ord_type":   ordType,
		"identifier": identifier,
	}
	return params
}

// parseOrderResponse converts UpbitOrder to core.OrderResponse
func parseOrderResponse(order UpbitOrder, tif string) *core.OrderResponse {
	var side string
	if order.Side == "bid" {
		side = "BUY"
	} else {
		side = "SELL"
	}

	status := parseOrderStatus(order.State)
	price, _ := decimal.NewFromString(order.Price)
	quantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)

	// Determine IsQuoteQuantity for market orders
	isQuoteQuantity := false
	if order.OrdType == "price" {
		isQuoteQuantity = true
	}

	return &core.OrderResponse{
		OrderID:         order.UUID,
		Symbol:          order.Market,
		Side:            side,
		Tif:             core.TimeInForce(tif),
		Status:          status,
		Price:           price,
		Quantity:        quantity,
		CreateTime:      createdAt,
		IsQuoteQuantity: isQuoteQuantity,
	}
}

// FetchOrder implements core.PrivateClient interface
func (u *UpbitClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	params := make(map[string]string)
	if len(orderId) <= 10 { // identifier
		params["identifier"] = orderId
	} else { // uuid
		params["uuid"] = orderId
	}

	body, err := u.makeRequest("GET", "/v1/order", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	// Convert to core types
	// Use helper function for basic order response
	baseResponse := parseOrderResponse(order, string(core.TimeInForceGTC))

	executedQty, _ := decimal.NewFromString(order.ExecutedVolume)
	paidFee, _ := decimal.NewFromString(order.PaidFee)

	// Calculate average price from trades array
	avgPrice := decimal.Zero
	if len(order.Trades) > 0 {
		totalValue := decimal.Zero
		totalVolume := decimal.Zero

		for _, trade := range order.Trades {
			tradePrice, _ := decimal.NewFromString(trade.Price)
			tradeVolume, _ := decimal.NewFromString(trade.Volume)
			totalValue = totalValue.Add(tradePrice.Mul(tradeVolume))
			totalVolume = totalVolume.Add(tradeVolume)
		}

		if totalVolume.IsPositive() {
			avgPrice = totalValue.Div(totalVolume)
		}
	}

	return &core.OrderResponseFull{
		OrderResponse:   *baseResponse,
		AvgPrice:        avgPrice,
		ExecutedQty:     executedQty,
		UpdateTime:      baseResponse.CreateTime, // Upbit doesn't provide separate update time
		Commission:      paidFee,
		CommissionAsset: "KRW", // Upbit doesn't specify commission asset
	}, nil
}

// LimitBuy implements core.PrivateClient interface
func (u *UpbitClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	// Upbit doesn't support traditional TIF values, but we can simulate some behaviors
	// For IOC and PostOnly, we'll just use regular limit orders since Upbit doesn't support these
	params := createOrderParams(symbol, "bid", "limit", uuid.New().String())
	params["price"] = price.String()
	params["volume"] = quantity.String()

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	return parseOrderResponse(order, tif), nil
}

// LimitSell implements core.PrivateClient interface
func (u *UpbitClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	params := createOrderParams(symbol, "ask", "limit", uuid.New().String())
	params["price"] = price.String()
	params["volume"] = quantity.String()

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	return parseOrderResponse(order, tif), nil
}

// MarketBuy implements core.PrivateClient interface
func (u *UpbitClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	params := createOrderParams(symbol, "bid", "price", uuid.New().String())
	params["price"] = quoteQuantity.String() // For market buy, price field contains quote amount

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	// parseOrderResponse will set IsQuoteQuantity correctly based on ord_type
	return parseOrderResponse(order, string(core.TimeInForceGTC)), nil
}

// MarketSell implements core.PrivateClient interface
func (u *UpbitClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	params := createOrderParams(symbol, "ask", "market", uuid.New().String())
	params["volume"] = quantity.String()

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	return parseOrderResponse(order, string(core.TimeInForceGTC)), nil
}

// StopLossSell implements core.PrivateClient interface
func (u *UpbitClient) StopLossSell(symbol string, quantity, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	return nil, errors.New("upbit does not support stop-loss orders")
}

// TakeProfitSell implements core.PrivateClient interface
func (u *UpbitClient) TakeProfitSell(symbol string, quantity, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	return nil, errors.New("upbit does not support take-profit orders")
}

// CancelOrder implements core.PrivateClient interface
func (u *UpbitClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	params := make(map[string]string)
	if len(orderId) <= 10 { // identifier
		params["identifier"] = orderId
	} else { // uuid
		params["uuid"] = orderId
	}

	body, err := u.makeRequest("DELETE", "/v1/order", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	return parseOrderResponse(order, string(core.TimeInForceGTC)), nil
}

// CancelAll implements core.PrivateClient interface
func (u *UpbitClient) CancelAll(symbol string) error {
	// Get open orders first
	params := map[string]string{"market": symbol}
	body, err := u.makeRequest("GET", "/v1/orders", params)
	if err != nil {
		return err
	}

	var orders []UpbitOrder
	if err := json.Unmarshal(body, &orders); err != nil {
		return err
	}

	// Cancel each order
	for _, order := range orders {
		_, err := u.CancelOrder(symbol, order.UUID)
		if err != nil {
			fmt.Printf("Failed to cancel order %s: %v\n", order.UUID, err)
		}
	}

	return nil
}
