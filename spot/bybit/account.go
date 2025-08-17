package bybit

import (
	"fmt"

	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (c *BybitClient) GetBalance(asset string, includeLocked bool) (float64, error) {
	resp, err := c.client.V5().Account().GetWalletBalance(bybit.AccountTypeV5UNIFIED, nil)
	if err != nil || len(resp.Result.List) == 0 {
		return 0, err
	}
	for _, coin := range resp.Result.List[0].Coin {
		if coin.Coin == asset {
			if includeLocked {
				return core.ToFloat(coin.Equity), nil
			} else {
				return core.ToFloat(coin.WalletBalance), nil
			}
		}
	}
	return 0, fmt.Errorf("Get Balance error: no matching balance")
}

// FetchBalance implements core.PrivateClient interface
func (c *BybitClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		return decimal.Zero, fmt.Errorf("Get Futures Position: Not futures exchange")
	}
	balance, err := c.GetBalance(asset, includeLocked)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromFloat(balance), nil
}
