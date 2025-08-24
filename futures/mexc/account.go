package mexc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// FetchBalance implements core.PrivateClient interface for futures
func (c *MexcFuturesClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if !futuresPosition {
		return decimal.Zero, fmt.Errorf("this is a futures client, use futuresPosition=true or use spot client")
	}
	return c.fetchFuturesPosition(asset)
}

// fetchFuturesPosition fetches futures position for a specific asset
func (c *MexcFuturesClient) fetchFuturesPosition(asset string) (decimal.Decimal, error) {
	params := url.Values{}
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v1/private/account/assets?%s", mexcFuturesBaseURL, signedParams)
	
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to create futures request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to fetch futures position: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return decimal.Zero, fmt.Errorf("futures position request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to read futures response: %w", err)
	}
	
	var futuresAccount struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    []struct {
			Currency         string `json:"currency"`
			PositionValue    string `json:"positionValue"`
			AvailableBalance string `json:"availableBalance"`
			CashBalance      string `json:"cashBalance"`
			FrozenBalance    string `json:"frozenBalance"`
			Equity           string `json:"equity"`
			UnrealizedPNL    string `json:"unrealizedPNL"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &futuresAccount); err != nil {
		return decimal.Zero, fmt.Errorf("failed to unmarshal futures response: %w", err)
	}
	
	if !futuresAccount.Success {
		return decimal.Zero, fmt.Errorf("futures API returned error code: %d", futuresAccount.Code)
	}
	
	for _, position := range futuresAccount.Data {
		if position.Currency == asset {
			// For futures, return the equity (available + frozen + unrealized PNL)
			equity, err := decimal.NewFromString(position.Equity)
			if err != nil {
				return decimal.Zero, fmt.Errorf("failed to parse equity: %w", err)
			}
			return equity, nil
		}
	}
	
	// Asset not found, return zero
	return decimal.Zero, nil
}

// GetAccount fetches the complete futures account information
func (c *MexcFuturesClient) GetAccount() (*core.Account, error) {
	params := url.Values{}
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v1/private/account/assets?%s", mexcFuturesBaseURL, signedParams)
	
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create account request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch account: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("account request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read account response: %w", err)
	}
	
	var accountResponse struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    []struct {
			Currency         string  `json:"currency"`
			PositionValue    string  `json:"positionValue"`
			AvailableBalance string  `json:"availableBalance"`
			CashBalance      string  `json:"cashBalance"`
			FrozenBalance    string  `json:"frozenBalance"`
			Equity           string  `json:"equity"`
			UnrealizedPNL    string  `json:"unrealizedPNL"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &accountResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account response: %w", err)
	}
	
	if !accountResponse.Success {
		return nil, fmt.Errorf("account API returned error code: %d", accountResponse.Code)
	}
	
	account := &core.Account{
		Assets:    make(map[string]*core.Wallet),
		Positions: make(map[string]*core.Position),
	}
	
	var totalEquity float64
	
	// Process assets
	for _, asset := range accountResponse.Data {
		availableBalance, _ := decimal.NewFromString(asset.AvailableBalance)
		frozenBalance, _ := decimal.NewFromString(asset.FrozenBalance)
		equity, _ := decimal.NewFromString(asset.Equity)
		
		wallet := &core.Wallet{
			Asset:  asset.Currency,
			Free:   availableBalance,
			Locked: frozenBalance,
			Total:  equity,
		}
		account.Assets[asset.Currency] = wallet
		
		equityFloat, _ := equity.Float64()
		totalEquity += equityFloat
	}
	
	account.CrossBalance = totalEquity
	
	return account, nil
}

// GetBalance is a convenience method for backward compatibility
func (c *MexcFuturesClient) GetBalance(asset string, includeLocked bool) (float64, error) {
	balance, err := c.fetchFuturesPosition(asset)
	if err != nil {
		return 0, err
	}
	result, _ := balance.Float64()
	return result, nil
}
