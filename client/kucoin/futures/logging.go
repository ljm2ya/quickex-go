package futures

import (
	"github.com/ljm2ya/quickex-go/client/kucoin"
)

// DisableKuCoinFuturesLogs disables all KuCoin SDK logs for futures
func DisableKuCoinFuturesLogs() {
	kucoin.DisableKuCoinLogs()
}

// EnableKuCoinFuturesLogs enables KuCoin SDK logs for futures
func EnableKuCoinFuturesLogs() {
	kucoin.ConfigureKuCoinLogging(true)
}