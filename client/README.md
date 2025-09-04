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

## Configuration Structure

### test_config.toml Format
```toml
max_latency_ms = 5000        # Maximum allowed API latency
test_asset = "DOGE"          # Base asset for testing
test_quote = "USDT"          # Quote currency for testing
test_amount_usd_spot = 10.0  # Test amount for spot trading
test_amount_usd_futures = 20.0  # Test amount for futures trading

[binance]
api_key = "your_api_key"
credentials_file = "path/to/credentials"  # Path to secret/PEM file
is_raw_credentials = false   # true = plain text, false = ED25519 PEM
spot = true                  # Enable spot trading tests
futures = false              # Enable futures trading tests

[bybit]
api_key = "your_api_key"
credentials_file = "path/to/credentials"
is_raw_credentials = true
spot = false
futures = true

[kucoin]
api_key = "your_api_key"  
credentials_file = "path/to/credentials"
is_raw_credentials = true
spot = true
futures = true
```

## Quick Start

### 1. Configure Test Settings

Create a `test_config.toml` file with your settings (see format above).

### 2. Create Credential Files

**For Raw Credentials** (`is_raw_credentials = true`):
```bash
# Bybit/Upbit - Single line
echo "your-secret-key" > testdata/bybit_creds.txt

# KuCoin - Two lines
echo "your-secret-key" > testdata/kucoin_creds.txt
echo "your-passphrase" >> testdata/kucoin_creds.txt
```

**For ED25519** (`is_raw_credentials = false` - Binance only):
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

# Test FetchQuotes implementation
go test -v -run TestFetchQuotes ./client

# Test specific exchange
go test -v -run "TestOrderScenarios/kucoin" ./client

# Note: Both spot and futures use the same test methods,
# configuration determines which type is tested
```

## Loading Configuration in Tests

### Basic Pattern
```go
// 1. Load config
config, err := LoadTestConfig("test_config.toml")
if err != nil {
    t.Fatalf("Failed to load config: %v", err)
}

// 2. Create performance tracker
tracker := NewPerformanceTracker()

// 3. Load test clients for all enabled exchanges
contexts, err := LoadTestClients(t, config, tracker, "spot") // or "futures"
if err != nil {
    t.Skipf("No exchanges available: %v", err)
}

// 4. Run tests
for _, tc := range contexts {
    defer tc.Client.Close()
    // Your test code here
}

// 5. Show performance summary
tracker.Summary(t)
```

## Test Context Usage

The `TestContext` provides everything needed for testing:

```go
type TestContext struct {
    Name        string              // Exchange name
    Client      core.PrivateClient  // Connected client
    Symbol      string              // Formatted symbol
    Tracker     *PerformanceTracker // Performance tracking
    Config      *TestConfig         // Test configuration
    TradingType string              // "spot" or "futures"
}

// Methods:
tc.Track()                              // Start timing
tc.ValidateResponse(op, data, err)     // Validate response
tc.GetTradingConfig()                   // Get test asset/quote
```

## Common Test Patterns

### 1. Simple Test
```go
func TestSimple(t *testing.T) {
    config, _ := LoadTestConfig("test_config.toml")
    tracker := NewPerformanceTracker()
    contexts, _ := LoadTestClients(t, config, tracker, "spot")
    
    for _, tc := range contexts {
        defer tc.Client.Close()
        
        tc.Track()
        balance, err := tc.Client.FetchBalance(config.TestQuote, false, false)
        if err := tc.ValidateResponse("FetchBalance", balance, err); err != nil {
            t.Error(err)
        }
    }
    
    tracker.Summary(t)
}
```

### 2. Manual Client Creation
```go
func TestManualClient(t *testing.T) {
    config, _ := LoadTestConfig("test_config.toml")
    
    // Load credentials
    apiKey, secret, passphrase, err := loadCredentialsFromConfig(config.Kucoin)
    if err != nil {
        t.Skip("Failed to load credentials")
    }
    
    // Create client
    client := NewPrivateClient("kucoin", apiKey, secret, passphrase)
    defer client.Close()
    
    // Connect and test
    client.Connect(context.Background())
}
```

### 3. Testing Specific Exchanges
```go
func TestKucoinOnly(t *testing.T) {
    config, _ := LoadTestConfig("test_config.toml")
    
    if !config.Kucoin.Spot {
        t.Skip("KuCoin spot not enabled")
    }
    
    // Test KuCoin specific features
}
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

## Best Practices

1. **Always use LoadTestConfig** - Don't hardcode credentials
2. **Use TestContext** - Provides consistent test setup
3. **Track Performance** - Use tracker.Record() for all operations  
4. **Validate Responses** - Use tc.ValidateResponse() for automatic checks
5. **Handle Missing Config** - Skip tests gracefully if config missing
6. **Close Clients** - Always defer client.Close()
7. **Check Trading Type** - Verify spot/futures is enabled before testing

## Troubleshooting

### Common Issues
1. **"Failed to load config"** - Check test_config.toml exists
2. **"Failed to load credentials"** - Check credentials_file path
3. **"No exchanges available"** - Enable exchanges in config
4. **"Exceeded max latency"** - Increase max_latency_ms or check connection
5. **"Insufficient balance"** - Need minimum 4 × testAmountUSD (default: $40 USDT)
6. **"Order didn't execute"** - Check if prices match current market

### Logging
```go
// TestContext logs operations automatically
t.Logf("[%s] Operation took %v", tc.Name, latency)

// Performance summary shows all operations
tracker.Summary(t) // Prints average latencies
```

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

- `test_config.toml` - Configuration file (NEW FORMAT)
- `test_common.go` - Test framework with validation
- `test_factory.go` - Multi-exchange client factory
- `client_test.go` - Order tests (⚠️ REAL TRADES)
- `example_config_test.go` - Example test patterns
- `test_fetchquotes.md` - FetchQuotes test documentation

Remember: These tests use **REAL MONEY**. Always double-check prices before running!