// Package mock - Mock provider for regtest testing
package mock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/xs-wallet/xscore/internal/provider"
)

// MockProvider simulates a swap provider for regtest testing
type MockProvider struct {
	mu     sync.RWMutex
	quotes map[string]*provider.Quote
	swaps  map[string]*mockSwap
	subs   map[string][]chan provider.Update
}

type mockSwap struct {
	ID            string
	QuoteID       string
	Status        string
	LockupAddress string
	LockupSeen    bool
	LockupConfirmed bool
	TimeoutBlock  int
}

// NewMockProvider creates a new mock provider
func NewMockProvider() *MockProvider {
	return &MockProvider{
		quotes: make(map[string]*provider.Quote),
		swaps:  make(map[string]*mockSwap),
		subs:   make(map[string][]chan provider.Update),
	}
}

// Quote generates a mock quote
func (m *MockProvider) Quote(ctx context.Context, req provider.QuoteRequest) (*provider.Quote, error) {
	quoteID := generateID("quote")

	// Calculate mock fees
	providerFee := int64(float64(req.AmountSat) * 0.001) // 0.1%
	networkFee := int64(300) // Mock network fee

	quote := &provider.Quote{
		QuoteID:     quoteID,
		Kind:        req.Kind,
		FromChain:   req.FromChain,
		ToChain:     req.ToChain,
		AmountSat:   req.AmountSat,
		ProviderFeeSat: providerFee,
		NetworkFeeSat:  networkFee,
		TotalFeeSat:    providerFee + networkFee,
		FeePercent:     0.1,
		LockupAddress:  generateMockAddress(req.FromChain),
		ClaimAddress:   "",
		Invoice:        req.Invoice,
		UserTimeoutBlocks:     144, // ~1 day
		ProviderTimeoutBlocks: 72,  // ~12 hours
		ExpiresAt:      time.Now().Add(5 * time.Minute),
	}

	m.mu.Lock()
	m.quotes[quoteID] = quote
	m.mu.Unlock()

	return quote, nil
}

// Create creates a swap from a quote
func (m *MockProvider) Create(ctx context.Context, quoteID string) (*provider.CreateResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	quote, ok := m.quotes[quoteID]
	if !ok {
		return nil, fmt.Errorf("quote not found: %s", quoteID)
	}

	if time.Now().After(quote.ExpiresAt) {
		return nil, fmt.Errorf("quote expired")
	}

	swapID := generateID("swap")

	swap := &mockSwap{
		ID:            swapID,
		QuoteID:       quoteID,
		Status:        provider.StatusCreated,
		LockupAddress: quote.LockupAddress,
		TimeoutBlock:  quote.UserTimeoutBlocks,
	}
	m.swaps[swapID] = swap

	// Emit created event
	go m.emitUpdate(swapID, provider.Update{
		SwapID:    swapID,
		Status:    provider.StatusCreated,
		Timestamp: time.Now(),
	})

	return &provider.CreateResponse{
		SwapID:          swapID,
		BoltzID:         swapID,
		LockupAddress:   quote.LockupAddress,
		ExpectedAmount:  quote.AmountSat + quote.TotalFeeSat,
		TimeoutBlockHeight: quote.UserTimeoutBlocks,
	}, nil
}

// Subscribe subscribes to updates for a swap
func (m *MockProvider) Subscribe(ctx context.Context, swapID string) (<-chan provider.Update, func(), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.swaps[swapID]; !ok {
		return nil, nil, fmt.Errorf("swap not found: %s", swapID)
	}

	ch := make(chan provider.Update, 10)
	m.subs[swapID] = append(m.subs[swapID], ch)

	cancel := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		// Remove channel from subs
		subs := m.subs[swapID]
		for i, sub := range subs {
			if sub == ch {
				m.subs[swapID] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, cancel, nil
}

// GetSwapStatus returns the current status of a swap
func (m *MockProvider) GetSwapStatus(ctx context.Context, swapID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	swap, ok := m.swaps[swapID]
	if !ok {
		return "", fmt.Errorf("swap not found: %s", swapID)
	}

	return swap.Status, nil
}

// NotifyTxSeen is called when the watcher detects a transaction
// This triggers the appropriate status updates
func (m *MockProvider) NotifyTxSeen(swapID, txid string, confirmed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	swap, ok := m.swaps[swapID]
	if !ok {
		return
	}

	if !swap.LockupSeen {
		swap.LockupSeen = true
		swap.Status = provider.StatusTransactionMempool
		go m.emitUpdate(swapID, provider.Update{
			SwapID:    swapID,
			Status:    provider.StatusTransactionMempool,
			TxID:      txid,
			Timestamp: time.Now(),
		})
	}

	if confirmed && !swap.LockupConfirmed {
		swap.LockupConfirmed = true
		swap.Status = provider.StatusTransactionConfirmed
		go m.emitUpdate(swapID, provider.Update{
			SwapID:    swapID,
			Status:    provider.StatusTransactionConfirmed,
			TxID:      txid,
			Confirmed: true,
			Timestamp: time.Now(),
		})

		// For submarine swaps, simulate invoice payment and completion
		go func() {
			time.Sleep(100 * time.Millisecond) // Simulate delay
			m.completeSwap(swapID)
		}()
	}
}

// completeSwap marks a swap as completed
func (m *MockProvider) completeSwap(swapID string) {
	m.mu.Lock()
	swap, ok := m.swaps[swapID]
	if !ok {
		m.mu.Unlock()
		return
	}
	swap.Status = provider.StatusCompleted
	m.mu.Unlock()

	m.emitUpdate(swapID, provider.Update{
		SwapID:    swapID,
		Status:    provider.StatusCompleted,
		Timestamp: time.Now(),
	})
}

// emitUpdate sends an update to all subscribers
func (m *MockProvider) emitUpdate(swapID string, update provider.Update) {
	m.mu.RLock()
	subs := m.subs[swapID]
	m.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- update:
		default:
			// Channel full, skip
		}
	}
}

// generateID generates a random ID with the given prefix
func generateID(prefix string) string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(bytes)[:16])
}

// generateMockAddress generates a mock address for testing
func generateMockAddress(chain provider.Chain) string {
	bytes := make([]byte, 20)
	rand.Read(bytes)
	
	switch chain {
	case provider.ChainBTC:
		return "bcrt1q" + hex.EncodeToString(bytes)[:32] // Regtest bech32
	case provider.ChainLiquid:
		return "el1q" + hex.EncodeToString(bytes)[:32]
	default:
		return hex.EncodeToString(bytes)
	}
}
