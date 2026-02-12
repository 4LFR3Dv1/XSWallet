// Package liquid provides an Elements/Liquid JSON-RPC client adapter
package liquid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps the Elements/Liquid JSON-RPC connection
type Client struct {
	rpcURL   string
	user     string
	password string
	client   *http.Client
}

// Config holds Elements connection configuration
type Config struct {
	Host     string // e.g., "elements:18884" or "localhost:18884"
	User     string
	Password string
}

// NewClient creates a new Elements/Liquid JSON-RPC client
func NewClient(cfg Config) *Client {
	return &Client{
		rpcURL:   fmt.Sprintf("http://%s", cfg.Host),
		user:     cfg.User,
		password: cfg.Password,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// BlockchainInfo represents getblockchaininfo response
type BlockchainInfo struct {
	Chain         string  `json:"chain"`
	Blocks        int64   `json:"blocks"`
	Headers       int64   `json:"headers"`
	BestBlockHash string  `json:"bestblockhash"`
	Mediantime    int64   `json:"mediantime"`
	Progress      float64 `json:"verificationprogress"`
}

// Transaction represents a transaction
type Transaction struct {
	TxID          string `json:"txid"`
	Hex           string `json:"hex"`
	Confirmations int64  `json:"confirmations"`
	BlockHash     string `json:"blockhash"`
}

// UTXO represents an unspent transaction output
type UTXO struct {
	TxID          string  `json:"txid"`
	Vout          uint32  `json:"vout"`
	Address       string  `json:"address"`
	Asset         string  `json:"asset"`
	Amount        float64 `json:"amount"`
	AmountSat     int64   `json:"-"` // Calculated from Amount
	Confirmations int64   `json:"confirmations"`
	ScriptPubKey  string  `json:"scriptPubKey"`
}

// RawInput represents a transaction input
type RawInput struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`
}

// PrevTx represents a previous output needed for segwit signing
type PrevTx struct {
	TxID         string  `json:"txid"`
	Vout         uint32  `json:"vout"`
	ScriptPubKey string  `json:"scriptPubKey"`
	Amount       float64 `json:"amount"`
}

// GetBlockchainInfo returns blockchain info for Liquid network
func (c *Client) GetBlockchainInfo(ctx context.Context) (*BlockchainInfo, error) {
	var result BlockchainInfo
	if err := c.call(ctx, "getblockchaininfo", []interface{}{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHeight returns the current blockchain height
func (c *Client) GetHeight(ctx context.Context) (int64, error) {
	info, err := c.GetBlockchainInfo(ctx)
	if err != nil {
		return 0, err
	}
	return info.Blocks, nil
}

// SendRawTransaction broadcasts a raw transaction and returns the txid
func (c *Client) SendRawTransaction(ctx context.Context, hexTx string) (string, error) {
	var txid string
	if err := c.call(ctx, "sendrawtransaction", []interface{}{hexTx}, &txid); err != nil {
		return "", err
	}
	return txid, nil
}

// GetTransaction retrieves a transaction by txid
func (c *Client) GetTransaction(ctx context.Context, txid string) (*Transaction, error) {
	var result Transaction
	if err := c.call(ctx, "getrawtransaction", []interface{}{txid, true}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetRawTransaction retrieves raw transaction hex by txid
func (c *Client) GetRawTransaction(ctx context.Context, txid string) (string, error) {
	var hexTx string
	if err := c.call(ctx, "getrawtransaction", []interface{}{txid, false}, &hexTx); err != nil {
		return "", err
	}
	return hexTx, nil
}

// ListUnspent returns unspent outputs for addresses
func (c *Client) ListUnspent(ctx context.Context, minConf, maxConf int, addresses []string) ([]UTXO, error) {
	var result []UTXO
	params := []interface{}{minConf, maxConf}
	if len(addresses) > 0 {
		params = append(params, addresses)
	}
	if err := c.call(ctx, "listunspent", params, &result); err != nil {
		return nil, err
	}

	// Convert amounts to satoshis
	for i := range result {
		result[i].AmountSat = int64(result[i].Amount * 100000000)
	}

	return result, nil
}

// GetNewAddress generates a new address
func (c *Client) GetNewAddress(ctx context.Context, label string) (string, error) {
	var addr string
	if err := c.call(ctx, "getnewaddress", []interface{}{label}, &addr); err != nil {
		return "", err
	}
	return addr, nil
}

// EstimateFee estimates the fee rate in sat/vB
func (c *Client) EstimateFee(ctx context.Context, blocks int) (float64, error) {
	var result struct {
		FeeRate float64 `json:"feerate"`
	}
	if err := c.call(ctx, "estimatesmartfee", []interface{}{blocks}, &result); err != nil {
		return 0, err
	}
	// Convert BTC/kvB to sat/vB
	return result.FeeRate * 100000, nil
}

// GenerateToAddress mines blocks to the specified address (regtest only).
func (c *Client) GenerateToAddress(ctx context.Context, numBlocks int, address string) ([]string, error) {
	var hashes []string
	if err := c.call(ctx, "generatetoaddress", []interface{}{numBlocks, address}, &hashes); err != nil {
		return nil, err
	}
	return hashes, nil
}

// IssueAsset issues a new asset (Liquid-specific)
func (c *Client) IssueAsset(ctx context.Context, amount, tokenAmount float64) (string, string, error) {
	var result struct {
		Asset string `json:"asset"`
		Token string `json:"token"`
		TxID  string `json:"txid"`
	}
	if err := c.call(ctx, "issueasset", []interface{}{amount, tokenAmount}, &result); err != nil {
		return "", "", err
	}
	return result.Asset, result.TxID, nil
}

// CreateBlindedAddress creates a confidential address from an unconfidential address and blinding pubkey.
func (c *Client) CreateBlindedAddress(ctx context.Context, address string, blindingPubKeyHex string) (string, error) {
	var blinded string
	if err := c.call(ctx, "createblindedaddress", []interface{}{address, blindingPubKeyHex}, &blinded); err != nil {
		return "", err
	}
	return blinded, nil
}

// ImportAddress imports a watch-only address.
func (c *Client) ImportAddress(ctx context.Context, address, label string, rescan bool) error {
	return c.call(ctx, "importaddress", []interface{}{address, label, rescan}, nil)
}

// ImportBlindingKey imports a blinding private key for a given address.
func (c *Client) ImportBlindingKey(ctx context.Context, address, blindingPrivKeyHex string) error {
	return c.call(ctx, "importblindingkey", []interface{}{address, blindingPrivKeyHex}, nil)
}

// CreateRawTransaction creates a raw transaction with given inputs/outputs.
func (c *Client) CreateRawTransaction(ctx context.Context, inputs []RawInput, outputs map[string]float64) (string, error) {
	var hex string
	if err := c.call(ctx, "createrawtransaction", []interface{}{inputs, outputs}, &hex); err != nil {
		return "", err
	}
	return hex, nil
}

// BlindRawTransaction blinds a raw transaction using wallet blinding keys.
func (c *Client) BlindRawTransaction(ctx context.Context, rawHex string) (string, error) {
	var blinded string
	if err := c.call(ctx, "blindrawtransaction", []interface{}{rawHex}, &blinded); err != nil {
		return "", err
	}
	return blinded, nil
}

// DecodeRawTransaction decodes a raw transaction and returns its vsize.
func (c *Client) DecodeRawTransaction(ctx context.Context, rawHex string) (int64, error) {
	var resp struct {
		Vsize int64 `json:"vsize"`
	}
	if err := c.call(ctx, "decoderawtransaction", []interface{}{rawHex}, &resp); err != nil {
		return 0, err
	}
	return resp.Vsize, nil
}

// SignRawTransactionWithKey signs a raw transaction with given privkeys and prevouts.
func (c *Client) SignRawTransactionWithKey(ctx context.Context, rawHex string, privKeys []string, prevTxs []PrevTx) (string, bool, error) {
	var resp struct {
		Hex      string `json:"hex"`
		Complete bool   `json:"complete"`
	}
	params := []interface{}{rawHex, privKeys, prevTxs}
	if err := c.call(ctx, "signrawtransactionwithkey", params, &resp); err != nil {
		return "", false, err
	}
	return resp.Hex, resp.Complete, nil
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

// rpcError represents a JSON-RPC error
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call makes a JSON-RPC call
func (c *Client) call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      "xs-wallet",
		Method:  method,
		Params:  params,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.rpcURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.user, c.password)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if result != nil {
		if err := json.Unmarshal(rpcResp.Result, result); err != nil {
			return fmt.Errorf("failed to parse result: %w", err)
		}
	}

	return nil
}
