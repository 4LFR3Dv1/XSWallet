// Package swap - Submarine swap orchestrator (BTC → LN)
package swap

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/vault"
)

// SubmarineOrchestrator manages submarine swaps (on-chain → lightning)
type SubmarineOrchestrator struct {
	engine   *Engine
	db       *db.DB
	provider provider.Provider
	quotes   *QuoteService
	network  string
}

// NewSubmarineOrchestrator creates a new submarine orchestrator
func NewSubmarineOrchestrator(engine *Engine, database *db.DB, prov provider.Provider) *SubmarineOrchestrator {
	return &SubmarineOrchestrator{
		engine:   engine,
		db:       database,
		provider: prov,
		quotes:   NewQuoteService(database, prov),
		network:  "regtest",
	}
}

// SetNetwork overrides the environment/network used when creating swaps.
func (so *SubmarineOrchestrator) SetNetwork(network string) {
	if network == "" {
		return
	}
	so.network = network
}

// CreateFromQuote creates a submarine swap from an accepted quote
func (so *SubmarineOrchestrator) CreateFromQuote(ctx context.Context, quoteID string) (*Swap, error) {
	// Get quote to validate it exists and is not expired
	quote, err := so.quotes.GetQuote(ctx, quoteID)
	if err != nil {
		return nil, err
	}
	if quote.Kind != provider.SwapKindSubmarine {
		return nil, fmt.Errorf("quote kind %s is not supported by submarine orchestrator", quote.Kind)
	}

	// Create swap in OPEN state
	swap, err := so.engine.Create(ctx, KindSubmarine, so.network, 0)
	if err != nil {
		return nil, err
	}

	// Auto-lock immediately after creation so swap parameters become immutable.
	return so.Lock(ctx, swap.ID, quoteID)
}

// Lock locks the swap parameters (makes them immutable)
func (so *SubmarineOrchestrator) Lock(ctx context.Context, swapID string, quoteID string) (*Swap, error) {
	// Get current swap
	swap, err := so.engine.Get(ctx, swapID)
	if err != nil {
		return nil, err
	}

	// Get quote
	quote, err := so.quotes.GetQuote(ctx, quoteID)
	if err != nil {
		return nil, err
	}

	// Create locked intent JSON
	lockedIntentPayload := map[string]interface{}{
		"quote_id":       quote.QuoteID,
		"kind":           string(quote.Kind),
		"amount_sat":     quote.AmountSat,
		"lockup_address": quote.LockupAddress,
		"timeout_blocks": quote.UserTimeoutBlocks,
	}
	lockedIntentBytes, err := json.Marshal(lockedIntentPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal locked intent: %w", err)
	}

	// Update swap with locked intent
	result, err := so.db.ExecContext(ctx, `
		UPDATE swaps 
		SET locked_intent = ?, 
		    lockup_address = ?,
		    timeout_block_height = ?,
		    invoice = ?
		WHERE id = ?
	`, string(lockedIntentBytes), quote.LockupAddress, quote.UserTimeoutBlocks, quote.Invoice, swapID)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, fmt.Errorf("failed to update swap %s before lock", swapID)
	}

	// Transition to LOCKED
	return so.engine.Transition(ctx, swapID, swap.Version, StateLocked, "lock", nil)
}

// Commit broadcasts the funding transaction
func (so *SubmarineOrchestrator) Commit(ctx context.Context, swapID string, vaultInstance *vault.Vault, btcClient *bitcoin.Client, changeAddress string, feeRate float64) (*Swap, error) {
	// Get current swap
	swap, err := so.engine.Get(ctx, swapID)
	if err != nil {
		return nil, err
	}

	if swap.State != StateLocked {
		return nil, fmt.Errorf("swap must be in LOCKED state to commit")
	}

	// Check idempotency
	opKey := fmt.Sprintf("commit:%s", swapID)
	done, _, err := so.engine.CheckIdempotency(ctx, swapID, opKey)
	if err != nil {
		return nil, err
	}
	if done {
		// Already committed, return current state
		return so.engine.Get(ctx, swapID)
	}

	// Get locked intent
	var lockupAddress string
	var amountSat int64
	err = so.db.QueryRowContext(ctx, `
		SELECT lockup_address, CAST(lockup_amount_sat AS INTEGER)
		FROM swaps WHERE id = ?
	`, swapID).Scan(&lockupAddress, &amountSat)
	if err != nil {
		return nil, fmt.Errorf("failed to get swap details: %w", err)
	}

	// Get seed from vault
	seed, err := vaultInstance.Seed()
	if err != nil {
		return nil, fmt.Errorf("vault locked: %w", err)
	}

	// Derive swap key
	swapKey, err := vault.GetSwapKeyPair(seed, uint32(swap.SwapKeyIndex), vault.NetworkRegtest)
	if err != nil {
		return nil, fmt.Errorf("failed to derive swap key: %w", err)
	}

	// Get UTXOs
	// For regtest, we'll use a simple approach: get all UTXOs and select
	utxos, err := btcClient.ListUnspent(ctx, 0, 9999999, []string{})
	if err != nil {
		return nil, fmt.Errorf("failed to list UTXOs: %w", err)
	}

	// Select UTXOs
	selected, err := bitcoin.SelectUTXOs(utxos, amountSat, feeRate)
	if err != nil {
		return nil, fmt.Errorf("insufficient funds: %w", err)
	}

	// Reserve UTXOs atomically
	tx, err := so.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, utxo := range selected {
		_, err := tx.ExecContext(ctx, `
				INSERT INTO utxo_reservations (chain, txid, vout, swap_id)
				VALUES ('btc', ?, ?, ?)
			`, utxo.TxID, utxo.Vout, swapID)
		if err != nil {
			return nil, fmt.Errorf("failed to reserve UTXO: %w", err)
		}
	}

	// Build funding transaction
	txBuilder := bitcoin.NewTxBuilder(&chaincfg.RegressionNetParams)

	// Prepare inputs with keys
	inputs := make([]bitcoin.FundingTxInput, len(selected))
	for i, utxo := range selected {
		privKey, _ := btcec.PrivKeyFromBytes(swapKey.PrivateKey)
		pubKey, parseErr := btcec.ParsePubKey(swapKey.PublicKey)
		if parseErr != nil {
			return nil, parseErr
		}
		inputs[i] = bitcoin.FundingTxInput{
			UTXO:       utxo,
			PrivateKey: privKey,
			PubKey:     pubKey,
		}
	}

	fundingTx, rawHex, err := txBuilder.BuildFundingTx(
		inputs,
		lockupAddress,
		amountSat,
		changeAddress,
		feeRate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build funding tx: %w", err)
	}

	// Broadcast transaction
	txid, err := btcClient.BroadcastTx(ctx, rawHex)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %w", err)
	}

	// Update swap with lockup_txid and raw_tx
	_, err = tx.ExecContext(ctx, `
		UPDATE swaps 
		SET lockup_txid = ?,
		    lockup_amount_sat = ?,
		    raw_funding_tx = ?
		WHERE id = ?
	`, txid, fmt.Sprint(amountSat), rawHex, swapID)
	if err != nil {
		return nil, err
	}

	// Record operation (idempotency)
	_, err = tx.ExecContext(ctx, `
			INSERT INTO swap_ops (swap_id, op_key, result, created_at)
			VALUES (?, ?, ?, datetime('now'))
		`, swapID, opKey, txid)
	if err != nil {
		return nil, err
	}

	// Transition to COMMIT_STARTED atomically with persisted commit evidence.
	valid := false
	for _, s := range ValidTransitions[swap.State] {
		if s == StateCommitStarted {
			valid = true
			break
		}
	}
	if !valid {
		return nil, ErrInvalidTransition
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		UPDATE swaps
		SET state = ?, version = version + 1, updated_at = ?
		WHERE id = ? AND version = ?
	`, StateCommitStarted, now, swapID, swap.Version)
	if err != nil {
		return nil, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, ErrConcurrentModification
	}

	if err := so.engine.logEventTx(ctx, tx, swapID, string(swap.State), string(StateCommitStarted), "commit", map[string]interface{}{
		"txid":   txid,
		"vbytes": fundingTx.SerializeSize(),
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return so.engine.Get(ctx, swapID)
}

// ApplyProviderUpdate handles updates from the provider
func (so *SubmarineOrchestrator) ApplyProviderUpdate(ctx context.Context, swapID string, update provider.Update) error {
	swap, err := so.engine.Get(ctx, swapID)
	if err != nil {
		return err
	}

	// Map provider status to state transitions
	switch update.Status {
	case provider.StatusTransactionMempool:
		if swap.State == StateCommitStarted {
			_, err = so.engine.Transition(ctx, swapID, swap.Version, StateWaiting, "tx_seen", nil)
		}
	case provider.StatusTransactionConfirmed:
		// Already in waiting, no transition needed
	case provider.StatusCompleted:
		if swap.State == StateWaiting || swap.State == StateCommitStarted {
			_, err = so.engine.Transition(ctx, swapID, swap.Version, StateCompleted, "provider_completed", nil)
		}
	case provider.StatusFailed, provider.StatusExpired:
		if swap.State != StateCompleted && swap.State != StateFailed {
			_, err = so.engine.Transition(ctx, swapID, swap.Version, StateFailed, "provider_failed", nil)
		}
	}

	return err
}
