package kucoin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/spot/order"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *KucoinSpotClient) FetchOrder(symbol, orderId string) (*core.OrderResponseFull, error) {
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	orderAPI := spotService.GetOrderAPI()

	// Get order by ID
	req := order.NewGetOrderByOrderIdReqBuilder().
		SetOrderId(orderId).
		SetSymbol(symbol).
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

	// Parse order data from response fields
	price, _ := decimal.NewFromString(resp.Price)
	quantity, _ := decimal.NewFromString(resp.Size)
	executedQty, _ := decimal.NewFromString(resp.DealSize)
	dealFunds, _ := decimal.NewFromString(resp.DealFunds)
	fee, _ := decimal.NewFromString(resp.Fee)

	// Calculate average price from order response first
	var avgPrice decimal.Decimal
	if !executedQty.IsZero() {
		avgPrice = dealFunds.Div(executedQty)
	}
	/*

		// If order has been executed (partially or fully), fetch trade history for accurate execution details
		if !executedQty.IsZero() || resp.DealSize != "0" {
			tradeReq := order.NewGetTradeHistoryReqBuilder().
				SetOrderId(orderId).
				SetSymbol(resp.Symbol). // Use symbol from order response
				SetLimit(100).          // Get up to 100 fills
				Build()

			tradeResp, err := orderAPI.GetTradeHistory(tradeReq, context.Background())
			if err == nil && len(tradeResp.Items) > 0 {
				// Calculate average price, total executed quantity, and total commission from trades
				var totalFunds, totalSize, totalFee decimal.Decimal
				var commissionAsset string

				for _, trade := range tradeResp.Items {
					tradeFunds, _ := decimal.NewFromString(trade.Funds)
					tradeSize, _ := decimal.NewFromString(trade.Size)
					tradeFee, _ := decimal.NewFromString(trade.Fee)

					totalFunds = totalFunds.Add(tradeFunds)
					totalSize = totalSize.Add(tradeSize)
					totalFee = totalFee.Add(tradeFee)

					// Use the fee currency from the first trade
					if commissionAsset == "" {
						commissionAsset = trade.FeeCurrency
					}
				}

				// Override with accurate values from trade history
				if !totalSize.IsZero() {
					executedQty = totalSize
					avgPrice = totalFunds.Div(totalSize)
					fee = totalFee
					if commissionAsset != "" {
						resp.FeeCurrency = commissionAsset
					}
				}
			}
		}

	*/
	// Convert timestamps
	createTime := time.Unix(resp.CreatedAt/1000, 0)
	updateTime := createTime // KuCoin doesn't provide separate update time in this response

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
		CommissionAsset: resp.FeeCurrency,
		UpdateTime:      updateTime,
	}, nil
}

func (c *KucoinSpotClient) LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeLimitOrder(symbol, "buy", quantity, price, tif)
}

func (c *KucoinSpotClient) LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	return c.placeLimitOrder(symbol, "sell", quantity, price, tif)
}

func (c *KucoinSpotClient) placeLimitOrder(symbol, side string, quantity, price decimal.Decimal, tif string) (*core.OrderResponse, error) {
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	orderAPI := spotService.GetOrderAPI()

	// Generate unique client order ID
	clientOid := fmt.Sprintf("quickex-%d", time.Now().UnixNano())

	// Create limit order request
	req := order.NewAddOrderReqBuilder().
		SetClientOid(clientOid).
		SetSide(side).
		SetSymbol(symbol).
		SetType("limit").
		SetSize(quantity.String()).
		SetPrice(price.String()).
		SetTimeInForce(mapTifToKucoin(tif)).
		Build()

	resp, err := orderAPI.AddOrder(req, context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to place %s order: %w", side, err)
	}

	return &core.OrderResponse{
		OrderID:    resp.OrderId,
		Symbol:     symbol,
		Side:       strings.ToUpper(side),
		Status:     core.OrderStatusOpen,
		Price:      price,
		Quantity:   quantity,
		CreateTime: time.Now(),
	}, nil
}

func (c *KucoinSpotClient) MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*core.OrderResponse, error) {
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	orderAPI := spotService.GetOrderAPI()

	clientOid := fmt.Sprintf("quickex-%d", time.Now().UnixNano())

	// For market buy orders, use funds (quote quantity) instead of size
	req := order.NewAddOrderReqBuilder().
		SetClientOid(clientOid).
		SetSide("buy").
		SetSymbol(symbol).
		SetType("market").
		SetFunds(quoteQuantity.String()).
		Build()

	resp, err := orderAPI.AddOrder(req, context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to place market buy order: %w", err)
	}

	return &core.OrderResponse{
		OrderID:         resp.OrderId,
		Symbol:          symbol,
		Side:            "BUY",
		Status:          core.OrderStatusOpen,
		Quantity:        quoteQuantity,
		IsQuoteQuantity: true,
		CreateTime:      time.Now(),
	}, nil
}

func (c *KucoinSpotClient) MarketSell(symbol string, quantity decimal.Decimal) (*core.OrderResponse, error) {
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	orderAPI := spotService.GetOrderAPI()

	clientOid := fmt.Sprintf("quickex-%d", time.Now().UnixNano())

	// For market sell orders, use size (base quantity)
	req := order.NewAddOrderReqBuilder().
		SetClientOid(clientOid).
		SetSide("sell").
		SetSymbol(symbol).
		SetType("market").
		SetSize(quantity.String()).
		Build()

	resp, err := orderAPI.AddOrder(req, context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to place market sell order: %w", err)
	}

	return &core.OrderResponse{
		OrderID:    resp.OrderId,
		Symbol:     symbol,
		Side:       "SELL",
		Status:     core.OrderStatusOpen,
		Quantity:   quantity,
		CreateTime: time.Now(),
	}, nil
}

func (c *KucoinSpotClient) CancelOrder(symbol, orderId string) (*core.OrderResponse, error) {
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	orderAPI := spotService.GetOrderAPI()

	// Cancel order by ID
	req := order.NewCancelOrderByOrderIdReqBuilder().
		SetOrderId(orderId).
		SetSymbol(symbol).
		Build()

	_, err := orderAPI.CancelOrderByOrderId(req, context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	return &core.OrderResponse{
		OrderID: orderId, // Return the original order ID
		Status:  core.OrderStatusCanceled,
	}, nil
}

func (c *KucoinSpotClient) CancelAll(symbol string) error {
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	orderAPI := spotService.GetOrderAPI()

	// Cancel all orders for the symbol
	req := order.NewCancelAllOrdersBySymbolReqBuilder().
		SetSymbol(symbol).
		Build()

	_, err := orderAPI.CancelAllOrdersBySymbol(req, context.Background())
	if err != nil {
		return fmt.Errorf("failed to cancel all orders for %s: %w", symbol, err)
	}

	return nil
}

// Helper functions

func mapOrderStatus(order *order.GetOrderByOrderIdResp) core.OrderStatus {
	if order == nil {
		return core.OrderStatusError
	}
	if order.Active {
		return core.OrderStatusOpen
	}
	if order.DealSize == order.Size {
		return core.OrderStatusFilled
	}
	return core.OrderStatusCanceled
}

func mapTifToKucoin(tif string) string {
	switch tif {
	case string(core.TimeInForceGTC):
		return "GTC"
	case string(core.TimeInForceIOC):
		return "IOC"
	case string(core.TimeInForceFOK):
		return "FOK"
	default:
		return "GTC"
	}
}
