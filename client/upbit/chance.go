package upbit

/* Upbit Chance Struct */

type UpbitWithdrawChance struct {
	MemberLevel   UpbitMemberLevel   `json:"member_level"`
	Currency      UpbitCurrency      `json:"currency"`
	Account       UpbitAccount       `json:"account"`
	WithdrawLimit UpbitWithdrawLimit `json:"withdraw_limit"`
}

type UpbitMemberLevel struct {
	SecurityLevel         int  `json:"security_level"`          //	사용자의 보안등급
	FeeLevel              int  `json:"fee_level"`               // 사용자의 수수료등급
	EmailVerified         bool `json:"email_verified"`          //	사용자의 이메일 인증 여부
	IdentityAuthVerified  bool `json:"identity_auth_verified"`  //	사용자의 실명 인증 여부
	BankAccountVerified   bool `json:"bank_account_verified"`   //	사용자의 계좌 인증 여부
	Kakao_payAuthVerified bool `json:"kakao_pay_auth_verified"` //	사용자의 카카오페이 인증 여부
	Locked                bool `json:"locked"`                  //	사용자의 계정 보호 상태
	WalletLocked          bool `json:"wallet_locked"`           //	사용자의 출금 보호 상태
}

type UpbitCurrency struct {
	Code          string   `json:"code"`           //	화폐를 의미하는 영문 대문자 코드
	WithdrawFee   string   `json:"withdraw_fee"`   //	해당 화폐의 출금 수수료
	IsCoin        bool     `json:"is_coin"`        // 화폐의 코인 여부
	WalletState   string   `json:"wallet_state"`   //	해당 화폐의 지갑 상태
	WalletSupport []string `json:"wallet_support"` //	해당 화폐가 지원하는 입출금 정보
}

type UpbitAccount struct {
	Currency            string `json:"currency"`               // 화폐를 의미하는 영문 대문자 코드
	Balance             string `json:"balance"`                // 주문가능 금액/수량
	Locked              string `json:"locked"`                 // 주문 중 묶여있는 금액/수량
	AvgBuyPrice         string `json:"avg_buy_price"`          // 매수평균가
	AvgBuyPriceModified bool   `json:"avg_buy_price_modified"` // 매수평균가 수정 여부
	UnitCurrency        string `json:"unit_currency"`          // 평단가 기준 화폐
}

type UpbitWithdrawLimit struct {
	Currency          string `json:"currency"`            // 화폐를 의미하는 영문 대문자 코드
	Minimum           string `json:"minimum"`             // 출금 최소 금액/수량
	Onetime           string `json:"onetime"`             // 1회 출금 한도
	Daily             string `json:"daily"`               // 1일 출금 한도
	RemainingDaily    string `json:"remaining_daily"`     // 1일 잔여 출금 한도
	RemainingDailyKrw string `json:"remaining_daily_krw"` // 통합 1일 잔여 출금 한도
	Fixed             int    `json:"fixed"`               // 출금 금액/수량 소수점 자리 수
	CanWithdraw       bool   `json:"can_withdraw"`        // 출금 지원 여부
}

/* Upbit Chance Struct END*/
