package watcher

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/xs-wallet/xscore/internal/boltz"
	"github.com/xs-wallet/xscore/internal/provider"
)

// integrationProviderOverride wraps boltz.Provider and can force execution status
// plus claim endpoint responses for integration tests without funding.
type integrationProviderOverride struct {
	base         *boltz.Provider
	forcedStatus string
	simulateExec bool

	mu              sync.RWMutex
	chainServerPub  map[string]string
	chainPubNonce   map[string]string
	syntheticTxHash map[string]string
}

func (p *integrationProviderOverride) Quote(ctx context.Context, req provider.QuoteRequest) (*provider.Quote, error) {
	return p.base.Quote(ctx, req)
}

func (p *integrationProviderOverride) Create(ctx context.Context, req provider.CreateRequest) (*provider.CreateResponse, error) {
	resp, err := p.base.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	if p.simulateExec && req.Kind == provider.SwapKindChain && resp != nil && resp.BoltzID != "" {
		var claim boltz.ChainSwapDetails
		_ = json.Unmarshal(resp.ClaimDetails, &claim)
		pubNonce, _ := newValidMuSigPubNonce()
		p.mu.Lock()
		if p.chainServerPub == nil {
			p.chainServerPub = make(map[string]string)
		}
		if p.chainPubNonce == nil {
			p.chainPubNonce = make(map[string]string)
		}
		if p.syntheticTxHash == nil {
			p.syntheticTxHash = make(map[string]string)
		}
		p.chainServerPub[resp.BoltzID] = claim.ServerPublicKey
		p.chainPubNonce[resp.BoltzID] = pubNonce
		p.syntheticTxHash[resp.BoltzID] = repeatHexByte("11", 32)
		p.mu.Unlock()
	}
	return resp, nil
}

func (p *integrationProviderOverride) Subscribe(ctx context.Context, swapID string) (<-chan provider.Update, func(), error) {
	return p.base.Subscribe(ctx, swapID)
}

func (p *integrationProviderOverride) GetSwapStatus(ctx context.Context, swapID string) (string, error) {
	if p.forcedStatus != "" {
		return p.forcedStatus, nil
	}
	return p.base.GetSwapStatus(ctx, swapID)
}

func (p *integrationProviderOverride) GetChainClaimDetails(ctx context.Context, swapID string) (*boltz.ChainClaimDetails, error) {
	if p.simulateExec {
		p.mu.RLock()
		pubKey := p.chainServerPub[swapID]
		pubNonce := p.chainPubNonce[swapID]
		txHash := p.syntheticTxHash[swapID]
		p.mu.RUnlock()
		if txHash == "" {
			txHash = repeatHexByte("11", 32)
		}
		if pubNonce == "" {
			pubNonce, _ = newValidMuSigPubNonce()
		}
		return &boltz.ChainClaimDetails{
			PubNonce:        pubNonce,
			PublicKey:       pubKey,
			TransactionHash: txHash,
		}, nil
	}
	return p.base.GetChainClaimDetails(ctx, swapID)
}

func (p *integrationProviderOverride) PostChainClaim(ctx context.Context, swapID string, sig boltz.PartialSignature, preimage string) error {
	if p.simulateExec {
		return nil
	}
	return p.base.PostChainClaim(ctx, swapID, sig, preimage)
}

func (p *integrationProviderOverride) PostReverseClaim(ctx context.Context, swapID string, req boltz.ReverseClaimRequest) (*boltz.ReverseClaimResponse, error) {
	if p.simulateExec {
		return &boltz.ReverseClaimResponse{
			PubNonce:         req.PubNonce,
			PartialSignature: randomHex(64),
		}, nil
	}
	return p.base.PostReverseClaim(ctx, swapID, req)
}

func randomHex(size int) string {
	b := make([]byte, size)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func repeatHexByte(hexByte string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		out += hexByte
	}
	return out
}

func newValidMuSigPubNonce() (string, error) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		return "", err
	}
	nonces, err := musig2.GenNonces(
		musig2.WithPublicKey(priv.PubKey()),
		musig2.WithNonceSecretKeyAux(priv),
	)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(nonces.PubNonce[:]), nil
}
