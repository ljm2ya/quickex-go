package upbit

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

// GetTicker gets ticker data for a symbol
func (u *UpbitClient) GetTicker(symbol string) (*UpbitTickerOfMarket, error) {
	// This is a public endpoint, no authentication needed
	resp, err := http.Get(baseURL + "/v1/ticker?markets=" + symbol)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tickers []UpbitTickerOfMarket
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, err
	}

	if len(tickers) == 0 {
		return nil, errors.New("ticker not found")
	}

	return &tickers[0], nil
}

// GetMarketRules gets market rules for a quote currency
func (u *UpbitClient) GetMarketRules(quote string) ([]UpbitMarket, error) {
	// This is a public endpoint, no authentication needed
	resp, err := http.Get(baseURL + "/v1/market/all")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var markets []UpbitMarket
	if err := json.Unmarshal(body, &markets); err != nil {
		return nil, err
	}

	// Filter by quote currency
	var filtered []UpbitMarket
	for _, market := range markets {
		if strings.HasPrefix(market.Market, quote+"-") {
			filtered = append(filtered, market)
		}
	}

	return filtered, nil
}

// SubscribeQuotes implements core.PublicClient interface
func (u *UpbitClient) SubscribeQuotes(ctx context.Context, symbols []string, errHandler func(err error)) (map[string]<-chan core.Quote, error) {
	quoteChans := make(map[string]<-chan core.Quote)

	// Create channels for each symbol
	for _, symbol := range symbols {
		quoteChan := make(chan core.Quote, 1)
		quoteChans[symbol] = quoteChan

		// Poll ticker data in a goroutine (Upbit doesn't have quote-specific WebSocket)
		go func(sym string, ch chan core.Quote) {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			defer close(ch)

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					tickerData, err := u.GetTicker(sym)
					if err != nil {
						if errHandler != nil {
							errHandler(err)
						}
						continue
					}

					select {
					case ch <- core.Quote{
						Symbol:   tickerData.Market,
						BidPrice: decimal.NewFromFloat(tickerData.TradePrice),
						BidQty:   decimal.Zero, // Upbit ticker doesn't provide bid/ask quantities
						AskPrice: decimal.NewFromFloat(tickerData.TradePrice),
						AskQty:   decimal.Zero,
						Time:     time.Now(),
					}:
					default:
						// Channel full, skip
					}
				}
			}
		}(symbol, quoteChan)
	}

	return quoteChans, nil
}

// FetchMarketRules implements core.PublicClient interface
func (u *UpbitClient) FetchMarketRules(quotes []string) ([]core.MarketRule, error) {
	var allRules []core.MarketRule

	for _, quote := range quotes {
		rules, err := u.GetMarketRules(quote)
		if err != nil {
			return nil, err
		}

		// Convert to core types
		for _, rule := range rules {
			allRules = append(allRules, core.MarketRule{
				Symbol:         rule.Market,
				BaseAsset:      rule.English_name,
				QuoteAsset:     quote,
				PricePrecision: 8, // Upbit default
				QtyPrecision:   8, // Upbit default
				MinPrice:       decimal.Zero,
				MaxPrice:       decimal.NewFromInt(1000000),
				MinQty:         decimal.NewFromFloat(0.00000001),
				MaxQty:         decimal.NewFromInt(1000000),
				TickSize:       decimal.NewFromFloat(0.00000001),
				StepSize:       decimal.NewFromFloat(0.00000001),
				RateLimits:     []core.RateLimit{},
			})
		}
	}

	return allRules, nil
}
