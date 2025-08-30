# QuickEx Go

A Go library for cryptocurrency exchange trading with unified interfaces for spot and futures trading across multiple exchanges.

## Supported Exchanges

- **Binance** (Spot & Futures)
- **Bybit** (Spot & Futures)
- **OKX** (Spot & Futures)

## Installation

```bash
go get github.com/ljm2ya/quickex-go
```

## Quick Start

### 1. Basic Client Creation

```go
package main

import (
    "github.com/ljm2ya/quickex-go/client"
)

func main() {
    // Create a spot trading client
    spotClient := client.NewPrivateClient(
        string(client.ExchangeBybit),
        "your-api-key",
        "your-api-secret", // Can be hex string or PEM file path
    )
    
    // Create a futures trading client
    futuresClient := client.NewFuturesPrivateClient(
        string(client.ExchangeBinanceFutures),
        "your-api-key",
        "./path/to/private-key.pem", // Auto-detects PEM files
    )
}
```

### 2. Private Key Loading

The library automatically detects whether your secret is a hex string or PEM file path:

```go
// Using hex string (traditional)
client := client.NewPrivateClient(
    string(client.ExchangeBybit),
    "api-key",
    "abcd1234...", // Hex string
)

// Using PEM file (new feature)
client := client.NewPrivateClient(
    string(client.ExchangeBinanceFutures),
    "api-key", 
    "./keys/binance-private.pem", // PEM file path
)
```

**Path Detection Rules:**
- Contains `.pem` extension → Treated as file path
- Starts with `/`, `./`, or `../` → Treated as file path
- Everything else → Treated as hex string

### 3. Trading Operations

```go
import (
    "context"
    "github.com/shopspring/decimal"
)

// Connect to exchange
ctx := context.Background()
_, err := client.Connect(ctx)
if err != nil {
    panic(err)
}
defer client.Close()

// Place a limit buy order
qty := decimal.NewFromFloat(0.001)
price := decimal.NewFromFloat(50000.0)

order, err := client.LimitBuy("BTCUSDT", qty, price, "GTC")
if err != nil {
    panic(err)
}

// Get account balance
balance, err := client.GetBalance("USDT", true)
if err != nil {
    panic(err)
}
```

### 4. Available Exchanges

```go
// Spot Trading
client.ExchangeBinance               // Binance Spot (Production)
client.ExchangeBinanceTestnet        // Binance Spot (Testnet)
client.ExchangeBybit                 // Bybit Spot
client.ExchangeOKX                   // OKX Spot

// Futures Trading  
client.ExchangeBinanceFutures        // Binance Futures (Production) 
client.ExchangeBinanceFuturesTestnet // Binance Futures (Testnet)
client.ExchangeBybitFutures          // Bybit Futures
client.ExchangeOKXFutures            // OKX Futures
```

## Key Features

- **Unified Interface**: Same API across all exchanges
- **Automatic Key Loading**: Supports both hex strings and PEM files
- **Decimal Precision**: Uses `shopspring/decimal` for financial calculations
- **WebSocket Support**: Real-time data and order management
- **Balance Management**: Automatic balance checking and order sizing
- **Error Handling**: Comprehensive error reporting and recovery

## Security Best Practices

1. **Never commit private keys to version control**
2. **Store PEM files outside your repository**
3. **Use environment variables for API credentials**
4. **Test with small amounts first**

```go
// Recommended approach
apiKey := os.Getenv("EXCHANGE_API_KEY")
keyPath := os.Getenv("EXCHANGE_KEY_PATH")

client := client.NewPrivateClient(
    string(client.ExchangeBinanceFutures),
    apiKey,
    keyPath,
)
```

## Examples

### Spot Trading Example

```go
// Check balance before trading
balance, err := client.GetBalance("USDT", true)
if err != nil {
    return err
}

// Calculate order size (10% of balance)
availableUSDT := decimal.NewFromFloat(balance)
orderValue := availableUSDT.Mul(decimal.NewFromFloat(0.1))
price := decimal.NewFromFloat(50000.0)
quantity := orderValue.Div(price)

// Place order
order, err := client.LimitBuy("BTCUSDT", quantity, price, "GTC")
if err != nil {
    return err
}

fmt.Printf("Order placed: %s\n", order.OrderID)
```

### Futures Trading with Balance Check

```go
// Get account info
account, err := client.GetAccount()
if err != nil {
    return err
}

// Check margin requirements
minOrderValue := decimal.NewFromFloat(5.0) // $5 minimum
if account.TotalWalletBalance < minOrderValue.InexactFloat64() {
    return fmt.Errorf("insufficient balance")
}

// Place futures order
qty := decimal.NewFromFloat(0.001)
price := decimal.NewFromFloat(45000.0)

order, err := client.LimitBuy("BTCUSDT", qty, price, "GTC")
if err != nil {
    return err
}
```

## Testing

The library includes comprehensive tests, but **test files are excluded from git** because they contain hardcoded API credentials.

To run tests locally:
1. Create your own test files based on the patterns in the codebase
2. Add your API credentials
3. Run with: `go test ./...`

## License

MIT License - see LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests (they won't be committed due to .gitignore)
5. Submit a pull request

## Support

For issues and questions, please open a GitHub issue.