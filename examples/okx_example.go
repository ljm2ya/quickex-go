package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ljm2ya/quickex-go/client"
	"github.com/shopspring/decimal"
)

func main() {
	fmt.Println("=== OKX Client Test ===")

	// Get credentials from environment variables (don't hardcode!)
	apiKey := os.Getenv("OKX_API_KEY")
	secretKey := os.Getenv("OKX_SECRET_KEY")
	passphrase := os.Getenv("OKX_PASSPHRASE")

	if apiKey == "" || secretKey == "" || passphrase == "" {
		fmt.Println("Please set OKX_API_KEY, OKX_SECRET_KEY, and OKX_PASSPHRASE environment variables")
		fmt.Println("For testing without real credentials, this will demonstrate the interface compatibility")
		testInterfaceCompatibility()
		return
	}

	// Test Spot Client
	fmt.Println("\n=== Testing OKX Spot Client ===")
	testSpotClient(apiKey, secretKey, passphrase)

	// Test Futures Client  
	fmt.Println("\n=== Testing OKX Futures Client ===")
	testFuturesClient(apiKey, secretKey, passphrase)
}

func testInterfaceCompatibility() {
	fmt.Println("\n=== Testing Interface Compatibility ===")
	
	// Test that OKX clients can be created through factory
	fmt.Println("✓ Creating OKX Spot client through factory...")
	spotClient := client.NewPrivateClient(string(client.ExchangeOKX), "", "", "")
	fmt.Printf("  Spot client type: %T\n", spotClient)
	
	fmt.Println("✓ Creating OKX Futures client through factory...")
	futuresClient := client.NewFuturesPrivateClient(string(client.ExchangeOKXFutures), "", "", "")
	fmt.Printf("  Futures client type: %T\n", futuresClient)
	
	fmt.Println("✓ Creating OKX Public client through factory...")
	publicClient := client.NewPublicClient(string(client.ExchangeOKX))
	fmt.Printf("  Public client type: %T\n", publicClient)
	
	fmt.Println("✓ Creating OKX Futures Public client through factory...")
	futuresPublicClient := client.NewFuturesPublicClient(string(client.ExchangeOKXFutures))
	fmt.Printf("  Futures Public client type: %T\n", futuresPublicClient)
	
	fmt.Println("\n✅ All interface compatibility tests passed!")
}

func testSpotClient(apiKey, secretKey, passphrase string) {
	// Create spot client
	spotClient := client.NewPrivateClient(string(client.ExchangeOKX), apiKey, secretKey, passphrase)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Test connection
	fmt.Println("1. Testing connection...")
	timeDiff, err := spotClient.Connect(ctx)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	fmt.Printf("✓ Connected successfully. Time diff: %d ms\n", timeDiff)
	defer spotClient.Close()
	
	// Test market rules
	fmt.Println("2. Testing FetchMarketRules...")
	rules, err := spotClient.FetchMarketRules([]string{"BTC-USDT"})
	if err != nil {
		log.Printf("FetchMarketRules failed: %v", err)
	} else {
		for _, rule := range rules {
			fmt.Printf("✓ Market rule for %s: MinQty=%s, TickSize=%s\n", 
				rule.Symbol, rule.MinQty.String(), rule.TickSize.String())
		}
	}
	
	// Test balance fetch
	fmt.Println("3. Testing FetchBalance...")
	balance, err := spotClient.FetchBalance("USDT", false, false)
	if err != nil {
		log.Printf("FetchBalance failed: %v", err)
	} else {
		fmt.Printf("✓ USDT Balance: %s\n", balance.String())
	}
	
	// Test quote subscription
	fmt.Println("4. Testing SubscribeQuotes...")
	quoteChans, err := spotClient.SubscribeQuotes(ctx, []string{"BTC-USDT"}, func(err error) {
		log.Printf("Quote error: %v", err)
	})
	if err != nil {
		log.Printf("SubscribeQuotes failed: %v", err)
	} else {
		fmt.Printf("✓ Subscribed to quotes for %d symbols\n", len(quoteChans))
		
		// Listen for a few quotes
		if btcChan, exists := quoteChans["BTC-USDT"]; exists {
			fmt.Println("  Waiting for BTC-USDT quotes (5 seconds)...")
			timeout := time.After(5 * time.Second)
			quoteCount := 0
			
			for quoteCount < 3 {
				select {
				case quote := <-btcChan:
					fmt.Printf("  📈 BTC-USDT: Bid=%s Ask=%s Time=%v\n", 
						quote.BidPrice.String(), quote.AskPrice.String(), quote.Time)
					quoteCount++
				case <-timeout:
					fmt.Println("  ⏰ Quote timeout reached")
					goto nextTest
				}
			}
		}
	}
	
nextTest:
	// Test small order (be careful with real money!)
	fmt.Println("5. Testing small limit order (CAREFUL - REAL MONEY!)...")
	fmt.Println("   Skipping order test for safety. Remove this to test orders.")
	
	/*
	// Uncomment to test actual orders (USE SMALL AMOUNTS!)
	qty := decimal.NewFromFloat(0.001) // Very small amount
	price := decimal.NewFromFloat(30000) // Below market price
	
	order, err := spotClient.LimitBuy("BTC-USDT", qty, price, "GTC")
	if err != nil {
		log.Printf("LimitBuy failed: %v", err)
	} else {
		fmt.Printf("✓ Order placed: %s\n", order.OrderID)
		
		// Cancel the order immediately
		cancelResp, err := spotClient.CancelOrder("BTC-USDT", order.OrderID)
		if err != nil {
			log.Printf("Cancel failed: %v", err)
		} else {
			fmt.Printf("✓ Order cancelled: %s\n", cancelResp.OrderID)
		}
	}
	*/
}

func testFuturesClient(apiKey, secretKey, passphrase string) {
	// Create futures client
	futuresClient := client.NewFuturesPrivateClient(string(client.ExchangeOKXFutures), apiKey, secretKey, passphrase)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Test connection
	fmt.Println("1. Testing connection...")
	timeDiff, err := futuresClient.Connect(ctx)
	if err != nil {
		log.Printf("Connection failed: %v", err)
		return
	}
	fmt.Printf("✓ Connected successfully. Time diff: %d ms\n", timeDiff)
	defer futuresClient.Close()
	
	// Test futures position balance
	fmt.Println("2. Testing FetchBalance with futuresPosition=true...")
	positionSize, err := futuresClient.FetchBalance("BTC", false, true)
	if err != nil {
		log.Printf("FetchBalance (futures position) failed: %v", err)
	} else {
		fmt.Printf("✓ BTC Position Size: %s\n", positionSize.String())
	}
	
	// Test regular balance
	fmt.Println("3. Testing FetchBalance with futuresPosition=false...")
	balance, err := futuresClient.FetchBalance("USDT", false, false)
	if err != nil {
		log.Printf("FetchBalance (margin) failed: %v", err)
	} else {
		fmt.Printf("✓ USDT Margin Balance: %s\n", balance.String())
	}
	
	// Test market rules for futures
	fmt.Println("4. Testing FetchMarketRules for futures...")
	rules, err := futuresClient.FetchMarketRules([]string{"BTC-USDT-SWAP"})
	if err != nil {
		log.Printf("FetchMarketRules failed: %v", err)
	} else {
		for _, rule := range rules {
			fmt.Printf("✓ Futures rule for %s: MinQty=%s, TickSize=%s\n", 
				rule.Symbol, rule.MinQty.String(), rule.TickSize.String())
		}
	}
	
	fmt.Println("5. Testing futures order (SKIPPED for safety)")
	fmt.Println("   Remove this line to test futures orders with EXTREME CAUTION!")
}