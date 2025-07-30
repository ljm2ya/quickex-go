package bybit

import ()

// For TIF, Side, etc
type TimeInForce string

const (
	TIF_GTC TimeInForce = "GTC"
	TIF_IOC TimeInForce = "IOC"
	TIF_FOK TimeInForce = "FOK"
)

// etc 필요시 추가

type wsListStatus struct {
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderId"`
	Status      string `json:"status"`
	OrderStatus string `json:"orderStatus"` // Bybit v5 naming
	// ... 필요시 추가 필드
}
