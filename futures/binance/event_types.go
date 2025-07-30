package binance

type wsAccountUpdate struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Balances  []struct {
		Asset  string `json:"a"`
		Free   string `json:"f"`
		Locked string `json:"l"`
	} `json:"B"`
}

//easyjson:json
type wsTickerArr []wsTicker

//easyjson:json
type wsTicker struct {
	Symbol   string `json:"symbol"`
	BidPrice string `json:"bidPrice"`
	BidQty   string `json:"bidQty"`
	AskPrice string `json:"askPrice"`
	AskQty   string `json:"askQty"`
}

type wsBalanceUpdate struct {
	EventType    string `json:"e"`
	EventTime    int64  `json:"E"`
	Asset        string `json:"a"`
	BalanceDelta string `json:"d"`
	ClearTime    int64  `json:"T"`
}

type wsOrderTradeUpdate struct {
	EventType        string `json:"e"`
	EventTime        int64  `json:"E"`
	Symbol           string `json:"s"`
	Side             string `json:"S"`
	OrderType        string `json:"o"`
	Status           string `json:"X"`
	OrderID          int64  `json:"i"`
	ClientOrderID    string `json:"c"`
	OrigQty          string `json:"q"`
	Price            string `json:"p"`
	ExecutedQty      string `json:"z"`
	CummulativeQuote string `json:"Z"`
	TradeID          int64  `json:"t"`
	LastFilledPrice  string `json:"L"`
	LastFilledQty    string `json:"l"`
	LastFilledTime   int64  `json:"T"`
}

type wsEventStreamTerminated struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
}

type wsExternalLockUpdate struct {
	EventType    string `json:"e"`
	EventTime    int64  `json:"E"`
	Asset        string `json:"a"`
	BalanceDelta string `json:"d"`
	ClearTime    int64  `json:"T"`
}

type wsListStatusOrder struct {
	Symbol        string `json:"s"` // Symbol
	OrderId       int64  `json:"i"` // OrderId
	ClientOrderId string `json:"c"` // ClientOrderId
}

type wsListStatus struct {
	EventType         string              `json:"e"` // Event Type
	EventTime         int64               `json:"E"` // Event Time
	Symbol            string              `json:"s"` // Symbol
	OrderListId       int64               `json:"g"` // OrderListId
	ContingencyType   string              `json:"c"` // Contingency Type
	ListStatusType    string              `json:"l"` // List Status Type
	ListOrderStatus   string              `json:"L"` // List Order Status
	ListRejectReason  string              `json:"r"` // List Reject Reason
	ListClientOrderId string              `json:"C"` // List Client Order ID
	TransactionTime   int64               `json:"T"` // Transaction Time
	Orders            []wsListStatusOrder `json:"O"` // Orders
}

type WsTradeResponse struct {
	ID         string             `json:"id"`
	Status     int                `json"status`
	Result     []WsTradeResult    `json:"result"`
	RateLimits []WsRateLimitEntry `json:"rateLimits"`
}

type WsTradeResult struct {
	Symbol          string `json:"symbol"`
	ID              int64  `json:"id"`
	OrderID         int64  `json:"orderId"`
	OrderListId     int    `json:"orderListId"`
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	QuoteQty        string `json:"quoteQty"`
	Commission      string `json:"commision"`
	CommissionAsset string `json:"commissionAsset"`
	Time            int64  `json:"time"`
	IsBuyer         bool   `json:"isBuyer"`
	IsMaker         bool   `json:"isMaker"`
	IsBestMatch     bool   `json:"isBestMatch"`
}

type WsOrderResponse struct {
	ID         string             `json:"id"`
	Status     int                `json:"status"`
	Result     WsOrderResult      `json:"result"`
	RateLimits []WsRateLimitEntry `json:"rateLimits"`
}

type WsOrderResult struct {
	Symbol                  string `json:"symbol"`
	OrderID                 int64  `json:"orderId"`
	ClientOrderID           string `json:"clientOrderId"`
	TransactTime            int64  `json:"transactTime"`
	Price                   string `json:"price"`
	AvgPrice                string `json:"avgPrice"`
	OrigQty                 string `json:"origQty"`
	ExecutedQty             string `json:"executedQty"`
	OrigQuoteQty            string `json:"origQuoteOrderQty"`
	CumQty                  string `json:"cumQty"`
	CumQuote                string `json:"cumQuote"`
	Status                  string `json:"status"`
	TimeInForce             string `json:"timeInForce"`
	Type                    string `json:"type"`
	ReduceOnly              bool   `json:"reduceOnly"`
	ClosePosition           bool   `json:"closePosition"`
	Side                    string `json:"side"`
	PositionSide            string `json:"positionSide"`
	StopPrice               string `json:"stopPrice"`
	WorkingType             string `json:"workingType"`
	PriceProtect            bool   `json:"priceProtect"`
	OrigType                string `json:"origType"`
	PriceMatch              string `json:"priceMatch"`
	SelfTradePreventionMode string `json:"selfTradePreventionMode"`
	GoodTillDate            int64  `json:"goodTillDate"`
	UpdateTime              int64  `json:"updateTime"`
	// You can add omitempty to optional fields
}

type WsOrderFill struct {
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
	TradeID         int64  `json:"tradeId"`
}

type WsRateLimitEntry struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
	Count         int    `json:"count"`
}

type wsRateLimitWithCount struct {
	RateLimitType string `json:"rateLimitType"`
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
	Count         int    `json:"count"`
}

type wsAccountInfo struct {
	TotalInitialMargin          string              `json:"totalInitialMargin"`
	TotalMaintMargin            string              `json:"totalMaintMargin"`
	TotalWalletBalance          string              `json:"totalWalletBalance"`
	TotalUnrealizedProfit       string              `json:"totalUnrealizedProfit"`
	TotalMarginBalance          string              `json:"totalMarginBalance"`
	TotalPositionInitialMargin  string              `json:"totalPositionInitialMargin"`
	TotalOpenOrderInitialMargin string              `json:"totalOpenOrderInitialMargin"`
	TotalCrossWalletBalance     string              `json:"totalCrossWalletBalance"`
	TotalCrossUnPnl             string              `json:"totalCrossUnPnl"`
	AvailableBalance            string              `json:"availableBalance"`
	MaxWithdrawAmount           string              `json:"maxWithdrawAmount"`
	Assets                      []wsAccountAsset    `json:"assets"`
	Positions                   []wsAccountPosition `json:"positions"`
}

type wsAccountAsset struct {
	Asset                  string `json:"asset"`
	WalletBalance          string `json:"walletBalance"`
	UnrealizedProfit       string `json:"unrealizedProfit"`
	MarginBalance          string `json:"marginBalance"`
	MaintMargin            string `json:"maintMargin"`
	InitialMargin          string `json:"initialMargin"`
	PositionInitialMargin  string `json:"positionInitialMargin"`
	OpenOrderInitialMargin string `json:"openOrderInitialMargin"`
	CrossWalletBalance     string `json:"crossWalletBalance"`
	CrossUnPnl             string `json:"crossUnPnl"`
	AvailableBalance       string `json:"availableBalance"`
	MaxWithdrawAmount      string `json:"maxWithdrawAmount"`
	UpdateTime             int64  `json:"updateTime"`
}

type wsAccountPosition struct {
	Symbol           string `json:"symbol"`
	PositionSide     string `json:"positionSide"`
	PositionAmt      string `json:"positionAmt"`
	UnrealizedProfit string `json:"unrealizedProfit"`
	IsolatedMargin   string `json:"isolatedMargin"`
	Notional         string `json:"notional"`
	IsolatedWallet   string `json:"isolatedWallet"`
	InitialMargin    string `json:"initialMargin"`
	MaintMargin      string `json:"maintMargin"`
	UpdateTime       int64  `json:"updateTime"`
}

type wsExchangeInfoResult struct {
	RateLimits []wsRateLimit  `json:"rateLimits"`
	Symbols    []wsSymbolInfo `json:"symbols"`
}

type wsRateLimit struct {
	Interval      string `json:"interval"`
	IntervalNum   int    `json:"intervalNum"`
	Limit         int    `json:"limit"`
	RateLimitType string `json:"rateLimitType"`
}

type wsSymbolInfo struct {
	Symbol             string     `json:"symbol"`
	Status             string     `json:"status"`
	BaseAsset          string     `json:"baseAsset"`
	QuoteAsset         string     `json:"quoteAsset"`
	BaseAssetPrecision int        `json:"baseAssetPrecision"`
	QuotePrecision     int        `json:"quotePrecision"`
	PricePrecision     int        `json:"pricePrecision"`
	QuantityPrecision  int        `json:"quantityPrecision"`
	Filters            []wsFilter `json:"filters"`
}

type wsFilter struct {
	FilterType string `json:"filterType"`
	MinPrice   string `json:"minPrice,omitempty"`
	MaxPrice   string `json:"maxPrice,omitempty"`
	TickSize   string `json:"tickSize,omitempty"`
	MinQty     string `json:"minQty,omitempty"`
	MaxQty     string `json:"maxQty,omitempty"`
	StepSize   string `json:"stepSize,omitempty"`
}

type WsfapiExchangeInfo struct {
	RateLimits []struct {
		RateLimitType string `json:"rateLimitType"`
		Interval      string `json:"interval"`
		IntervalNum   int    `json:"intervalNum"`
		Limit         int    `json:"limit"`
	} `json:"rateLimits"`
	Symbols []struct {
		Symbol            string `json:"symbol"`
		Status            string `json:"status"`
		BaseAsset         string `json:"baseAsset"`
		QuoteAsset        string `json:"quoteAsset"`
		PricePrecision    int    `json:"pricePrecision"`
		QuantityPrecision int    `json:"quantityPrecision"`
		Filters           []struct {
			FilterType string `json:"filterType"`
			MinPrice   string `json:"minPrice,omitempty"`
			MaxPrice   string `json:"maxPrice,omitempty"`
			TickSize   string `json:"tickSize,omitempty"`
			MinQty     string `json:"minQty,omitempty"`
			MaxQty     string `json:"maxQty,omitempty"`
			StepSize   string `json:"stepSize,omitempty"`
		} `json:"filters"`
		OrderType []string `json:"OrderType"`
	} `json:"symbols"`
}

type wsTickerStream struct {
	EventType       string `json:"e"`
	UpdateID        int64  `json:"u"`
	EventTime       int64  `json:"E"`
	TransactionTime int64  `json:"T"`
	Symbol          string `json:"s"`
	BestBidPrice    string `json:"b"`
	BestBidQty      string `json:"B"`
	BestAskPrice    string `json:"a"`
	BestAskQty      string `json:"A"`
}

type wsSubscribeRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int      `json:"id"`
}
