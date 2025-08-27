package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/ljm2ya/quickex-go/client"
	"github.com/shopspring/decimal"
)

func main() {
	// Get credentials from environment variables (recommended)
	apiKey := os.Getenv("EXCHANGE_API_KEY")
	secretOrPath := os.Getenv("EXCHANGE_SECRET") // Can be hex string or PEM file path

	if apiKey == "" || secretOrPath == "" {
		log.Fatal("Please set EXCHANGE_API_KEY and EXCHANGE_SECRET environment variables")
	}

	// Example 1: Create a Bybit spot client
	fmt.Println("=== Creating Bybit Spot Client ===")
	spotClient := client.NewPrivateClient(
		string(client.ExchangeBybit),
		apiKey,
		secretOrPath,
	)

	// Connect to the exchange
	ctx := context.Background()
	_, err := spotClient.Connect(ctx)
	if err != nil {
		log.Printf("Failed to connect to Bybit spot: %v", err)
	} else {
		fmt.Println("✓ Connected to Bybit spot successfully")
		
		// Get balance
		balance, err := spotClient.FetchBalance("USDT", true, false)
		if err != nil {
			log.Printf("Failed to get balance: %v", err)
		} else {
			fmt.Printf("USDT Balance: %s\n", balance.String())
		}
		
		spotClient.Close()
	}

	// Example 2: Create a Binance futures client (testnet)
	fmt.Println("\n=== Creating Binance Futures Testnet Client ===")
	futuresClient := client.NewFuturesPrivateClient(
		string(client.ExchangeBinanceFuturesTestnet),
		apiKey,
		secretOrPath, // Library auto-detects if this is a PEM file path
	)

	_, err = futuresClient.Connect(ctx)
	if err != nil {
		log.Printf("Failed to connect to Binance futures testnet: %v", err)
	} else {
		fmt.Println("✓ Connected to Binance futures testnet successfully")
		futuresClient.Close()
	}

	// Example 3: Place a small test order (BE CAREFUL!)
	fmt.Println("\n=== Order Placement Example ===")
	fmt.Println("⚠️  This is just an example - be careful with real trading!")
	
	// Calculate a small order
	price := decimal.NewFromFloat(30000.0)      // Well below market
	quantity := decimal.NewFromFloat(0.001)     // Small amount
	orderValue := price.Mul(quantity)
	
	fmt.Printf("Example order: %s BTC at $%s = $%s total\n", 
		quantity.String(), price.String(), orderValue.String())
	
	// Note: Uncomment below to actually place order (use testnet first!)
	/*
	order, err := spotClient.LimitBuy("BTCUSDT", quantity, price, "GTC")
	if err != nil {
		log.Printf("Order failed: %v", err)
	} else {
		fmt.Printf("✓ Order placed successfully: %s\n", order.OrderID)
	}
	*/

	fmt.Println("\n=== Examples completed ===")
	fmt.Println("For actual trading:")
	fmt.Println("1. Start with testnet endpoints")
	fmt.Println("2. Use small amounts for testing") 
	fmt.Println("3. Implement proper error handling")
	fmt.Println("4. Monitor balances and positions")
}