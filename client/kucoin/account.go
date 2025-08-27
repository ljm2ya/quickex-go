package kucoin

import (
	"context"
	"fmt"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/account/account"
	"github.com/shopspring/decimal"
)

func (c *KucoinSpotClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	restService := c.client.RestService()
	accountService := restService.GetAccountService()
	accountAPI := accountService.GetAccountAPI()
	
	// Get spot account list
	req := account.NewGetSpotAccountListReqBuilder().
		SetCurrency(asset).
		SetType("trade"). // trade account for spot trading
		Build()
	
	resp, err := accountAPI.GetSpotAccountList(req, context.Background())
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get account balance: %w", err)
	}
	
	var totalBalance decimal.Decimal
	
	// Sum balances from all matching accounts
	for _, acc := range resp.Data {
		if acc.Currency == asset {
			available, _ := decimal.NewFromString(acc.Available)
			holds, _ := decimal.NewFromString(acc.Holds)
			
			if includeLocked {
				totalBalance = totalBalance.Add(available).Add(holds)
			} else {
				totalBalance = totalBalance.Add(available)
			}
		}
	}
	
	return totalBalance, nil
}