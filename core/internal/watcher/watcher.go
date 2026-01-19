// Package watcher - Blockchain monitoring and swap reconciliation
package watcher

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/swap"
)

const (
	pollInterval = 10 * time.Second
)

// Watcher monitors blockchain state and reconciles swaps
type Watcher struct {
	db     *db.DB
	btc    *bitcoin.Client
	engine *swap.Engine

	mu            sync.RWMutex
	currentHeight int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWatcher creates a new watcher
func NewWatcher(database *db.DB, btcClient *bitcoin.Client, engine *swap.Engine) *Watcher {
	return &Watcher{
		db:     database,
		btc:    btcClient,
		engine: engine,
	}
}

// Start starts the watcher loops
func (w *Watcher) Start(ctx context.Context) error {
	w.ctx, w.cancel = context.WithCancel(ctx)

	// Run initial reconciliation
	if err := w.ReconcileAllActiveSwaps(w.ctx); err != nil {
		log.Printf("Warning: initial reconciliation failed: %v", err)
	}

	// Start height loop
	w.wg.Add(1)
	go w.heightLoop()

	// Start swap reconciliation loop
	w.wg.Add(1)
	go w.swapLoop()

	log.Println("Watcher started")
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	log.Println("Watcher stopped")
}

// CurrentHeight returns the cached current height
func (w *Watcher) CurrentHeight() int64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentHeight
}

// heightLoop periodically fetches the current block height
func (w *Watcher) heightLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Initial fetch
	w.updateHeight()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.updateHeight()
		}
	}
}

func (w *Watcher) updateHeight() {
	height, err := w.btc.Height(w.ctx)
	if err != nil {
		log.Printf("Failed to get height: %v", err)
		return
	}

	w.mu.Lock()
	oldHeight := w.currentHeight
	w.currentHeight = height
	w.mu.Unlock()

	if height != oldHeight {
		log.Printf("Block height: %d", height)
	}
}

// swapLoop periodically reconciles active swaps
func (w *Watcher) swapLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.reconcileActiveSwaps(w.ctx); err != nil {
				log.Printf("Reconciliation error: %v", err)
			}
		}
	}
}

// ReconcileAllActiveSwaps reconciles all non-terminal swaps
// This should be called on startup for crash recovery
func (w *Watcher) ReconcileAllActiveSwaps(ctx context.Context) error {
	log.Println("Running full reconciliation...")

	swaps, err := w.listActiveSwaps(ctx)
	if err != nil {
		return err
	}

	log.Printf("Found %d active swaps to reconcile", len(swaps))

	for _, s := range swaps {
		if err := w.reconcileSwap(ctx, s); err != nil {
			log.Printf("Failed to reconcile swap %s: %v", s.ID, err)
		}
	}

	return nil
}

// reconcileActiveSwaps reconciles all active swaps
func (w *Watcher) reconcileActiveSwaps(ctx context.Context) error {
	swaps, err := w.listActiveSwaps(ctx)
	if err != nil {
		return err
	}

	for _, s := range swaps {
		if err := w.reconcileSwap(ctx, s); err != nil {
			log.Printf("Failed to reconcile swap %s: %v", s.ID, err)
		}
	}

	return nil
}

// reconcileSwap reconciles a single swap based on objective facts
func (w *Watcher) reconcileSwap(ctx context.Context, s *swap.Swap) error {
	height := w.CurrentHeight()

	// Get provider status (from cache/events - simplified for MVP)
	providerStatus := "" // Would come from WebSocket events in full implementation

	// Compute facts
	facts := swap.ComputeFacts(ctx, s, height, nil, providerStatus)

	// Determine next action
	action, newState := swap.NextAction(s, facts)

	if action == swap.ActionNone || newState == s.State {
		return nil
	}

	// Execute action
	switch action {
	case swap.ActionTransitionToWaiting, swap.ActionTransitionToCompleted, swap.ActionTransitionToFailed:
		_, err := w.engine.Transition(ctx, s.ID, s.Version, newState, "reconcile", nil)
		if err != nil {
			if err == swap.ErrConcurrentModification {
				// Someone else made the transition, that's ok
				return nil
			}
			return err
		}
		log.Printf("Swap %s: %s -> %s (reconcile)", s.ID, s.State, newState)

	case swap.ActionPrepareRefund:
		// Transition to refund state
		_, err := w.engine.Transition(ctx, s.ID, s.Version, newState, "timeout_approaching", nil)
		if err != nil && err != swap.ErrConcurrentModification {
			return err
		}
		log.Printf("Swap %s: preparing refund (timeout approaching)", s.ID)

	case swap.ActionBroadcastRefund:
		// Would build and broadcast refund tx here
		// For MVP, just transition state
		_, err := w.engine.Transition(ctx, s.ID, s.Version, swap.StateRefunding, "timeout_reached", nil)
		if err != nil && err != swap.ErrConcurrentModification {
			return err
		}
		log.Printf("Swap %s: broadcasting refund", s.ID)
	}

	return nil
}

// listActiveSwaps returns all non-terminal swaps
func (w *Watcher) listActiveSwaps(ctx context.Context) ([]*swap.Swap, error) {
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, kind, env, version, state, swap_key_index,
		       COALESCE(preimage_hex, ''), COALESCE(preimage_hash_hex, ''),
		       COALESCE(lockup_txid, ''), COALESCE(lockup_amount_sat, ''),
		       COALESCE(error_message, ''), timeout_block_height,
		       created_at, updated_at
		FROM swaps
		WHERE state NOT IN ('completed', 'failed', 'canceled')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var swaps []*swap.Swap
	for rows.Next() {
		s := &swap.Swap{}
		var createdAt, updatedAt string
		var timeoutBlock *int64

		err := rows.Scan(
			&s.ID, &s.Kind, &s.Env, &s.Version, &s.State, &s.SwapKeyIndex,
			&s.PreimageHex, &s.PreimageHashHex,
			&s.LockupTxid, &s.LockupAmountSat,
			&s.ErrorMessage, &timeoutBlock,
			&createdAt, &updatedAt,
		)
		if err != nil {
			continue
		}

		if timeoutBlock != nil {
			s.TimeoutBlock = *timeoutBlock
		}

		swaps = append(swaps, s)
	}

	return swaps, nil
}
