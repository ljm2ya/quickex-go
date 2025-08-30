package common

import "github.com/ljm2ya/quickex-go/core"

// MapTifToKucoin maps time-in-force values to KuCoin format
func MapTifToKucoin(tif string) string {
	switch tif {
	case string(core.TimeInForceGTC):
		return "GTC"
	case string(core.TimeInForceIOC):
		return "IOC"
	case string(core.TimeInForceFOK):
		return "FOK"
	default:
		return "GTC"
	}
}