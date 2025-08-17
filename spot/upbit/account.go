package upbit

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// FetchBalance implements core.PrivateClient interface
func (u *UpbitClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		return decimal.Zero, fmt.Errorf("Get Futures Position: Not futures exchange")
	}
	body, err := u.makeRequest("GET", "/v1/accounts", nil)
	if err != nil {
		return decimal.Zero, err
	}

	var balances []UpbitBalance
	if err := json.Unmarshal(body, &balances); err != nil {
		return decimal.Zero, err
	}

	for _, balance := range balances {
		if balance.Currency == asset {
			free, err := decimal.NewFromString(balance.Balance)
			if err != nil {
				return decimal.Zero, err
			}

			if includeLocked {
				locked, err := decimal.NewFromString(balance.Locked)
				if err != nil {
					return decimal.Zero, err
				}
				return free.Add(locked), nil
			}
			return free, nil
		}
	}

	return decimal.Zero, errors.New("asset not found")
}
