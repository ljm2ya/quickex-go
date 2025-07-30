package core

type Exchanges string

const (
	ExchangeUpbit                 Exchanges = "upbit"
	ExchangeBinance               Exchanges = "binance"
	ExchangeBinanceTestnet        Exchanges = "binance-testnet"
	ExchangeBinanceFutures        Exchanges = "binance-futures"
	ExchangeBinanceFuturesTestnet Exchanges = "binance-futures-testnet"
	ExchangeBybit                 Exchanges = "bybit"
	ExchangeBybitFutures          Exchanges = "bybit-futures"
)