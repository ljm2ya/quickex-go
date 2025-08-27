package upbit

type WsTicker struct {
	AccAskVolume       float64 `json:"acc_ask_volume"`
	AccBidVolume       float64 `json:"acc_bid_volume"`
	AccTradePrice      float64 `json:"acc_trade_price"`
	AccTradePrice24H   float64 `json:"acc_trade_price_24h"`
	AccTradeVolume     float64 `json:"acc_trade_volume"`
	AccTradeVolume24H  float64 `json:"acc_trade_volume_24h"`
	AskBid             string  `json:"ask_bid"`
	Change             string  `json:"change"`
	ChangePrice        float64 `json:"change_price"`
	ChangeRate         float64 `json:"change_rate"`
	Code               string  `json:"code"`
	Symbol             string  `json:"market"`
	DelistingDate      any     `json:"delisting_date"`
	HighPrice          float64 `json:"high_price"`
	Highest52WeekDate  string  `json:"highest_52_week_date"`
	Highest52WeekPrice float64 `json:"highest_52_week_price"`
	IsTradingSuspended bool    `json:"is_trading_suspended"`
	LowPrice           float64 `json:"low_price"`
	Lowest52WeekDate   string  `json:"lowest_52_week_date"`
	Lowest52WeekPrice  float64 `json:"lowest_52_week_price"`
	MarketState        string  `json:"market_state"`
	MarketWarning      string  `json:"market_warning"`
	OpeningPrice       float64 `json:"opening_price"`
	PrevClosingPrice   float64 `json:"prev_closing_price"`
	SignedChangePrice  float64 `json:"signed_change_price"`
	SignedChangeRate   float64 `json:"signed_change_rate"`
	StreamType         string  `json:"stream_type"`
	Timestamp          int64   `json:"timestamp"`
	TradeDate          string  `json:"trade_date"`
	TradePrice         float64 `json:"trade_price"`
	TradeTime          string  `json:"trade_time"`
	TradeTimestamp     int64   `json:"trade_timestamp"`
	TradeVolume        float64 `json:"trade_volume"`
	Type               string  `json:"type"`
}

type WsOrder struct {
	Type            string  `json:"type"`             // 타입: "myOrder"
	Code            string  `json:"code"`             // 마켓 코드 (예: "KRW-BTC")
	UUID            string  `json:"uuid"`             // 주문 고유 아이디
	AskBid          string  `json:"ask_bid"`          // 매수/매도 구분 ("ASK", "BID")
	OrderType       string  `json:"order_type"`       // 주문 타입 (limit, price, market, best)
	State           string  `json:"state"`            // 주문 상태 (wait, watch, trade, done, cancel)
	TradeUUID       string  `json:"trade_uuid"`       // 체결의 고유 아이디
	Price           float64 `json:"price"`            // 주문 가격 (또는 체결 가격)
	AvgPrice        float64 `json:"avg_price"`        // 평균 체결 가격
	Volume          float64 `json:"volume"`           // 주문량 (또는 체결량)
	RemainingVolume float64 `json:"remaining_volume"` // 남은 주문량
	ExecutedVolume  float64 `json:"executed_volume"`  // 체결된 양
	TradesCount     int     `json:"trades_count"`     // 체결 수
	ReservedFee     float64 `json:"reserved_fee"`     // 예약된 수수료
	RemainingFee    float64 `json:"remaining_fee"`    // 남은 수수료
	PaidFee         float64 `json:"paid_fee"`         // 사용된 수수료
	Locked          float64 `json:"locked"`           // 거래에 사용 중인 비용
	ExecutedFunds   float64 `json:"executed_funds"`   // 체결된 금액
	TimeInForce     string  `json:"time_in_force"`    // IOC/FOK 설정 (예: "ioc", "fok")
	TradeFee        float64 `json:"trade_fee"`        // 체결 시 발생한 수수료 (trade 상태 아닐 경우 null)
	IsMaker         bool    `json:"is_maker"`         // 메이커 여부 (trade 상태 아닐 경우 null)
	Identifier      string  `json:"identifier"`       // 사용자 지정 식별자 (2024-10-18 이후 제공)
	TradeTimestamp  int64   `json:"trade_timestamp"`  // 체결 타임스탬프 (ms)
	OrderTimestamp  int64   `json:"order_timestamp"`  // 주문 타임스탬프 (ms)
	Timestamp       int64   `json:"timestamp"`        // 전체 타임스탬프 (ms)
	StreamType      string  `json:"stream_type"`      // 스트림 타입 (예: "REALTIME")
}

type WsOrderbookUnit struct {
	AskPrice float64 `json:"ap"` // 매도 호가
	BidPrice float64 `json:"bp"` // 매수 호가
	AskSize  float64 `json:"as"` // 매도 잔량
	BidSize  float64 `json:"bs"` // 매수 잔량
}

type WsOrderbook struct {
	Type           string            `json:"ty"`  // 타입
	Code           string            `json:"cd"`  // 마켓 코드 (ex. KRW-BTC)
	TotalAskSize   float64           `json:"tas"` // 호가 매도 총 잔량
	TotalBidSize   float64           `json:"tbs"` // 호가 매수 총 잔량
	OrderbookUnits []WsOrderbookUnit `json:"obu"` // 호가
	Timestamp      int64             `json:"tms"` // 타임스탬프 (millisecond)
	Level          int               `json:"level,omitempty"` // 호가 모아보기 단위 (optional in SIMPLE format)
}
