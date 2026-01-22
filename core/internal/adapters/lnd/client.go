// Package lnd provides an LND gRPC client adapter using isolated proto stubs
package lnd

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/xs-wallet/xscore/internal/adapters"
	"github.com/xs-wallet/xscore/internal/adapters/lnd/gen/lnrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client wraps the LND gRPC connection with isolated proto stubs
type Client struct {
	conn    *grpc.ClientConn
	client  lnrpc.LightningClient
	network string
}

// Config holds LND connection configuration
type Config struct {
	Host         string // e.g., "lnd:10009" or "localhost:10009"
	TLSCertPath  string // Path to tls.cert
	MacaroonPath string // Path to admin.macaroon
	Network      string // mainnet, testnet, regtest
}

// NewClient creates a new LND gRPC client using isolated proto stubs
func NewClient(cfg Config) (*Client, error) {
	// Load TLS certificate
	creds, err := loadTLSCredentials(cfg.TLSCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	// Load macaroon
	macaroon, err := loadMacaroonHex(cfg.MacaroonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load macaroon: %w", err)
	}

	// Connect to LND with timeout
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(macaroonCredentials{macaroon: macaroon}),
		grpc.WithBlock(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LND at %s: %w", cfg.Host, err)
	}

	return &Client{
		conn:    conn,
		client:  lnrpc.NewLightningClient(conn),
		network: cfg.Network,
	}, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetInfo returns Lightning node information
func (c *Client) GetInfo(ctx context.Context) (*adapters.LNInfo, error) {
	resp, err := c.client.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetInfo failed: %w", err)
	}
	return &adapters.LNInfo{
		PubKey:            resp.IdentityPubkey,
		Alias:             resp.Alias,
		NumActiveChannels: int(resp.NumActiveChannels),
		BlockHeight:       int64(resp.BlockHeight),
		SyncedToChain:     resp.SyncedToChain,
	}, nil
}

// WalletBalance returns the wallet balance in satoshis
func (c *Client) WalletBalance(ctx context.Context) (int64, error) {
	resp, err := c.client.WalletBalance(ctx, &lnrpc.WalletBalanceRequest{})
	if err != nil {
		return 0, fmt.Errorf("WalletBalance failed: %w", err)
	}
	return resp.TotalBalance, nil
}

// ChannelBalance returns local and remote channel balance in satoshis
func (c *Client) ChannelBalance(ctx context.Context) (int64, int64, error) {
	resp, err := c.client.ChannelBalance(ctx, &lnrpc.ChannelBalanceRequest{})
	if err != nil {
		return 0, 0, fmt.Errorf("ChannelBalance failed: %w", err)
	}
	var local, remote int64
	if resp.LocalBalance != nil {
		local = int64(resp.LocalBalance.Sat)
	}
	if resp.RemoteBalance != nil {
		remote = int64(resp.RemoteBalance.Sat)
	}
	return local, remote, nil
}

// PayInvoice pays a BOLT11 invoice
func (c *Client) PayInvoice(ctx context.Context, bolt11 string) (*adapters.PayResult, error) {
	resp, err := c.client.SendPaymentSync(ctx, &lnrpc.SendRequest{
		PaymentRequest: bolt11,
	})
	if err != nil {
		return nil, fmt.Errorf("SendPaymentSync failed: %w", err)
	}
	if resp.PaymentError != "" {
		return nil, fmt.Errorf("payment failed: %s", resp.PaymentError)
	}

	var amountSat, feeSat int64
	if resp.PaymentRoute != nil {
		amountSat = resp.PaymentRoute.TotalAmtMsat / 1000
		feeSat = resp.PaymentRoute.TotalFeesMsat / 1000
	}

	return &adapters.PayResult{
		PaymentHash:     hex.EncodeToString(resp.PaymentHash),
		PaymentPreimage: hex.EncodeToString(resp.PaymentPreimage),
		AmountSat:       amountSat,
		FeeSat:          feeSat,
		Status:          "succeeded",
	}, nil
}

// DecodeInvoice decodes a BOLT11 invoice
func (c *Client) DecodeInvoice(ctx context.Context, bolt11 string) (*adapters.Invoice, error) {
	resp, err := c.client.DecodePayReq(ctx, &lnrpc.PayReqString{
		PayReq: bolt11,
	})
	if err != nil {
		return nil, fmt.Errorf("DecodePayReq failed: %w", err)
	}
	return &adapters.Invoice{
		PaymentHash: resp.PaymentHash,
		AmountSat:   resp.NumSatoshis,
		Description: resp.Description,
		Expiry:      resp.Expiry,
		Destination: resp.Destination,
	}, nil
}

// AddInvoice creates a new invoice, returns payment request and hash
func (c *Client) AddInvoice(ctx context.Context, amountSat int64, memo string, expiry int64) (string, string, error) {
	resp, err := c.client.AddInvoice(ctx, &lnrpc.Invoice{
		Value:  amountSat,
		Memo:   memo,
		Expiry: expiry,
	})
	if err != nil {
		return "", "", fmt.Errorf("AddInvoice failed: %w", err)
	}
	return resp.PaymentRequest, hex.EncodeToString(resp.RHash), nil
}

// ListChannels returns list of active channels
func (c *Client) ListChannels(ctx context.Context) ([]*adapters.Channel, error) {
	resp, err := c.client.ListChannels(ctx, &lnrpc.ListChannelsRequest{})
	if err != nil {
		return nil, fmt.Errorf("ListChannels failed: %w", err)
	}

	channels := make([]*adapters.Channel, len(resp.Channels))
	for i, ch := range resp.Channels {
		channels[i] = &adapters.Channel{
			ChanID:        ch.ChanId,
			RemotePubkey:  ch.RemotePubkey,
			Capacity:      ch.Capacity,
			LocalBalance:  ch.LocalBalance,
			RemoteBalance: ch.RemoteBalance,
			Active:        ch.Active,
		}
	}
	return channels, nil
}

// NewAddress generates a new on-chain address
func (c *Client) NewAddress(ctx context.Context) (string, error) {
	resp, err := c.client.NewAddress(ctx, &lnrpc.NewAddressRequest{
		Type: lnrpc.AddressType_WITNESS_PUBKEY_HASH,
	})
	if err != nil {
		return "", fmt.Errorf("NewAddress failed: %w", err)
	}
	return resp.Address, nil
}

// SendCoins sends on-chain coins to an address
func (c *Client) SendCoins(ctx context.Context, addr string, amount int64, satPerVbyte int64) (string, error) {
	resp, err := c.client.SendCoins(ctx, &lnrpc.SendCoinsRequest{
		Addr:        addr,
		Amount:      amount,
		SatPerVbyte: satPerVbyte,
	})
	if err != nil {
		return "", fmt.Errorf("SendCoins failed: %w", err)
	}
	return resp.Txid, nil
}

// loadTLSCredentials loads TLS certificate from file
func loadTLSCredentials(certPath string) (credentials.TransportCredentials, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS cert at %s: %w", certPath, err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(certBytes) {
		return nil, fmt.Errorf("failed to parse TLS cert")
	}

	return credentials.NewClientTLSFromCert(certPool, ""), nil
}

// loadMacaroonHex loads macaroon from file and returns hex-encoded string
func loadMacaroonHex(macaroonPath string) (string, error) {
	macaroonBytes, err := os.ReadFile(macaroonPath)
	if err != nil {
		return "", fmt.Errorf("failed to read macaroon at %s: %w", macaroonPath, err)
	}
	return hex.EncodeToString(macaroonBytes), nil
}

// macaroonCredentials implements grpc.PerRPCCredentials for macaroon auth
type macaroonCredentials struct {
	macaroon string
}

func (m macaroonCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"macaroon": m.macaroon,
	}, nil
}

func (m macaroonCredentials) RequireTransportSecurity() bool {
	return true
}

// Ensure Client implements LNAdapter interface
var _ adapters.LNAdapter = (*Client)(nil)
