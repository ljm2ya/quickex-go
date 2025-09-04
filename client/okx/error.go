package okx

import (
	"fmt"
)

// OKXError represents an error from OKX API
type OKXError struct {
	Code    string `json:"code"`
	Message string `json:"msg"`
}

func (e OKXError) Error() string {
	return fmt.Sprintf("OKX API Error %s: %s", e.Code, e.Message)
}

// Common OKX error codes and their meanings
var OKXErrorCodes = map[string]string{
	"0":     "Success",
	"1":     "Operation failed",
	"2":     "Bulk operation partially succeeded",
	"50000": "Body can not be empty",
	"50001": "Service temporarily unavailable",
	"50002": "JSON syntax error",
	"50004": "Endpoint request timeout (does not indicate success or failure of order, please check order status)",
	"50005": "API key doesn't exist",
	"50006": "Invalid API key",
	"50007": "Invalid sign",
	"50008": "Invalid timestamp",
	"50009": "Invalid passphrase",
	"50010": "Invalid IP",
	"50011": "Invalid content-type, please use application/json format",
	"50012": "Invalid request format",
	"50013": "System is busy, please try again later",
	"50014": "Parameter {param0} can not be empty",
	"50015": "Either parameter {param0} or {param1} is required",
	"50016": "Parameter {param0} does not match parameter {param1}",
	"50017": "The position is frozen due to ADL. Operation restricted",
	"50018": "The currency is frozen due to ADL. Operation restricted",
	"50019": "The account is frozen due to ADL. Operation restricted",
	"50020": "The position is frozen due to liquidation. Operation restricted",
	"50021": "The currency is frozen due to liquidation. Operation restricted",
	"50022": "The account is frozen due to liquidation. Operation restricted",
	"50023": "Funding fee is being settled. Operation restricted",
	"50024": "Parameter {param0} should be {param1}",
	"50025": "Parameter {param0} count exceeds the limit {param1}",
	"50026": "System error",
	"50027": "The account is restricted from trading",
	"50028": "Unable to take the order, please reach out to support center for details",
	"51000": "Parameter {param0} error",
	"51001": "Instrument ID does not exist",
	"51002": "Instrument ID does not match underlying index",
	"51003": "Either client order ID or order ID is required",
	"51004": "Order amount should be greater than the min available amount",
	"51005": "Order amount should be less than the max available amount",
	"51006": "Order price is out of the available range",
	"51007": "Order placement failed. Order amount should be at least 1 contract (showing up when placing an order with less than 1 contract)",
	"51008": "Order placement failed. Order amount should be greater than {param0} (showing up when placing an order with less than the minimum amount)",
	"51009": "Order placement failed. Order amount should be less than {param0} (showing up when placing an order with more than the maximum amount)",
	"51010": "Order placement failed. The price should be better than {param0}",
	"51011": "Order placement failed. The price should be {param0}",
	"51012": "Order placement failed",
	"51013": "Order cancellation failed",
	"51014": "Order modification failed",
	"51015": "Order does not exist",
	"51016": "Order modification failed. The order is completely filled",
	"51017": "Order modification failed. The order is cancelled",
	"51018": "Order modification failed. Order status is invalid",
	"51019": "Order modification failed. Order is pending cancel",
	"51020": "Order quantity should be greater than {param0}",
	"51021": "Order quantity should be less than {param0}",
	"51022": "Order price should be greater than {param0}",
	"51023": "Order price should be less than {param0}",
	"51024": "Data requested is loading",
	"51025": "Data requested is not supported currently",
	"51026": "Parameter {param0} should be greater than parameter {param1}",
	"51027": "Parameter {param0} should be less than parameter {param1}",
	"51028": "Parameter {param0} should be greater than {param1}",
	"51029": "Parameter {param0} should be less than {param1}",
	"51030": "Parameter {param0} is invalid",
	"58000": "Account configuration retrieving failed",
	"58001": "Account {param0} does not exist",
	"58002": "Account {param0} is suspended",
	"58003": "Sub-account does not exist",
	"58004": "Sub-account {param0} is suspended",
	"58005": "Sub-account does not match master account",
	"58006": "The API Key can only be used by master account",
	"58007": "The current account mode does not support this API endpoint",
	"58008": "The account is suspended",
	"58009": "This operation can not be performed under the current account mode",
	"58010": "The account is restricted from trading",
	"58011": "Withdrawal address is not on the whitelist for this API Key",
	"58012": "The account mode should be {param0}",
	"58013": "The account is not unified margin account",
	"58100": "Trading account does not exist",
	"58101": "Account balance is insufficient",
	"58102": "Account balance is insufficient, and the remaining balance after deduction would be less than 0",
	"58103": "Account equity is insufficient",
	"58104": "Available balance is insufficient",
	"58105": "Available margin is insufficient",
	"58106": "Transferring funds failed",
	"58107": "Transferring funds failed. The remaining balance after deduction would be less than 0",
	"58108": "Transferring funds failed. Available balance is insufficient",
	"58109": "Transferring funds failed. Available margin is insufficient",
	"58110": "Transferring funds failed. Position margin will be insufficient after transfer",
	"58111": "Transferring funds failed. Available equity is insufficient",
	"58112": "Transferring funds failed. The remaining available balance would be less than 0",
	"58113": "Transferring funds failed. You can only transfer {param0}",
	"58114": "Transfer suspended",
	"58115": "Sub-account does not exist",
	"58116": "Transfer amount is too small",
	"58117": "Transfer amount is too large",
	"58118": "Transferring funds failed. Available balance of the sub-account is insufficient",
	"58200": "Withdrawal authentication failed",
	"58201": "Withdrawal address does not exist or has not been approved",
	"58202": "Withdrawal amount is less than the minimum withdrawal amount {param0}",
	"58203": "Withdrawal amount exceeds the maximum withdrawal amount {param0}",
	"58204": "Withdrawal amount exceeds the remaining daily withdrawal amount {param0}",
	"58205": "Withdrawal request failed. Insufficient balance",
	"58206": "Withdrawal to address failed",
	"58207": "Please bind your email before withdrawal",
	"58208": "Please bind your funds password before withdrawal",
	"58209": "Funds password verification failed",
	"58210": "Withdrawal fee exceeds the maximum withdrawal fee {param0}",
	"58211": "Withdrawal fee is less than the minimum withdrawal fee {param0}",
	"58212": "Withdrawal fee should be {param0}% of the withdrawal amount",
	"59000": "Your account has not opened futures trading. Please refer to the following link to open futures trading: https://www.okx.com/balance/futures-account-activate",
	"59001": "Futures account is being liquidated",
	"59100": "You have open orders. Please cancel all open orders before changing the leverage",
	"59101": "You have open positions. Please close all positions before changing the leverage",
	"59102": "Margin ratio is too high. Please add margin or reduce positions",
	"59103": "Position does not exist",
	"59104": "Insufficient positions",
	"59105": "There are open orders. Please cancel all open orders before closing positions",
	"59106": "There are open orders. Please cancel all open orders or wait for the orders to be completed before adjusting your margin",
	"59107": "Position does not match the order",
	"59108": "The order price deviates significantly from the current market price",
	"59109": "The order price is beyond the liquidation price. Please adjust the order price",
	"59200": "Insufficient account balance",
	"59201": "Greeks disabled",
	"60001": "Order number does not exist",
	"60002": "Order modification failed",
	"60003": "Order cancellation failed",
	"60004": "Order placement failed",
	"60005": "Order placement failed. Please modify the order and try again",
	"60006": "Order modification failed. Please place a new order",
	"60007": "Order modification failed. The order has been completed",
	"60008": "Order modification failed. The order does not exist",
	"60009": "Order cancellation failed. The order has been completed",
	"60010": "Order cancellation failed. The order does not exist",
	"60011": "Order cancellation failed",
	"60012": "This type of order cannot be cancelled",
	"60013": "Order placement failed. This trading pair does not exist",
	"60014": "Order placement failed. This order type is not supported for the current trading mode",
	"60015": "Order placement failed. Orders of this size cannot be accepted",
	"60016": "Order placement failed. Market order size should be at least {param0} {param1}",
	"60017": "Order placement failed. The order price is invalid",
	"60018": "Order placement failed. The order size is invalid",
	"60019": "Order placement failed. The order has exceeded the maximum order amount of a single order {param0}",
	"60020": "Order placement failed. User does not exist",
	"60021": "Order placement failed. The order amount is lower than the minimum amount {param0}",
	"60022": "Order placement failed. The order amount exceeds the user's maximum order amount {param0}",
	"60023": "Order placement failed. The market price has changed significantly. Please place the order again",
	"60024": "Order placement failed. The current price deviates from the oracle price by more than 5%",
	"60025": "Order placement failed. The order price should not be {param0} than {param1}",
	"60026": "Order placement failed. The order price should be {param0} than the bankruptcy price {param1}",
	"60027": "Order placement failed. The order price should be {param0} than the current index price {param1}",
	"60028": "Order placement failed. The total amount of pending orders and positions has exceeded the maximum position limit",
	"60029": "Order placement failed. Position side does not match order side",
	"60030": "Order placement failed. Reduce-only orders are only allowed to reduce your current position",
	"60031": "The order price or trigger price exceeds {param0}%",
	"60032": "Order placement failed. Leverage cannot be 0",
	"60033": "Order placement failed. Order amount should be an integer multiple of the contract value {param0}",
	"60034": "Order placement failed. The order price should be higher than the minimum price {param0}",
	"60035": "Order placement failed. The order price should be lower than the maximum price {param0}",
	"60036": "Order placement failed. The leverage is invalid",
	"60037": "Order placement failed. The trading amount does not meet the minimum trading amount {param0}",
	"60038": "Order placement failed. The trading amount does not meet the accuracy requirements. The decimal part can be at most {param0} digits",
	"60039": "Order placement failed. The order price does not meet the accuracy requirements. The decimal part can be at most {param0} digits",
}

// ParseOKXError creates an OKXError from response data
func ParseOKXError(code, message string) *OKXError {
	if code == "0" {
		return nil // Success
	}
	
	// Use predefined message if available, otherwise use provided message
	if knownMsg, exists := OKXErrorCodes[code]; exists && knownMsg != "Success" {
		message = knownMsg
	}
	
	return &OKXError{
		Code:    code,
		Message: message,
	}
}

// IsRateLimitError checks if the error is related to rate limiting
func IsRateLimitError(code string) bool {
	rateLimitCodes := []string{"50001", "50004", "50013"}
	for _, rlCode := range rateLimitCodes {
		if code == rlCode {
			return true
		}
	}
	return false
}

// IsTemporaryError checks if the error is temporary and operation should be retried
func IsTemporaryError(code string) bool {
	temporaryErrorCodes := []string{"50001", "50004", "50013", "50026", "51024"}
	for _, tempCode := range temporaryErrorCodes {
		if code == tempCode {
			return true
		}
	}
	return false
}