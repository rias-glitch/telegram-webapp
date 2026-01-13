package ton

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a TON API client
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	network    Network
}

// NewClient creates a new TON API client
func NewClient(network Network, apiKey string) *Client {
	baseURL := TonAPIMainnet
	if network == NetworkTestnet {
		baseURL = TonAPITestnet
	}

	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		network: network,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Transaction represents a TON transaction
type Transaction struct {
	Hash        string `json:"hash"`
	Lt          int64  `json:"lt"`
	Account     string `json:"account"`
	Now         int64  `json:"now"`
	OrigStatus  string `json:"orig_status"`
	EndStatus   string `json:"end_status"`
	TotalFees   int64  `json:"total_fees"`
	InMsg       *Message `json:"in_msg"`
	OutMsgs     []Message `json:"out_msgs"`
	Success     bool   `json:"success"`
}

// Message represents a TON message
type Message struct {
	Source       string `json:"source"`
	Destination  string `json:"destination"`
	Value        int64  `json:"value"`
	Bounce       bool   `json:"bounce"`
	Body         string `json:"body"`
	DecodedBody  *DecodedBody `json:"decoded_body"`
}

// DecodedBody represents decoded message body
type DecodedBody struct {
	Text string `json:"text"`
}

// AccountInfo represents account information
type AccountInfo struct {
	Address   string `json:"address"`
	Balance   int64  `json:"balance"`
	Status    string `json:"status"`
	LastTxLt  int64  `json:"last_transaction_lt"`
	LastTxHash string `json:"last_transaction_hash"`
}

// GetAccountInfo retrieves account information
func (c *Client) GetAccountInfo(ctx context.Context, address string) (*AccountInfo, error) {
	url := fmt.Sprintf("%s/accounts/%s", c.baseURL, address)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var account AccountInfo
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, err
	}

	return &account, nil
}

// GetTransactions retrieves recent transactions for an address
func (c *Client) GetTransactions(ctx context.Context, address string, limit int, beforeLt int64) ([]Transaction, error) {
	url := fmt.Sprintf("%s/accounts/%s/transactions?limit=%d", c.baseURL, address, limit)
	if beforeLt > 0 {
		url = fmt.Sprintf("%s&before_lt=%d", url, beforeLt)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Transactions []Transaction `json:"transactions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Transactions, nil
}

// GetTransaction retrieves a specific transaction by hash
func (c *Client) GetTransaction(ctx context.Context, hash string) (*Transaction, error) {
	url := fmt.Sprintf("%s/transactions/%s", c.baseURL, hash)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(body))
	}

	var tx Transaction
	if err := json.NewDecoder(resp.Body).Decode(&tx); err != nil {
		return nil, err
	}

	return &tx, nil
}

// WaitForTransaction waits for a transaction to appear on chain
func (c *Client) WaitForTransaction(ctx context.Context, hash string, timeout time.Duration) (*Transaction, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		tx, err := c.GetTransaction(ctx, hash)
		if err != nil {
			return nil, err
		}
		if tx != nil {
			return tx, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return nil, fmt.Errorf("transaction not found within timeout")
}

// ParseIncomingTransactions filters transactions for incoming payments to a specific address
func ParseIncomingTransactions(txs []Transaction, recipientAddress string) []Transaction {
	var incoming []Transaction
	for _, tx := range txs {
		if tx.InMsg != nil && tx.InMsg.Destination == recipientAddress && tx.InMsg.Value > 0 {
			incoming = append(incoming, tx)
		}
	}
	return incoming
}

// ExtractMemo extracts text memo from a transaction
func ExtractMemo(tx *Transaction) string {
	if tx.InMsg != nil && tx.InMsg.DecodedBody != nil {
		return tx.InMsg.DecodedBody.Text
	}
	return ""
}
