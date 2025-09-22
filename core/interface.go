package core

import (
	"context"
	"github.com/shopspring/decimal"
)

type PublicClient interface {
	SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan Quote, error)
	//	SubscribeOrderbook(ctx context.Context, symbols []string, depth int, errHandler func(err error)) (map[string]<-chan Orderbook, error)
	FetchQuotes(symbols []string) (map[string]Quote, error)

	ToSymbol(asset, quote string) string
	ToAsset(symbol string) string
	FetchMarketRules(quotes []string) ([]MarketRule, error)
}

type PrivateClient interface {
	PublicClient
	Connect(context.Context) (int64, error) // delta timestamp of local time - server time / POS means server time is slower or same NEG means server time is faster
	Close() error

	FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error)
	FetchOrder(symbol, orderId string) (*OrderResponseFull, error)

	LimitBuy(symbol string, quantity, price decimal.Decimal, tif string) (*OrderResponse, error)
	LimitSell(symbol string, quantity, price decimal.Decimal, tif string) (*OrderResponse, error)
	MarketBuy(symbol string, quoteQuantity decimal.Decimal) (*OrderResponse, error)
	MarketSell(symbol string, quantity decimal.Decimal) (*OrderResponse, error)
	//StopLossSell(symbol string, quantity, triggerPrice decimal.Decimal) (*OrderResponse, error)
	//TakeProfitSell(symbol string, quantity, triggerPrice decimal.Decimal) (*OrderResponse, error)
	CancelOrder(symbol, orderId string) (*OrderResponse, error)
	CancelAll(symbol string) error
}

type TransactionClient interface {
	PrivateClient
	FetchWithdrawableAmount(asset string) (decimal.Decimal, error)
	FetchDepositAddress(asset string, chain Chain) (string, error)
	Withdraw(asset string, chain Chain, address, tag string, amount decimal.Decimal) (string, error)
	FetchWithdrawTxid(withdrawId string) (string, error)
}

// PrivateClient is enough to manage linear order for cross margin futures account, it is needed if you need risk managing
type FuturesClient interface {
	PrivateClient
	SetLeverage(ctx context.Context, symbol string, leverage int) error
	GetLiquidationPrice(ctx context.Context, symbol string) (decimal.Decimal, error)
	GetFundingRate(ctx context.Context, symbol string) (*FundingRate, error)
	SetMarginMode(ctx context.Context, mode MarginMode) error
}
