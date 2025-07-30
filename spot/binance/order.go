package binance

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

type OrderOptions struct {
	Price                   decimal.Decimal
	Quantity                decimal.Decimal
	QuoteOrderQty           decimal.Decimal
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
	opts := &OrderOptions{Quantity: quantity, Price: price}
	return b.placeOrder(symbol, "BUY", "LIMIT_MAKER", opts)
}

func (b *BinanceClient) LimitMakerSell(symbol string, quantity, price decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price}
	return b.placeOrder(symbol, "SELL", "LIMIT_MAKER", opts)
}

func (b *BinanceClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{QuoteOrderQty: quoteQuantity}
	return b.placeOrder(symbol, "BUY", "MARKET", opts)
}

func (b *BinanceClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity}
	return b.placeOrder(symbol, "SELL", "MARKET", opts)
}

func (b *BinanceClient) MarketBuyQuote(symbol string, quoteOrderQty decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{QuoteOrderQty: quoteOrderQty}
	return b.placeOrder(symbol, "BUY", "MARKET", opts)
}

func (b *BinanceClient) MarketSellQuote(symbol string, quoteOrderQty decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{QuoteOrderQty: quoteOrderQty}
	return b.placeOrder(symbol, "SELL", "MARKET", opts)
}

func (b *BinanceClient) StopLossBuy(symbol string, quantity, stopPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: stopPrice}
	return b.placeOrder(symbol, "BUY", "STOP_LOSS", opts)
}

func (b *BinanceClient) StopLossSell(symbol string, quantity, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: triggerPrice}
	return b.placeOrder(symbol, "SELL", "STOP_LOSS", opts)
}

func (b *BinanceClient) StopLossLimitBuy(symbol string, quantity, price, stopPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, StopPrice: stopPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "BUY", "STOP_LOSS_LIMIT", opts)
}

func (b *BinanceClient) StopLossLimitSell(symbol string, quantity, price, stopPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, StopPrice: stopPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "SELL", "STOP_LOSS_LIMIT", opts)
}

func (b *BinanceClient) TakeProfitBuy(symbol string, quantity, stopPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: stopPrice}
	return b.placeOrder(symbol, "BUY", "TAKE_PROFIT", opts)
}

func (b *BinanceClient) TakeProfitSell(symbol string, quantity, triggerPrice decimal.Decimal) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: triggerPrice}
	return b.placeOrder(symbol, "SELL", "TAKE_PROFIT", opts)
}

func (b *BinanceClient) TakeProfitTrailingSell(symbol string, quantity, triggerPrice decimal.Decimal, delta int) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, StopPrice: triggerPrice, TrailingDelta: delta}
	return b.placeOrder(symbol, "SELL", "TAKE_PROFIT", opts)
}

func (b *BinanceClient) TakeProfitLimitBuy(symbol string, quantity, price, stopPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: price, StopPrice: stopPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "BUY", "TAKE_PROFIT_LIMIT", opts)
}

func (b *BinanceClient) TakeProfitLimitSell(symbol string, quantity, targetPrice, triggerPrice decimal.Decimal, tif string) (*core.OrderResponse, error) {
	opts := &OrderOptions{Quantity: quantity, Price: targetPrice, StopPrice: triggerPrice, TimeInForce: tif}
	return b.placeOrder(symbol, "SELL", "TAKE_PROFIT_LIMIT", opts)
}

// --- ORDER INTERNAL API ---

func (b *BinanceClient) placeOrder(symbol, side, orderType string, opt *OrderOptions) (*core.OrderResponse, error) {
	params := map[string]interface{}{
		"symbol":    symbol,
		"side":      side,
		"type":      orderType,
		"timestamp": time.Now().UnixMilli(),
	}
	if !opt.Price.IsZero() {
		params["price"] = opt.Price.String()
	}
	if !opt.Quantity.IsZero() {
		params["quantity"] = opt.Quantity.String()
	}
	if !opt.QuoteOrderQty.IsZero() {
		params["quoteOrderQty"] = opt.QuoteOrderQty.String()
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
	if opt.TrailingDelta > 0 {
		params["trailingDelta"] = opt.TrailingDelta
	}
	if !opt.IcebergQty.IsZero() {
		params["icebergQty"] = opt.IcebergQty.String()
	}
	if opt.StrategyId > 0 {
		params["strategyId"] = opt.StrategyId
	}
	if opt.StrategyType > 0 {
		params["strategyType"] = opt.StrategyType
	}
	if opt.SelfTradePreventionMode != "" {
		params["selfTradePreventionMode"] = opt.SelfTradePreventionMode
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

	// For special order handling such as OCO or list order status
	// Check if there was a list status update in the queue
	lsoTimer := time.NewTimer(500 * time.Millisecond)
	defer lsoTimer.Stop()
	b.wsRejectMu.Lock()
	ch, hasPending := b.wsReject[symbol]
	b.wsRejectMu.Unlock()
	if hasPending && ord.OrderListID > 0 {
		// Wait for the listStatus event
		select {
		case lso := <-ch:
			// Check if the order was rejected
			if lso.ListOrderStatus == "EXEC_STARTED" {
				// At least one leg executed
			} else if lso.ListOrderStatus == "ALL_DONE" {
				// All legs executed
			} else {
				// Rejected or other status
				return nil, fmt.Errorf("oco order rejected: %v", lso.ListOrderStatus)
			}
		case <-lsoTimer.C:
			// Timeout, proceed anyway
		}
		// Clean up the channel
		b.wsRejectMu.Lock()
		delete(b.wsReject, symbol)
		b.wsRejectMu.Unlock()
	}

	qty := decimal.RequireFromString(ord.OrigQty)
	isQuoteQty := false
	if ord.OrigQty == "" {
		qty = decimal.RequireFromString(ord.OrigQuoteOrderQty)
		isQuoteQty = true
	}
	resp := &core.OrderResponse{
		OrderID:         strconv.FormatInt(ord.OrderID, 10),
		Symbol:          ord.Symbol,
		Side:            ord.Side,
		Tif:             core.TimeInForce(ord.TimeInForce),
		Status:          parseOrderStatus(ord.Status),
		Price:           decimal.RequireFromString(ord.Price),
		Quantity:        qty,
		IsQuoteQuantity: isQuoteQty,
		CreateTime:      time.UnixMilli(ord.TransactTime),
	}
	return resp, nil
}

func parseOrderStatus(status string) core.OrderStatus {
	switch status {
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

func (b *BinanceClient) CancelAll(symbol string) error {
	params := map[string]interface{}{"symbol": symbol, "timestamp": time.Now().UnixMilli()}
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "openOrders.cancelAll",
		"params": params,
	}
	_, err := b.SendRequest(req)
	if err != nil {
		return err
	}
	return nil
}

func (b *BinanceClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	params := map[string]interface{}{"symbol": symbol, "orderId": orderId, "timestamp": time.Now().UnixMilli()}
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

	qty := decimal.RequireFromString(ord.OrigQty)
	isQuoteQty := false
	if ord.OrigQty == "" {
		qty = decimal.RequireFromString(ord.OrigQuoteOrderQty)
		isQuoteQty = true
	}
	resp := &core.OrderResponse{
		OrderID:         strconv.FormatInt(ord.OrderID, 10),
		Symbol:          ord.Symbol,
		Side:            ord.Side,
		Tif:             core.TimeInForce(ord.TimeInForce),
		Status:          parseOrderStatus(ord.Status),
		Price:           decimal.RequireFromString(ord.Price),
		Quantity:        qty,
		IsQuoteQuantity: isQuoteQty,
		CreateTime:      time.UnixMilli(ord.TransactTime),
	}
	return resp, nil
}

func (b *BinanceClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	params := map[string]interface{}{"symbol": symbol, "orderId": orderId, "timestamp": time.Now().UnixMilli()}
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

	qty := decimal.RequireFromString(ord.OrigQty)
	isQuoteQty := false
	if ord.OrigQty == "" {
		qty = decimal.RequireFromString(ord.OrigQuoteOrderQty)
		isQuoteQty = true
	}
	// Get trade history to calculate actual average price
	avgPrice, executedQty, commission, commissionAsset, updateTime, err := b.getOrderTradeHistory(symbol, strconv.FormatInt(ord.OrderID, 10))
	if err != nil {
		// If trade history fails, use order data as fallback
		avgPrice = decimal.RequireFromString(ord.Price)
		executedQty = decimal.RequireFromString(ord.ExecutedQty)
		commission = decimal.Zero
		commissionAsset = ""
	}

	resp := &core.OrderResponseFull{
		OrderResponse: core.OrderResponse{
			OrderID:         strconv.FormatInt(ord.OrderID, 10),
			Symbol:          ord.Symbol,
			Side:            ord.Side,
			Tif:             core.TimeInForce(ord.TimeInForce),
			Status:          parseOrderStatus(ord.Status),
			Price:           decimal.RequireFromString(ord.Price),
			Quantity:        qty,
			IsQuoteQuantity: isQuoteQty,
			CreateTime:      time.UnixMilli(ord.TransactTime),
		},
		AvgPrice:        avgPrice,
		ExecutedQty:     executedQty,
		Commission:      commission,
		CommissionAsset: commissionAsset,
		UpdateTime:      updateTime,
	}
	return resp, nil
}

// getOrderTradeHistory fetches trade history for a specific order and calculates average price
func (b *BinanceClient) getOrderTradeHistory(symbol, orderId string) (avgPrice, executedQty, commission decimal.Decimal, commissionAsset string, updateTime time.Time, err error) {
	params := map[string]interface{}{
		"symbol":    symbol,
		"orderId":   orderId,
		"timestamp": time.Now().UnixMilli(),
	}
	id := nextWSID()
	req := map[string]interface{}{
		"id":     id,
		"method": "myTrades",
		"params": params,
	}

	root, err := b.SendRequest(req)
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, "", time.Time{}, err
	}

	// Parse the trade history response
	var wsResp struct {
		Result []struct {
			Symbol          string `json:"symbol"`
			ID              int64  `json:"id"`
			OrderID         int64  `json:"orderId"`
			OrderListID     int64  `json:"orderListId"`
			Price           string `json:"price"`
			Qty             string `json:"qty"`
			QuoteQty        string `json:"quoteQty"`
			Commission      string `json:"commission"`
			CommissionAsset string `json:"commissionAsset"`
			Time            int64  `json:"time"`
			IsBuyer         bool   `json:"isBuyer"`
			IsMaker         bool   `json:"isMaker"`
			IsBestMatch     bool   `json:"isBestMatch"`
		} `json:"result"`
	}

	rootByte, _ := json.Marshal(root)
	if err := json.Unmarshal(rootByte, &wsResp); err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, "", time.Time{}, err
	}

	trades := wsResp.Result
	if len(trades) == 0 {
		// No trades found, return zeros
		return decimal.Zero, decimal.Zero, decimal.Zero, "", time.Time{}, nil
	}

	// Calculate weighted average price and total quantities
	var totalValue decimal.Decimal = decimal.Zero
	var totalQty decimal.Decimal = decimal.Zero
	var totalCommission decimal.Decimal = decimal.Zero
	var latestTime int64 = 0
	var firstCommissionAsset string

	for _, trade := range trades {
		tradePrice := decimal.RequireFromString(trade.Price)
		tradeQty := decimal.RequireFromString(trade.Qty)
		tradeCommission := decimal.RequireFromString(trade.Commission)

		// Calculate value = price * quantity
		tradeValue := tradePrice.Mul(tradeQty)
		totalValue = totalValue.Add(tradeValue)
		totalQty = totalQty.Add(tradeQty)
		totalCommission = totalCommission.Add(tradeCommission)

		// Track the latest trade time for updateTime
		if trade.Time > latestTime {
			latestTime = trade.Time
		}

		// Use the first commission asset found
		if firstCommissionAsset == "" {
			firstCommissionAsset = trade.CommissionAsset
		}
	}

	// Calculate average price = total value / total quantity
	if totalQty.IsZero() {
		avgPrice = decimal.Zero
	} else {
		avgPrice = totalValue.Div(totalQty)
	}

	executedQty = totalQty
	commission = totalCommission
	commissionAsset = firstCommissionAsset
	updateTime = time.UnixMilli(latestTime)

	return avgPrice, executedQty, commission, commissionAsset, updateTime, nil
}
