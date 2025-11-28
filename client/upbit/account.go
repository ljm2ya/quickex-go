package upbit

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

// FetchBalance implements core.PrivateClient interface
// Uses REST API for fetching current balance, websocket for real-time updates
func (u *UpbitClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		return decimal.Zero, fmt.Errorf("Get Futures Position: Not futures exchange")
	}

	// FetchBalance should use REST API to get current balance state
	// WebSocket is used for real-time balance updates, not fetching existing balances
	return u.fetchBalanceFromREST(asset, includeLocked)
}


// fetchBalanceFromREST fetches balance using REST API (fallback method)
func (u *UpbitClient) fetchBalanceFromREST(asset string, includeLocked bool) (decimal.Decimal, error) {
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

	return decimal.Zero, nil
}
