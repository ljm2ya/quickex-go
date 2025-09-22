package phemex

import (
	"time"

	"github.com/shopspring/decimal"
)

// Phemex WebSocket Message Types
type PhemexWSMessage struct {
	ID     interface{} `json:"id"`
	Method string      `json:"method,omitempty"`
	Params interface{} `json:"params,omitempty"`
	Result interface{} `json:"result,omitempty"`
	Error  *PhemexError `json:"error,omitempty"`
}

type PhemexError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type PhemexRestResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type PhemexAuthParams struct {
	APIToken string `json:"api-token"`
	Signature string `json:"signature"`
	Expiry   int64  `json:"expiry"`
}

// Market Data Types
type PhemexSymbol struct {
	Symbol          string `json:"symbol"`
	DisplaySymbol   string `json:"displaySymbol"`
	IndexSymbol     string `json:"indexSymbol"`
	MarkSymbol      string `json:"markSymbol"`
	FundingRateSymbol string `json:"fundingRateSymbol"`
	QuoteCurrency   string `json:"quoteCurrency"`
	BaseCurrency    string `json:"baseCurrency"`
	SettleCurrency  string `json:"settleCurrency"`
	MaxOrderQty     int64  `json:"maxOrderQty"`
	MaxPriceEp      int64  `json:"maxPriceEp"`
	LotSize         int64  `json:"lotSize"`
	TickSize        int64  `json:"tickSizeEp"`
	ContractSize    int64  `json:"contractSize"`
	PriceScale      int    `json:"priceScale"`
	RatioScale      int    `json:"ratioScale"`
	PricePrecision  int    `json:"pricePrecision"`
	MinPriceEp      int64  `json:"minPriceEp"`
	MaxOrderValue   int64  `json:"maxOrderValue"`
	Type            string `json:"type"`
	Status          string `json:"status"`
}

type PhemexTicker struct {
	Symbol     string `json:"symbol"`
	BidEp      int64  `json:"bidEp"`
	BidSizeEp  int64  `json:"bidSizeEp"`
	AskEp      int64  `json:"askEp"`
	AskSizeEp  int64  `json:"askSizeEp"`
	LastEp     int64  `json:"lastEp"`
	Timestamp  int64  `json:"timestamp"`
}

type PhemexQuoteSubscription struct {
	Symbol string `json:"symbol"`
}

// Account & Order Types
type PhemexAccount struct {
	UserID         int64            `json:"userID"`
	AccountID      int64            `json:"accountID"`
	Currency       string           `json:"currency"`
	BalanceEv      int64            `json:"balanceEv"`
	TotalUsedBalanceEv int64        `json:"totalUsedBalanceEv"`
	BonusBalanceEv int64            `json:"bonusBalanceEv"`
}

type PhemexPosition struct {
	AccountID      int64  `json:"accountID"`
	Symbol         string `json:"symbol"`
	Currency       string `json:"currency"`
	Side           string `json:"side"`
	SizeEv         int64  `json:"sizeEv"`
	Value          int64  `json:"value"`
	ValueEv        int64  `json:"valueEv"`
	AvgEntryPrice  int64  `json:"avgEntryPrice"`
	AvgEntryPriceEp int64 `json:"avgEntryPriceEp"`
	PosCostEv      int64  `json:"posCostEv"`
	AssignedPosBalance int64 `json:"assignedPosBalance"`
	BankruptCommission int64 `json:"bankruptCommission"`
	BankruptPrice  int64  `json:"bankruptPrice"`
	PositionStatus string `json:"positionStatus"`
	CrossMargin    bool   `json:"crossMargin"`
	LeverageEr     int64  `json:"leverageEr"`
	LiquidationPrice int64 `json:"liquidationPrice"`
	RealisedPnl    int64  `json:"realisedPnl"`
	RealisedPnlEv  int64  `json:"realisedPnlEv"`
	UnrealisedPnlEv int64 `json:"unrealisedPnlEv"`
	CumRealisedPnlEv int64 `json:"cumRealisedPnlEv"`
	Term           int64  `json:"term"`
	LastTermEndTime int64 `json:"lastTermEndTime"`
	Timestamp      int64  `json:"timestamp"`
}

type PhemexOrder struct {
	OrderID       string `json:"orderID"`
	ClOrderID     string `json:"clOrderID"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	OrderType     string `json:"orderType"`
	ActionTimeNs  int64  `json:"actionTimeNs"`
	PriceEp       int64  `json:"priceEp"`
	OrderQty      int64  `json:"orderQty"`
	DisplayQty    int64  `json:"displayQty"`
	TimeInForce   string `json:"timeInForce"`
	ReduceOnly    bool   `json:"reduceOnly"`
	CloseOnTrigger bool  `json:"closeOnTrigger"`
	OrderStatus   string `json:"orderStatus"`
	CumQty        int64  `json:"cumQty"`
	CumValueEv    int64  `json:"cumValueEv"`
	LeavesQty     int64  `json:"leavesQty"`
	LeavesValueEv int64  `json:"leavesValueEv"`
	AvgPriceEp    int64  `json:"avgPriceEp"`
	CumFeeEv      int64  `json:"cumFeeEv"`
	TransactTimeNs int64 `json:"transactTimeNs"`
	TriggerTimeNs int64  `json:"triggerTimeNs,omitempty"`
}

type PhemexOrderRequest struct {
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	OrderQty    int64  `json:"orderQty"`
	OrdType     string `json:"ordType"`
	PriceEp     int64  `json:"priceEp,omitempty"`
	TimeInForce string `json:"timeInForce"`
	ClOrderID   string `json:"clOrderID,omitempty"`
	ReduceOnly  bool   `json:"reduceOnly,omitempty"`
}

// Helper functions for Phemex's scaled pricing system
func ToEp(value decimal.Decimal, scale int) int64 {
	multiplier := decimal.NewFromFloat(1)
	for i := 0; i < scale; i++ {
		multiplier = multiplier.Mul(decimal.NewFromInt(10))
	}
	return value.Mul(multiplier).IntPart()
}

func FromEp(ep int64, scale int) decimal.Decimal {
	divisor := decimal.NewFromFloat(1)
	for i := 0; i < scale; i++ {
		divisor = divisor.Mul(decimal.NewFromInt(10))
	}
	return decimal.NewFromInt(ep).Div(divisor)
}

// Convert timestamp to time.Time
func ToTime(timestampMs int64) time.Time {
	return time.Unix(0, timestampMs*int64(time.Millisecond))
}

func ToTimeNs(timestampNs int64) time.Time {
	return time.Unix(0, timestampNs)
}