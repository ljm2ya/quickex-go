package binance

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/ljm2ya/quickex-go/core"
)

func (b *BinanceClient) GetBalance(asset string, includeLocked bool) (float64, error) {
	ts := time.Now().UnixMilli()
	req := map[string]interface{}{
		"id":     nextWSID(),
		"method": "account.status",
		"params": map[string]interface{}{
			"timestamp": ts,
		},
	}
	root, err := b.SendRequest(req)
	if err != nil {
		return 0, fmt.Errorf("failed account.status: %w", err)
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal(root["result"], &resultMap); err != nil {
		return 0, err
	}
	bals, _ := resultMap["balances"].([]interface{})
	for _, bal := range bals {
		balMap, _ := bal.(map[string]interface{})
		resAsset := core.StringFromMap(balMap, "asset")
		if resAsset == asset {
			free := core.ParseStringFloat(core.StringFromMap(balMap, "free"))
			locked := core.ParseStringFloat(core.StringFromMap(balMap, "locked"))
			if includeLocked {
				return free + locked, nil
			} else {
				return free, nil
			}
		}
	}
	return 0, nil
}

// FetchBalance implements core.PrivateClient interface
func (b *BinanceClient) FetchBalance(asset string, includeLocked bool) (decimal.Decimal, error) {
	balance, err := b.GetBalance(asset, includeLocked)
	if err != nil {
		return decimal.Zero, err
	}
	return decimal.NewFromFloat(balance), nil
}
