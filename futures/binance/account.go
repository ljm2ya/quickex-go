package binance

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

func (b *BinanceClient) GetCachedBalance(asset string, includeLocked bool) (float64, error) {
	if b.balances[asset] == nil {
		return 0, fmt.Errorf("Asset %s doesn't exists or not loaded", asset)
	}
	b.balancesMu.RLock()
	defer b.balancesMu.RUnlock()
	if !includeLocked {
		free, _ := b.balances[asset].Free.Float64()
		return free, nil
	} else {
		total, _ := b.balances[asset].Total.Float64()
		return total, nil
	}
}

func (b *BinanceClient) GetPositionAmount(asset string) (float64, error) {
	acct, err := b.GetAccount()
	if err != nil {
		return 0, fmt.Errorf("Get account err: %v", err)
	}
	for _, position := range acct.Positions {
		if position.Symbol == asset+"USDT" {
			return position.Amount, nil
		}
	}
	return 0, nil
}

func (b *BinanceClient) GetBalance(asset string, includeLocked bool) (float64, error) {
	acct, err := b.GetAccount()
	if err != nil {
		return 0, fmt.Errorf("Get account err: %v", err)
	}
	for _, wallet := range acct.Assets {
		if wallet.Asset == asset {
			if includeLocked {
				total, _ := wallet.Total.Float64()
				return total, nil
			} else {
				free, _ := wallet.Free.Float64()
				return free, nil
			}
		}
	}
	return 0, nil
}

func (b *BinanceClient) GetAccount() (*core.Account, error) {
	ts := time.Now().UnixMilli()
	req := map[string]interface{}{
		"id":     nextWSID(),
		"method": "v2/account.status",
		"params": map[string]interface{}{
			"timestamp": ts,
		},
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON into wsAccountInfo
	resultBytes, err := json.Marshal(root["result"])
	if err != nil {
		return nil, err
	}

	//fmt.Printf("result JSON: %s\n", resultBytes)
	var info wsAccountInfo
	if err := json.Unmarshal(resultBytes, &info); err != nil {
		return nil, err
	}

	acct := &core.Account{
		CrossBalance:   core.ParseStringFloat(info.TotalCrossWalletBalance),
		CrossUrlProfit: core.ParseStringFloat(info.TotalCrossUnPnl),
		Assets:         make(map[string]*core.Wallet),
		Positions:      make(map[string]*core.Position),
	}
	// Assets
	for _, a := range info.Assets {
		wb := core.ParseStringFloat(a.CrossWalletBalance)
		cwb := core.ParseStringFloat(a.AvailableBalance)
		acct.Assets[a.Asset] = &core.Wallet{
			Asset:  a.Asset,
			Free:   decimal.NewFromFloat(cwb),
			Locked: decimal.NewFromFloat(wb - cwb),
			Total:  decimal.NewFromFloat(wb),
		}
	}

	for _, p := range info.Positions {
		acct.Positions[p.Symbol] = &core.Position{
			Symbol:         p.Symbol,
			Side:           toPositionSide(p.PositionSide),
			Amount:         core.ParseStringFloat(p.PositionAmt),
			UrlProfit:      core.ParseStringFloat(p.UnrealizedProfit),
			IsolatedMargin: core.ParseStringFloat(p.IsolatedMargin),
			Notional:       core.ParseStringFloat(p.Notional),
			IsolatedWallet: core.ParseStringFloat(p.IsolatedWallet),
			InitialMargin:  core.ParseStringFloat(p.InitialMargin),
			MaintMargin:    core.ParseStringFloat(p.MaintMargin),
			UpdatedTime:    time.UnixMilli(p.UpdateTime),
		}
	}
	return acct, nil
}

func toPositionSide(s string) core.PositionSide {
	switch s {
	case "LONG":
		return core.LONG
	case "SHORT":
		return core.SHORT
	default:
		return core.BOTH
	}
}

// FetchBalance implements core.PrivateClient interface
func (b *BinanceClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		pos, err := b.GetPositionAmount(asset)
		if err != nil {
			return decimal.Zero, err
		}
		return decimal.NewFromFloat(pos), nil
	}
	balance, err := b.GetBalance(asset, includeLocked)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromFloat(balance), nil
}
