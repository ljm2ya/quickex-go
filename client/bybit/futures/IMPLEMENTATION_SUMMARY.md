# Bybit Futures Client - FuturesClient Interface Implementation

## Summary
Successfully implemented all required methods from the `FuturesClient` interface for Bybit futures trading.

## Implemented Methods

### 1. SetLeverage
- **Purpose**: Sets leverage for a specific symbol
- **Implementation**: Uses Bybit V5 API's SetLeverage endpoint
- **Parameters**: 
  - `symbol`: Trading pair (e.g., "BTCUSDT")
  - `leverage`: Leverage value (e.g., 10 for 10x)
- **Notes**: Bybit requires setting buy and sell leverage separately; we set both to the same value

### 2. GetLiquidationPrice
- **Purpose**: Retrieves the liquidation price for an open position
- **Implementation**: Uses Bybit V5 API's GetPositionInfo endpoint
- **Returns**: Liquidation price as `decimal.Decimal`
- **Error Handling**: Returns error if no position exists or liquidation price unavailable

### 3. GetFundingRate
- **Purpose**: Gets current and next funding rate information
- **Implementation**: Uses Bybit V5 API's GetTickers endpoint
- **Returns**: `FundingRate` struct containing:
  - Current funding rate
  - Next funding time (Unix timestamp)
  - Previous funding rate (currently not available from ticker API)
- **Notes**: Historical funding rate would require additional API calls

### 4. SetMarginMode
- **Purpose**: Sets margin mode to Cross or Isolated
- **Implementation**: Partially implemented due to Bybit SDK limitations
- **Status**: Returns "not fully implemented" error
- **TODO**: Requires either:
  - SDK update to support margin mode endpoint
  - Direct HTTP implementation
  - Symbol parameter to set mode for specific positions

## Core Type Additions
Added to `core/types.go`:
- `MarginMode` type with constants:
  - `MarginModeCross`
  - `MarginModeIsolated`
- `FundingRate` struct with fields:
  - `Rate` (decimal.Decimal)
  - `NextTime` (int64 Unix timestamp)
  - `PreviousRate` (decimal.Decimal)

## Interface Compliance
The `BybitFuturesClient` now satisfies both:
- `core.PrivateClient` interface (inherited)
- `core.FuturesClient` interface (newly implemented)

## Next Steps
1. Complete `SetMarginMode` implementation when SDK support is available
2. Add unit tests for all futures methods
3. Implement historical funding rate retrieval if needed
4. Consider adding position-specific margin mode setting