// Package boltz - Provider implementation para Boltz API v2
// Implementa interface provider.Provider
package boltz

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/swap"
)

const pairCacheTTL = 5 * time.Minute

// Provider é o provider Boltz que implementa provider.Provider
type Provider struct {
	client *Client
	ws     *WSClient
	config Config

	// Cache de pares
	mu         sync.RWMutex
	subPairs   map[string]map[string]PairInfo
	revPairs   map[string]map[string]PairInfo
	subPairsAt time.Time
	revPairsAt time.Time
}

// Config configuração do provider Boltz
type Config struct {
	BaseURL string // https://api.boltz.exchange
	WSURL   string // wss://api.boltz.exchange/v2/ws
	Network string // mainnet, testnet, regtest
}

// DefaultMainnetConfig retorna config para mainnet
func DefaultMainnetConfig() Config {
	return Config{
		BaseURL: "https://api.boltz.exchange",
		WSURL:   "wss://api.boltz.exchange/v2/ws",
		Network: "mainnet",
	}
}

// DefaultTestnetConfig retorna config para testnet
func DefaultTestnetConfig() Config {
	return Config{
		BaseURL: "https://api.testnet.boltz.exchange",
		WSURL:   "wss://api.testnet.boltz.exchange/v2/ws",
		Network: "testnet",
	}
}

// NewProvider cria um novo provider Boltz
func NewProvider(cfg Config) (*Provider, error) {
	client := NewClient(ClientConfig{BaseURL: cfg.BaseURL})
	ws := NewWSClient(cfg.WSURL, client)

	if err := ws.Connect(); err != nil {
		return nil, fmt.Errorf("websocket connect: %w", err)
	}

	return &Provider{
		client:   client,
		ws:       ws,
		config:   cfg,
		subPairs: make(map[string]map[string]PairInfo),
		revPairs: make(map[string]map[string]PairInfo),
	}, nil
}

// Quote implementa provider.Provider
func (p *Provider) Quote(ctx context.Context, req provider.QuoteRequest) (*provider.Quote, error) {
	// Determina tipo de par baseado no kind
	var pair PairInfo
	var found bool

	from := chainToBoltz(req.FromChain)
	to := chainToBoltz(req.ToChain)

	switch req.Kind {
	case provider.SwapKindSubmarine:
		if err := p.refreshSubmarinePairs(ctx); err != nil {
			return nil, err
		}
		p.mu.RLock()
		if fromPairs, ok := p.subPairs[from]; ok {
			pair, found = fromPairs[to]
		}
		p.mu.RUnlock()

	case provider.SwapKindReverse:
		if err := p.refreshReversePairs(ctx); err != nil {
			return nil, err
		}
		p.mu.RLock()
		if fromPairs, ok := p.revPairs[from]; ok {
			pair, found = fromPairs[to]
		}
		p.mu.RUnlock()

	default:
		return nil, fmt.Errorf("swap kind não suportado: %s", req.Kind)
	}

	if !found {
		return nil, fmt.Errorf("par não encontrado: %s -> %s", from, to)
	}

	// Validar amount
	if req.AmountSat < pair.Limits.Minimal {
		return nil, ErrAmountOutOfBounds
	}
	if req.AmountSat > pair.Limits.Maximal {
		return nil, ErrAmountOutOfBounds
	}

	// Calcular fees
	providerFee := int64(float64(req.AmountSat) * pair.Fees.Percentage / 100)
	networkFee := pair.Fees.MinerFees.Total()

	return &provider.Quote{
		QuoteID:               fmt.Sprintf("%s:%s:%s:%d", req.Kind, from, to, time.Now().UnixNano()),
		Kind:                  req.Kind,
		FromChain:             req.FromChain,
		ToChain:               req.ToChain,
		AmountSat:             req.AmountSat,
		ProviderFeeSat:        providerFee,
		NetworkFeeSat:         networkFee,
		TotalFeeSat:           providerFee + networkFee,
		FeePercent:            pair.Fees.Percentage,
		UserTimeoutBlocks:     144, // ~1 dia BTC
		ProviderTimeoutBlocks: 72,
		ExpiresAt:             time.Now().Add(5 * time.Minute),
	}, nil
}

// Create implementa provider.Provider (stub - requer keys)
func (p *Provider) Create(ctx context.Context, quoteID string) (*provider.CreateResponse, error) {
	return nil, fmt.Errorf("use CreateSubmarine ou CreateReverse diretamente com keys")
}

// CreateSubmarineSwap cria um submarine swap
func (p *Provider) CreateSubmarineSwap(ctx context.Context, req SubmarineRequest) (*SubmarineResponse, error) {
	return p.client.CreateSubmarine(ctx, req)
}

// CreateReverseSwap cria um reverse swap
func (p *Provider) CreateReverseSwap(ctx context.Context, req ReverseRequest) (*ReverseResponse, error) {
	return p.client.CreateReverse(ctx, req)
}

// Subscribe implementa provider.Provider
func (p *Provider) Subscribe(ctx context.Context, swapID string) (<-chan provider.Update, func(), error) {
	ch := make(chan provider.Update, 10)

	handler := func(id, status string, tx *TxInfo) {
		update := provider.Update{
			SwapID:    id,
			Status:    status,
			Timestamp: time.Now(),
		}
		if tx != nil {
			update.TxID = tx.ID
		}

		select {
		case ch <- update:
		default:
			// Channel cheio, dropar update antigo
		}
	}

	if err := p.ws.Subscribe(swapID, handler); err != nil {
		close(ch)
		return nil, nil, err
	}

	cancel := func() {
		p.ws.Unsubscribe(swapID)
		close(ch)
	}

	return ch, cancel, nil
}

// GetSwapStatus implementa provider.Provider
func (p *Provider) GetSwapStatus(ctx context.Context, swapID string) (string, error) {
	status, err := p.client.GetSwapStatus(ctx, swapID)
	if err != nil {
		return "", err
	}
	return status.Status, nil
}

// GetSubmarineClaimDetails obtém detalhes para cooperative claim
func (p *Provider) GetSubmarineClaimDetails(ctx context.Context, swapID string) (*SubmarineClaimDetails, error) {
	return p.client.GetSubmarineClaimDetails(ctx, swapID)
}

// PostSubmarineClaim envia partial signature
func (p *Provider) PostSubmarineClaim(ctx context.Context, swapID string, sig PartialSignature) error {
	return p.client.PostSubmarineClaim(ctx, swapID, sig)
}

// PostReverseClaim envia claim para reverse swap
func (p *Provider) PostReverseClaim(ctx context.Context, swapID string, req ReverseClaimRequest) (*ReverseClaimResponse, error) {
	return p.client.PostReverseClaim(ctx, swapID, req)
}

// Close fecha o provider
func (p *Provider) Close() error {
	return p.ws.Close()
}

// IsConnected verifica se WebSocket está conectado
func (p *Provider) IsConnected() bool {
	return p.ws.IsConnected()
}

// NormalizeStatus normaliza status Boltz para estado interno
func (p *Provider) NormalizeStatus(boltzStatus string, kind swap.Kind) StatusMapping {
	return Normalize(boltzStatus, kind)
}

// =============================================================================
// INTERNAL
// =============================================================================

func (p *Provider) refreshSubmarinePairs(ctx context.Context) error {
	p.mu.RLock()
	if time.Since(p.subPairsAt) < pairCacheTTL {
		p.mu.RUnlock()
		return nil
	}
	p.mu.RUnlock()

	pairs, err := p.client.GetSubmarinePairs(ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.subPairs = pairs
	p.subPairsAt = time.Now()
	p.mu.Unlock()

	return nil
}

func (p *Provider) refreshReversePairs(ctx context.Context) error {
	p.mu.RLock()
	if time.Since(p.revPairsAt) < pairCacheTTL {
		p.mu.RUnlock()
		return nil
	}
	p.mu.RUnlock()

	pairs, err := p.client.GetReversePairs(ctx)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.revPairs = pairs
	p.revPairsAt = time.Now()
	p.mu.Unlock()

	return nil
}

func chainToBoltz(chain provider.Chain) string {
	switch chain {
	case provider.ChainBTC:
		return "BTC"
	case provider.ChainLiquid:
		return "L-BTC"
	case provider.ChainLN:
		return "BTC" // LN usa BTC
	default:
		return string(chain)
	}
}
