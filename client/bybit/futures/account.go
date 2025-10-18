package bybit

import (
	"fmt"
	"time"

	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
	"strconv"
)

func (c *BybitFuturesClient) GetCachedBalance(asset string, includeLocked bool) (float64, error) {
	c.balancesMu.RLock()
	defer c.balancesMu.RUnlock()
	if wal, ok := c.balances[asset]; ok {
		if includeLocked {
			total, _ := wal.Total.Float64()
			return total, nil
		}
		free, _ := wal.Free.Float64()
		return free, nil
	}
	return 0, fmt.Errorf("not found")
}

func (c *BybitFuturesClient) GetBalance(asset string, includeLocked bool) (float64, error) {
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

func (c *BybitFuturesClient) GetPositionAmount(symbol string) (decimal.Decimal, error) {
	resp, err := c.client.V5().Position().GetPositionInfo(bybit.V5GetPositionInfoParam{
		Category: bybit.CategoryV5Linear,
		Symbol:   &symbol,
	})
	if err != nil || len(resp.Result.List) == 0 {
		return decimal.Zero, err
	}
	posValue, err := decimal.NewFromString(resp.Result.List[0].Size)
	if err != nil {
		posValue = decimal.Zero
	}
	return posValue, nil
}

func (c *BybitFuturesClient) GetAccount() (*core.Account, error) {
	resp, err := c.client.V5().Account().GetWalletBalance(bybit.AccountTypeV5UNIFIED, nil)
	if err != nil || len(resp.Result.List) == 0 {
		return nil, err
	}
	var account core.Account
	account.Assets = make(map[string]*core.Wallet)
	var quotes []string
	for _, coin := range resp.Result.List[0].Coin {
		avail := core.ToFloat(coin.WalletBalance)
		total := core.ToFloat(coin.Equity)
		if total > 10 {
			quotes = append(quotes, coin.Coin)
		}
		account.Assets[coin.Coin] = &core.Wallet{
			Asset:  coin.Coin,
			Free:   decimal.NewFromFloat(avail),
			Locked: decimal.NewFromFloat(total - avail),
			Total:  decimal.NewFromFloat(total),
		}
	}

	// TODO: Add position handling when Position type is defined
	return &account, nil
}

// FetchBalance implements core.PrivateClient interface
func (c *BybitFuturesClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		pos, err := c.GetPositionAmount(asset + "USDT")
		if err != nil {
			return decimal.Zero, err
		}
		return pos, nil
	}
	balance, err := c.GetBalance(asset, includeLocked)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromFloat(balance), nil
}

func (c *BybitFuturesClient) FetchPositionState(symbol string) (*core.PositionState, error) {
	resp, err := c.client.V5().Position().GetPositionInfo(bybit.V5GetPositionInfoParam{
		Category: bybit.CategoryV5Linear,
		Symbol:   &symbol,
	})
	if err != nil || len(resp.Result.List) == 0 {
		return nil, err
	}
	var side core.PositionSide
	switch resp.Result.List[0].Side {
	case bybit.SideBuy:
		side = core.LONG
	case bybit.SideSell:
		side = core.SHORT
	case "":
		return nil, nil
	default:
		return nil, fmt.Errorf("Invalid position side: %s", resp.Result.List[0].Side)
	}
	size, _ := decimal.NewFromString(resp.Result.List[0].Size)
	avgPrice, _ := decimal.NewFromString(resp.Result.List[0].AvgPrice)
	uPnl, _ := decimal.NewFromString(resp.Result.List[0].UnrealisedPnl)
	rPnl, _ := decimal.NewFromString(resp.Result.List[0].CurRealisedPnl)
	createdTimeInt, _ := strconv.ParseInt(resp.Result.List[0].CreatedTime, 10, 64)
	createdTime := time.UnixMilli(createdTimeInt)
	updatedTimeInt, _ := strconv.ParseInt(resp.Result.List[0].UpdatedTime, 10, 64)
	updateTime := time.UnixMilli(updatedTimeInt)
	return &core.PositionState{
		Symbol:        resp.Result.List[0].Symbol,
		Side:          side,
		Size:          size,
		AvgPrice:      avgPrice,
		UnrealizedPnl: uPnl,
		RealizedPnl:   rPnl,
		CreatedTime:   createdTime,
		UpdatedTime:   updateTime,
	}, nil
}
