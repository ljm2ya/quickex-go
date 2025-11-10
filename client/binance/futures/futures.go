package binance

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/ed25519"
)

// SetLeverageResponse represents the response from the set leverage API
type SetLeverageResponse struct {
	Leverage         int    `json:"leverage"`
	MaxNotionalValue string `json:"maxNotionalValue"`
	Symbol           string `json:"symbol"`
}

// makeRestRequest makes an authenticated REST API request to Binance Futures
func (b *BinanceClient) makeRestRequest(method, endpoint string, params map[string]interface{}) ([]byte, error) {
	// Add timestamp and recvWindow
	params["timestamp"] = time.Now().UnixMilli()
	params["recvWindow"] = 5000

	// Build query string with sorted parameters
	values := url.Values{}
	for key, value := range params {
		values.Set(key, fmt.Sprintf("%v", value))
	}
	queryString := values.Encode() // url.Values.Encode() already sorts keys alphabetically

	// Create signature
	sig := ed25519.Sign(b.privateKey, []byte(queryString))
	signature := base64.StdEncoding.EncodeToString(sig)

	values.Set("signature", signature)

	// Create request
	var req *http.Request
	var err error

	fullURL := b.baseURL + endpoint
	if method == "POST" {
		req, err = http.NewRequest("POST", fullURL, bytes.NewBufferString(values.Encode()))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		// For GET and other methods, parameters go in query string
		fullURL += "?" + values.Encode()
		req, err = http.NewRequest(method, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
	}

	// Add headers
	req.Header.Set("X-MBX-APIKEY", b.apiKey)

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (b *BinanceClient) SetLeverage(symbol string, leverage int) error {
	params := map[string]interface{}{
		"symbol":   symbol,
		"leverage": leverage,
	}

	body, err := b.makeRestRequest("POST", "/fapi/v1/leverage", params)
	if err != nil {
		return fmt.Errorf("failed to set leverage: %w", err)
	}

	var response SetLeverageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Verify the leverage was set correctly
	if response.Leverage != leverage {
		return fmt.Errorf("leverage not set correctly: expected %d, got %d", leverage, response.Leverage)
	}

	return nil
}

// PositionRiskInfo represents the response from the position risk REST API
type PositionRiskInfo struct {
	Symbol           string `json:"symbol"`
	PositionSide     string `json:"positionSide"`
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	BreakEvenPrice   string `json:"breakEvenPrice"`
	MarkPrice        string `json:"markPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	LiquidationPrice string `json:"liquidationPrice"`
	IsolatedMargin   string `json:"isolatedMargin"`
	Notional         string `json:"notional"`
	MarginType       string `json:"marginType"`
	IsolatedWallet   string `json:"isolatedWallet"`
	UpdateTime       int64  `json:"updateTime"`
}

// FundingRateInfo represents the response from the funding rate info API
type FundingRateInfo struct {
	Symbol                   string `json:"symbol"`
	AdjustedFundingRateCap   string `json:"adjustedFundingRateCap"`
	AdjustedFundingRateFloor string `json:"adjustedFundingRateFloor"`
	FundingIntervalHours     int    `json:"fundingIntervalHours"`
	Disclaimer               bool   `json:"disclaimer,omitempty"`
}

func (b *BinanceClient) GetFundingRate(symbol string) (*core.FundingRate, error) {
	// Use public API endpoint (no authentication required)
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/fundingInfo")

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch funding rate info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var fundingInfos []FundingRateInfo
	if err := json.NewDecoder(resp.Body).Decode(&fundingInfos); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Find funding info for the specified symbol
	for _, info := range fundingInfos {
		if info.Symbol == symbol {
			// Parse the funding rate cap as the current rate
			rate, err := decimal.NewFromString(info.AdjustedFundingRateCap)
			if err != nil {
				return nil, fmt.Errorf("failed to parse funding rate %s: %w", info.AdjustedFundingRateCap, err)
			}

			// Calculate next funding time (every 8 hours typically)
			nextTime := time.Now().Add(time.Duration(info.FundingIntervalHours) * time.Hour).Unix()

			// For now, use the cap as both current and previous rate
			// In a real implementation, you might want to fetch historical data
			return &core.FundingRate{
				Rate:         rate,
				NextTime:     nextTime,
				PreviousRate: rate,
			}, nil
		}
	}

	return nil, fmt.Errorf("funding rate info not found for symbol %s", symbol)
}

// SetMarginModeResponse represents the response from the set margin type API
type SetMarginModeResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (b *BinanceClient) SetMarginMode(symbol string, mode core.MarginMode) error {
	// Convert core.MarginMode to Binance API format
	var marginType string
	switch mode {
	case core.MarginModeCross:
		marginType = "CROSSED"
	case core.MarginModeIsolated:
		marginType = "ISOLATED"
	default:
		return fmt.Errorf("unsupported margin mode: %s", mode)
	}

	params := map[string]interface{}{
		"symbol":     symbol,
		"marginType": marginType,
	}

	body, err := b.makeRestRequest("POST", "/fapi/v1/marginType", params)
	if err != nil {
		return fmt.Errorf("failed to set margin mode: %w", err)
	}

	var response SetMarginModeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if the operation was successful
	if response.Code != 200 {
		return fmt.Errorf("failed to set margin mode: %s (code: %d)", response.Msg, response.Code)
	}

	return nil
}

func (b *BinanceClient) SetHedgeMode(hedgeMode bool) error {
	/*
		mode := "true"
		if !hedgeMode {
			mode = "false"
		}
		params := map[string]interface{}{
			"dualSidePosition": mode,
		}

		body, err := b.makeRestRequest("POST", "/fapi/v1/positionSide/dual", params)
		if err != nil {
			return fmt.Errorf("failed to set hedge mode: %w", err)
		}

		var response SetMarginModeResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Check if the operation was successful
		if response.Code != 200 {
			return fmt.Errorf("failed to set hedge mode: %s (code: %d)", response.Msg, response.Code)
		}*/

	b.hedgeMode = hedgeMode
	return nil
}
