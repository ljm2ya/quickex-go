package kucoin

import (
	"context"
	"fmt"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/api"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/types"
)

type KucoinSpotClient struct {
	client        *api.DefaultClient
	apiKey        string
	apiSecret     string
	apiPassphrase string

	wsService api.KucoinWSService
	privateWS *PrivateWebSocket // Private WebSocket for order placement

	serverTimeDelta int64
}

func NewClient(apiKey, apiSecret, apiPassphrase string) *KucoinSpotClient {
	// Disable SDK logs by default
	DisableKuCoinSDKLogs()

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
		WithSpotEndpoint(types.GlobalApiEndpoint).
		WithTransportOption(httpOption).
		WithWebSocketClientOption(wsOption).
		Build()

	client := api.NewClient(option)
	wsService := client.WsService()

	return &KucoinSpotClient{
		client:        client,
		apiKey:        apiKey,
		apiSecret:     apiSecret,
		apiPassphrase: apiPassphrase,
		wsService:     wsService,
	}
}

func (c *KucoinSpotClient) Connect(ctx context.Context) (int64, error) {
	// Get server time to calculate time delta
	restService := c.client.RestService()
	spotService := restService.GetSpotService()
	marketAPI := spotService.GetMarketAPI()

	// Make API call to get server time
	resp, err := marketAPI.GetServerTime(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get server time: %w", err)
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
				panic(fmt.Errorf("failed to connect private WebSocket: %w", err))
			}
			retryCount += 1
			time.Sleep(time.Second)
			continue
		}
		break
	}

	return c.serverTimeDelta, nil
}

func (c *KucoinSpotClient) Close() error {
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
// KuCoin format: BTC-USDT (hyphen separator)
func (c *KucoinSpotClient) ToSymbol(asset, quote string) string {
	return asset + "-" + quote
}
