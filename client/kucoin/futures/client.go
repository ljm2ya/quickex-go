package futures

import (
	"context"
	"fmt"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/api"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/types"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

type KucoinFuturesClient struct {
	client        *api.DefaultClient
	apiKey        string
	apiSecret     string
	apiPassphrase string

	wsService api.KucoinWSService
	privateWS *PrivateWebSocket // Private WebSocket for order placement

	multiplierMap   map[string]decimal.Decimal
	serverTimeDelta int64
}

func NewClient(apiKey, apiSecret, apiPassphrase string) *KucoinFuturesClient {
	// Disable SDK logs by default
	DisableKuCoinFuturesLogs()

	// Configure HTTP transport options
	httpOption := types.NewTransportOptionBuilder().
		SetKeepAlive(true).
		SetMaxIdleConnsPerHost(10).
		Build()

	// Configure WebSocket options
	wsOption := types.NewWebSocketClientOptionBuilder().
		Build()

	option := types.NewClientOptionBuilder().
		WithKey(apiKey).
		WithSecret(apiSecret).
		WithPassphrase(apiPassphrase).
		WithFuturesEndpoint(types.GlobalFuturesApiEndpoint).
		WithTransportOption(httpOption).
		WithWebSocketClientOption(wsOption).
		Build()

	client := api.NewClient(option)
	wsService := client.WsService()

	return &KucoinFuturesClient{
		client:        client,
		apiKey:        apiKey,
		apiSecret:     apiSecret,
		apiPassphrase: apiPassphrase,
		wsService:     wsService,
	}
}

func (c *KucoinFuturesClient) Connect(ctx context.Context) (int64, error) {
	// Get server time to calculate time delta
	restService := c.client.RestService()
	futuresService := restService.GetFuturesService()
	marketAPI := futuresService.GetMarketAPI()

	// Make API call - GetServerTime doesn't need request parameters
	resp, err := marketAPI.GetServerTime(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get futures server time: %w", err)
	}

	// Extract server time from response
	serverTime := resp.Data
	localTime := time.Now().UnixMilli()
	c.serverTimeDelta = localTime - serverTime

	// Initialize and connect private WebSocket for order placement with server time delta
	retryCount := 0
	c.privateWS = NewPrivateWebSocket(c.apiKey, c.apiSecret, c.apiPassphrase, c.serverTimeDelta)
	for {
		if err := c.privateWS.Connect(); err != nil {
			if retryCount >= 5 {
				return 0, fmt.Errorf("failed to connect private WebSocket: %w", err)
			}
			retryCount += 1
			time.Sleep(time.Second)
			continue
		}
		break
	}

	c.multiplierMap = make(map[string]decimal.Decimal)
	symbolResp, err := c.GetAllSymbols()
	if err != nil {
		return 0, fmt.Errorf("kucoin-futures: %w", err)
	}
	for _, info := range symbolResp.Data {
		c.multiplierMap[info.Symbol] = decimal.NewFromFloat(info.Multiplier) // kucoin uses custom multiplier quantity
	}

	return c.serverTimeDelta, nil
}

func (c *KucoinFuturesClient) Close() error {
	var err error

	// Close private WebSocket connection
	if c.privateWS != nil {
		err = c.privateWS.Close()
	}

	// Since each SubscribeQuotes creates its own WebSocket connection,
	// and they are managed by their own contexts, there's nothing else to clean up.
	return err
}

// ToSymbol converts asset and quote to exchange-specific symbol format
// KuCoin Futures format: BTCUSDTM (no separator, M suffix for futures)
func (c *KucoinFuturesClient) ToSymbol(asset, quote string) string {
	return asset + quote + "M"
}

// ToAsset extracts the asset from a symbol (reverse of ToSymbol)
func (c *KucoinFuturesClient) ToAsset(symbol string) string {
	// KuCoin Futures uses concatenation with "M" suffix: BTCUSDTM
	// First, remove the "M" suffix if present
	if len(symbol) > 1 && symbol[len(symbol)-1:] == "M" {
		symbol = symbol[:len(symbol)-1]
	}

	// Common quote currencies to check (ordered by likelihood)
	quotes := []string{"USDT", "USD", "BTC", "ETH"}

	for _, quote := range quotes {
		if len(symbol) > len(quote) && symbol[len(symbol)-len(quote):] == quote {
			return symbol[:len(symbol)-len(quote)]
		}
	}

	// If no match found, return the symbol as-is (shouldn't happen with valid symbols)
	return symbol
}

// SubscribeOrderEvents implements core.PrivateClient interface
// Note: KuCoin Futures real-time order events not implemented yet
func (c *KucoinFuturesClient) SubscribeOrderEvents(ctx context.Context, symbols []string, errHandler func(err error)) (<-chan core.OrderEvent, error) {
	return nil, fmt.Errorf("real-time order events not implemented for KuCoin Futures")
}

// SubscribeBalanceEvents implements core.PrivateClient interface
// Note: KuCoin Futures real-time balance events not implemented yet
func (c *KucoinFuturesClient) SubscribeBalanceEvents(ctx context.Context, assets []string, errHandler func(err error)) (<-chan core.BalanceEvent, error) {
	return nil, fmt.Errorf("real-time balance events not implemented for KuCoin Futures")
}

// UnsubscribeOrderEvents implements core.PrivateClient interface
func (c *KucoinFuturesClient) UnsubscribeOrderEvents() error {
	return fmt.Errorf("real-time order events not implemented for KuCoin Futures")
}

// UnsubscribeBalanceEvents implements core.PrivateClient interface
func (c *KucoinFuturesClient) UnsubscribeBalanceEvents() error {
	return fmt.Errorf("real-time balance events not implemented for KuCoin Futures")
}
