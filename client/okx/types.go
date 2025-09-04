package okx

import (
	"time"

	"github.com/shopspring/decimal"
)

// OKX API Response Types

type OKXResponse struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type OKXInstrument struct {
	InstType  string `json:"instType"`  // SPOT, SWAP, FUTURES
	InstID    string `json:"instId"`    // BTC-USDT
	BaseCcy   string `json:"baseCcy"`   // BTC
	QuoteCcy  string `json:"quoteCcy"`  // USDT
	SettleCcy string `json:"settleCcy"` // Settlement currency
	TickSz    string `json:"tickSz"`    // Price tick size
	LotSz     string `json:"lotSz"`     // Lot size
	MinSz     string `json:"minSz"`     // Minimum order size
	MaxLmtSz  string `json:"maxLmtSz"`  // Maximum limit order size
	MaxMktSz  string `json:"maxMktSz"`  // Maximum market order size
	State     string `json:"state"`     // live, suspend, preopen
}

type OKXBalance struct {
	Ccy       string `json:"ccy"`       // Currency
	Bal       string `json:"bal"`       // Balance
	FrozenBal string `json:"frozenBal"` // Frozen balance
	AvailBal  string `json:"availBal"`  // Available balance
}

type OKXAccount struct {
	TotalEq  string       `json:"totalEq"`  // Total equity in USD
	Details  []OKXBalance `json:"details"`  // Balance details
	UTime    string       `json:"uTime"`    // Update time
	AdjEq    string       `json:"adjEq"`    // Adjusted equity
	IsoEq    string       `json:"isoEq"`    // Isolated equity
	OrdFroz  string       `json:"ordFroz"`  // Margin frozen for orders
	Imr      string       `json:"imr"`      // Initial margin requirement
	Mmr      string       `json:"mmr"`      // Maintenance margin requirement
}

type OKXPosition struct {
	InstType    string `json:"instType"`    // SWAP, FUTURES
	InstID      string `json:"instId"`      // Instrument ID
	MgnMode     string `json:"mgnMode"`     // isolated, cross
	PosSide     string `json:"posSide"`     // long, short, net
	Pos         string `json:"pos"`         // Position size
	AvailPos    string `json:"availPos"`    // Available position
	AvgPx       string `json:"avgPx"`       // Average price
	UPL         string `json:"upl"`         // Unrealized PnL
	UplRatio    string `json:"uplRatio"`    // Unrealized PnL ratio
	NotionalUsd string `json:"notionalUsd"` // Notional value in USD
	Lever       string `json:"lever"`       // Leverage
	Liab        string `json:"liab"`        // Liability
	LiabCcy     string `json:"liabCcy"`     // Liability currency
	Interest    string `json:"interest"`    // Interest
	TradeID     string `json:"tradeId"`     // Trade ID
	OptVal      string `json:"optVal"`      // Option value
	PTime       string `json:"pTime"`       // Position time
	UTime       string `json:"uTime"`       // Update time
	CTime       string `json:"cTime"`       // Creation time
}

type OKXOrder struct {
	InstType    string `json:"instType"`    // SPOT, SWAP, FUTURES
	InstID      string `json:"instId"`      // Instrument ID
	TdMode      string `json:"tdMode"`      // cash, isolated, cross
	Ccy         string `json:"ccy"`         // Currency
	OrdID       string `json:"ordId"`       // Order ID
	ClOrdID     string `json:"clOrdId"`     // Client order ID
	Tag         string `json:"tag"`         // Order tag
	Px          string `json:"px"`          // Price
	Sz          string `json:"sz"`          // Size
	NotionalUsd string `json:"notionalUsd"` // Notional value in USD
	OrdType     string `json:"ordType"`     // limit, market, post_only, fok, ioc
	Side        string `json:"side"`        // buy, sell
	PosSide     string `json:"posSide"`     // long, short, net
	TgtCcy      string `json:"tgtCcy"`      // base_ccy, quote_ccy
	AccFillSz   string `json:"accFillSz"`   // Accumulated fill size
	FillPx      string `json:"fillPx"`      // Fill price
	TradeID     string `json:"tradeId"`     // Trade ID
	FillSz      string `json:"fillSz"`      // Fill size
	FillTime    string `json:"fillTime"`    // Fill time
	AvgPx       string `json:"avgPx"`       // Average price
	State       string `json:"state"`       // live, partially_filled, filled, canceled
	Lever       string `json:"lever"`       // Leverage
	TpTriggerPx string `json:"tpTriggerPx"` // Take profit trigger price
	TpOrdPx     string `json:"tpOrdPx"`     // Take profit order price
	SlTriggerPx string `json:"slTriggerPx"` // Stop loss trigger price
	SlOrdPx     string `json:"slOrdPx"`     // Stop loss order price
	FeeCcy      string `json:"feeCcy"`      // Fee currency
	Fee         string `json:"fee"`         // Fee
	RebateCcy   string `json:"rebateCcy"`   // Rebate currency
	Rebate      string `json:"rebate"`      // Rebate
	Pnl         string `json:"pnl"`         // PnL
	Source      string `json:"source"`      // Order source
	Category    string `json:"category"`    // normal, twap, adl, full_liquidation
	UTime       string `json:"uTime"`       // Update time
	CTime       string `json:"cTime"`       // Creation time
}

type OKXTicker struct {
	InstType  string `json:"instType"`  // SPOT, SWAP, FUTURES
	InstID    string `json:"instId"`    // Instrument ID
	Last      string `json:"last"`      // Last traded price
	LastSz    string `json:"lastSz"`    // Last traded size
	AskPx     string `json:"askPx"`     // Best ask price
	AskSz     string `json:"askSz"`     // Best ask size
	BidPx     string `json:"bidPx"`     // Best bid price
	BidSz     string `json:"bidSz"`     // Best bid size
	Open24h   string `json:"open24h"`   // 24h open price
	High24h   string `json:"high24h"`   // 24h high price
	Low24h    string `json:"low24h"`    // 24h low price
	VolCcy24h string `json:"volCcy24h"` // 24h volume in quote currency
	Vol24h    string `json:"vol24h"`    // 24h volume in base currency
	SodUtc0   string `json:"sodUtc0"`   // Start of day UTC0
	SodUtc8   string `json:"sodUtc8"`   // Start of day UTC8
	Ts        string `json:"ts"`        // Timestamp
}

// WebSocket Message Types

type OKXWSMessage struct {
	Op   string                 `json:"op,omitempty"`   // subscribe, unsubscribe, login
	Args []OKXWSArg             `json:"args,omitempty"` // Arguments
	ID   string                 `json:"id,omitempty"`   // Request ID
	Data interface{}            `json:"data,omitempty"` // Data
	Event string                `json:"event,omitempty"` // Event type
	Code  string                `json:"code,omitempty"`  // Response code
	Msg   string                `json:"msg,omitempty"`   // Response message
	ConnID string               `json:"connId,omitempty"` // Connection ID
}

type OKXWSArg struct {
	Channel  string `json:"channel,omitempty"`  // Channel name
	InstType string `json:"instType,omitempty"` // SPOT, SWAP, FUTURES
	InstID   string `json:"instId,omitempty"`   // Instrument ID
	Ccy      string `json:"ccy,omitempty"`      // Currency
}

type OKXWSLoginArg struct {
	APIKey     string `json:"apiKey"`
	Passphrase string `json:"passphrase"`
	Timestamp  string `json:"timestamp"`
	Sign       string `json:"sign"`
}

// Helper functions to convert strings to decimal
func ToDecimal(s string) decimal.Decimal {
	if s == "" || s == "0" {
		return decimal.Zero
	}
	d, _ := decimal.NewFromString(s)
	return d
}

func ToFloat64(s string) float64 {
	if s == "" {
		return 0.0
	}
	d, _ := decimal.NewFromString(s)
	f, _ := d.Float64()
	return f
}

func ToTime(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	// OKX timestamps are in milliseconds
	if len(ts) > 10 {
		// Milliseconds timestamp
		ms, _ := decimal.NewFromString(ts)
		sec := ms.DivRound(decimal.NewFromInt(1000), 0)
		nsec := ms.Mod(decimal.NewFromInt(1000)).Mul(decimal.NewFromInt(1000000))
		secInt, _ := sec.Float64()
		nsecInt, _ := nsec.Float64()
		return time.Unix(int64(secInt), int64(nsecInt))
	} else {
		// Seconds timestamp
		sec, _ := decimal.NewFromString(ts)
		secInt, _ := sec.Float64()
		return time.Unix(int64(secInt), 0)
	}
}