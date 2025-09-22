package core

import (
	"time"

	"github.com/shopspring/decimal"
)

type OrderStatus string

const (
	OrderStatusOpen     OrderStatus = "OPEN"
	OrderStatusFilled   OrderStatus = "FILLED"
	OrderStatusCanceled OrderStatus = "CANCELLED"
	OrderStatusError    OrderStatus = "ERROR"
)

type RateLimitCategory string

const (
	RateLimitRequest    RateLimitCategory = "REQUEST"
	RateLimitOrder      RateLimitCategory = "ORDER"
	RateLimitConnection RateLimitCategory = "CONNECTION"
)

type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "GTC" // Good til cancel
	TimeInForceIOC TimeInForce = "IOC" // Immediate or cancel
	TimeInForceFOK TimeInForce = "FOK" // Fill or kill
)

type OrderResponse struct {
	OrderID         string
	Symbol          string
	Side            string
	Tif             TimeInForce
	Status          OrderStatus
	Price           decimal.Decimal
	Quantity        decimal.Decimal
	IsQuoteQuantity bool
	CreateTime      time.Time
}

type OrderResponseFull struct {
	OrderResponse
	AvgPrice        decimal.Decimal
	ExecutedQty     decimal.Decimal
	Commission      decimal.Decimal
	CommissionAsset string
	UpdateTime      time.Time
}

type Quote struct {
	Symbol   string
	BidPrice decimal.Decimal
	BidQty   decimal.Decimal
	AskPrice decimal.Decimal
	AskQty   decimal.Decimal
	Time     time.Time
}

type Orderbook struct {
	Symbol string
	Bids   []OrderbookEntry
	Asks   []OrderbookEntry
}

type OrderbookEntry struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
	Level    int
}

type RateLimit struct {
	Category RateLimitCategory
	Interval time.Duration
	Limit    int64
	Count    int64
}

type MarketRule struct {
	Symbol         string
	BaseAsset      string
	QuoteAsset     string
	PricePrecision int64 // decimal places ex) = 3 max decimal accuracy is 0.001
	QtyPrecision   int64
	MinPrice       decimal.Decimal
	MaxPrice       decimal.Decimal
	MinQty         decimal.Decimal
	MaxQty         decimal.Decimal
	TickSize       decimal.Decimal // price tick size
	StepSize       decimal.Decimal // quantity step size
	RateLimits     []RateLimit
}

type Wallet struct {
	Asset  string
	Free   decimal.Decimal
	Locked decimal.Decimal
	Total  decimal.Decimal
}

type Ticker struct {
	Symbol    string
	BidPrice  float64
	BidQty    float64
	AskPrice  float64
	AskQty    float64
	LastPrice float64
	Volume    float64
	Time      time.Time
}

type Account struct {
	CrossBalance   float64
	CrossUrlProfit float64
	Assets         map[string]*Wallet
	Positions      map[string]*Position
}

type PositionSide string

const (
	LONG  PositionSide = "LONG"
	SHORT PositionSide = "SHORT"
	BOTH  PositionSide = "BOTH"
)

type Position struct {
	Symbol         string
	Side           PositionSide
	Amount         float64
	UrlProfit      float64
	IsolatedMargin float64
	Notional       float64
	IsolatedWallet float64
	InitialMargin  float64
	MaintMargin    float64
	UpdatedTime    time.Time
}

type OrderFill struct {
	Price       decimal.Decimal
	Quantity    decimal.Decimal
	TradeTime   time.Time
	IsBuyer     bool
	IsBestMatch bool
}

type Chain string

const (
	ERC20 Chain = "Ethereum"
	BASE  Chain = "Base Mainnet"
)

// MarginMode represents the margin mode for futures trading
type MarginMode string

const (
	MarginModeCross    MarginMode = "CROSS"
	MarginModeIsolated MarginMode = "ISOLATED"
)

// FundingRate represents funding rate information
type FundingRate struct {
	Rate         decimal.Decimal
	NextTime     int64           // Unix timestamp in seconds
	PreviousRate decimal.Decimal
}
