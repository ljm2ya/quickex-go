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
	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}

	var side string
	if order.Side == "bid" {
		side = "BUY"
	} else {
		side = "SELL"
	}

	price, _ := decimal.NewFromString(order.Price)
	quantity, _ := decimal.NewFromString(order.Volume)
	executedQty, _ := decimal.NewFromString(order.ExecutedVolume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)

	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:    order.UUID,
			Symbol:     order.Market,
			Side:       side,
			Tif:        core.TimeInForceGTC,
			Status:     status,
			Price:      price,
			Quantity:   quantity,
			CreateTime: createdAt,
		},
		AvgPrice:    price, // Upbit doesn't provide avg price
		ExecutedQty: executedQty,
		UpdateTime:  createdAt, // Upbit doesn't provide update time
	}, nil
}

// LimitBuy implements core.PrivateClient interface
func (u *UpbitClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "bid",
		"ord_type":   "limit",
		"price":      price.String(),
		"volume":     quantity.String(),
		"identifier": uuid.New().String(),
	}

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}

	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)

	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "BUY",
		Tif:        core.TimeInForce(tif),
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// LimitSell implements core.PrivateClient interface
func (u *UpbitClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "ask",
		"ord_type":   "limit",
		"price":      price.String(),
		"volume":     quantity.String(),
		"identifier": uuid.New().String(),
	}

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}

	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)

	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "SELL",
		Tif:        core.TimeInForce(tif),
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// MarketBuy implements core.PrivateClient interface
func (u *UpbitClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "bid",
		"ord_type":   "price",
		"price":      quoteQuantity.String(),
		"identifier": uuid.New().String(),
	}

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}

	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)

	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "BUY",
		Tif:        core.TimeInForceGTC,
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
}

// MarketSell implements core.PrivateClient interface
func (u *UpbitClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	params := map[string]string{
		"market":     symbol,
		"side":       "ask",
		"ord_type":   "market",
		"volume":     quantity.String(),
		"identifier": uuid.New().String(),
	}

	body, err := u.makeRequest("POST", "/v1/orders", params)
	if err != nil {
		return nil, err
	}

	var order UpbitOrder
	if err := json.Unmarshal(body, &order); err != nil {
		return nil, err
	}

	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}

	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)

	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       "SELL",
		Tif:        core.TimeInForceGTC,
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
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

	var status core.OrderStatus
	switch order.State {
	case "wait":
		status = core.OrderStatusOpen
	case "done":
		status = core.OrderStatusFilled
	case "cancel":
		status = core.OrderStatusCanceled
	default:
		status = core.OrderStatusOpen
	}

	var side string
	if order.Side == "bid" {
		side = "BUY"
	} else {
		side = "SELL"
	}

	orderPrice, _ := decimal.NewFromString(order.Price)
	orderQuantity, _ := decimal.NewFromString(order.Volume)
	createdAt, _ := time.Parse(time.RFC3339, order.CreatedAt)

	return &core.OrderResponse{
		OrderID:    order.UUID,
		Symbol:     order.Market,
		Side:       side,
		Tif:        core.TimeInForceGTC,
		Status:     status,
		Price:      orderPrice,
		Quantity:   orderQuantity,
		CreateTime: createdAt,
	}, nil
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
