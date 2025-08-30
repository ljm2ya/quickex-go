package futures

import (
	"github.com/ljm2ya/quickex-go/client/kucoin"
	"github.com/ljm2ya/quickex-go/client/kucoin/common"
)

// PrivateWebSocket is a wrapper around the common implementation for futures
type PrivateWebSocket struct {
	*kucoin.PrivateWebSocket
}

// NewPrivateWebSocket creates a new private WebSocket connection for futures
func NewPrivateWebSocket(apiKey, apiSecret, apiPassphrase string, serverTimeDelta int64) *PrivateWebSocket {
	baseWS := kucoin.NewPrivateWebSocketWithMarket(
		apiKey,
		apiSecret,
		apiPassphrase,
		serverTimeDelta,
		common.MarketTypeFutures,
	)

	return &PrivateWebSocket{
		PrivateWebSocket: baseWS,
	}
}

// Type aliases for backward compatibility
type OrderWSRequest = common.OrderWSRequest
type OrderWSResponse = common.OrderWSResponse