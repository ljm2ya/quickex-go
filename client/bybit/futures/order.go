package bybit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

type OrderOptions struct {
	Symbol           string
	Side             string // "Buy" or "Sell"
	OrderType        string // "Limit" or "Market"
	Qty              decimal.Decimal
	Price            decimal.Decimal
	TimeInForce      string // "GTC", "IOC", "FOK", "PostOnly"
	TriggerPrice     decimal.Decimal
	TriggerDirection int
	OrderLinkID      string
	ReduceOnly       bool
	CloseOnTrigger   bool
	MarketUnit       string
}

func orderOptionsToParams(opt OrderOptions) map[string]interface{} {
	params := map[string]interface{}{
		"category":   "linear",
		"symbol":     opt.Symbol,
		"side":       opt.Side,
		"orderType":  opt.OrderType,
		"qty":        opt.Qty.String(),
		"marketUnit": opt.MarketUnit,
	}
	if !opt.Price.IsZero() && opt.OrderType == "Limit" {
		params["price"] = opt.Price.String()
	}
	if opt.TimeInForce != "" {
		// Bybit: "GTC", "IOC", "FOK"
		params["timeInForce"] = opt.TimeInForce
	}
	if !opt.TriggerPrice.IsZero() {
		params["triggerPrice"] = opt.TriggerPrice.String()
	}
	if opt.OrderLinkID != "" {
		params["orderLinkId"] = opt.OrderLinkID
	}
	if opt.TriggerDirection != 0 {
		params["triggerDirection"] = opt.TriggerDirection
	}
	if opt.ReduceOnly {
		params["reduceOnly"] = true
	}
	if opt.CloseOnTrigger {
		params["closeOnTrigger"] = true
	}
	return params
}

func (c *BybitFuturesClient) wsPlaceOrder(opt OrderOptions) (*core.OrderResponse, error) {
	id := nextWSID()
	params := orderOptionsToParams(opt)
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	header := map[string]interface{}{
		"X-BAPI-TIMESTAMP":   timestamp,
		"X-BAPI-RECV-WINDOW": "8000",
	}
	req := map[string]interface{}{
		"reqId":  id,
		"header": header,
		"op":     "order.create",
		"args":   []interface{}{params},
	}
	root, err := c.WsClient.SendRequest(req)
	if err != nil {
		return nil, err
	}
	res, err := parseOrderResponse(root)
	if err != nil {
		return nil, err
	}
	c.ordersMu.Lock()
	c.orders[res.OrderID] = res
	c.ordersMu.Unlock()
	res.Symbol = opt.Symbol
	res.CreateTime = time.Now()
	res.Quantity = opt.Qty
	if opt.MarketUnit == "quoteCoin" {
		res.IsQuoteQuantity = true
	} else {
		res.IsQuoteQuantity = false
	}
	res.Price = opt.Price
	// Convert string side to core.OrderSide
	if strings.ToUpper(opt.Side) == "BUY" {
		res.Side = core.OrderSideBuy
	} else if strings.ToUpper(opt.Side) == "SELL" {
		res.Side = core.OrderSideSell
	}
	res.Status = core.OrderStatusOpen
	return res, nil
}

func parseOrderResponse(root map[string]json.RawMessage) (*core.OrderResponse, error) {
	// retCode 체크
	var retCode int
	if err := json.Unmarshal(root["retCode"], &retCode); err != nil || retCode != 0 {
		var retMsg string
		_ = json.Unmarshal(root["retMsg"], &retMsg)
		return nil, fmt.Errorf("bybit ws order error: %v, msg: %v", retCode, retMsg)
	}

	// data 필드 파싱
	var data struct {
		OrderID     string `json:"orderId"`
		OrderLinkID string `json:"orderLinkId"`
	}
	if d, ok := root["data"]; ok {
		if err := json.Unmarshal(d, &data); err != nil {
			return nil, err
		}
	}

	return &core.OrderResponse{
		OrderID: data.OrderID,
		// 필수 필드만 (symbol, price 등 없음)
	}, nil
}

func (c *BybitFuturesClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	id := nextWSID()
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	header := map[string]interface{}{
		"X-BAPI-TIMESTAMP":   timestamp,
		"X-BAPI-RECV-WINDOW": "8000",
	}
	req := map[string]interface{}{
		"reqId":  id,
		"header": header,
		"op":     "order.cancel",
		"args": []interface{}{map[string]interface{}{
			"category": "linear",
			"symbol":   symbol,
			"orderId":  orderId,
		}},
	}
	root, err := c.WsClient.SendRequest(req)
	if err != nil {
		return nil, err
	}
	res, err := parseOrderResponse(root)
	if err != nil {
		return nil, err
	}
	res.Status = core.OrderStatusCanceled
	return res, nil
}

func (c *BybitFuturesClient) CancelAll(symbol string) error {
	for id, _ := range c.orders {
		_, err := c.CancelOrder(symbol, id)
		if err != nil {
			if strings.Contains(err.Error(), "order not exists") {
				c.ordersMu.Lock()
				delete(c.orders, id)
				c.ordersMu.Unlock()
				continue
			}
			return err
		}
	}
	return nil
}

func (c *BybitFuturesClient) LimitBuy(symbol string, qty, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:      symbol,
		Side:        "Buy",
		OrderType:   "Limit",
		Qty:         qty,
		Price:       price,
		TimeInForce: tif,
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitFuturesClient) LimitSell(symbol string, qty, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:      symbol,
		Side:        "Sell",
		OrderType:   "Limit",
		Qty:         qty,
		Price:       price,
		TimeInForce: tif,
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitFuturesClient) MarketBuy(symbol string, quoteQty decimal.Decimal) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:     symbol,
		Side:       "Buy",
		OrderType:  "Market",
		Qty:        quoteQty,
		MarketUnit: "quoteCoin",
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitFuturesClient) MarketSell(symbol string, qty decimal.Decimal) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:    symbol,
		Side:      "Sell",
		OrderType: "Market",
		Qty:       qty,
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitFuturesClient) StopLossSell(symbol string, qty, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:           symbol,
		Side:             "Sell",
		OrderType:        "Market",
		Qty:              qty,
		TriggerPrice:     triggerPrice,
		ReduceOnly:       true,
		CloseOnTrigger:   true,
		TriggerDirection: 2,
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitFuturesClient) TakeProfitTrailingSell(symbol string, qty, triggerPrice decimal.Decimal, trailingDelta int) (*core.OrderResponse, error) {
	// bybit doesn't support trailing order, just simplize to take profit order
	opt := OrderOptions{
		Symbol:           symbol,
		Side:             "Sell",
		OrderType:        "Market",
		Qty:              qty,
		TriggerPrice:     triggerPrice,
		ReduceOnly:       true,
		CloseOnTrigger:   true,
		TriggerDirection: 1,
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitFuturesClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	param := bybit.V5GetOpenOrdersParam{
		Category: "linear",
		Symbol:   &symbol,
		OrderID:  &orderId,
	}
	resp, err := c.client.V5().Order().GetOpenOrders(param)
	if err != nil {
		return nil, handleBybitError(err)
	}
	if len(resp.Result.List) == 0 {
		return nil, errors.New("no order found")
	}
	order := resp.Result.List[0]

	toTime := func(s string) time.Time {
		ms, _ := strconv.ParseInt(s, 10, 64)
		return time.UnixMilli(ms)
	}
	toSide := func(s bybit.Side) core.OrderSide {
		if s == bybit.SideBuy {
			return core.OrderSideBuy
		} else if s == bybit.SideSell {
			return core.OrderSideSell
		} else {
			return ""
		}
	}
	toOrderStatus := func(s bybit.OrderStatus) core.OrderStatus {
		switch s {
		case bybit.OrderStatusActive, bybit.OrderStatusCreated, bybit.OrderStatusNew, bybit.OrderStatusPartiallyFilled, bybit.OrderStatusPendingCancel, bybit.OrderStatusTriggered, bybit.OrderStatusUntriggered:
			return core.OrderStatusOpen
		case bybit.OrderStatusFilled:
			return core.OrderStatusFilled
		case bybit.OrderStatusCancelled:
			return core.OrderStatusCanceled
		default:
			return core.OrderStatusError
		}
	}

	avgPrice, err := decimal.NewFromString(order.AvgPrice)
	if err != nil {
		avgPrice = decimal.Zero
	}

	return &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:    order.OrderID,
			Symbol:     order.Symbol,
			Side:       toSide(order.Side),
			Tif:        core.TimeInForce(order.TimeInForce),
			Status:     toOrderStatus(order.OrderStatus),
			Price:      decimal.RequireFromString(order.Price),
			Quantity:   decimal.RequireFromString(order.Qty),
			CreateTime: toTime(order.CreatedTime),
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     decimal.RequireFromString(order.CumExecQty),
		Commission:      decimal.RequireFromString(order.CumExecFee),
		CommissionAsset: SymbolToAsset(symbol),
		UpdateTime:      toTime(order.UpdatedTime),
	}, nil
}
