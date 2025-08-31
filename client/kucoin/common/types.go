package common

// MarketType represents the market type (spot or futures)
type MarketType string

const (
	MarketTypeSpot    MarketType = "spot"
	MarketTypeFutures MarketType = "futures"
)

// WebSocketToken response from KuCoin API
type WebSocketToken struct {
	Code string `json:"code"`
	Data struct {
		Token           string `json:"token"`
		InstanceServers []struct {
			Endpoint     string `json:"endpoint"`
			Protocol     string `json:"protocol"`
			Encrypt      bool   `json:"encrypt"`
			PingInterval int    `json:"pingInterval"`
			PingTimeout  int    `json:"pingTimeout"`
		} `json:"instanceServers"`
	} `json:"data"`
}

// PingMessage represents a ping message
type PingMessage struct {
	Id        string `json:"id"`
	Op        string `json:"op"`
	Timestamp int64  `json:"timestamp"`
}

// OrderWSRequest represents the data for placing an order
// Supports both spot and futures with optional fields
type OrderWSRequest struct {
	ClientOid   string `json:"clientOid"`
	Side        string `json:"side"`
	Symbol      string `json:"symbol"`
	Type        string `json:"type"`
	Price       string `json:"price,omitempty"`
	Size        string `json:"size,omitempty"`
	Qty         string `json:"qty,omitempty"`        // Futures only: base currency quantity
	ValueQty    string `json:"valueQty,omitempty"`   // Futures only: quote currency quantity
	Funds       string `json:"funds,omitempty"`      // Spot only: for market buy orders
	Leverage    string `json:"leverage,omitempty"`   // Futures only
	MarginMode  string `json:"marginMode,omitempty"` // Futures only ISOLATED/CROSS
	StopPrice   string `json:"stopPrice,omitempty"`  // Futures only: for stop orders
	TimeInForce string `json:"timeInForce,omitempty"`
}

// OrderWSResponse represents the response from order placement
type OrderWSResponse struct {
	OrderID   string `json:"orderId"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	ClientOid string `json:"clientOid"`
}
