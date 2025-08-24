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
	// Get credentials from environment variables (recommended)
	apiKey := os.Getenv("MEXC_API_KEY")
	secretOrPath := os.Getenv("MEXC_SECRET")

	if apiKey == "" || secretOrPath == "" {
		log.Fatal("Please set MEXC_API_KEY and MEXC_SECRET environment variables")
	}

	// Example 1: Create a MEXC spot client
	fmt.Println("=== Creating MEXC Spot Client ===")
	spotClient := client.NewPrivateClient(
		string(client.ExchangeMexc),
		apiKey,
		secretOrPath,
	)

	// Connect to the exchange
	ctx := context.Background()
	_, err := spotClient.Connect(ctx)
	if err != nil {
		log.Printf("Failed to connect to MEXC spot: %v", err)
	} else {
		fmt.Println("✓ Connected to MEXC spot successfully")
		
		// Get balance
		balance, err := spotClient.FetchBalance("USDT", true, false)
		if err != nil {
			log.Printf("Failed to get spot balance: %v", err)
		} else {
			fmt.Printf("USDT Spot Balance: %s\n", balance.String())
		}
		
		// Subscribe to real-time quotes
		symbols := []string{"ADAUSDT", "ETHUSDT"}
		quoteChans, err := spotClient.SubscribeQuotes(ctx, symbols, func(err error) {
			log.Printf("Quote error: %v", err)
		})
		if err != nil {
			log.Printf("Failed to subscribe to quotes: %v", err)
		} else {
			fmt.Printf("✓ Subscribed to %d symbols\n", len(symbols))
			
			// Read some quotes (note: WebSocket may have connection issues)
			go func() {
				for i := 0; i < 3; i++ {
					select {
					case quote := <-quoteChans["ADAUSDT"]:
						fmt.Printf("ADA Quote: Bid=%s Ask=%s Time=%s\n", 
							quote.BidPrice.String(), quote.AskPrice.String(), quote.Time.Format("15:04:05"))
					case <-time.After(3 * time.Second):
						fmt.Println("No ADA quote received within 3 seconds")
					}
				}
			}()
			
			time.Sleep(5 * time.Second) // Wait for some quotes
		}
		
		spotClient.Close()
	}

	// Example 2: MEXC Futures Note
	fmt.Println("\n=== MEXC Futures Note ===")
	fmt.Println("⚠️  MEXC futures trading is discontinued.")
	fmt.Println("    Use futuresPosition=false for spot balance queries.")
	
	// Demonstrate futures parameter handling (will return error)
	_, err = spotClient.FetchBalance("USDT", true, true) // futuresPosition=true
	if err != nil {
		fmt.Printf("✓ Expected error for futures: %v\n", err)
	}

	// Example 3: Order placement example (BE CAREFUL!)
	fmt.Println("\n=== Order Placement Example ===")
	fmt.Println("⚠️  This is just an example - be careful with real trading!")
	
	// Calculate a small test order (example only)
	price := decimal.NewFromFloat(0.50)         // Well below ADA market price (~$0.89)
	quantity := decimal.NewFromFloat(3.0)       // 3 ADA = ~$1.50 total
	orderValue := price.Mul(quantity)
	
	fmt.Printf("Example order: %s ADA at $%s = $%s total\n", 
		quantity.String(), price.String(), orderValue.String())
	
	// Note: Uncomment below to actually place order (be very careful!)
	/*
	if spotClient != nil {
		order, err := spotClient.LimitBuy("ADAUSDT", quantity, price, "GTC")
		if err != nil {
			log.Printf("Order failed: %v", err)
		} else {
			fmt.Printf("✓ Order placed successfully: %s\n", order.OrderID)
			
			// Check order status
			orderDetail, err := spotClient.FetchOrder("ADAUSDT", order.OrderID)
			if err != nil {
				log.Printf("Failed to fetch order: %v", err)
			} else {
				fmt.Printf("Order Status: %s, Executed: %s\n", 
					orderDetail.Status, orderDetail.ExecutedQty.String())
			}
			
			// Cancel the order if still open
			if orderDetail.Status == "OPEN" {
				cancelResult, err := spotClient.CancelOrder("ADAUSDT", order.OrderID)
				if err != nil {
					log.Printf("Cancel failed: %v", err)
				} else {
					fmt.Printf("✓ Order canceled: %s\n", cancelResult.Status)
				}
			}
		}
	}
	*/

	fmt.Println("\n=== Examples completed ===")
	fmt.Println("For actual trading:")
	fmt.Println("1. Test with MEXC testnet if available")
	fmt.Println("2. Use small amounts for testing") 
	fmt.Println("3. Implement proper error handling")
	fmt.Println("4. Monitor balances and positions")
	fmt.Println("5. Set up proper API key permissions (spot/futures)")
}
