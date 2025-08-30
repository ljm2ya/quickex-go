package phemex

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateSignature creates HMAC SHA256 signature for Phemex WebSocket authentication
func GenerateSignature(apiSecret, apiKey string, expiry int64) string {
	message := apiKey + fmt.Sprintf("%d", expiry)
	h := hmac.New(sha256.New, []byte(apiSecret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// GetExpiryTime returns expiry timestamp (current time + 60 seconds)
func GetExpiryTime() int64 {
	return time.Now().Unix() + 60
}

// ValidateCredentials validates API credentials format
func ValidateCredentials(apiKey, apiSecret string) error {
	if apiKey == "" {
		return fmt.Errorf("API key cannot be empty")
	}
	if apiSecret == "" {
		return fmt.Errorf("API secret cannot be empty")
	}
	return nil
}