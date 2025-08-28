package okx

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"
)

// GenerateSignature creates HMAC SHA256 signature for OKX API
func GenerateSignature(timestamp, method, requestPath, body, secretKey string) string {
	message := timestamp + method + requestPath + body
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// GenerateTimestamp generates ISO 8601 timestamp for OKX API
func GenerateTimestamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// GenerateWSSignature creates signature for WebSocket login
func GenerateWSSignature(timestamp, secretKey string) string {
	// For WebSocket login, the message is: timestamp + 'GET' + '/users/self/verify'
	message := timestamp + "GET" + "/users/self/verify"
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// GetTimestampSeconds returns current timestamp in seconds (for WebSocket)
func GetTimestampSeconds() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// CreateAuthHeaders creates the required authentication headers for REST API
func CreateAuthHeaders(apiKey, secretKey, passphrase, method, requestPath, body string) map[string]string {
	timestamp := GenerateTimestamp()
	signature := GenerateSignature(timestamp, method, requestPath, body, secretKey)
	
	return map[string]string{
		"OK-ACCESS-KEY":        apiKey,
		"OK-ACCESS-SIGN":       signature,
		"OK-ACCESS-TIMESTAMP":  timestamp,
		"OK-ACCESS-PASSPHRASE": passphrase,
		"Content-Type":         "application/json",
	}
}

// CreateWSLoginMessage creates WebSocket login message
func CreateWSLoginMessage(apiKey, secretKey, passphrase string) map[string]interface{} {
	timestamp := GetTimestampSeconds()
	signature := GenerateWSSignature(timestamp, secretKey)
	
	return map[string]interface{}{
		"op": "login",
		"args": []map[string]interface{}{
			{
				"apiKey":     apiKey,
				"passphrase": passphrase,
				"timestamp":  timestamp,
				"sign":       signature,
			},
		},
	}
}

// ValidateCredentials checks if all required credentials are provided
func ValidateCredentials(apiKey, secretKey, passphrase string) error {
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}
	if secretKey == "" {
		return fmt.Errorf("secret key is required")
	}
	if passphrase == "" {
		return fmt.Errorf("passphrase is required")
	}
	return nil
}