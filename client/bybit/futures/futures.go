package bybit

import (
	"fmt"
	"strconv"

	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// SetLeverage sets the leverage for a specific symbol
func (c *BybitFuturesClient) SetLeverage(symbol string, leverage int) error {
	// Bybit V5 API requires buy and sell leverage to be set separately
	buyLeverage := strconv.Itoa(leverage)
	sellLeverage := strconv.Itoa(leverage)

	params := &bybit.V5SetLeverageParam{
		Category:     bybit.CategoryV5Linear,
		Symbol:       bybit.SymbolV5(symbol),
		BuyLeverage:  buyLeverage,
		SellLeverage: sellLeverage,
	}

	_, err := c.client.V5().Position().SetLeverage(*params)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	return nil
}

// GetLiquidationPrice gets the liquidation price for the current position
func (c *BybitFuturesClient) GetLiquidationPrice(symbol string) (decimal.Decimal, error) {
	params := &bybit.V5GetPositionInfoParam{
		Category: bybit.CategoryV5Linear,
		Symbol:   &symbol,
	}

	resp, err := c.client.V5().Position().GetPositionInfo(*params)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get position info: %w", err)
	}

	if len(resp.Result.List) == 0 {
		return decimal.Zero, fmt.Errorf("no position found for symbol %s", symbol)
	}

	position := resp.Result.List[0]
	liqPrice := position.LiqPrice
	if liqPrice == "" {
		return decimal.Zero, fmt.Errorf("no liquidation price available")
	}

	liqPriceDecimal, err := decimal.NewFromString(liqPrice)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to parse liquidation price: %w", err)
	}

	return liqPriceDecimal, nil
}

// GetFundingRate gets the funding rate for a specific symbol
func (c *BybitFuturesClient) GetFundingRate(symbol string) (*core.FundingRate, error) {
	// Get current funding rate
	params := &bybit.V5GetTickersParam{
		Category: bybit.CategoryV5Linear,
		Symbol:   &symbol,
	}

	resp, err := c.client.V5().Market().GetTickers(*params)
	if err != nil {
		return nil, fmt.Errorf("failed to get tickers: %w", err)
	}

	if len(resp.Result.LinearInverse.List) == 0 {
		return nil, fmt.Errorf("no ticker found for symbol %s", symbol)
	}

	ticker := resp.Result.LinearInverse.List[0]

	// Parse funding rate
	fundingRate, err := decimal.NewFromString(ticker.FundingRate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse funding rate: %w", err)
	}

	// Get next funding time
	nextFundingTime := ticker.NextFundingTime
	if nextFundingTime == "" {
		return nil, fmt.Errorf("no next funding time available")
	}

	// Parse next funding time (Bybit returns milliseconds)
	nextFundingTimeMs, err := strconv.ParseInt(nextFundingTime, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse next funding time: %w", err)
	}

	// Get historical funding rate if available
	prevFundingRate := decimal.Zero
	if ticker.PrevPrice24H != "" {
		// Note: Bybit doesn't provide previous funding rate directly in ticker
		// You might need to call historical funding rate API for this
		// For now, we'll leave it as zero
	}

	return &core.FundingRate{
		Rate:         fundingRate,
		NextTime:     nextFundingTimeMs / 1000, // Convert to seconds
		PreviousRate: prevFundingRate,
	}, nil
}

// SetMarginMode sets the margin mode (Cross or Isolated)
func (c *BybitFuturesClient) SetMarginMode(symbol string, mode core.MarginMode) error {
	// Map our MarginMode to Bybit's trade mode
	// Bybit V5 uses 0 for cross margin and 1 for isolated margin
	var tradeModeInt int
	switch mode {
	case core.MarginModeCross:
		tradeModeInt = 0
	case core.MarginModeIsolated:
		tradeModeInt = 1
	default:
		return fmt.Errorf("unsupported margin mode: %s", mode)
	}

	// Note: Bybit V5 API requires setting margin mode per position
	// For now, we'll implement a simplified version that assumes BTCUSDT
	// In a production system, you might want to set this for all active positions
	_ = map[string]interface{}{
		"category":  "linear",
		"tradeMode": tradeModeInt,
		"symbol":    "BTCUSDT", // This should ideally be parameterized
	}

	// Note: The Bybit Go SDK might not have this endpoint yet
	// You may need to implement a raw HTTP call or update the SDK
	// For now, we'll return a not implemented error
	return fmt.Errorf("SetMarginMode not fully implemented: Bybit SDK support pending")
}
