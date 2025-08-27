package futures

import (
	"context"
	"fmt"
	"time"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/api"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/types"
)

type KucoinFuturesClient struct {
	client       *api.DefaultClient
	apiKey       string
	apiSecret    string
	apiPassphrase string
	
	wsService       api.KucoinWSService
	
	serverTimeDelta int64
}

func NewClient(apiKey, apiSecret, apiPassphrase string) *KucoinFuturesClient {
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
		client:          client,
		apiKey:          apiKey,
		apiSecret:       apiSecret,
		apiPassphrase:   apiPassphrase,
		wsService:       wsService,
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
	
	return c.serverTimeDelta, nil
}

func (c *KucoinFuturesClient) Close() error {
	// Since each SubscribeQuotes creates its own WebSocket connection,
	// and they are managed by their own contexts, there's nothing to clean up here.
	// The connections will be closed when their contexts are cancelled.
	return nil
}