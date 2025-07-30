package upbit

/* Upbit Status */
type UpbitDepositStatus struct {
	Type            string `json:"type"`             //	입출금 종류
	UUID            string `json:"uuid"`             //	입금에 대한 고유 아이디
	Currency        string `json:"currency"`         //	화폐를 의미하는 영문 대문자 코드
	Txid            string `json:"txid"`             //	입금의 트랜잭션 아이디
	State           string `json:"state"`            //	입금 상태
	CreatedAt       string `json:"created_at"`       //	입금 생성 시간
	DoneAt          string `json:"done_at"`          //	입금 완료 시간
	Amount          string `json:"amount"`           //	입금 수량
	Fee             string `json:"fee"`              //	입금 수수료
	TransactionType string `json:"transaction_type"` //	입금 유형 "Default", "Internal"
}

type UpbitWithdrawStatus struct {
	Type            string `json:"type"`             //	입출금 종류
	UUID            string `json:"uuid"`             //	입금에 대한 고유 아이디
	Currency        string `json:"currency"`         //	화폐를 의미하는 영문 대문자 코드
	Txid            string `json:"txid"`             //	입금의 트랜잭션 아이디
	State           string `json:"state"`            //	입금 상태
	CreatedAt       string `json:"created_at"`       //	입금 생성 시간
	DoneAt          string `json:"done_at"`          //	입금 완료 시간
	Amount          string `json:"amount"`           //	입금 수량
	Fee             string `json:"fee"`              //	입금 수수료
	TransactionType string `json:"transaction_type"` //	입금 유형 "Default", "Internal"
}

/* Upbit Status END */
