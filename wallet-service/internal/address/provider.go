package address

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
)

const (
	NetworkBTC    = "btc"
	NetworkLiquid = "liquid"
	NetworkTron   = "tron"
)

type Provider struct {
	btcRPC     *rpcClient
	btcAdmin   *rpcClient
	btcWallet  string
	liquidRPC  *rpcClient
	tronPrefix byte
}

func NewProvider(btcURL, btcUser, btcPass, btcWallet, liquidURL, liquidUser, liquidPass string) *Provider {
	p := &Provider{tronPrefix: 0x41, btcWallet: btcWallet}
	if btcURL != "" {
		p.btcRPC = newRPC(btcURL, btcUser, btcPass)
	}
	p.btcAdmin = deriveAdminRPC(btcURL, btcUser, btcPass)
	if btcWallet != "" {
		p.btcRPC = ensureWalletURL(btcURL, btcUser, btcPass, btcWallet)
	}
	if liquidURL != "" {
		p.liquidRPC = newRPC(liquidURL, liquidUser, liquidPass)
	}
	return p
}

func (p *Provider) NextAddress(ctx context.Context, network, asset string) (string, string, error) {
	switch network {
	case NetworkBTC:
		if p.btcRPC == nil {
			return "", "", fmt.Errorf("btc rpc not configured")
		}
		addr, err := p.btcRPC.GetNewAddress(ctx, "wallet-service", "bech32m")
		if err != nil && p.btcAdmin != nil && p.btcWallet != "" && isWalletRPCError(err) {
			if err := p.ensureWallet(ctx); err != nil {
				return "", "", err
			}
			addr, err = p.btcRPC.GetNewAddress(ctx, "wallet-service", "bech32m")
		}
		return addr, "", err
	case NetworkLiquid:
		if p.liquidRPC == nil {
			return "", "", fmt.Errorf("liquid rpc not configured")
		}
		addr, err := p.liquidRPC.GetNewAddress(ctx, "wallet-service", "")
		return addr, "", err
	case NetworkTron:
		addr, err := generateTronAddress(p.tronPrefix)
		return addr, "", err
	default:
		return "", "", fmt.Errorf("unsupported network")
	}
}

type rpcClient struct {
	url  string
	user string
	pass string
	cli  *http.Client
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

func newRPC(url, user, pass string) *rpcClient {
	return &rpcClient{
		url:  url,
		user: user,
		pass: pass,
		cli: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func deriveAdminRPC(btcURL, user, pass string) *rpcClient {
	if btcURL == "" {
		return nil
	}
	base := btcURL
	if idx := strings.Index(btcURL, "/wallet/"); idx != -1 {
		base = btcURL[:idx]
	}
	return newRPC(base, user, pass)
}

func ensureWalletURL(btcURL, user, pass, wallet string) *rpcClient {
	if wallet == "" || btcURL == "" {
		return newRPC(btcURL, user, pass)
	}
	if strings.Contains(btcURL, "/wallet/") {
		return newRPC(btcURL, user, pass)
	}
	base := strings.TrimRight(btcURL, "/")
	return newRPC(base+"/wallet/"+wallet, user, pass)
}

func (p *Provider) ensureWallet(ctx context.Context) error {
	if p.btcAdmin == nil || p.btcWallet == "" {
		return nil
	}

	// Try to load wallet if it already exists.
	if _, err := p.btcAdmin.call(ctx, "loadwallet", p.btcWallet); err == nil {
		return nil
	} else {
		msg := err.Error()
		if strings.Contains(msg, "already loaded") {
			return nil
		}
	}

	// Create wallet if it doesn't exist.
	if _, err := p.btcAdmin.call(ctx, "createwallet", p.btcWallet); err == nil {
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
	msg := err.Error()
	return strings.Contains(msg, "Wallet file not specified") ||
		strings.Contains(msg, "Requested wallet does not exist") ||
		strings.Contains(msg, "not loaded")
}

func (c *rpcClient) call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	if params == nil {
		params = []interface{}{}
	}
	body, err := json.Marshal(rpcReq{JSONRPC: "1.0", ID: "wallet-service", Method: method, Params: params})
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

func (c *rpcClient) GetNewAddress(ctx context.Context, label, addrType string) (string, error) {
	params := []interface{}{label}
	if addrType != "" {
		params = append(params, addrType)
	}
	result, err := c.call(ctx, "getnewaddress", params...)
	if err != nil {
		return "", err
	}
	var addr string
	if err := json.Unmarshal(result, &addr); err != nil {
		return "", err
	}
	return addr, nil
}

func generateTronAddress(prefix byte) (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	payload := append([]byte{prefix}, buf...)
	return base58.CheckEncode(payload, 0), nil
}
