package upbit

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	if method == "GET" && len(params) > 0 {
		urlValues := url.Values{}
		for key, value := range params {
			urlValues.Add(key, value)
		}
		fullURL += "?" + urlValues.Encode()
		req, err = http.NewRequest(method, fullURL, nil)
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	return body, nil
}
