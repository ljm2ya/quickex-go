package bybit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hirokisan/bybit/v2"
	"github.com/ljm2ya/quickex-go/core"
	"github.com/shopspring/decimal"
)

const (
	BaseUSDCWithdrawFee = 0.5
	BaseETHWithdrawFee  = 0.0003
)

// SetDepositAccountResponse represents the response from the set deposit account API
type SetDepositAccountResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Status int `json:"status"`
	} `json:"result"`
	Time int64 `json:"time"`
}

// InternalTransferResponse represents the response from the internal transfer API
type InternalTransferResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		TransferID string `json:"transferId"`
	} `json:"result"`
	Time int64 `json:"time"`
}

// UnifiedTransferableAmountResponse represents the response from the unified transferable amount API
type UnifiedTransferableAmountResponse struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		Avail string `json:"availableWithdrawal"`
	} `json:"result"`
	Time int64 `json:"time"`
}

func withdrawFee(asset string, chain core.Chain) decimal.Decimal {
	switch asset {
	case "ETH":
		if chain == core.BASE {
			return decimal.NewFromFloat(BaseETHWithdrawFee)
		}
	case "USDC":
		if chain == core.BASE {
			return decimal.NewFromFloat(BaseUSDCWithdrawFee)
		}
	}
	return decimal.Zero
}

// FetchWithdrawableAmount implements core.TransactionClient interface
// Uses the Bybit unified transferable amount API endpoint to get withdrawable amount
func (c *BybitClient) FetchWithdrawableAmount(asset string) (decimal.Decimal, error) {
	if asset == "" {
		return decimal.Zero, fmt.Errorf("asset cannot be empty")
	}

	// Call the unified transferable amount API
	resp, err := c.callUnifiedTransferableAmountAPI(asset)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get transferable amount: %w", err)
	}

	if resp.RetCode != 0 {
		return decimal.Zero, fmt.Errorf("API error: %s (code: %d)", resp.RetMsg, resp.RetCode)
	}

	if resp.Result.Avail == "" {
		return decimal.Zero, fmt.Errorf("API passed zero value for asset: %s", asset)
	}
	return decimal.RequireFromString(resp.Result.Avail), nil
}

// callUnifiedTransferableAmountAPI makes the HTTP request to the Bybit unified transferable amount endpoint
func (c *BybitClient) callUnifiedTransferableAmountAPI(coin string) (*UnifiedTransferableAmountResponse, error) {
	// Build the URL with query parameters
	params := url.Values{}
	params.Set("coinName", coin)

	// Create the full URL
	baseURL := "https://api.bybit.com"
	endpoint := "/v5/account/withdrawal"
	fullURL := baseURL + endpoint + "?" + params.Encode()

	// Create timestamp and signature
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"

	// Create the signature
	queryString := params.Encode()
	signature := c.createSignature(timestamp, c.apiKey, recvWindow, queryString)

	// Create HTTP request
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("X-BAPI-API-KEY", c.apiKey)
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var transferableResp UnifiedTransferableAmountResponse
	if err := json.Unmarshal(body, &transferableResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &transferableResp, nil
}

// SetDepositAccount sets the deposit account type to UNIFIED
func (c *BybitClient) SetDepositAccount(accountType string) error {
	// Create request body
	requestBody := map[string]interface{}{
		"accountType": accountType,
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create the full URL
	baseURL := "https://api.bybit.com"
	endpoint := "/v5/asset/deposit/deposit-to-account"
	fullURL := baseURL + endpoint

	// Create timestamp and signature
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"

	// Create the signature for POST request
	signature := c.createSignature(timestamp, c.apiKey, recvWindow, string(bodyBytes))

	// Create HTTP request
	req, err := http.NewRequest("POST", fullURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("X-BAPI-API-KEY", c.apiKey)
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var setDepositResp SetDepositAccountResponse
	if err := json.Unmarshal(body, &setDepositResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if setDepositResp.RetCode != 0 {
		return fmt.Errorf("API error: %s (code: %d)", setDepositResp.RetMsg, setDepositResp.RetCode)
	}

	return nil
}

// createSignature creates the HMAC SHA256 signature for Bybit API
func (c *BybitClient) createSignature(timestamp, apiKey, recvWindow, queryString string) string {
	// Create the raw signature string
	rawSignature := timestamp + apiKey + recvWindow + queryString

	// Create HMAC SHA256 hash
	h := hmac.New(sha256.New, []byte(c.apiSecret))
	h.Write([]byte(rawSignature))

	return hex.EncodeToString(h.Sum(nil))
}

// CreateInternalTransfer transfers funds from UNIFIED to FUNDING account
func (c *BybitClient) CreateInternalTransfer(coin string, amount decimal.Decimal) (string, error) {
	// Generate a unique transfer ID using UUID
	transferID := uuid.New().String()

	// Create request body
	requestBody := map[string]interface{}{
		"transferId":      transferID,
		"coin":            coin,
		"amount":          amount.String(),
		"fromAccountType": "UNIFIED",
		"toAccountType":   "FUND",
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create the full URL
	baseURL := "https://api.bybit.com"
	endpoint := "/v5/asset/transfer/inter-transfer"
	fullURL := baseURL + endpoint

	// Create timestamp and signature
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recvWindow := "5000"

	// Create the signature for POST request
	signature := c.createSignature(timestamp, c.apiKey, recvWindow, string(bodyBytes))

	// Create HTTP request
	req, err := http.NewRequest("POST", fullURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("X-BAPI-API-KEY", c.apiKey)
	req.Header.Set("X-BAPI-TIMESTAMP", timestamp)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWindow)
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var transferResp InternalTransferResponse
	if err := json.Unmarshal(body, &transferResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if transferResp.RetCode != 0 {
		return "", fmt.Errorf("API error: %s (code: %d)", transferResp.RetMsg, transferResp.RetCode)
	}

	return transferResp.Result.TransferID, nil
}

// FetchDepositAddress implements core.AccountClient interface
// Uses GetMasterDepositAddress to get deposit address for specified asset and chain
func (c *BybitClient) FetchDepositAddress(asset string, chain core.Chain) (string, error) {
	// Convert core.Chain to Bybit chain format
	chainType := c.mapChainToBybit(chain)

	resp, err := c.client.V5().Asset().GetMasterDepositAddress(bybit.V5GetMasterDepositAddressParam{
		Coin: bybit.Coin(asset),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get deposit address: %w", err)
	}

	// Find the chain in the response
	for _, chainInfo := range resp.Result.Chains {
		if chainInfo.ChainType == chainType || chainInfo.Chain == chainType {
			if chainInfo.AddressDeposit == "" {
				return "", fmt.Errorf("no deposit address available for %s on chain %s", asset, chain)
			}

			// If there's a tag/memo, append it to the address
			if chainInfo.TagDeposit != "" {
				return fmt.Sprintf("%s?tag=%s", chainInfo.AddressDeposit, chainInfo.TagDeposit), nil
			}

			return chainInfo.AddressDeposit, nil
		}
	}

	return "", fmt.Errorf("chain %s not found for asset %s", chain, asset)
}

// Withdraw implements core.AccountClient interface
// Initiates a withdrawal to the specified address
func (c *BybitClient) Withdraw(asset string, chain core.Chain, address, tag string, amount decimal.Decimal) (string, error) {
	// First, transfer funds from UNIFIED to FUNDING account (required for withdrawal)
	transferID, err := c.CreateInternalTransfer(asset, amount.Add(withdrawFee(asset, chain)))
	if err != nil {
		return "", fmt.Errorf("failed to transfer funds to funding account: %w", err)
	}

	// Log the transfer for reference
	fmt.Printf("Successfully transferred %s %s to funding account (Transfer ID: %s)\n", amount.String(), asset, transferID)

	// Convert core.Chain to Bybit chain format
	chainType := c.mapChainToBybit(chain)
	accountType := bybit.AccountTypeV5FUND // Use FUND account for withdrawal

	// Build withdrawal parameters
	params := bybit.V5WithdrawParam{
		Coin:        bybit.Coin(asset),
		Chain:       &chainType,
		Address:     address,
		Amount:      amount.String(),
		Timestamp:   c.getCurrentTimestamp(),
		AccountType: &accountType,
	}

	// Add tag if provided
	if tag != "" {
		params.Tag = &tag
	}

	resp, err := c.client.V5().Asset().Withdraw(params)
	if err != nil {
		return "", fmt.Errorf("failed to initiate withdrawal: %w", err)
	}

	return resp.Result.ID, nil
}

// FetchWithdrawTxid implements core.AccountClient interface
// Gets the transaction ID for a withdrawal using GetWithdrawalRecords
func (c *BybitClient) FetchWithdrawTxid(withdrawId string) (string, error) {
	resp, err := c.client.V5().Asset().GetWithdrawalRecords(bybit.V5GetWithdrawalRecordsParam{
		WithdrawID: &withdrawId,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get withdrawal records: %w", err)
	}

	if len(resp.Result.Rows) == 0 {
		return "", fmt.Errorf("withdrawal record not found for ID: %s", withdrawId)
	}

	// Return the transaction hash
	txId := resp.Result.Rows[0].TxID
	if txId == "" {
		return "", fmt.Errorf("transaction ID not yet available for withdrawal: %s", withdrawId)
	}

	return txId, nil
}

// Helper method to map core.Chain to Bybit chain format
func (c *BybitClient) mapChainToBybit(chain core.Chain) string {
	switch chain {
	case core.ERC20:
		return "ETH"
	case core.BASE:
		return "BASE"
	default:
		// Return the chain as-is if not mapped
		return string(chain)
	}
}

// Helper method to get current timestamp in milliseconds
func (c *BybitClient) getCurrentTimestamp() int64 {
	return time.Now().UnixMilli()
}

// Helper method to convert chain string from Bybit format to core.Chain
func (c *BybitClient) mapBybitToChain(chainStr string) core.Chain {
	// Convert to uppercase for consistent comparison
	chainStr = strings.ToUpper(chainStr)

	switch chainStr {
	case "ETH", "ETHEREUM":
		return core.ERC20
	case "BASE", "BASE-MAINNET":
		return core.BASE
	default:
		// Return as custom chain
		return core.Chain(chainStr)
	}
}
