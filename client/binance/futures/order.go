package binance

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

type OrderOptions struct {
	Price                   decimal.Decimal
	Quantity                decimal.Decimal
	QuoteOrderQty           decimal.Decimal
	ReduceOnly              bool
	TimeInForce             string
	NewClientOrderId        string
	NewOrderRespType        string
	StopPrice               decimal.Decimal
	TrailingDelta           int
	IcebergQty              decimal.Decimal
	StrategyId              int64
	StrategyType            int
	SelfTradePreventionMode string
}

// --- ORDER PUBLIC API ---

func (b *BinanceClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, TimeInForce: tif}
	return b.placeOrder(symbol, "BUY", "LIMIT", opts)
}

func (b *BinanceClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, TimeInForce: tif}
	return b.placeOrder(symbol, "SELL", "LIMIT", opts)
}

func (b *BinanceClient) LimitMakerBuy(symbol string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, TimeInForce: string(core.TimeInForceGTC)}
	return b.placeOrder(symbol, "BUY", "LIMIT", opts)
}

func (b *BinanceClient) LimitMakerSell(symbol string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, TimeInForce: string(core.TimeInForceGTC)}
	return b.placeOrder(symbol, "SELL", "LIMIT", opts)
}

func (b *BinanceClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quoteQuantity}
	return b.placeOrder(symbol, "BUY", "MARKET", opts)
}

func (b *BinanceClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity}
	return b.placeOrder(symbol, "SELL", "MARKET", opts)
}

func (b *BinanceClient) StopLossBuy(symbol string, quantity, stopPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: stopPrice}
	return b.placeOrder(symbol, "BUY", "STOP_MARKET", opts)
}

func (b *BinanceClient) StopLossSell(symbol string, quantity, stopPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: stopPrice, ReduceOnly: true}
	return b.placeOrder(symbol, "SELL", "STOP_MARKET", opts)
}

func (b *BinanceClient) StopLossLimitBuy(symbol string, quantity, price, stopPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, StopPrice: stopPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "BUY", "STOP", opts)
}

func (b *BinanceClient) StopLossLimitSell(symbol string, quantity, price, stopPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, StopPrice: stopPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "SELL", "STOP", opts)
}

func (b *BinanceClient) TakeProfitBuy(symbol string, quantity, stopPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: stopPrice}
	return b.placeOrder(symbol, "BUY", "TAKE_PROFIT_MARKET", opts)
}

func (b *BinanceClient) TakeProfitSell(symbol string, quantity, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: triggerPrice}
	return b.placeOrder(symbol, "SELL", "TAKE_PROFIT_MARKET", opts)
}

// for futures delta min 0.1 max 10 where 1 for 1%
func (b *BinanceClient) TakeProfitTrailingSell(symbol string, quantity, triggerPrice decimal.Decimal, delta int) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: triggerPrice, TrailingDelta: delta, ReduceOnly: true}
	return b.placeOrder(symbol, "SELL", "TRAILING_STOP_MARKET", opts)
}

func (b *BinanceClient) TakeProfitLimitBuy(symbol string, quantity, price, stopPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, StopPrice: stopPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "BUY", "TAKE_PROFIT", opts)
}

func (b *BinanceClient) TakeProfitLimitSell(symbol string, quantity, targetPrice, triggerPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: targetPrice, StopPrice: triggerPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "SELL", "TAKE_PROFIT", opts)
}

// --- ORDER CORE LOGIC ---

func (b *BinanceClient) placeOrder(symbol, side, orderType string, opt *OrderOptions) (*core.OrderResponse, error) {
	if err := checkMandatory(orderType, opt); err != nil {
		return nil, err
	}
	params := map[string]interface{}{
		"symbol": symbol, "side": side, "type": orderType, "timestamp": time.Now().UnixMilli(),
	}
	if !opt.Quantity.IsZero() {
		params["quantity"] = opt.Quantity.String()
	}
	if !opt.Price.IsZero() {
		params["price"] = opt.Price.String()
	}
	if !opt.QuoteOrderQty.IsZero() {
		params["quoteOrderQty"] = opt.QuoteOrderQty.String()
	}
	if opt.ReduceOnly != false {
		params["reduceOnly"] = true
	}
	if opt.TimeInForce != "" {
		params["timeInForce"] = opt.TimeInForce
	}
	if opt.NewClientOrderId != "" {
		params["newClientOrderId"] = opt.NewClientOrderId
	}
	if opt.NewOrderRespType != "" {
		params["newOrderRespType"] = opt.NewOrderRespType
	}
	if !opt.StopPrice.IsZero() {
		params["stopPrice"] = opt.StopPrice.String()
	}
	if opt.TrailingDelta != 0 {
		params["callbackRate"] = opt.TrailingDelta
	}
	if !opt.IcebergQty.IsZero() {
		params["icebergQty"] = opt.IcebergQty.String()
	}
	if opt.StrategyId != 0 {
		params["strategyId"] = opt.StrategyId
	}
	if opt.StrategyType != 0 {
		params["strategyType"] = opt.StrategyType
	}
	if opt.SelfTradePreventionMode != "" {
		params["selfTradePreventionMode"] = opt.SelfTradePreventionMode
	}
	if side == "BUY" && b.hedgeMode {
		params["reduceOnly"] = true
	}
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "order.place",
		"params": params,
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}
	var wsResp WsOrderResponse
	rootByte, _ := json.Marshal(root)
	if err := wsResp.UnmarshalJSON(rootByte); err != nil {
		return nil, err
	}
	ord := wsResp.Result

	resp := &core.OrderResponse{
		OrderID:    strconv.FormatInt(ord.OrderID, 10),
		Symbol:     ord.Symbol,
		Side:       ord.Side,
		Tif:        core.TimeInForce(ord.TimeInForce),
		Status:     parseOrderStatus(ord.Status),
		Price:      decimal.RequireFromString(ord.Price),
		CreateTime: time.UnixMilli(ord.TransactTime),
	}
	if ord.OrigQuoteQty != "" {
		resp.IsQuoteQuantity = true
		resp.Quantity = decimal.RequireFromString(ord.OrigQuoteQty)
	} else {
		resp.IsQuoteQuantity = false
		resp.Quantity = decimal.RequireFromString(ord.OrigQty)
	}
	b.ordersMu.Lock()
	b.orders[resp.OrderID] = resp
	b.ordersMu.Unlock()
	return resp, nil
}

func checkMandatory(orderType string, opt *OrderOptions) error {
	switch orderType {
	case "LIMIT":
		if opt.TimeInForce == "" || opt.Price.IsZero() || opt.Quantity.IsZero() {
			return fmt.Errorf("LIMIT order requires TimeInForce, Price, Quantity")
		}
	case "MARKET":
		if opt.Quantity.IsZero() && opt.QuoteOrderQty.IsZero() {
			return fmt.Errorf("MARKET order requires Quantity or QuoteOrderQty")
		}
	case "STOP_MARKET":
		if opt.Quantity.IsZero() || (opt.StopPrice.IsZero() && opt.TrailingDelta == 0) {
			return fmt.Errorf("STOP_LOSS order requires Quantity, StopPrice/TrailingDelta")
		}
	case "STOP":
		if opt.TimeInForce == "" || opt.Price.IsZero() || opt.Quantity.IsZero() || (opt.StopPrice.IsZero() && opt.TrailingDelta == 0) {
			return fmt.Errorf("STOP_LOSS_LIMIT order requires TimeInForce, Price, Quantity, StopPrice/TrailingDelta")
		}
	case "TAKE_PROFIT_MARKET":
		if opt.Quantity.IsZero() || (opt.StopPrice.IsZero() && opt.TrailingDelta == 0) {
			return fmt.Errorf("TAKE_PROFIT order requires Quantity, StopPrice/TrailingDelta")
		}
	case "TAKE_PROFIT":
		if opt.TimeInForce == "" || opt.Price.IsZero() || opt.Quantity.IsZero() || (opt.StopPrice.IsZero() && opt.TrailingDelta == 0) {
			return fmt.Errorf("TAKE_PROFIT_LIMIT order requires TimeInForce, Price, Quantity, StopPrice/TrailingDelta")
		}
	case "TRAILING_STOP_MARKET":
		if opt.Quantity.IsZero() || opt.StopPrice.IsZero() || opt.TrailingDelta == 0 {
			return fmt.Errorf("TRAILING_STOP_MARKET order requires Quantity, StopPrice/TrailingDelta")
		}
	default:
		return fmt.Errorf("unsupported orderType: %s", orderType)
	}
	return nil
}

func parseOrderStatus(s string) core.OrderStatus {
	switch s {
	case "NEW":
		return core.OrderStatusOpen
	case "PARTIALLY_FILLED":
		return core.OrderStatusOpen
	case "FILLED":
		return core.OrderStatusFilled
	case "CANCELED":
		return core.OrderStatusCanceled
	case "PENDING_CANCEL":
		return core.OrderStatusOpen
	case "REJECTED":
		return core.OrderStatusError
	case "EXPIRED", "EXPIRED_IN_MATCH":
		return core.OrderStatusError
	default:
		return core.OrderStatusError
	}
}

func toOrderResponse(ord wsOrderTradeUpdate) *core.OrderResponse {
	return &core.OrderResponse{
		OrderID:    strconv.FormatInt(ord.OrderID, 10),
		Symbol:     ord.Symbol,
		Side:       ord.Side,
		Status:     parseOrderStatus(ord.Status),
		Price:      decimal.RequireFromString(ord.Price),
		Quantity:   decimal.RequireFromString(ord.OrigQty),
		CreateTime: time.UnixMilli(ord.EventTime),
	}
}

func (b *BinanceClient) CancelAll(symbol string) error {
	for id, _ := range b.orders {
		_, err := b.CancelOrder(symbol, id)
		if err != nil {
			if strings.Contains(err.Error(), "Unknown order sent.") {
				b.ordersMu.Lock()
				delete(b.orders, id)
				b.ordersMu.Unlock()
				continue
			}
			return err
		}
	}
	return nil
}

func (b *BinanceClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	orderIdInt, _ := strconv.ParseInt(orderId, 10, 64)
	params := map[string]interface{}{"symbol": symbol, "orderId": orderIdInt, "timestamp": time.Now().UnixMilli()}
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "order.cancel",
		"params": params,
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}
	var wsResp WsOrderResponse
	rootByte, _ := json.Marshal(root)
	if err := wsResp.UnmarshalJSON(rootByte); err != nil {
		return nil, err
	}
	ord := wsResp.Result

	resp := &core.OrderResponse{
		OrderID:    strconv.FormatInt(ord.OrderID, 10),
		Symbol:     ord.Symbol,
		Side:       ord.Side,
		Tif:        core.TimeInForce(ord.TimeInForce),
		Status:     parseOrderStatus(ord.Status),
		Price:      decimal.RequireFromString(ord.Price),
		CreateTime: time.UnixMilli(ord.TransactTime),
	}
	if ord.OrigQuoteQty != "" {
		resp.IsQuoteQuantity = true
		resp.Quantity = decimal.RequireFromString(ord.OrigQuoteQty)
	} else {
		resp.IsQuoteQuantity = false
		resp.Quantity = decimal.RequireFromString(ord.OrigQty)
	}

	b.ordersMu.Lock()
	delete(b.orders, resp.OrderID)
	b.ordersMu.Unlock()
	return resp, nil
}

func (b *BinanceClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	orderIdInt, _ := strconv.ParseInt(orderId, 10, 64)
	params := map[string]interface{}{"symbol": symbol, "orderId": orderIdInt, "timestamp": time.Now().UnixMilli()}
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "order.status",
		"params": params,
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}
	var wsResp WsOrderResponse
	rootByte, _ := json.Marshal(root)
	if err := wsResp.UnmarshalJSON(rootByte); err != nil {
		return nil, err
	}
	ord := wsResp.Result
	price, _ := decimal.NewFromString(ord.Price)
	avgPrice, _ := decimal.NewFromString(ord.AvgPrice)
	execQty, _ := decimal.NewFromString(ord.ExecutedQty)
	var side, commAsset string
	if ord.Side == "BUY" {
		side = "buy"
		commAsset = "USDT"
	} else if ord.Side == "SELL" {
		side = "sell"
		commAsset = strings.TrimSuffix(ord.Symbol, "USDT")
	}

	resp := &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:    strconv.FormatInt(ord.OrderID, 10),
			Symbol:     ord.Symbol,
			Side:       side,
			Tif:        core.TimeInForce(ord.TimeInForce),
			Status:     parseOrderStatus(ord.Status),
			Price:      price,
			CreateTime: time.UnixMilli(ord.TransactTime),
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     execQty,
		Commission:      execQty.Mul(decimal.NewFromFloat(commisionRate)),
		CommissionAsset: commAsset,
		UpdateTime:      time.Now(),
	}
	if ord.OrigQuoteQty != "" {
		resp.IsQuoteQuantity = true
		resp.Quantity = decimal.RequireFromString(ord.OrigQuoteQty)
	} else {
		resp.IsQuoteQuantity = false
		resp.Quantity = decimal.RequireFromString(ord.OrigQty)
	}
	return resp, nil
}

func (b *BinanceClient) ModifyBuyPrice(symbol, orderId string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	return b.modifyOrderPrice("BUY", symbol, orderId, quantity, price)
}

func (b *BinanceClient) ModifySellPrice(symbol, orderId string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	return b.modifyOrderPrice("SELL", symbol, orderId, quantity, price)
}
func (b *BinanceClient) modifyOrderPrice(positionSide string, symbol, orderId string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	orderIdInt, _ := strconv.ParseInt(orderId, 10, 64)
	params := map[string]interface{}{"symbol": symbol,
		"orderId":   orderIdInt,
		"timestamp": time.Now().UnixMilli(),
		"quantity":  quantity.String(),
		"price":     price.String(),
		"side":      positionSide,
	}
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "order.modify",
		"params": params,
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}
	var wsResp WsOrderResponse
	rootByte, _ := json.Marshal(root)
	if err := wsResp.UnmarshalJSON(rootByte); err != nil {
		return nil, err
	}
	ord := wsResp.Result

	resp := &core.OrderResponse{
		OrderID:    strconv.FormatInt(ord.OrderID, 10),
		Symbol:     ord.Symbol,
		Side:       ord.Side,
		Tif:        core.TimeInForce(ord.TimeInForce),
		Status:     parseOrderStatus(ord.Status),
		Price:      decimal.RequireFromString(ord.Price),
		CreateTime: time.UnixMilli(ord.TransactTime),
	}
	if ord.OrigQuoteQty != "" {
		resp.IsQuoteQuantity = true
		resp.Quantity = decimal.RequireFromString(ord.OrigQuoteQty)
	} else {
		resp.IsQuoteQuantity = false
		resp.Quantity = decimal.RequireFromString(ord.OrigQty)
	}

	b.ordersMu.Lock()
	b.orders[resp.OrderID] = resp
	b.ordersMu.Unlock()
	return resp, nil
}
