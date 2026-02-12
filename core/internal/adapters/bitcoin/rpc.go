// Package bitcoin - Bitcoin RPC adapter for interacting with bitcoind
package bitcoin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Bitcoin Core JSON-RPC client
type Client struct {
	url      string
	user     string
	pass     string
	httpClient *http.Client
}

// NewClient creates a new Bitcoin RPC client
func NewClient(url, user, pass string) *Client {
	return &Client{
		url:  url,
		user: user,
		pass: pass,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UTXO represents an unspent transaction output
type UTXO struct {
	TxID          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Address       string  `json:"address"`
	Amount        float64 `json:"amount"`
	Confirmations int64   `json:"confirmations"`
	Spendable     bool    `json:"spendable"`
	ScriptPubKey  string  `json:"scriptPubKey"`
}

// Tx represents a transaction
type Tx struct {
	TxID          string `json:"txid"`
	Hash          string `json:"hash"`
	Size          int    `json:"size"`
	Vsize         int    `json:"vsize"`
	Confirmations int64  `json:"confirmations"`
	BlockHash     string `json:"blockhash,omitempty"`
	Time          int64  `json:"time,omitempty"`
	Hex           string `json:"hex"`
}

// ScanUtxo represents an unspent output from scantxoutset
type ScanUtxo struct {
	TxID          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Address       string  `json:"address"`
	Amount        float64 `json:"amount"`
	Confirmations int64   `json:"-"`
	Height        int64   `json:"height"`
	ScriptPubKey  string  `json:"scriptPubKey"`
}

// ScanTxOutSetResult is the response from scantxoutset
type ScanTxOutSetResult struct {
	Success bool       `json:"success"`
	Height  int64      `json:"height"`
	TxOuts  int64      `json:"txouts"`
	Unspents []ScanUtxo `json:"unspents"`
}

// rpcRequest represents a JSON-RPC request
type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// rpcResponse represents a JSON-RPC response
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call makes a JSON-RPC call to bitcoind
func (c *Client) call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	if params == nil {
		params = []interface{}{}
	}

	reqBody := rpcRequest{
		JSONRPC: "1.0",
		ID:      "xscore",
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.user, c.pass)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("RPC call failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// Height returns the current blockchain height
func (c *Client) Height(ctx context.Context) (int64, error) {
	result, err := c.call(ctx, "getblockcount")
	if err != nil {
		return 0, err
	}

	var height int64
	if err := json.Unmarshal(result, &height); err != nil {
		return 0, err
	}

	return height, nil
}

// BroadcastTx broadcasts a raw transaction and returns the txid
func (c *Client) BroadcastTx(ctx context.Context, rawHex string) (string, error) {
	result, err := c.call(ctx, "sendrawtransaction", rawHex)
	if err != nil {
		return "", err
	}

	var txid string
	if err := json.Unmarshal(result, &txid); err != nil {
		return "", err
	}

	return txid, nil
}

// GetTx retrieves a transaction by txid
func (c *Client) GetTx(ctx context.Context, txid string) (*Tx, error) {
	// true = verbose output
	result, err := c.call(ctx, "getrawtransaction", txid, true)
	if err != nil {
		return nil, err
	}

	var tx Tx
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, err
	}

	return &tx, nil
}

// EstimateFee estimates the fee rate in sat/vB for confirmation in `blocks` blocks
func (c *Client) EstimateFee(ctx context.Context, blocks int) (float64, error) {
	result, err := c.call(ctx, "estimatesmartfee", blocks)
	if err != nil {
		return 0, err
	}

	var response struct {
		FeeRate float64 `json:"feerate"` // BTC/kB
		Errors  []string `json:"errors"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return 0, err
	}

	if len(response.Errors) > 0 {
		return 0, fmt.Errorf("fee estimation error: %s", response.Errors[0])
	}

	// Convert BTC/kB to sat/vB
	// 1 BTC = 100,000,000 sat
	// 1 kB = 1000 vB
	// BTC/kB * 100,000,000 / 1000 = sat/vB
	satPerVB := response.FeeRate * 100000

	return satPerVB, nil
}

// ListUnspent returns unspent outputs for the given addresses
func (c *Client) ListUnspent(ctx context.Context, minConf, maxConf int, addresses []string) ([]UTXO, error) {
	result, err := c.call(ctx, "listunspent", minConf, maxConf, addresses)
	if err != nil {
		return nil, err
	}

	var utxos []UTXO
	if err := json.Unmarshal(result, &utxos); err != nil {
		return nil, err
	}

	return utxos, nil
}

// GetRawTransaction returns the raw hex of a transaction
func (c *Client) GetRawTransaction(ctx context.Context, txid string) (string, error) {
	// false = non-verbose (hex only)
	result, err := c.call(ctx, "getrawtransaction", txid, false)
	if err != nil {
		return "", err
	}

	var hex string
	if err := json.Unmarshal(result, &hex); err != nil {
		return "", err
	}

	return hex, nil
}

// GetBlockHash returns the block hash for a given height
func (c *Client) GetBlockHash(ctx context.Context, height int64) (string, error) {
	result, err := c.call(ctx, "getblockhash", height)
	if err != nil {
		return "", err
	}

	var hash string
	if err := json.Unmarshal(result, &hash); err != nil {
		return "", err
	}

	return hash, nil
}

// GenerateToAddress mines blocks to the specified address (regtest only)
func (c *Client) GenerateToAddress(ctx context.Context, numBlocks int, address string) ([]string, error) {
	result, err := c.call(ctx, "generatetoaddress", numBlocks, address)
	if err != nil {
		return nil, err
	}

	var hashes []string
	if err := json.Unmarshal(result, &hashes); err != nil {
		return nil, err
	}

	return hashes, nil
}

// ScanTxOutSet scans the UTXO set for the given addresses.
func (c *Client) ScanTxOutSet(ctx context.Context, addresses []string) (*ScanTxOutSetResult, error) {
	if len(addresses) == 0 {
		return &ScanTxOutSetResult{Success: true, Unspents: []ScanUtxo{}}, nil
	}

	scanObjects := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		scanObjects = append(scanObjects, fmt.Sprintf("addr(%s)", addr))
	}

	result, err := c.call(ctx, "scantxoutset", "start", scanObjects)
	if err != nil {
		return nil, err
	}

	var resp ScanTxOutSetResult
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
