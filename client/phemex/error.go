package phemex

import (
	"fmt"
)

// PhemexAPIError represents an error from Phemex WebSocket API
type PhemexAPIError struct {
	Code    int
	Message string
}

func (e PhemexAPIError) Error() string {
	return fmt.Sprintf("Phemex API Error %d: %s", e.Code, e.Message)
}

// Common Phemex error codes and their meanings
var PhemexErrorCodes = map[int]string{
	0:     "Success",
	1:     "System error",
	2:     "Invalid request",
	3:     "Invalid argument",
	4:     "Invalid symbol",
	5:     "Invalid order type",
	6:     "Invalid side",
	7:     "Invalid quantity",
	8:     "Invalid price",
	9:     "Insufficient balance",
	10:    "Order not found",
	11:    "Order cannot be cancelled",
	12:    "Market closed",
	13:    "Rate limit exceeded",
	14:    "Invalid time in force",
	15:    "Duplicate client order ID",
	1001:  "Authentication required",
	1002:  "Invalid API key",
	1003:  "Invalid signature",
	1004:  "Request expired",
	1005:  "Access denied",
	2001:  "Symbol trading is not active",
	2002:  "Symbol is invalid",
	2003:  "Order quantity is invalid",
	2004:  "Order price is invalid",
	2005:  "Insufficient margin",
	2006:  "Position does not exist",
	2007:  "Position risk too high",
	3001:  "Connection limit exceeded",
	3002:  "Subscription limit exceeded",
	3003:  "Invalid subscription",
	3004:  "WebSocket connection closed",
}

// ParsePhemexError creates a PhemexAPIError from response data
func ParsePhemexError(code int, message string) *PhemexAPIError {
	if message == "" {
		if knownMsg, exists := PhemexErrorCodes[code]; exists {
			message = knownMsg
		} else {
			message = "Unknown error"
		}
	}
	
	return &PhemexAPIError{
		Code:    code,
		Message: message,
	}
}