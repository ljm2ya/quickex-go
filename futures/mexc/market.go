package mexc

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// SubscribeQuotes implements core.PublicClient interface for futures
func (c *MexcFuturesClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]chan core.Quote, error) {
	if !c.connected {
		return nil, fmt.Errorf("client not connected, call Connect() first")
	}
	
	quoteChans := make(map[string]chan core.Quote)
	
	// Create channels for each symbol
	c.quoteMu.Lock()
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 100)
		quoteChans[symbol] = quoteChan
		c.quoteChans[symbol] = quoteChan
	}
	c.quoteMu.Unlock()
	
	// Subscribe to each symbol via WebSocket
	for _, symbol := range symbols {
		subMsg := map[string]interface{}{
			"method": "sub.ticker",
			"param":  map[string]interface{}{
				"symbol": symbol,
			},
		}
		
		c.wsMu.Lock()
		if c.wsConn != nil {
			if err := c.wsConn.WriteJSON(subMsg); err != nil {
				c.wsMu.Unlock()
				if errHandler != nil {
					errHandler(fmt.Errorf("failed to subscribe to %s: %w", symbol, err))
				}
				continue
			}
		}
		c.wsMu.Unlock()
	}
	
	return quoteChans, nil
}

// FetchMarketRules implements core.PublicClient interface for futures
func (c *MexcFuturesClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	// MEXC futures contract info endpoint
	url := mexcFuturesBaseURL + "/api/v1/contract/detail"
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch contract info: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("contract info request failed with status: %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var contractInfo struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    []struct {
			Symbol            string  `json:"symbol"`
			DisplayName       string  `json:"displayName"`
			DisplayNameEn     string  `json:"displayNameEn"`
			PositionOpenType  int     `json:"positionOpenType"`
			BaseCoin          string  `json:"baseCoin"`
			QuoteCoin         string  `json:"quoteCoin"`
			SettleCoin        string  `json:"settleCoin"`
			ContractSize      float64 `json:"contractSize"`
			MinLeverage       int     `json:"minLeverage"`
			MaxLeverage       int     `json:"maxLeverage"`
			PriceScale        int     `json:"priceScale"`
			VolScale          int     `json:"volScale"`
			AmountScale       int     `json:"amountScale"`
			PriceUnit         float64 `json:"priceUnit"`
			VolUnit           float64 `json:"volUnit"`
			MinVol            float64 `json:"minVol"`
			MaxVol            float64 `json:"maxVol"`
			BidLimitPriceRate float64 `json:"bidLimitPriceRate"`
			AskLimitPriceRate float64 `json:"askLimitPriceRate"`
			State             int     `json:"state"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &contractInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract info: %w", err)
	}
	
	if !contractInfo.Success {
		return nil, fmt.Errorf("contract info API returned error code: %d", contractInfo.Code)
	}
	
	// Create set of requested quotes for filtering
	quoteSet := make(map[string]struct{})
	for _, quote := range quotes {
		quoteSet[quote] = struct{}{}
	}
	
	var rules []core.MarketRule
	for _, contract := range contractInfo.Data {
		// Filter by quote asset if specified
		if len(quotes) > 0 {
			if _, exists := quoteSet[contract.QuoteCoin]; !exists {
				continue
			}
		}
		
		// Only include active contracts
		if contract.State != 0 {
			continue
		}
		
		rule := core.MarketRule{
			Symbol:         contract.Symbol,
			BaseAsset:      contract.BaseCoin,
			QuoteAsset:     contract.QuoteCoin,
			PricePrecision: int64(contract.PriceScale),
			QtyPrecision:   int64(contract.VolScale),
			MinPrice:       decimal.Zero,
			MaxPrice:       decimal.NewFromInt(999999999),
			MinQty:         decimal.NewFromFloat(contract.MinVol),
			MaxQty:         decimal.NewFromFloat(contract.MaxVol),
			TickSize:       decimal.NewFromFloat(contract.PriceUnit),
			StepSize:       decimal.NewFromFloat(contract.VolUnit),
		}
		
		rules = append(rules, rule)
	}
	
	if len(rules) == 0 {
		return nil, fmt.Errorf("no market rules found for quotes: %v", quotes)
	}
	
	return rules, nil
}

// GetTicker fetches current ticker for a futures symbol
func (c *MexcFuturesClient) GetTicker(symbol string) (*core.Ticker, error) {
	reqURL := fmt.Sprintf("%s/api/v1/contract/ticker?symbol=%s", mexcFuturesBaseURL, symbol)
	
	resp, err := http.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch futures ticker: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("futures ticker request failed with status: %d", resp.StatusCode)
	}
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read futures ticker response: %w", err)
	}
	
	var tickerResponse struct {
		Success bool `json:"success"`
		Code    int  `json:"code"`
		Data    struct {
			Symbol       string  `json:"symbol"`
			LastPrice    float64 `json:"lastPrice"`
			Bid1         float64 `json:"bid1"`
			Ask1         float64 `json:"ask1"`
			Volume24     float64 `json:"volume24"`
			Amount24     float64 `json:"amount24"`
			HoldVol      float64 `json:"holdVol"`
			Lower24Price float64 `json:"lower24Price"`
			High24Price  float64 `json:"high24Price"`
			RiseFallRate float64 `json:"riseFallRate"`
			RiseFallValue float64 `json:"riseFallValue"`
			IndexPrice   float64 `json:"indexPrice"`
			FairPrice    float64 `json:"fairPrice"`
			FundingRate  float64 `json:"fundingRate"`
			MaxBidPrice  float64 `json:"maxBidPrice"`
			MinAskPrice  float64 `json:"minAskPrice"`
			Timestamp    int64   `json:"timestamp"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &tickerResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal futures ticker: %w", err)
	}
	
	if !tickerResponse.Success {
		return nil, fmt.Errorf("futures ticker API returned error code: %d", tickerResponse.Code)
	}
	
	return &core.Ticker{
		Symbol:    tickerResponse.Data.Symbol,
		LastPrice: tickerResponse.Data.LastPrice,
		BidPrice:  tickerResponse.Data.Bid1,
		BidQty:    0, // MEXC futures ticker doesn't provide bid/ask quantities
		AskPrice:  tickerResponse.Data.Ask1,
		AskQty:    0,
		Volume:    tickerResponse.Data.Volume24,
		Time:      time.UnixMilli(tickerResponse.Data.Timestamp),
	}, nil
}
