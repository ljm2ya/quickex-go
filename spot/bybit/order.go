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
	Symbol       string
	Side         string // "Buy" or "Sell"
	OrderType    string // "Limit" or "Market"
	Qty          decimal.Decimal
	MarketUnit   string
	Price        decimal.Decimal
	TriggerPrice decimal.Decimal
	TimeInForce  string // "GTC", "IOC", "FOK", "PostOnly"
	OrderLinkID  string
	ReduceOnly   bool
	TakeProfit   decimal.Decimal
	StopLoss     decimal.Decimal
	TpOrderType  string // "Limit" or "Market"
	SlOrderType  string // "Limit" or "Market"
	OrderFilter  string // "tpslOrder"
}

func orderOptionsToParams(opt OrderOptions) map[string]interface{} {
	params := map[string]interface{}{
		"category":  "spot", // consider parameterizing if needed
		"symbol":    opt.Symbol,
		"side":      opt.Side,
		"orderType": opt.OrderType,
		"qty":       opt.Qty.String(),
	}
	if !opt.Price.IsZero() && opt.OrderType == "Limit" {
		params["price"] = opt.Price.String()
	}
	if opt.TimeInForce != "" {
		params["timeInForce"] = opt.TimeInForce
	}
	if !opt.TriggerPrice.IsZero() {
		params["triggerPrice"] = opt.TriggerPrice.String()
	}
	if opt.OrderLinkID != "" {
		params["orderLinkId"] = opt.OrderLinkID
	}
	if opt.ReduceOnly {
		params["reduceOnly"] = true
	}
	if opt.MarketUnit != "" {
		params["marketUnit"] = opt.MarketUnit
	}
	// New fields: TakeProfit, StopLoss, TpOrderType, SlOrderType
	if !opt.TakeProfit.IsZero() {
		params["takeProfit"] = opt.TakeProfit.String()
	}
	if !opt.StopLoss.IsZero() {
		params["stopLoss"] = opt.StopLoss.String()
	}
	if opt.TpOrderType != "" {
		params["tpOrderType"] = opt.TpOrderType
	}
	if opt.SlOrderType != "" {
		params["slOrderType"] = opt.SlOrderType
	}
	if opt.OrderFilter != "" {
		params["orderFilter"] = opt.OrderFilter
	}
	return params
}

func (c *BybitClient) wsPlaceOrder(opt OrderOptions) (*core.OrderResponse, error) {
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
	res.Side = opt.Side
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

func (c *BybitClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
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
			"category": "spot",
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

func (c *BybitClient) CancelAll(symbol string) error {
	for id, _ := range c.orders {
		_, err := c.CancelOrder(symbol, id)
		if err != nil {
			if strings.Contains(err.Error(), "Order does not exists.") {
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

func (c *BybitClient) LimitBuy(symbol string, qty, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
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

func (c *BybitClient) LimitSell(symbol string, qty, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:      symbol,
		Side:        "Sell",
		OrderType:   "Limit",
		Qty:         qty,
		Price:       price,
		ReduceOnly:  true,
		TimeInForce: tif,
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitClient) MarketBuy(symbol string, quoteQty decimal.Decimal) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:     symbol,
		Side:       "Buy",
		OrderType:  "Market",
		Qty:        quoteQty,
		MarketUnit: "quoteCoin",
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitClient) MarketSell(symbol string, qty decimal.Decimal) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:    symbol,
		Side:      "Sell",
		OrderType: "Market",
		Qty:       qty,
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitClient) StopLossSell(symbol string, qty, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	opt := OrderOptions{
		Symbol:       symbol,
		Side:         "Sell",
		OrderType:    "Market",
		Qty:          qty,
		TriggerPrice: triggerPrice,
		ReduceOnly:   true,
		OrderFilter:  "tpslOrder",
	}
	return c.wsPlaceOrder(opt)
}

func (c *BybitClient) TakeProfitTrailingSell(symbol string, qty, triggerPrice decimal.Decimal, trailingDelta int) (*core.OrderResponse, error) {
	// bybit doesn't support trailing order, just simplize to take profit order
	opt := OrderOptions{
		Symbol:       symbol,
		Side:         "Sell",
		OrderType:    "Market",
		Qty:          qty,
		TriggerPrice: triggerPrice,
		ReduceOnly:   true,
		OrderFilter:  "tpsOrder",
	}
	return c.wsPlaceOrder(opt)
}

// FetchOrder implements core.PrivateClient interface
func (c *BybitClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	param := bybit.V5GetOpenOrdersParam{
		Category: "spot",
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
	toSide := func(s bybit.Side) string {
		if s == bybit.SideBuy {
			return "BUY"
		} else if s == bybit.SideSell {
			return "SELL"
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
		AvgPrice:    decimal.RequireFromString(order.AvgPrice),
		ExecutedQty: decimal.RequireFromString(order.CumExecQty),
		UpdateTime:  toTime(order.UpdatedTime),
	}, nil
}
