package upbit

/* upbit structs */
type UpbitMarket struct {
	Market       string `json:"market"`
	Korean_name  string `json:"korean_name"`
	English_name string `json:"english_name"`
}

type UpbitBalance struct {
	Currency            string `json:"currency"`
	Balance             string `json:"balance"`
	Locked              string `json:"locked"`
	AvgBuyPrice         string `json:"avg_buy_price"`
	AvgBuyPriceModified bool   `json:"avg_buy_price_modified"`
	UnitCurrency        string `json:"unit_currency"`
}

type UpbitOrderbookUnit struct {
	AskPrice float64 `json:"ask_price"`
	BidPrice float64 `json:"bid_price"`
	AskSize  float64 `json:"ask_size"`
	BidSize  float64 `json:"bid_size"`
}

type UpbitOrderbook struct {
	Market         string               `json:"market"`
	Timestamp      int                  `json:"timestamp"`
	TotalAskSize   float64              `json:"total_ask_size"`
	TotalBidSize   float64              `json:"total_bid_size"`
	OrderbookUnits []UpbitOrderbookUnit `json:"orderbook_units"`
}

type UpbitOrder struct {
	UUID            string            `json:"uuid"`             //"cdd92199-2897-4e14-9448-f923320408ad",
	Side            string            `json:"side"`             // "bid",
	OrdType         string            `json:"ord_type"`         // "limit",
	Price           string            `json:"price"`            // "100.0",
	AvgPrice        string            `json:"avg_price"`        // "0.0",
	State           string            `json:"state"`            // "wait", "watch", "done", "cancel"
	Market          string            `json:"market"`           // "KRW-BTC",
	CreatedAt       string            `json:"created_at"`       // "2018-04-10T15:42:23+09:00",
	Volume          string            `json:"volume"`           // "0.01",
	RemainingVolume string            `json:"remaining_volume"` // "0.01",
	ReservedFee     string            `json:"reserved_fee"`     // "0.0015",
	RemainingFee    string            `json:"remaining_fee"`    // "0.0015",
	PaidFee         string            `json:"paid_fee"`         // "0.0",
	Locked          string            `json:"locked"`           // "1.0015",
	ExecutedVolume  string            `json:"executed_volume"`  // "0.0",
	TradesCount     int               `json:"trades_count"`     // 0
	Trades          []UpbitOrderTrade `json:"trades"`           // []
}

type UpbitOrderTrade struct {
	Market string `json:"market"` // "KRW-BTC",
	UUID   string `json:"uuid"`   // "9e8f8eba-7050-4837-8969-cfc272cbe083",
	Price  string `json:"price"`  // "4280000.0",
	Volume string `json:"volume"` // "1.0",
	Funds  string `json:"funds"`  // "4280000.0",
	Side   string `json:"side"`   // "ask"
}

type UpbitDepositAddress struct {
	Currency         string `json:"currency"`          // "BTC",
	DepositAddress   string `json:"deposit_address"`   // "3EusRwybuZUhVDeHL7gh3HSLmbhLcy7NqD",
	SecondaryAddress string `json:"secondary_address"` // null
}

type UpbitWithdrawResponse struct {
	UUID            string             `json:"uuid"`             //출금의 고유 아이디
	Currency        string             `json:"currency"`         //화폐를 의미하는 영문 대문자 코드
	Txid            string             `json:"txid"`             //출금의 트랜잭션 아이디
	State           string             `json:"state"`            //출금 상태
	CreatedAt       string             `json:"created_at"`       //출금 생성 시간
	DoneAt          string             `json:"done_at"`          //출금 완료 시간
	Amount          string             `json:"amount"`           //출금 금액/수량
	Fee             string             `json:"fee"`              //출금 수수료
	KrwAmount       string             `json:"krw_amount"`       //원화 환산 가격
	TransactionType string             `json:"transaction_type"` //출금 유형
	Error           UpbitWithdrawError `json:"error"`
}

type UpbitWithdrawError struct {
	Message string `json:"message"`
	Name    string `json:"name"`
}

type UpbitMinuteCandleResponse struct {
	Market               string  `json:"market"`                  //"KRW-BTC",
	CandleDateTimeUtc    string  `json:"candle_date_time_utc"`    //"2018-04-18T10:16:00",
	CandleDateTimeKst    string  `json:"candle_date_time_kst"`    //"2018-04-18T19:16:00",
	OpeningPrice         float64 `json:"opening_price"`           //8615000,
	HighPrice            float64 `json:"high_price"`              //8618000,
	LowPrice             float64 `json:"low_price"`               //8611000,
	TradePrice           float64 `json:"trade_price"`             //8616000,
	Timestamp            int64   `json:"timestamp"`               //1524046594584,
	CandleAccTradePrice  float64 `json:"candle_acc_trade_price"`  //60018891.90054,
	CandleAccTradeVolume float64 `json:"candle_acc_trade_volume"` //6.96780929,
	Unit                 int     `json:"unit"`                    //1
}

type UpbitOrdersChanceResponse struct {
	BidFee string                  `json:"bid_fee"` // 매수 수수료 비율
	AskFee string                  `json:"ask_fee"` // 매도 수수료 비율
	Market UpbitOrdersChanceMarket `json:"market"`  //마켓에 대한 정보
	// AskTypes   []string                 `json:"ask_types"`   //	매도 주문 지원 방식	Array[String]
	// BidTypes   []string                 `json:"bid_types"`   //매수 주문 지원 방식	Array[String]
	BidAccount UpbitOrdersChanceAccount `json:"bid_account"` //	매수 시 사용하는 화폐의 계좌 상태	Object
	AskAccount UpbitOrdersChanceAccount `json:"ask_account"` //	매도 시 사용하는 화폐의 계좌 상태	Object
}

type UpbitOrdersChanceMarket struct {
	Id         string                       `json:"id"`          // 마켓의 유일 키
	Name       string                       `json:"name"`        // 마켓 이름
	OrderTypes []string                     `json:"order_types"` //지원 주문 방식 (만료)
	OrderSides []string                     `json:"order_sides"` //지원 주문 종류
	Bid        UpbitOrdersChanceRestriction `json:"bid"`         // 매수 시 제약사항
	Ask        UpbitOrdersChanceRestriction `json:"ask"`         //매도 시 제약사항
	MaxTotal   string                       `json:"max_total"`   // 최대 매도/매수 금액 numberstring
	State      string                       `json:"state"`       // 마켓 운영 상태
}

type UpbitOrdersChanceRestriction struct {
	Currency  string `json:"currency"`   //화폐를 의미하는 영문 대문자 코드
	PriceUnit string `json:"price_unit"` //주문금액 단위
	MinTotal  int    `json:"min_total"`  //최소 매도/매수 금액 numberstring
}

type UpbitOrdersChanceAccount struct {
	Currency            string `json:"currency"`               //	화폐를 의미하는 영문 대문자 코드 String
	Balance             string `json:"balance"`                //	주문가능 금액/수량	NumberString
	Locked              string `json:"locked"`                 //	주문 중 묶여있는 금액/수량	NumberString
	AvgBuyPrice         string `json:"avg_buy_price"`          //	매수평균가	NumberString
	AvgBuyPriceModified string `json:"avg_buy_price_modified"` //	매수평균가 수정 여부	Boolean
	UnitCurrency        string `json:"unit_currency"`          //	평단가 기준 화폐	String
}

type UpbitAPILimit struct {
	Group string
	Min   int
	Sec   int
}

type APILimitStruct struct {
	Default *UpbitAPILimit
	Order   *UpbitAPILimit
}

type UpbitTickerOfMarket struct {
	Market             string  `json:"market"`         // 마켓명	String
	TradeDate          string  `json:"trade_date"`     // 최근 거래 일자(UTC)	String
	TradeTime          string  `json:"trade_time"`     // 최근 거래 시각(UTC)	String
	TradeDateKst       string  `json:"trade_date_kst"` // 최근 거래 일자(KST)	String
	TradeTimeKst       string  `json:"trade_time_kst"` // 최근 거래 시각(KST)	String
	TradeTimestamp     int64   `json:"trade_timestamp"`
	OpeningPrice       float64 `json:"opening_price"`         // 시가	Double
	HighPrice          float64 `json:"high_price"`            // 고가	Double
	LowPrice           float64 `json:"low_price"`             // 저가	Double
	TradePrice         float64 `json:"trade_price"`           // 종가	Double
	PrevClosingPrice   float64 `json:"prev_closing_price"`    // 전일 종가	Double
	Change             string  `json:"change"`                // EVEN : 보합 RISE : 상승 FALL : 하락	String
	ChangePrice        float64 `json:"change_price"`          // 변화액의 절대값	Double
	ChangeRate         float64 `json:"change_rate"`           // 변화율의 절대값	Double
	SignedChangePrice  float64 `json:"signed_change_price"`   // 부호가 있는 변화액	Double
	TradeVolume        float64 `json:"trade_volume"`          // 가장 최근 거래량	Double
	AccTradePrice      float64 `json:"acc_trade_price"`       // 24시간 누적 거래대금	Double
	AccTradePrice24h   float64 `json:"acc_trade_price_24h"`   // 24시간 누적 거래대금 (UTC 0시 기준)	Double
	AccTradeVolume     float64 `json:"acc_trade_volume"`      // 24시간 누적 거래량	Double
	AccTradeVolume24h  float64 `json:"acc_trade_volume_24h"`  // 24시간 누적 거래량 (UTC 0시 기준)	Double
	Highest52WeekPrice float64 `json:"highest_52_week_price"` // 52주 신고가	Double
	Highest52WeekDate  string  `json:"highest_52_week_date"`  // 52주 신고가 달성일	String
	Lowest52WeekPrice  float64 `json:"lowest_52_week_price"`  // 52주 신저가	Double
	Lowest52WeekDate   string  `json:"lowest_52_week_date"`   // 52주 신저가 달성일	String
	Timestamp          int64   `json:"timestamp"`             // 타임스탬프	Long
}
