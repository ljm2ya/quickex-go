package bybit

import (
	"github.com/ljm2ya/quickex-go/core"
)

// Compile-time check to ensure BybitFuturesClient implements all required interfaces
var (
	_ core.PrivateClient = (*BybitFuturesClient)(nil)
	_ core.FuturesClient = (*BybitFuturesClient)(nil)
)