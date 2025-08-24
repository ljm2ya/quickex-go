package kucoin

import (
	"context"
	"fmt"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/positions"
	"github.com/shopspring/decimal"
)

// FetchBalance implements PrivateClient interface
func (c *KucoinFuturesClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	ctx := context.Background()
	
	if futuresPosition {
		// Fetch position values using the position endpoint
		futuresService := c.client.RestService().GetFuturesService()
		positionsAPI := futuresService.GetPositionsAPI()
		
		req := &positions.GetPositionListReq{}
		if asset != "" {
			req.Currency = &asset
		}
		
		resp, err := positionsAPI.GetPositionList(req, ctx)
		if err != nil {
			return decimal.Zero, fmt.Errorf("failed to fetch positions: %w", err)
		}
		
		if resp == nil || resp.Data == nil || len(resp.Data) == 0 {
			return decimal.Zero, nil
		}
		
		// Sum up position values for the specified asset
		totalPositionValue := decimal.Zero
		for _, position := range resp.Data {
			if asset == "" || position.SettleCurrency == asset {
				// Use position value (mark price * current position size)
				if position.CurrentQty != 0 {
					posValue := decimal.NewFromFloat(position.MarkPrice).Mul(decimal.NewFromInt(int64(position.CurrentQty)))
					totalPositionValue = totalPositionValue.Add(posValue.Abs())
				}
			}
		}
		
		return totalPositionValue, nil
	}
	
	// For regular balance fetching, we'll use a simpler approach for now
	// The exact account API structure for futures needs verification
	return decimal.Zero, fmt.Errorf("regular balance fetching for futures not yet implemented")
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}