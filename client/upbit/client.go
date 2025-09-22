package upbit

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

const (
	baseURL = "https://api.upbit.com"
)

type UpbitClient struct {
	apiKey    string
	secretKey string
	client    *http.Client
}

func NewUpbitClient(accessKey, secretKey string) *UpbitClient {
	return &UpbitClient{
		apiKey:    accessKey,
		secretKey: secretKey,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Connect implements core.PrivateClient interface
func (u *UpbitClient) Connect(ctx context.Context) (int64, error) {
	// Upbit doesn't use persistent connections like WebSocket
	// Return 0 delta timestamp since no time sync is needed
	return 0, nil
}

// Close implements core.PrivateClient interface
func (u *UpbitClient) Close() error {
	// Upbit doesn't maintain persistent connections
	return nil
}

// Token generates JWT token for authenticated requests
func (u *UpbitClient) Token(query map[string]string) (string, error) {
	claim := jwt.MapClaims{
		"access_key": u.apiKey,
		"nonce":      uuid.New().String(),
	}

	if query != nil && len(query) > 0 {
		urlValues := url.Values{}
		for key, value := range query {
			urlValues.Add(key, value)
		}
		rawQuery := urlValues.Encode()

		// Create SHA512 hash of query string
		hash := sha512.Sum512([]byte(rawQuery))
		claim["query_hash"] = hex.EncodeToString(hash[:])
		claim["query_hash_alg"] = "SHA512"
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claim)
	tokenStr, err := token.SignedString([]byte(u.secretKey))
	if err != nil {
		return "", err
	}

	return "Bearer " + tokenStr, nil
}

// makeRequest makes authenticated HTTP request to Upbit API
func (u *UpbitClient) makeRequest(method, endpoint string, params map[string]string) ([]byte, error) {
	fullURL := baseURL + endpoint

	token, err := u.Token(params)
	if err != nil {
		return nil, err
	}

	var req *http.Request
	if (method == "GET" || method == "DELETE") && len(params) > 0 {
		// For GET and DELETE requests, add parameters to query string
		urlValues := url.Values{}
		for key, value := range params {
			urlValues.Add(key, value)
		}
		fullURL += "?" + urlValues.Encode()
		req, err = http.NewRequest(method, fullURL, nil)
	} else if method == "POST" && len(params) > 0 {
		// For POST requests, send parameters as JSON in body
		jsonBody, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, fullURL, strings.NewReader(string(jsonBody)))
		if err != nil {
			return nil, err
		}
	} else {
		req, err = http.NewRequest(method, fullURL, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// ToSymbol converts asset and quote to exchange-specific symbol format
// Upbit format: USDT-BTC (quote-asset, reversed order with hyphen)
func (u *UpbitClient) ToSymbol(asset, quote string) string {
	return quote + "-" + asset
}

// ToAsset extracts the asset from a symbol (reverse of ToSymbol)
func (u *UpbitClient) ToAsset(symbol string) string {
	// Upbit uses reversed order with hyphen: USDT-BTC (quote-asset)
	// Split by hyphen and return the second part
	parts := strings.Split(symbol, "-")
	if len(parts) >= 2 {
		return parts[1] // Return the second part (asset)
	}
	
	// If no hyphen found, return the symbol as-is (shouldn't happen with valid symbols)
	return symbol
}
