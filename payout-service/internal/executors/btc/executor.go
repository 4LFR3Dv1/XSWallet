package btc

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"domniwallet/payout-service/internal/store"
	"domniwallet/reuse/xs_wallet/adapters/bitcoin"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

const (
	p2trInputVBytes  = 58.0
	p2trOutputVBytes = 43.0
	txOverheadVBytes = 10.5
	dustThresholdSat = int64(546)
)

type Executor struct {
	rpc         *rpcClient
	walletRPC   *rpcClient
	adminRPC    *rpcClient
	walletName  string
	params      *chaincfg.Params
	confirms    int64
	feeFallback float64
}

func New(rpcURL, rpcUser, rpcPass, walletName, network string, confirms int64, feeFallback float64) (*Executor, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("btc rpc not configured")
	}
	params, err := chainParams(network)
	if err != nil {
		return nil, err
	}
	baseURL := rpcURL
	if idx := strings.Index(rpcURL, "/wallet/"); idx != -1 {
		baseURL = rpcURL[:idx]
	}
	walletURL := rpcURL
	if walletName != "" && !strings.Contains(rpcURL, "/wallet/") {
		walletURL = strings.TrimRight(baseURL, "/") + "/wallet/" + walletName
	}
	return &Executor{
		rpc:         newRPC(baseURL, rpcUser, rpcPass),
		walletRPC:   newRPC(walletURL, rpcUser, rpcPass),
		adminRPC:    newRPC(baseURL, rpcUser, rpcPass),
		walletName:  walletName,
		params:      params,
		confirms:    confirms,
		feeFallback: feeFallback,
	}, nil
}

func (e *Executor) Execute(ctx context.Context, payout *store.Payout, utxos []store.ReservedUTXO) (string, error) {
	if payout == nil {
		return "", fmt.Errorf("nil payout")
	}
	if !bitcoin.IsTaprootAddress(payout.Destination, e.params) {
		return "", fmt.Errorf("destination must be taproot (p2tr)")
	}
	if len(utxos) == 0 {
		return "", fmt.Errorf("no reserved utxos")
	}

	changeAddr, err := e.walletRPC.GetRawChangeAddress(ctx, "bech32m")
	if err != nil {
		if isWalletRPCError(err) {
			if err := e.ensureWallet(ctx); err == nil {
				changeAddr, err = e.walletRPC.GetRawChangeAddress(ctx, "bech32m")
			}
		}
		if err != nil {
			return "", fmt.Errorf("getrawchangeaddress: %w", err)
		}
	}

	feeRate, err := e.rpc.EstimateFeeSatVb(ctx)
	if err != nil {
		feeRate = e.feeFallback
	}

	rawHex, err := buildUnsignedTx(utxos, payout.Destination, payout.Amount, changeAddr, feeRate, e.params)
	if err != nil {
		return "", err
	}

	signedHex, complete, err := e.walletRPC.SignRawTransactionWithWallet(ctx, rawHex)
	if err != nil {
		if isWalletRPCError(err) {
			if err := e.ensureWallet(ctx); err == nil {
				signedHex, complete, err = e.walletRPC.SignRawTransactionWithWallet(ctx, rawHex)
			}
		}
		if err != nil {
			return "", fmt.Errorf("signrawtransactionwithwallet: %w", err)
		}
	}
	if !complete {
		return "", fmt.Errorf("signing incomplete")
	}

	txid, err := e.rpc.SendRawTransaction(ctx, signedHex)
	if err != nil {
		return "", fmt.Errorf("sendrawtransaction: %w", err)
	}
	return txid, nil
}

func (e *Executor) Confirmations(ctx context.Context, txid string) (int64, error) {
	tx, err := e.rpc.GetRawTransactionVerbose(ctx, txid)
	if err != nil {
		return 0, err
	}
	return tx.Confirmations, nil
}

func (e *Executor) RequiredConfirmations() int64 {
	return e.confirms
}

func (e *Executor) Params() *chaincfg.Params {
	return e.params
}

func (e *Executor) ensureWallet(ctx context.Context) error {
	if e.adminRPC == nil || e.walletName == "" {
		return nil
	}

	if _, err := e.adminRPC.call(ctx, "loadwallet", e.walletName); err == nil {
		return nil
	} else if strings.Contains(err.Error(), "already loaded") {
		return nil
	}

	if _, err := e.adminRPC.call(ctx, "createwallet", e.walletName); err == nil {
		return nil
	} else {
		msg := err.Error()
		if strings.Contains(msg, "already exists") || strings.Contains(msg, "Database already exists") {
			return nil
		}
		return err
	}
}

func isWalletRPCError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Wallet file not specified") ||
		strings.Contains(msg, "Requested wallet does not exist") ||
		strings.Contains(msg, "not loaded")
}

func chainParams(network string) (*chaincfg.Params, error) {
	switch network {
	case "", "mainnet":
		return &chaincfg.MainNetParams, nil
	case "testnet", "testnet3":
		return &chaincfg.TestNet3Params, nil
	case "regtest":
		return &chaincfg.RegressionNetParams, nil
	default:
		return nil, fmt.Errorf("unknown btc network: %s", network)
	}
}

func buildUnsignedTx(inputs []store.ReservedUTXO, dest string, amount int64, changeAddr string, feeRate float64, params *chaincfg.Params) (string, error) {
	tx := wire.NewMsgTx(wire.TxVersion)
	totalInput := int64(0)

	for _, input := range inputs {
		hash, err := chainhash.NewHashFromStr(input.TxID)
		if err != nil {
			return "", fmt.Errorf("invalid txid: %w", err)
		}
		outPoint := wire.NewOutPoint(hash, input.Vout)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		txIn.Sequence = 0xfffffffd
		tx.AddTxIn(txIn)
		totalInput += input.Amount
	}

	destScript, err := bitcoin.TaprootScriptPubKey(dest, params)
	if err != nil {
		return "", fmt.Errorf("invalid destination: %w", err)
	}
	tx.AddTxOut(wire.NewTxOut(amount, destScript))

	estimatedVBytes := txOverheadVBytes + float64(len(inputs))*p2trInputVBytes + p2trOutputVBytes
	fee := int64(estimatedVBytes * feeRate)
	change := totalInput - amount - fee

	if change > dustThresholdSat {
		changeScript, err := bitcoin.TaprootScriptPubKey(changeAddr, params)
		if err != nil {
			return "", fmt.Errorf("invalid change address: %w", err)
		}
		tx.AddTxOut(wire.NewTxOut(change, changeScript))
		estimatedVBytes += p2trOutputVBytes
		fee = int64(estimatedVBytes * feeRate)
		change = totalInput - amount - fee
		if change <= dustThresholdSat {
			// Drop change output; absorb into fee.
			tx.TxOut = tx.TxOut[:1]
			fee = totalInput - amount
		} else {
			tx.TxOut[1].Value = change
		}
	} else if change < 0 {
		return "", fmt.Errorf("insufficient funds: need %d sat, have %d sat", amount+fee, totalInput)
	}

	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf.Bytes()), nil
}

type rpcClient struct {
	url  string
	user string
	pass string
	cli  *http.Client
}

func newRPC(url, user, pass string) *rpcClient {
	return &rpcClient{
		url:  url,
		user: user,
		pass: pass,
		cli:  &http.Client{Timeout: 30 * time.Second},
	}
}

type rpcReq struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResp struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (c *rpcClient) call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	if params == nil {
		params = []interface{}{}
	}
	body, err := json.Marshal(rpcReq{JSONRPC: "1.0", ID: "payout-service", Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out rpcResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}

type feeResp struct {
	FeeRate float64  `json:"feerate"`
	Errors  []string `json:"errors"`
}

func (c *rpcClient) EstimateFeeSatVb(ctx context.Context) (float64, error) {
	result, err := c.call(ctx, "estimatesmartfee", 2)
	if err != nil {
		return 0, err
	}
	var resp feeResp
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, err
	}
	if len(resp.Errors) > 0 {
		return 0, fmt.Errorf("%s", resp.Errors[0])
	}
	return resp.FeeRate * 100000, nil
}

func (c *rpcClient) GetRawChangeAddress(ctx context.Context, addrType string) (string, error) {
	result, err := c.call(ctx, "getrawchangeaddress", addrType)
	if err != nil {
		return "", err
	}
	var addr string
	if err := json.Unmarshal(result, &addr); err != nil {
		return "", err
	}
	return addr, nil
}

func (c *rpcClient) SignRawTransactionWithWallet(ctx context.Context, hexTx string) (string, bool, error) {
	result, err := c.call(ctx, "signrawtransactionwithwallet", hexTx)
	if err != nil {
		return "", false, err
	}
	var resp struct {
		Hex      string `json:"hex"`
		Complete bool   `json:"complete"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", false, err
	}
	return resp.Hex, resp.Complete, nil
}

func (c *rpcClient) SendRawTransaction(ctx context.Context, hexTx string) (string, error) {
	result, err := c.call(ctx, "sendrawtransaction", hexTx)
	if err != nil {
		return "", err
	}
	var txid string
	if err := json.Unmarshal(result, &txid); err != nil {
		return "", err
	}
	return txid, nil
}

type verboseTx struct {
	TxID          string `json:"txid"`
	Confirmations int64  `json:"confirmations"`
}

func (c *rpcClient) GetRawTransactionVerbose(ctx context.Context, txid string) (*verboseTx, error) {
	result, err := c.call(ctx, "getrawtransaction", txid, true)
	if err != nil {
		return nil, err
	}
	var tx verboseTx
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}
