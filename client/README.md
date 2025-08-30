# QuickEx Test Documentation

## ⚠️ CRITICAL WARNING: REAL MONEY AT RISK

**These tests execute REAL TRADES on REAL EXCHANGES with REAL MONEY!**

### Hardcoded Test Prices (in `private_client_test.go`)

```go
lowPrice := decimal.NewFromFloat(0.1)    // $0.10 per DOGE
highPrice := decimal.NewFromFloat(1)     // $1.00 per DOGE
```

**Before running tests**, check current DOGE price:
- If DOGE < $0.15: Lower `lowPrice` to $0.01
- If DOGE > $0.50: Raise `highPrice` to $2.00 or higher
- If DOGE > $1.00: Update both prices immediately!

## Quick Start

### 1. Configure `test_config.json`

```json
{
  "spot": {
    "exchanges": {
      "binance": {
        "enabled": true,
        "apiKey": "your-api-key-here",
        "credentialsFile": "./testdata/binance_creds.txt",
        "isRawCredentials": false    // false = ED25519 PEM file
      },
      "kucoin": {
        "enabled": true,
        "apiKey": "your-api-key-here",
        "credentialsFile": "./testdata/kucoin_creds.txt",
        "isRawCredentials": true     // true = plain text file
      }
    },
    "testAsset": "DOGE",
    "testQuote": "USDT",
    "testAmountUSD": 10.0
  },
  "futures": {
    "exchanges": {
      "binance": {
        "enabled": false,
        "apiKey": "your-futures-api-key",
        "credentialsFile": "./testdata/binance_futures_creds.txt",
        "isRawCredentials": false
      },
      "kucoin": {
        "enabled": false,
        "apiKey": "your-futures-api-key",
        "credentialsFile": "./testdata/kucoin_futures_creds.txt",
        "isRawCredentials": true
      }
    },
    "testAsset": "BTC",
    "testQuote": "USDT",
    "testAmountUSD": 100.0
  },
  "maxLatencyMs": 5000
}
```

### 2. Create Credential Files

**For Raw Credentials** (`isRawCredentials: true`):
```bash
# Bybit/Upbit - Single line
echo "your-secret-key" > testdata/bybit_creds.txt

# KuCoin - Two lines
echo "your-secret-key" > testdata/kucoin_creds.txt
echo "your-passphrase" >> testdata/kucoin_creds.txt
```

**For ED25519** (`isRawCredentials: false` - Binance only):
```
testdata/binance_creds.txt:
-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIKD4... (your key)
-----END PRIVATE KEY-----
```

### 3. Run Tests

```bash
# Test spot trading for one exchange
go test -v ./client -run TestOrderScenarios/kucoin

# Test all enabled spot exchanges
go test -v ./client -run TestOrderScenarios

# Check API performance (spot)
go test -v ./client -run TestAPIPerformance

# Note: Both spot and futures use the same test methods,
# configuration determines which type is tested
```

## Exchange Symbol Formats

| Exchange | Spot Format | Futures Format | Example |
|----------|-------------|----------------|------|
| Binance  | No separator | No separator | `BTCUSDT` |
| Bybit    | No separator | No separator | `BTCUSDT` |
| KuCoin   | Hyphen | Hyphen + M suffix | Spot: `BTC-USDT`, Futures: `BTCUSDTM` |
| Upbit    | Reversed + hyphen | N/A | `USDT-BTC` |

## Test Scenarios

1. **Limit Buy → Limit Sell**: Tests limit order execution
2. **IOC Auto-Cancel**: Tests immediate-or-cancel orders
3. **Post-Only → Cancel**: Tests order placement and cancellation
4. **Market Buy → Market Sell**: Tests market order execution
5. **Multiple Orders → Cancel All**: Tests bulk operations

## Safety Checklist

Before running tests:
- [ ] Using test account or account with limited funds
- [ ] Checked current price for test asset (DOGE for spot, BTC for futures)
- [ ] Verified `lowPrice` is well below market (10-20% recommended)
- [ ] Verified `highPrice` is well above market (200-300% recommended) 
- [ ] Have exchange app ready to cancel orders if needed
- [ ] For futures: Understand leverage and margin requirements

## Performance Metrics

Tests automatically track and report:
```
📊 Performance Summary:
  kucoin.Connect: avg=45ms, count=1
  kucoin.FetchMarketRules: avg=123ms, count=1
  kucoin.LimitBuy: avg=89ms, count=5
```

Operations exceeding `maxLatencyMs` (5000ms default) will fail validation.

## Troubleshooting

**"No exchanges available for testing"**
- Enable at least one exchange in config
- Verify credential files exist
- Check API key validity

**"Insufficient balance"**
- Need minimum 4 × testAmountUSD (default: $40 USDT)
- Top up account or reduce testAmountUSD

**"Order didn't execute"**
- Check if prices match current market
- Verify symbol format for exchange
- Ensure market is open

**"Exceeded max latency"**
- Increase maxLatencyMs for slow connections
- Test during less busy hours

## Emergency Stop

If orders execute unexpectedly:
1. Use exchange app/website to cancel all orders
2. Run `CancelAll` through API if possible
3. Disable API key if necessary

## Requirements

- Go 1.19+
- Exchange API keys with trading permissions
- Minimum balance: $40 USDT (4 × testAmountUSD)
- Network connection to exchanges

## Files Overview

- `test_config.json` - Configuration file
- `test_common.go` - Test framework with validation
- `test_factory.go` - Multi-exchange client factory
- `private_client_test.go` - Order tests (⚠️ REAL TRADES)
- `public_client_test.go` - Public API tests (safe)
- `tosymbol_test.go` - Symbol formatting tests

Remember: These tests use **REAL MONEY**. Always double-check prices before running!