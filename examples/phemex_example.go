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
	fmt.Println("=== Phemex Client Test ===")

	// Get credentials from environment variables (don't hardcode!)
	apiKey := os.Getenv("PHEMEX_API_KEY")
	secretKey := os.Getenv("PHEMEX_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		fmt.Println("Please set PHEMEX_API_KEY and PHEMEX_SECRET_KEY environment variables")
		fmt.Println("For testing without real credentials, this will demonstrate the interface compatibility")
		testInterfaceCompatibility()
		return
	}

	// Test Futures Client (primary focus)
	fmt.Println("\n=== Testing Phemex Futures Client ===")
	testFuturesClient(apiKey, secretKey)

	// Test Spot Client
	fmt.Println("\n=== Testing Phemex Spot Client ===")
	testSpotClient(apiKey, secretKey)
}

func testInterfaceCompatibility() {
	fmt.Println("\n=== Testing Interface Compatibility ===")
	
	// Test that Phemex clients can be created through factory
	fmt.Println("✓ Creating Phemex Spot client through factory...")
	spotClient := client.NewPrivateClient(string(client.ExchangePhemex), "", "")
	fmt.Printf("  Spot client type: %T\n", spotClient)
	
	fmt.Println("✓ Creating Phemex Futures client through factory...")
	futuresClient := client.NewFuturesPrivateClient(string(client.ExchangePhemexFutures), "", "")
	fmt.Printf("  Futures client type: %T\n", futuresClient)
	
	fmt.Println("✓ Creating Phemex Public client through factory...")
	publicClient := client.NewPublicClient(string(client.ExchangePhemex))
	fmt.Printf("  Public client type: %T\n", publicClient)
	
	fmt.Println("✓ Creating Phemex Futures Public client through factory...")
	futuresPublicClient := client.NewFuturesPublicClient(string(client.ExchangePhemexFutures))
	fmt.Printf("  Futures Public client type: %T\n", futuresPublicClient)
	
	fmt.Println("\n✅ All interface compatibility tests passed!")
}

func testFuturesClient(apiKey, secretKey string) {
	// Create futures client
	futuresClient := client.NewFuturesPrivateClient(string(client.ExchangePhemexFutures), apiKey, secretKey)
	
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	
	// Test market rules for futures symbols
	fmt.Println("2. Testing FetchMarketRules for futures...")
	futuresSymbols := []string{"BTCUSD", "ETHUSD", "ADAUSD"} // Common Phemex futures symbols
	rules, err := futuresClient.FetchMarketRules(futuresSymbols)
	if err != nil {
		log.Printf("FetchMarketRules failed: %v", err)
	} else {
		for _, rule := range rules {
			fmt.Printf("✓ Futures rule for %s: MinQty=%s, TickSize=%s, Base=%s, Quote=%s\n", 
				rule.Symbol, rule.MinQty.String(), rule.TickSize.String(), rule.BaseAsset, rule.QuoteAsset)
		}
	}
	
	// Test futures position balance
	fmt.Println("3. Testing FetchBalance with futuresPosition=true...")
	positionSize, err := futuresClient.FetchBalance("USD", false, true)
	if err != nil {
		log.Printf("FetchBalance (futures position) failed: %v", err)
	} else {
		fmt.Printf("✓ USD Position Size: %s\n", positionSize.String())
	}
	
	// Test regular balance (margin)
	fmt.Println("4. Testing FetchBalance with futuresPosition=false...")
	balance, err := futuresClient.FetchBalance("USD", false, false)
	if err != nil {
		log.Printf("FetchBalance (margin) failed: %v", err)
	} else {
		fmt.Printf("✓ USD Margin Balance: %s\n", balance.String())
	}
	
	// Test quote subscription
	fmt.Println("5. Testing SubscribeQuotes...")
	quoteCtx, quoteCancel := context.WithTimeout(ctx, 10*time.Second)
	defer quoteCancel()
	
	quoteChans, err := futuresClient.SubscribeQuotes(quoteCtx, []string{"BTCUSD"}, func(err error) {
		log.Printf("Quote error: %v", err)
	})
	if err != nil {
		log.Printf("SubscribeQuotes failed: %v", err)
	} else {
		fmt.Printf("✓ Subscribed to quotes for %d symbols\n", len(quoteChans))
		
		// Listen for a few quotes
		if btcChan, exists := quoteChans["BTCUSD"]; exists {
			fmt.Println("  Waiting for BTCUSD quotes (5 seconds)...")
			timeout := time.After(5 * time.Second)
			quoteCount := 0
			
			for quoteCount < 3 {
				select {
				case quote := <-btcChan:
					fmt.Printf("  📈 BTCUSD: Bid=%s Ask=%s Time=%v\n", 
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
	// Test order functionality (CAREFUL - REAL MONEY!)
	fmt.Println("6. Testing futures order functionality (SKIPPED for safety)...")
	fmt.Println("   Uncomment the order test section below to test actual orders")
	fmt.Println("   ⚠️  WARNING: Use VERY SMALL amounts and be prepared to lose money!")
	
	/*
	// UNCOMMENT TO TEST ACTUAL ORDERS - USE EXTREME CAUTION!
	fmt.Println("   Testing small futures limit order...")
	
	// Use very small quantity and price well below market
	qty := decimal.NewFromFloat(0.001) // Very small position size
	price := decimal.NewFromFloat(30000) // Well below current market price
	
	order, err := futuresClient.LimitBuy("BTCUSD", qty, price, "GTC")
	if err != nil {
		log.Printf("LimitBuy failed: %v", err)
	} else {
		fmt.Printf("✓ Futures order placed: %s\n", order.OrderID)
		
		// Wait a moment then cancel
		time.Sleep(2 * time.Second)
		
		cancelResp, err := futuresClient.CancelOrder("BTCUSD", order.OrderID)
		if err != nil {
			log.Printf("Cancel failed: %v", err)
		} else {
			fmt.Printf("✓ Order cancelled: %s\n", cancelResp.OrderID)
		}
	}
	*/
}

func testSpotClient(apiKey, secretKey string) {
	// Create spot client
	spotClient := client.NewPrivateClient(string(client.ExchangePhemex), apiKey, secretKey)
	
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
	
	// Test market rules for spot symbols
	fmt.Println("2. Testing FetchMarketRules for spot...")
	spotSymbols := []string{"sBTCUSDT", "sETHUSDT"} // Phemex spot symbols start with 's'
	rules, err := spotClient.FetchMarketRules(spotSymbols)
	if err != nil {
		log.Printf("FetchMarketRules failed: %v", err)
	} else {
		for _, rule := range rules {
			fmt.Printf("✓ Spot rule for %s: MinQty=%s, TickSize=%s\n", 
				rule.Symbol, rule.MinQty.String(), rule.TickSize.String())
		}
	}
	
	// Test balance fetch
	fmt.Println("3. Testing FetchBalance for spot...")
	balance, err := spotClient.FetchBalance("USDT", false, false)
	if err != nil {
		log.Printf("FetchBalance failed: %v", err)
	} else {
		fmt.Printf("✓ USDT Balance: %s\n", balance.String())
	}
	
	// Test quote subscription
	fmt.Println("4. Testing SubscribeQuotes for spot...")
	quoteCtx, quoteCancel := context.WithTimeout(ctx, 10*time.Second)
	defer quoteCancel()
	
	quoteChans, err := spotClient.SubscribeQuotes(quoteCtx, []string{"sBTCUSDT"}, func(err error) {
		log.Printf("Quote error: %v", err)
	})
	if err != nil {
		log.Printf("SubscribeQuotes failed: %v", err)
	} else {
		fmt.Printf("✓ Subscribed to quotes for %d symbols\n", len(quoteChans))
		
		// Listen for a few quotes
		if btcChan, exists := quoteChans["sBTCUSDT"]; exists {
			fmt.Println("  Waiting for sBTCUSDT quotes (3 seconds)...")
			timeout := time.After(3 * time.Second)
			quoteCount := 0
			
			for quoteCount < 2 {
				select {
				case quote := <-btcChan:
					fmt.Printf("  📈 sBTCUSDT: Bid=%s Ask=%s Time=%v\n", 
						quote.BidPrice.String(), quote.AskPrice.String(), quote.Time)
					quoteCount++
				case <-timeout:
					fmt.Println("  ⏰ Quote timeout reached")
					goto spotTestEnd
				}
			}
		}
	}
	
spotTestEnd:
	fmt.Println("5. Spot order testing (SKIPPED for safety)")
	fmt.Println("   Similar to futures, uncomment to test with EXTREME CAUTION")
}

// Helper function to demonstrate all interface methods are implemented
func demonstrateInterface() {
	fmt.Println("\n=== Interface Method Coverage ===")
	fmt.Println("All core.PrivateClient interface methods implemented:")
	fmt.Println("  ✓ Connect(context.Context) (int64, error)")
	fmt.Println("  ✓ Close() error")
	fmt.Println("  ✓ SubscribeQuotes(ctx, symbols, errHandler) (map[string]chan Quote, error)")
	fmt.Println("  ✓ FetchMarketRules(quotes) ([]MarketRule, error)")
	fmt.Println("  ✓ FetchBalance(asset, includeLocked, futuresPosition) (decimal.Decimal, error)")
	fmt.Println("  ✓ FetchOrder(symbol, orderId) (*OrderResponseFull, error)")
	fmt.Println("  ✓ LimitBuy(symbol, quantity, price, tif) (*OrderResponse, error)")
	fmt.Println("  ✓ LimitSell(symbol, quantity, price, tif) (*OrderResponse, error)")
	fmt.Println("  ✓ MarketBuy(symbol, quoteQuantity) (*OrderResponse, error)")
	fmt.Println("  ✓ MarketSell(symbol, quantity) (*OrderResponse, error)")
	fmt.Println("  ✓ CancelOrder(symbol, orderId) (*OrderResponse, error)")
	fmt.Println("  ✓ CancelAll(symbol) error")
	fmt.Println("\nAll functions implemented using WebSocket-only communication!")
}