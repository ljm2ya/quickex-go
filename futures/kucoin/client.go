package kucoin

import (
	"context"

	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/api"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/common/logger"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/generate/futures/futurespublic"
	"github.com/Kucoin/kucoin-universal-sdk/sdk/golang/pkg/types"
)

// KuCoin Architecture:
// - WebSocket: Used for real-time subscriptions (market data, order updates, balance changes)
// - REST API: Used for actions (placing orders, canceling orders, querying balances)
// - Sync methods: Synchronous REST calls that wait for order processing before returning

type KucoinFuturesClient struct {
	client         api.Client
	wsPublicClient futurespublic.FuturesPublicWS
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewClient(apiKey, apiSecret, passphrase string) *KucoinFuturesClient {
	// Set up default logger
	defaultLogger := logger.NewDefaultLogger()
	logger.SetLogger(defaultLogger)

	// Create client using the SDK's pattern
	option := types.NewClientOptionBuilder().
		WithKey(apiKey).
		WithSecret(apiSecret).
		WithPassphrase(passphrase).
		WithFuturesEndpoint(types.GlobalFuturesApiEndpoint).
		Build()

	sdkClient := api.NewClient(option)

	ctx, cancel := context.WithCancel(context.Background())

	client := &KucoinFuturesClient{
		client: sdkClient,
		ctx:    ctx,
		cancel: cancel,
	}

	return client
}

func (c *KucoinFuturesClient) Connect(ctx context.Context) (int64, error) {
	// No need to connect for futures client as we only use public WebSocket on demand
	return 0, nil
}

func (c *KucoinFuturesClient) Close() error {
	if c.cancel != nil {
		c.cancel()
	}

	if c.wsPublicClient != nil {
		c.wsPublicClient.Stop()
	}

	return nil
}
