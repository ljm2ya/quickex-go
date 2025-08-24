package mexc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/shopspring/decimal"
)

// FetchBalance implements core.PrivateClient interface
func (c *MexcClient) FetchBalance(asset string, includeLocked bool, futuresPosition bool) (decimal.Decimal, error) {
	if futuresPosition {
		return decimal.Zero, fmt.Errorf("MEXC futures is discontinued, use futuresPosition=false for spot balance")
	}
	return c.fetchSpotBalance(asset, includeLocked)
}

// fetchSpotBalance fetches spot balance for a specific asset
func (c *MexcClient) fetchSpotBalance(asset string, includeLocked bool) (decimal.Decimal, error) {
	params := url.Values{}
	signedParams := c.buildSignedParams(params)
	
	reqURL := fmt.Sprintf("%s/api/v3/account?%s", mexcSpotBaseURL, signedParams)
	
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to fetch spot balance: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return decimal.Zero, fmt.Errorf("spot balance request failed with status %d: %s", resp.StatusCode, string(body))
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to read balance response: %w", err)
	}
	
	var accountInfo struct {
		Balances []struct {
			Asset  string `json:"asset"`
			Free   string `json:"free"`
			Locked string `json:"locked"`
		} `json:"balances"`
	}
	
	if err := json.Unmarshal(body, &accountInfo); err != nil {
		return decimal.Zero, fmt.Errorf("failed to unmarshal balance response: %w", err)
	}
	
	for _, balance := range accountInfo.Balances {
		if balance.Asset == asset {
			free, err := decimal.NewFromString(balance.Free)
			if err != nil {
				return decimal.Zero, fmt.Errorf("failed to parse free balance: %w", err)
			}
			
			if includeLocked {
				locked, err := decimal.NewFromString(balance.Locked)
				if err != nil {
					return decimal.Zero, fmt.Errorf("failed to parse locked balance: %w", err)
				}
				return free.Add(locked), nil
			}
			
			return free, nil
		}
	}
	
	// Asset not found, return zero
	return decimal.Zero, nil
}



// GetBalance is a convenience method for backward compatibility
func (c *MexcClient) GetBalance(asset string, includeLocked bool) (float64, error) {
	balance, err := c.fetchSpotBalance(asset, includeLocked)
	if err != nil {
		return 0, err
	}
	result, _ := balance.Float64()
	return result, nil
}
