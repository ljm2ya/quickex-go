package futures

import (
	"context"
	"fmt"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/account/account"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/positions"
	"github.com/shopspring/decimal"
)

func (c *KucoinFuturesClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		return c.fetchPositionAmount(asset)
	}
	restService := c.client.RestService()
	accountService := restService.GetAccountService()
	accountAPI := accountService.GetAccountAPI()

	req := account.NewGetFuturesAccountReqBuilder().
		SetCurrency(asset).Build()

	resp, err := accountAPI.GetFuturesAccount(req, context.Background())
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get account balance: %w", err)
	}

	available := decimal.NewFromFloat(resp.MaxWithdrawAmount)
	locked := decimal.NewFromFloat(resp.PositionMargin + resp.OrderMargin - resp.UnrealisedPNL)
	if includeLocked {
		return available.Add(locked), nil
	}
	return available, nil
}

func (c *KucoinFuturesClient) fetchPositionAmount(asset string) (decimal.Decimal, error) {
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	futuresAPI := futuresService.GetPositionsAPI()

	req := positions.NewGetPositionDetailsReqBuilder().
		SetSymbol(c.ToSymbol(asset, "USDT")).
		Build()

	resp, err := futuresAPI.GetPositionDetails(req, context.Background())

	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get position detail: %w", err)
	}
	if mul, on := c.multiplierMap[resp.Symbol]; on {
		return decimal.NewFromInt32(resp.CurrentQty).Abs().Mul(mul), nil
	}
	return decimal.Zero, fmt.Errorf("Failed to get lot of position symbol: did you connected?")
}
