// Package watcher - Blockchain monitoring and swap reconciliation
package watcher

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/boltz"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/swap"
	"github.com/xs-wallet/xscore/internal/swapkey"
)

const (
	pollInterval = 10 * time.Second
)

// Watcher monitors blockchain state and reconciles swaps
type Watcher struct {
	db         *db.DB
	btc        *bitcoin.Client
	engine     *swap.Engine
	prov       provider.Provider
	seedSource interface {
		Seed() ([]byte, error)
	}
	network string

	mu            sync.RWMutex
	currentHeight int64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type providerStatusInfoReader interface {
	GetSwapStatusInfo(ctx context.Context, swapID string) (*boltz.SwapStatus, error)
}

// NewWatcher creates a new watcher
func NewWatcher(database *db.DB, btcClient *bitcoin.Client, engine *swap.Engine, prov provider.Provider, seedSource interface {
	Seed() ([]byte, error)
}, network string) *Watcher {
	if network == "" {
		network = "regtest"
	}
	return &Watcher{
		db:         database,
		btc:        btcClient,
		engine:     engine,
		prov:       prov,
		seedSource: seedSource,
		network:    network,
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

	// Get provider status from backend for provider-backed swaps.
	providerStatus := s.BoltzStatus
	if w.prov != nil && s.BoltzID != "" {
		status, err := w.prov.GetSwapStatus(ctx, s.BoltzID)
		if err != nil {
			log.Printf("failed to fetch provider status for swap %s: %v", s.ID, err)
		} else if status != "" {
			providerStatus = status
			if err := w.persistProviderStatus(ctx, s.ID, status); err != nil {
				log.Printf("failed to persist provider status for swap %s: %v", s.ID, err)
			}
		}
	}

	if providerStatus != "" {
		if transitioned, err := w.applyProviderStatusState(ctx, s, providerStatus); err != nil {
			return err
		} else if transitioned {
			updated, getErr := w.engine.Get(ctx, s.ID)
			if getErr == nil {
				s = updated
			}
		}
	}

	if s.State == swap.StateSigningMusig2Partial {
		if err := w.executeMuSig2Claim(ctx, s); err != nil {
			return err
		}
	}

	// Compute facts
	facts := swap.ComputeFacts(ctx, s, height, nil, providerStatus)
	w.applyOnchainRefundEvidence(ctx, s, &facts)

	// Determine next action
	action, newState := swap.NextAction(s, facts)

	if action == swap.ActionNone || newState == s.State {
		return nil
	}

	// Dedupe by external evidence to avoid replaying the same reconcile stimulus.
	eventKey := buildReconcileEventKey(s, facts, action, newState)
	done, _, err := w.engine.CheckIdempotency(ctx, s.ID, eventKey)
	if err != nil {
		return err
	}
	if done {
		log.Printf("Swap %s: skipping duplicated reconcile event (%s)", s.ID, eventKey)
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
		if err := w.engine.RecordOperation(ctx, s.ID, eventKey, string(newState)); err != nil {
			return err
		}
		log.Printf("Swap %s: %s -> %s (reconcile)", s.ID, s.State, newState)

	case swap.ActionPrepareRefund:
		// Transition to refund state
		_, err := w.engine.Transition(ctx, s.ID, s.Version, newState, "timeout_approaching", nil)
		if err != nil && err != swap.ErrConcurrentModification {
			return err
		}
		if err == nil {
			if recErr := w.engine.RecordOperation(ctx, s.ID, eventKey, string(newState)); recErr != nil {
				return recErr
			}
		}
		log.Printf("Swap %s: preparing refund (timeout approaching)", s.ID)

	case swap.ActionBroadcastRefund:
		details, detailErr := w.prepareFallbackRefundPlan(ctx, s)
		if detailErr != nil {
			if perr := w.persistExecutionError(ctx, s.ID, detailErr.Error()); perr != nil {
				return perr
			}
			return nil
		}
		if s.RefundTxid == "" {
			txid, statusInfo, err := w.broadcastFallbackRefund(ctx, s)
			if err != nil {
				if perr := w.persistExecutionError(ctx, s.ID, err.Error()); perr != nil {
					return perr
				}
				return nil
			}
			details["refund_txid"] = txid
			if statusInfo != nil && statusInfo.Status != "" {
				details["provider_status"] = statusInfo.Status
			}
			if statusInfo != nil && statusInfo.Transaction != nil && statusInfo.Transaction.ID != "" {
				details["provider_refund_txid"] = statusInfo.Transaction.ID
			}
		}
		_, err := w.engine.Transition(ctx, s.ID, s.Version, swap.StateRefunding, "timeout_reached", details)
		if err != nil && err != swap.ErrConcurrentModification {
			return err
		}
		if err == nil {
			if recErr := w.engine.RecordOperation(ctx, s.ID, eventKey, string(swap.StateRefunding)); recErr != nil {
				return recErr
			}
		}
		log.Printf("Swap %s: broadcasting refund", s.ID)
	}

	return nil
}

func (w *Watcher) persistProviderStatus(ctx context.Context, swapID, providerStatus string) error {
	_, err := w.db.ExecContext(ctx, `
		UPDATE swaps
		SET boltz_status = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, providerStatus, swapID)
	return err
}

func (w *Watcher) applyProviderStatusState(ctx context.Context, s *swap.Swap, providerStatus string) (bool, error) {
	mapping := boltz.Normalize(providerStatus, s.Kind)
	targetState := resolveProviderTargetState(s.State, mapping, providerStatus)
	if targetState == "" || targetState == s.State {
		return false, nil
	}
	if s.Kind == swap.KindChain && mapping.Action == boltz.ActionSign {
		hasDetails, err := w.chainHasExecutionDetails(ctx, s.ID)
		if err != nil {
			return false, err
		}
		if !hasDetails {
			log.Printf("Swap %s: provider status=%s requires chain details, but locked_intent has no lockup_details/claim_details yet", s.ID, providerStatus)
			return false, nil
		}
	}

	path, ok := findTransitionPath(s.State, targetState)
	if !ok || len(path) == 0 {
		return false, nil
	}

	eventKey := fmt.Sprintf("provider_status:%s:%s->%s", providerStatus, s.State, targetState)
	done, _, err := w.engine.CheckIdempotency(ctx, s.ID, eventKey)
	if err != nil {
		return false, err
	}
	if done {
		return false, nil
	}

	current := s
	for _, nextState := range path {
		var details map[string]interface{}
		if nextState == swap.StateRefunding {
			plan, err := w.prepareFallbackRefundPlan(ctx, s)
			if err != nil {
				if perr := w.persistExecutionError(ctx, s.ID, err.Error()); perr != nil {
					return false, perr
				}
				return false, nil
			}
			details = plan
		}
		updated, err := w.engine.Transition(ctx, current.ID, current.Version, nextState, mapping.Trigger, map[string]interface{}{
			"provider_status": providerStatus,
			"mapped_action":   mapping.Action,
			"mapped_target":   string(mapping.State),
			"applied_target":  string(targetState),
			"refund_plan":     details,
		})
		if err != nil {
			if errors.Is(err, swap.ErrConcurrentModification) {
				return false, nil
			}
			return false, err
		}
		current = updated
	}
	if err := w.engine.RecordOperation(ctx, s.ID, eventKey, string(targetState)); err != nil {
		return false, err
	}

	log.Printf("Swap %s: %s -> %s (provider status=%s)", s.ID, s.State, targetState, providerStatus)
	return true, nil
}

func (w *Watcher) applyOnchainRefundEvidence(ctx context.Context, s *swap.Swap, facts *swap.Facts) {
	if s == nil || facts == nil {
		return
	}
	if strings.TrimSpace(s.RefundTxid) == "" || w.btc == nil {
		return
	}
	tx, err := w.btc.GetTx(ctx, s.RefundTxid)
	if err != nil {
		return
	}
	// Objective evidence from chain adapter:
	// tx lookup success means tx exists at least in mempool (0-conf) or confirmed (>0-conf).
	if tx != nil && strings.TrimSpace(tx.TxID) != "" {
		facts.RefundSeen = true
	}
}

func resolveProviderTargetState(current swap.State, mapping boltz.StatusMapping, providerStatus string) swap.State {
	// Refund terminalization must be objective from on-chain evidence.
	// While already in refunding, ignore provider-status-driven transitions.
	if current == swap.StateRefunding {
		return current
	}

	target := mapping.State

	if current == swap.StateWaitingProviderBroadcast {
		// Explicitly close cooperative flow with terminal success evidence from provider.
		if looksLikeProviderSuccess(providerStatus) {
			return swap.StateCompleted
		}
		// On terminal failures after partial submission, immediately enter refund execution path
		// (path solver will traverse waiting_provider_broadcast -> fallback_script_ready -> refunding).
		if mapping.Action == boltz.ActionRefund || looksLikeProviderFailure(providerStatus) || target == swap.StateFailed {
			return swap.StateRefunding
		}
	}
	return target
}

func looksLikeProviderSuccess(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(s, "claimed") ||
		strings.Contains(s, "invoice.settled") ||
		strings.Contains(s, "completed")
}

func looksLikeProviderFailure(status string) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(s, "expired") ||
		strings.Contains(s, "failed") ||
		strings.Contains(s, "refunded")
}

func findTransitionPath(from, to swap.State) ([]swap.State, bool) {
	if from == to {
		return nil, true
	}
	type node struct {
		state swap.State
		path  []swap.State
	}
	queue := []node{{state: from, path: nil}}
	visited := map[swap.State]bool{from: true}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		nextStates := swap.ValidTransitions[cur.state]
		for _, n := range nextStates {
			if visited[n] {
				continue
			}
			newPath := append(append([]swap.State{}, cur.path...), n)
			if n == to {
				return newPath, true
			}
			visited[n] = true
			queue = append(queue, node{state: n, path: newPath})
		}
	}
	return nil, false
}

func (w *Watcher) chainHasExecutionDetails(ctx context.Context, swapID string) (bool, error) {
	lockedIntent, err := w.loadLockedIntent(ctx, swapID)
	if err != nil {
		return false, err
	}
	if !lockedIntent.HasChainExecutionDetails() {
		return false, nil
	}
	claimDetails, err := w.getPersistedChainClaimDetails(ctx, swapID)
	if err != nil {
		return false, err
	}
	if claimDetails == nil {
		return false, nil
	}
	// serverPublicKey is mandatory for cooperative MuSig2 execution.
	return claimDetails.ServerPublicKey != "", nil
}

func (w *Watcher) executeMuSig2Claim(ctx context.Context, s *swap.Swap) error {
	if w.prov == nil || s.BoltzID == "" {
		return nil
	}

	eventKey := fmt.Sprintf("musig_claim:%s:%s", s.Kind, s.BoltzStatus)
	done, _, err := w.engine.CheckIdempotency(ctx, s.ID, eventKey)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	var result string
	switch s.Kind {
	case swap.KindReverse:
		result, err = w.executeReverseClaim(ctx, s)
	case swap.KindChain:
		result, err = w.executeChainClaim(ctx, s)
	default:
		return nil
	}
	if err != nil {
		if perr := w.persistExecutionError(ctx, s.ID, err.Error()); perr != nil {
			return perr
		}
		return nil
	}

	eventDetails := map[string]interface{}{
		"swap_kind":      string(s.Kind),
		"boltz_status":   s.BoltzStatus,
		"musig_result":   result,
		"partial_length": len(result),
	}
	if s.Kind == swap.KindReverse {
		if reverse, revErr := w.getPersistedReverseDetails(ctx, s.ID); revErr == nil && reverse != nil {
			eventDetails["reverse_lockup_address"] = reverse.LockupAddress
			eventDetails["reverse_timeout"] = reverse.TimeoutBlockHeight
			eventDetails["reverse_onchain_amount"] = reverse.OnchainAmount
			eventDetails["has_invoice"] = reverse.Invoice != ""
		}
	}
	if s.Kind == swap.KindChain {
		if claim, claimErr := w.getPersistedChainClaimDetails(ctx, s.ID); claimErr == nil && claim != nil {
			eventDetails["claim_lockup_address"] = claim.LockupAddress
			eventDetails["claim_timeout"] = claim.TimeoutBlockHeight
			eventDetails["has_blinding_key"] = claim.BlindingKey != ""
			eventDetails["claim_leaf_version"] = claim.SwapTree.ClaimLeaf.Version
			eventDetails["refund_leaf_version"] = claim.SwapTree.RefundLeaf.Version
		}
	}
	sent, err := w.engine.Transition(ctx, s.ID, s.Version, swap.StateSentPartialToProvider, "musig_claim_sent", eventDetails)
	if err != nil {
		if errors.Is(err, swap.ErrConcurrentModification) {
			return nil
		}
		return err
	}
	waitingDetails := map[string]interface{}{
		"swap_kind":    string(s.Kind),
		"from_boltz":   s.BoltzID,
		"await_status": "transaction.claim.pending",
	}
	if v, ok := eventDetails["claim_lockup_address"]; ok {
		waitingDetails["claim_lockup_address"] = v
	}
	if v, ok := eventDetails["claim_timeout"]; ok {
		waitingDetails["claim_timeout"] = v
	}
	if v, ok := eventDetails["has_blinding_key"]; ok {
		waitingDetails["has_blinding_key"] = v
	}
	if v, ok := eventDetails["reverse_lockup_address"]; ok {
		waitingDetails["reverse_lockup_address"] = v
	}
	if v, ok := eventDetails["reverse_timeout"]; ok {
		waitingDetails["reverse_timeout"] = v
	}
	if v, ok := eventDetails["reverse_onchain_amount"]; ok {
		waitingDetails["reverse_onchain_amount"] = v
	}
	if v, ok := eventDetails["has_invoice"]; ok {
		waitingDetails["has_invoice"] = v
	}
	waiting, err := w.engine.Transition(ctx, s.ID, sent.Version, swap.StateWaitingProviderBroadcast, "musig_claim_sent", waitingDetails)
	if err != nil {
		if errors.Is(err, swap.ErrConcurrentModification) {
			return nil
		}
		return err
	}
	if err := w.engine.RecordOperation(ctx, s.ID, eventKey, result); err != nil {
		return err
	}
	log.Printf("Swap %s: muSig claim sent (%s), now %s", s.ID, s.Kind, waiting.State)
	return nil
}

func (w *Watcher) executeReverseClaim(ctx context.Context, s *swap.Swap) (string, error) {
	type reverseClaimer interface {
		PostReverseClaim(ctx context.Context, swapID string, req boltz.ReverseClaimRequest) (*boltz.ReverseClaimResponse, error)
	}
	claimer, ok := w.prov.(reverseClaimer)
	if !ok {
		return "", errors.New("provider does not support reverse claim execution")
	}
	if w.seedSource == nil {
		return "", errors.New("seed source unavailable for reverse MuSig2 nonce generation")
	}
	seed, err := w.seedSource.Seed()
	if err != nil {
		return "", fmt.Errorf("vault locked: %w", err)
	}
	priv, pub, err := swapkey.Derive(seed, uint32(s.SwapKeyIndex), w.network)
	if err != nil {
		return "", err
	}

	preimage, err := w.engine.GetPreimage(ctx, s.ID)
	if err != nil {
		return "", fmt.Errorf("load preimage: %w", err)
	}
	var preimageMsg [32]byte
	copy(preimageMsg[:], preimage)
	nonces, err := musig2.GenNonces(
		musig2.WithPublicKey(pub),
		musig2.WithNonceSecretKeyAux(priv),
		musig2.WithNonceMessageAux(preimageMsg),
	)
	if err != nil {
		return "", err
	}
	localPubNonce := hex.EncodeToString(nonces.PubNonce[:])
	sessionID := hex.EncodeToString(preimageMsg[:16])

	resp, err := claimer.PostReverseClaim(ctx, s.BoltzID, boltz.ReverseClaimRequest{
		Preimage: hex.EncodeToString(preimage),
		PubNonce: localPubNonce,
	})
	if err != nil {
		return "", fmt.Errorf("reverse claim post failed: %w", err)
	}

	partial := ""
	if resp != nil {
		partial = resp.PartialSignature
	}
	if err := w.persistMusigArtifacts(ctx, s.ID, sessionID, nonces.SecNonce[:], localPubNonce, nil, partial); err != nil {
		return "", err
	}
	return partial, nil
}

func (w *Watcher) executeChainClaim(ctx context.Context, s *swap.Swap) (string, error) {
	type chainClaimer interface {
		GetChainClaimDetails(ctx context.Context, swapID string) (*boltz.ChainClaimDetails, error)
		PostChainClaim(ctx context.Context, swapID string, sig boltz.PartialSignature, preimage string) error
	}
	claimer, ok := w.prov.(chainClaimer)
	if !ok {
		return "", errors.New("provider does not support chain claim execution")
	}
	if w.seedSource == nil {
		return "", errors.New("seed source unavailable for chain MuSig2 signing")
	}
	seed, err := w.seedSource.Seed()
	if err != nil {
		return "", fmt.Errorf("vault locked: %w", err)
	}
	priv, localPub, err := swapkey.Derive(seed, uint32(s.SwapKeyIndex), w.network)
	if err != nil {
		return "", err
	}

	preimage, err := w.engine.GetPreimage(ctx, s.ID)
	if err != nil {
		return "", fmt.Errorf("load preimage: %w", err)
	}
	preimageHex := hex.EncodeToString(preimage)

	persistedClaimDetails, err := w.getPersistedChainClaimDetails(ctx, s.ID)
	if err != nil {
		return "", err
	}
	claimContext := map[string]interface{}{}
	if persistedClaimDetails != nil {
		claimContext["claim_lockup_address"] = persistedClaimDetails.LockupAddress
		claimContext["claim_timeout"] = persistedClaimDetails.TimeoutBlockHeight
		claimContext["has_blinding_key"] = persistedClaimDetails.BlindingKey != ""
		claimContext["claim_leaf_version"] = persistedClaimDetails.SwapTree.ClaimLeaf.Version
		claimContext["refund_leaf_version"] = persistedClaimDetails.SwapTree.RefundLeaf.Version
	}
	claimDetails, err := claimer.GetChainClaimDetails(ctx, s.BoltzID)
	if err != nil {
		return "", fmt.Errorf("get chain claim details: %w", err)
	}
	remotePubHex := ""
	if persistedClaimDetails != nil && persistedClaimDetails.ServerPublicKey != "" {
		remotePubHex = persistedClaimDetails.ServerPublicKey
	} else {
		remotePubHex = claimDetails.PublicKey
	}
	if remotePubHex == "" {
		return "", errors.New("missing provider chain claim public key")
	}
	remotePub, err := btcec.ParsePubKey(decodeHexOrRaw(remotePubHex))
	if err != nil {
		return "", fmt.Errorf("invalid provider claim public key: %w", err)
	}
	msgHash, err := parseMuSigMessage(claimDetails.TransactionHash)
	if err != nil {
		return "", err
	}
	nonces, err := musig2.GenNonces(
		musig2.WithPublicKey(localPub),
		musig2.WithNonceSecretKeyAux(priv),
		musig2.WithNonceMessageAux(msgHash),
	)
	if err != nil {
		return "", err
	}
	remoteNonce, err := parsePubNonceHex(claimDetails.PubNonce)
	if err != nil {
		return "", fmt.Errorf("invalid provider pub nonce: %w", err)
	}
	combinedNonce, err := musig2.AggregateNonces([][musig2.PubNonceSize]byte{
		nonces.PubNonce, remoteNonce,
	})
	if err != nil {
		return "", fmt.Errorf("aggregate nonces: %w", err)
	}
	partial, err := musig2.Sign(
		nonces.SecNonce, priv, combinedNonce,
		[]*btcec.PublicKey{localPub, remotePub},
		msgHash,
		musig2.WithSortedKeys(),
	)
	if err != nil {
		return "", fmt.Errorf("musig2 sign: %w", err)
	}
	localPubNonce := hex.EncodeToString(nonces.PubNonce[:])
	localPartialSig := encodeMusigPartialSig(partial)
	sessionID := hex.EncodeToString(msgHash[:16])
	aggNonce := combinedNonce[:]

	if err := claimer.PostChainClaim(ctx, s.BoltzID, boltz.PartialSignature{
		PubNonce:         localPubNonce,
		PartialSignature: localPartialSig,
	}, preimageHex); err != nil {
		return "", fmt.Errorf("chain claim post failed: %w", err)
	}
	if err := w.persistMusigArtifacts(ctx, s.ID, sessionID, nonces.SecNonce[:], localPubNonce, aggNonce, localPartialSig); err != nil {
		return "", err
	}
	// Record contextual claim data sourced from persisted locked_intent claim_details.
	if err := w.engine.RecordOperation(ctx, s.ID, "chain_claim_context", mustJSON(claimContext)); err != nil {
		return "", err
	}
	return localPartialSig, nil
}

func (w *Watcher) persistMusigArtifacts(ctx context.Context, swapID, sessionID string, secNonce []byte, pubNonceHex string, aggNonce []byte, partialSigHex string) error {
	pubNonce := decodeHexOrRaw(pubNonceHex)
	partialSig := decodeHexOrRaw(partialSigHex)
	_, err := w.db.ExecContext(ctx, `
		UPDATE swaps
		SET musig_session_id = CASE WHEN ? != '' THEN ? ELSE musig_session_id END,
		    musig_secnonce = CASE WHEN ? != X'' THEN ? ELSE musig_secnonce END,
		    musig_pubnonce = CASE WHEN ? != X'' THEN ? ELSE musig_pubnonce END,
		    musig_agg_nonce = CASE WHEN ? != X'' THEN ? ELSE musig_agg_nonce END,
		    musig_partial_sig = CASE WHEN ? != X'' THEN ? ELSE musig_partial_sig END,
		    error_message = NULL,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, sessionID, sessionID, secNonce, secNonce, pubNonce, pubNonce, aggNonce, aggNonce, partialSig, partialSig, swapID)
	if err != nil {
		return err
	}

	return w.persistLockedIntentMuSig(ctx, swapID, sessionID, pubNonceHex, aggNonce, partialSigHex)
}

func (w *Watcher) persistRefundTxid(ctx context.Context, swapID, refundTxid string) error {
	_, err := w.db.ExecContext(ctx, `
		UPDATE swaps
		SET refund_txid = ?,
		    error_message = NULL,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, refundTxid, swapID)
	return err
}

func (w *Watcher) broadcastFallbackRefund(ctx context.Context, s *swap.Swap) (string, *boltz.SwapStatus, error) {
	if w.btc == nil {
		return "", nil, errors.New("refund broadcast requires bitcoin adapter")
	}
	if txHex, txID := w.loadPersistedRefundRawTx(ctx, s.ID); txHex != "" || txID != "" {
		localHex := txHex
		if localHex == "" {
			if built, err := w.tryBuildLocalRefundHexFromTemplate(ctx, s); err == nil {
				localHex = strings.TrimSpace(built)
			} else {
				log.Printf("Swap %s: local template rebuild unavailable: %v", s.ID, err)
			}
		}
		if localHex == "" && txID != "" {
			fetched, err := w.btc.GetRawTransaction(ctx, txID)
			if err != nil {
				log.Printf("Swap %s: failed to fetch cached refund tx hex by txid=%s: %v", s.ID, txID, err)
			} else {
				localHex = strings.TrimSpace(fetched)
			}
		}
		if localHex != "" {
			txid, err := w.broadcastValidatedRefundHex(ctx, s, localHex, txID)
			if err == nil {
				if err := w.persistRefundTxid(ctx, s.ID, txid); err != nil {
					return "", nil, err
				}
				if err := w.persistLockedIntentRefundArtifacts(ctx, s.ID, "script_path_fallback", "locked_intent_refund_cache", nil, txid, txID, localHex); err != nil {
					return "", nil, err
				}
				return txid, nil, nil
			}
			log.Printf("Swap %s: cached local refund hex failed, falling back to provider hex: %v", s.ID, err)
		}
	}
	if localHex, err := w.tryBuildLocalRefundHexFromArtifacts(ctx, s); err == nil && strings.TrimSpace(localHex) != "" {
		txid, bErr := w.broadcastValidatedRefundHex(ctx, s, localHex, "")
		if bErr != nil {
			log.Printf("Swap %s: local zero-template refund build failed to broadcast, falling back to provider: %v", s.ID, bErr)
		} else {
			if err := w.persistRefundTxid(ctx, s.ID, txid); err != nil {
				return "", nil, err
			}
			if err := w.persistLockedIntentRefundArtifacts(ctx, s.ID, "script_path_fallback", "local_zero_builder", nil, txid, "", localHex); err != nil {
				return "", nil, err
			}
			return txid, nil, nil
		}
	} else if err != nil {
		log.Printf("Swap %s: local zero-template refund build unavailable: %v", s.ID, err)
	}
	if w.prov == nil || s.BoltzID == "" {
		return "", nil, errors.New("refund broadcast requires provider swap id when no local refund tx is available")
	}

	statusReader, ok := w.prov.(providerStatusInfoReader)
	if !ok {
		return "", nil, errors.New("provider does not support detailed swap status")
	}

	statusInfo, err := statusReader.GetSwapStatusInfo(ctx, s.BoltzID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get provider swap status info: %w", err)
	}
	if statusInfo == nil || statusInfo.Transaction == nil {
		return "", nil, errors.New("provider status has no refund transaction")
	}
	if statusInfo.Transaction.Hex == "" {
		if statusInfo.Transaction.ID == "" {
			return "", nil, errors.New("provider status has no refund transaction hex")
		}
		fetched, fetchErr := w.btc.GetRawTransaction(ctx, statusInfo.Transaction.ID)
		if fetchErr != nil {
			return "", nil, errors.New("provider status has no refund transaction hex")
		}
		statusInfo.Transaction.Hex = strings.TrimSpace(fetched)
		if statusInfo.Transaction.Hex == "" {
			return "", nil, errors.New("provider status has no refund transaction hex")
		}
	}
	refundTxid, err := w.broadcastValidatedRefundHex(ctx, s, statusInfo.Transaction.Hex, statusInfo.Transaction.ID)
	if err != nil {
		return "", nil, err
	}
	if refundTxid == "" {
		return "", nil, errors.New("missing refund txid after broadcast")
	}
	if err := w.persistRefundTxid(ctx, s.ID, refundTxid); err != nil {
		return "", nil, err
	}
	if err := w.persistLockedIntentRefundArtifacts(ctx, s.ID, "script_path_fallback", "provider_tx_hex", statusInfo, refundTxid, "", ""); err != nil {
		return "", nil, err
	}
	if statusInfo.Status != "" {
		if err := w.persistProviderStatus(ctx, s.ID, statusInfo.Status); err != nil {
			log.Printf("failed to persist provider status for swap %s: %v", s.ID, err)
		}
	}
	return refundTxid, statusInfo, nil
}

func (w *Watcher) broadcastValidatedRefundHex(ctx context.Context, s *swap.Swap, rawHex, fallbackTxid string) (string, error) {
	if strings.TrimSpace(rawHex) == "" {
		return "", errors.New("missing refund transaction hex")
	}
	signedHex := strings.TrimSpace(rawHex)
	if localHex, err := w.tryBuildLocalRefundHex(ctx, s, signedHex); err != nil {
		log.Printf("Swap %s: local refund signer skipped/fallback: %v", s.ID, err)
	} else if strings.TrimSpace(localHex) != "" {
		signedHex = strings.TrimSpace(localHex)
	}
	if err := w.validateRefundScriptPathHex(ctx, s, signedHex); err != nil {
		return "", err
	}
	refundTxid, err := w.btc.BroadcastTx(ctx, signedHex)
	if err != nil {
		if strings.TrimSpace(fallbackTxid) == "" {
			return "", fmt.Errorf("broadcast refund tx: %w", err)
		}
		refundTxid = fallbackTxid
	}
	if strings.TrimSpace(refundTxid) == "" {
		refundTxid = strings.TrimSpace(fallbackTxid)
	}
	if strings.TrimSpace(refundTxid) == "" {
		return "", errors.New("missing refund txid after broadcast")
	}
	return refundTxid, nil
}

func (w *Watcher) tryBuildLocalRefundHex(ctx context.Context, s *swap.Swap, templateHex string) (string, error) {
	if w.seedSource == nil {
		return "", errors.New("seed source unavailable for local refund signing")
	}
	seed, err := w.seedSource.Seed()
	if err != nil {
		return "", fmt.Errorf("vault locked: %w", err)
	}
	refundPriv, _, err := swapkey.Derive(seed, uint32(s.SwapKeyIndex+1), w.network)
	if err != nil {
		return "", fmt.Errorf("derive refund key: %w", err)
	}

	raw, err := hex.DecodeString(strings.TrimSpace(templateHex))
	if err != nil {
		return "", fmt.Errorf("invalid template tx hex: %w", err)
	}
	var tx wire.MsgTx
	if err := tx.Deserialize(bytes.NewReader(raw)); err != nil {
		return "", fmt.Errorf("decode template tx: %w", err)
	}
	if len(tx.TxIn) == 0 {
		return "", errors.New("template tx has no inputs")
	}

	leafByScript, err := w.expectedRefundLeaves(ctx, s)
	if err != nil {
		return "", err
	}
	if len(leafByScript) == 0 {
		return "", errors.New("missing refund leaf metadata")
	}

	prevFetcher := txscript.NewMultiPrevOutFetcher(nil)
	for i, in := range tx.TxIn {
		if len(in.Witness) < 2 {
			return "", fmt.Errorf("input %d witness too short for tapscript signing", i)
		}
		scriptBytes := in.Witness[len(in.Witness)-2]
		scriptHex := strings.ToLower(hex.EncodeToString(scriptBytes))
		leafVersion, ok := leafByScript[scriptHex]
		if !ok {
			return "", fmt.Errorf("input %d witness script not recognized as refund leaf", i)
		}

		prevHash := in.PreviousOutPoint.Hash.String()
		prevOut, err := w.btc.GetTxOut(ctx, prevHash, in.PreviousOutPoint.Index)
		if err != nil {
			return "", fmt.Errorf("load prevout %s:%d: %w", prevHash, in.PreviousOutPoint.Index, err)
		}
		prevScript, err := hex.DecodeString(strings.TrimSpace(prevOut.ScriptHex))
		if err != nil {
			return "", fmt.Errorf("decode prevout script: %w", err)
		}
		prevFetcher.AddPrevOut(in.PreviousOutPoint, &wire.TxOut{
			Value:    prevOut.ValueSat,
			PkScript: prevScript,
		})

		sigHashes := txscript.NewTxSigHashes(&tx, prevFetcher)
		tapLeaf := txscript.NewTapLeaf(txscript.TapscriptLeafVersion(leafVersion), scriptBytes)
		sig, err := txscript.RawTxInTapscriptSignature(
			&tx, sigHashes, i, prevOut.ValueSat, prevScript, tapLeaf, txscript.SigHashDefault, refundPriv,
		)
		if err != nil {
			return "", fmt.Errorf("sign tapscript input %d: %w", i, err)
		}
		tx.TxIn[i].Witness[0] = sig
	}

	var out bytes.Buffer
	if err := tx.Serialize(&out); err != nil {
		return "", fmt.Errorf("serialize signed tx: %w", err)
	}
	return hex.EncodeToString(out.Bytes()), nil
}

func (w *Watcher) tryBuildLocalRefundHexFromTemplate(ctx context.Context, s *swap.Swap) (string, error) {
	if w.seedSource == nil {
		return "", errors.New("seed source unavailable for local refund signing")
	}
	lockedIntent, err := w.loadLockedIntent(ctx, s.ID)
	if err != nil {
		return "", err
	}
	if lockedIntent.Refund == nil || lockedIntent.Refund.Template == nil {
		return "", errors.New("refund template metadata unavailable")
	}
	tpl := lockedIntent.Refund.Template
	if tpl.PrevTxID == "" || tpl.OutputPkScriptHex == "" || tpl.RefundScriptHex == "" || tpl.ControlBlockHex == "" {
		return "", errors.New("refund template metadata incomplete")
	}
	seed, err := w.seedSource.Seed()
	if err != nil {
		return "", fmt.Errorf("vault locked: %w", err)
	}
	refundPriv, _, err := swapkey.Derive(seed, uint32(s.SwapKeyIndex+1), w.network)
	if err != nil {
		return "", fmt.Errorf("derive refund key: %w", err)
	}
	prevHash, err := chainhash.NewHashFromStr(tpl.PrevTxID)
	if err != nil {
		return "", fmt.Errorf("invalid template prev_txid: %w", err)
	}
	outScript, err := hex.DecodeString(strings.TrimSpace(tpl.OutputPkScriptHex))
	if err != nil {
		return "", fmt.Errorf("invalid template output script: %w", err)
	}
	refundScript, err := hex.DecodeString(strings.TrimSpace(tpl.RefundScriptHex))
	if err != nil {
		return "", fmt.Errorf("invalid template refund script: %w", err)
	}
	controlBlock, err := hex.DecodeString(strings.TrimSpace(tpl.ControlBlockHex))
	if err != nil {
		return "", fmt.Errorf("invalid template control block: %w", err)
	}
	prevScript, err := hex.DecodeString(strings.TrimSpace(tpl.PrevPkScriptHex))
	if err != nil {
		return "", fmt.Errorf("invalid template prev pkscript: %w", err)
	}

	tx := wire.NewMsgTx(tpl.TxVersion)
	tx.LockTime = tpl.LockTime
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *prevHash,
			Index: tpl.PrevVout,
		},
		Sequence: tpl.Sequence,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    tpl.OutputValueSat,
		PkScript: outScript,
	})

	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevScript, tpl.PrevValueSat)
	sigHashes := txscript.NewTxSigHashes(tx, prevFetcher)
	tapLeaf := txscript.NewTapLeaf(txscript.TapscriptLeafVersion(tpl.RefundLeafVersion), refundScript)
	sig, err := txscript.RawTxInTapscriptSignature(
		tx, sigHashes, 0, tpl.PrevValueSat, prevScript, tapLeaf, txscript.SigHashDefault, refundPriv,
	)
	if err != nil {
		return "", fmt.Errorf("sign template tx: %w", err)
	}

	witness := make(wire.TxWitness, 0, 3+len(tpl.WitnessArgsHex))
	witness = append(witness, sig)
	for _, a := range tpl.WitnessArgsHex {
		b, decErr := hex.DecodeString(strings.TrimSpace(a))
		if decErr != nil {
			return "", fmt.Errorf("invalid template witness arg: %w", decErr)
		}
		witness = append(witness, b)
	}
	witness = append(witness, refundScript, controlBlock)
	tx.TxIn[0].Witness = witness

	var out bytes.Buffer
	if err := tx.Serialize(&out); err != nil {
		return "", fmt.Errorf("serialize rebuilt tx: %w", err)
	}
	return hex.EncodeToString(out.Bytes()), nil
}

func (w *Watcher) tryBuildLocalRefundHexFromArtifacts(ctx context.Context, s *swap.Swap) (string, error) {
	if w.seedSource == nil {
		return "", errors.New("seed source unavailable for local refund signing")
	}
	lockedIntent, err := w.loadLockedIntent(ctx, s.ID)
	if err != nil {
		return "", err
	}
	lockupAddress, timeoutBlock, refundScriptHex, refundLeafVersion, controlBlockHex, witnessArgsHex, err := w.resolveRefundBuildArtifacts(ctx, s, lockedIntent)
	if err != nil {
		return "", err
	}
	lockupAddress = strings.TrimSpace(lockupAddress)
	if lockupAddress == "" {
		return "", errors.New("missing lockup address for refund builder")
	}

	utxo, err := w.findRefundLockupUTXO(ctx, lockupAddress)
	if err != nil {
		return "", err
	}
	prevOut, err := w.btc.GetTxOut(ctx, utxo.TxID, utxo.Vout)
	if err != nil {
		return "", fmt.Errorf("load lockup prevout %s:%d: %w", utxo.TxID, utxo.Vout, err)
	}
	prevScript, err := hex.DecodeString(strings.TrimSpace(prevOut.ScriptHex))
	if err != nil {
		return "", fmt.Errorf("decode lockup prevout script: %w", err)
	}

	seed, err := w.seedSource.Seed()
	if err != nil {
		return "", fmt.Errorf("vault locked: %w", err)
	}
	refundPriv, refundPub, err := swapkey.Derive(seed, uint32(s.SwapKeyIndex+1), w.network)
	if err != nil {
		return "", fmt.Errorf("derive refund key: %w", err)
	}
	destScript, _, err := canonicalRefundDestination(refundPub, w.network)
	if err != nil {
		return "", err
	}

	feeRate := 2.0
	if est, feeErr := w.btc.EstimateFee(ctx, 2); feeErr == nil && est > 0 {
		feeRate = est
	}
	// One tapscript input + one segwit output, conservative estimate.
	const estVBytes = 150.0
	feeSat := int64(math.Ceil(feeRate * estVBytes))
	if feeSat < 500 {
		feeSat = 500
	}
	outputValue := prevOut.ValueSat - feeSat
	if outputValue <= 546 {
		return "", fmt.Errorf("refund output below dust after fee: prev=%d fee=%d", prevOut.ValueSat, feeSat)
	}

	prevHash, err := chainhash.NewHashFromStr(utxo.TxID)
	if err != nil {
		return "", fmt.Errorf("invalid utxo txid: %w", err)
	}
	refundScript, err := hex.DecodeString(strings.TrimSpace(refundScriptHex))
	if err != nil {
		return "", fmt.Errorf("invalid refund script hex: %w", err)
	}
	controlBlock, err := hex.DecodeString(strings.TrimSpace(controlBlockHex))
	if err != nil {
		return "", fmt.Errorf("invalid control block hex: %w", err)
	}

	tx := wire.NewMsgTx(2)
	if timeoutBlock > 0 {
		tx.LockTime = timeoutBlock
	}
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *prevHash,
			Index: utxo.Vout,
		},
		Sequence: 0,
	})
	tx.AddTxOut(&wire.TxOut{
		Value:    outputValue,
		PkScript: destScript,
	})

	prevFetcher := txscript.NewCannedPrevOutputFetcher(prevScript, prevOut.ValueSat)
	sigHashes := txscript.NewTxSigHashes(tx, prevFetcher)
	tapLeaf := txscript.NewTapLeaf(txscript.TapscriptLeafVersion(refundLeafVersion), refundScript)
	sig, err := txscript.RawTxInTapscriptSignature(
		tx, sigHashes, 0, prevOut.ValueSat, prevScript, tapLeaf, txscript.SigHashDefault, refundPriv,
	)
	if err != nil {
		return "", fmt.Errorf("sign local refund tx: %w", err)
	}

	witness := make(wire.TxWitness, 0, 3+len(witnessArgsHex))
	witness = append(witness, sig)
	for _, argHex := range witnessArgsHex {
		arg, decErr := hex.DecodeString(strings.TrimSpace(argHex))
		if decErr != nil {
			return "", fmt.Errorf("invalid witness arg hex: %w", decErr)
		}
		witness = append(witness, arg)
	}
	witness = append(witness, refundScript, controlBlock)
	tx.TxIn[0].Witness = witness

	var out bytes.Buffer
	if err := tx.Serialize(&out); err != nil {
		return "", fmt.Errorf("serialize local refund tx: %w", err)
	}
	return hex.EncodeToString(out.Bytes()), nil
}

func (w *Watcher) resolveRefundBuildArtifacts(ctx context.Context, s *swap.Swap, intent swap.LockedIntent) (
	lockupAddress string,
	timeoutBlock uint32,
	refundScriptHex string,
	refundLeafVersion int,
	controlBlockHex string,
	witnessArgsHex []string,
	err error,
) {
	switch s.Kind {
	case swap.KindChain:
		if len(intent.LockupDetails) == 0 {
			err = errors.New("missing lockup_details for local refund build")
			return
		}
		var lockup boltz.ChainSwapDetails
		if uErr := json.Unmarshal(intent.LockupDetails, &lockup); uErr != nil {
			err = errors.New("invalid lockup_details for local refund build")
			return
		}
		lockupAddress = lockup.LockupAddress
		if lockup.TimeoutBlockHeight > 0 {
			timeoutBlock = uint32(lockup.TimeoutBlockHeight)
		}
		refundScriptHex = lockup.SwapTree.RefundLeaf.Output
		refundLeafVersion = lockup.SwapTree.RefundLeaf.Version
	case swap.KindReverse:
		if len(intent.ReverseDetails) == 0 {
			err = errors.New("missing reverse_details for local refund build")
			return
		}
		var reverse boltz.ReverseResponse
		if uErr := json.Unmarshal(intent.ReverseDetails, &reverse); uErr != nil {
			err = errors.New("invalid reverse_details for local refund build")
			return
		}
		lockupAddress = reverse.LockupAddress
		if reverse.TimeoutBlockHeight > 0 {
			timeoutBlock = uint32(reverse.TimeoutBlockHeight)
		}
		refundScriptHex = reverse.SwapTree.RefundLeaf.Output
		refundLeafVersion = reverse.SwapTree.RefundLeaf.Version
	default:
		err = fmt.Errorf("unsupported kind for local refund build: %s", s.Kind)
		return
	}

	if lockupAddress == "" {
		lockupAddress = intent.LockupAddress
	}
	if refundScriptHex == "" || refundLeafVersion == 0 {
		err = errors.New("missing refund script metadata for local refund build")
		return
	}
	if timeoutBlock == 0 && intent.TimeoutBlocks > 0 {
		timeoutBlock = uint32(intent.TimeoutBlocks)
	}

	if intent.Refund != nil && intent.Refund.Template != nil {
		controlBlockHex = strings.TrimSpace(intent.Refund.Template.ControlBlockHex)
		witnessArgsHex = append([]string(nil), intent.Refund.Template.WitnessArgsHex...)
	}
	if controlBlockHex == "" && intent.Refund != nil && strings.TrimSpace(intent.Refund.RawTxHex) != "" {
		if tpl, tplErr := extractRefundTemplate(strings.TrimSpace(intent.Refund.RawTxHex)); tplErr == nil && tpl != nil {
			controlBlockHex = strings.TrimSpace(tpl.ControlBlockHex)
			if len(witnessArgsHex) == 0 {
				witnessArgsHex = append([]string(nil), tpl.WitnessArgsHex...)
			}
		}
	}
	if controlBlockHex == "" && intent.Refund != nil && strings.TrimSpace(intent.Refund.ProviderTxID) != "" && w.btc != nil {
		if raw, rawErr := w.btc.GetRawTransaction(ctx, strings.TrimSpace(intent.Refund.ProviderTxID)); rawErr == nil {
			if tpl, tplErr := extractRefundTemplate(strings.TrimSpace(raw)); tplErr == nil && tpl != nil {
				controlBlockHex = strings.TrimSpace(tpl.ControlBlockHex)
				if len(witnessArgsHex) == 0 {
					witnessArgsHex = append([]string(nil), tpl.WitnessArgsHex...)
				}
			}
		}
	}
	if controlBlockHex == "" {
		err = errors.New("missing control block for local refund build")
		return
	}
	return
}

func (w *Watcher) findRefundLockupUTXO(ctx context.Context, lockupAddress string) (*bitcoin.ScanUtxo, error) {
	scan, err := w.btc.ScanTxOutSet(ctx, []string{lockupAddress})
	if err != nil {
		return nil, fmt.Errorf("scan lockup utxo for %s: %w", lockupAddress, err)
	}
	if scan == nil || len(scan.Unspents) == 0 {
		return nil, fmt.Errorf("no unspent lockup utxo found for %s", lockupAddress)
	}
	best := scan.Unspents[0]
	for _, u := range scan.Unspents[1:] {
		if u.Amount > best.Amount {
			best = u
		}
	}
	return &best, nil
}

func canonicalRefundDestination(refundPub *btcec.PublicKey, network string) ([]byte, string, error) {
	params := btcParamsFromNetwork(network)
	addr, err := btcutil.NewAddressWitnessPubKeyHash(btcutil.Hash160(refundPub.SerializeCompressed()), params)
	if err != nil {
		return nil, "", fmt.Errorf("build canonical refund destination: %w", err)
	}
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, "", fmt.Errorf("build canonical refund script: %w", err)
	}
	return script, addr.EncodeAddress(), nil
}

func btcParamsFromNetwork(network string) *chaincfg.Params {
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "mainnet":
		return &chaincfg.MainNetParams
	case "testnet":
		return &chaincfg.TestNet3Params
	default:
		return &chaincfg.RegressionNetParams
	}
}

func (w *Watcher) loadPersistedRefundRawTx(ctx context.Context, swapID string) (rawTxHex string, txID string) {
	lockedIntent, err := w.loadLockedIntent(ctx, swapID)
	if err != nil || lockedIntent.Refund == nil {
		return "", ""
	}
	return strings.TrimSpace(lockedIntent.Refund.RawTxHex), strings.TrimSpace(lockedIntent.Refund.ProviderTxID)
}

func (w *Watcher) validateRefundScriptPathHex(ctx context.Context, s *swap.Swap, rawHex string) error {
	expectedScripts, err := w.expectedRefundLeafScripts(ctx, s)
	if err != nil {
		return err
	}
	if len(expectedScripts) == 0 {
		return errors.New("missing expected refund leaf scripts for script-path validation")
	}
	if err := validateTxWitnessContainsAnyRefundScript(rawHex, expectedScripts); err != nil {
		return fmt.Errorf("refund script-path validation failed: %w", err)
	}
	return nil
}

func (w *Watcher) expectedRefundLeafScripts(ctx context.Context, s *swap.Swap) ([]string, error) {
	lockedIntent, err := w.loadLockedIntent(ctx, s.ID)
	if err != nil {
		return nil, err
	}

	switch s.Kind {
	case swap.KindChain:
		if len(lockedIntent.LockupDetails) == 0 {
			return nil, errors.New("missing lockup_details for chain refund validation")
		}
		var lockup boltz.ChainSwapDetails
		if err := json.Unmarshal(lockedIntent.LockupDetails, &lockup); err != nil {
			return nil, errors.New("invalid lockup_details for chain refund validation")
		}
		scriptHex := strings.ToLower(strings.TrimSpace(lockup.SwapTree.RefundLeaf.Output))
		if scriptHex == "" {
			return nil, errors.New("missing lockup_details.swapTree.refundLeaf.output")
		}
		return []string{scriptHex}, nil

	case swap.KindReverse:
		if len(lockedIntent.ReverseDetails) == 0 {
			return nil, errors.New("missing reverse_details for reverse refund validation")
		}
		var reverse boltz.ReverseResponse
		if err := json.Unmarshal(lockedIntent.ReverseDetails, &reverse); err != nil {
			return nil, errors.New("invalid reverse_details for reverse refund validation")
		}
		scriptHex := strings.ToLower(strings.TrimSpace(reverse.SwapTree.RefundLeaf.Output))
		if scriptHex == "" {
			return nil, errors.New("missing reverse_details.swapTree.refundLeaf.output")
		}
		return []string{scriptHex}, nil
	default:
		return nil, fmt.Errorf("refund script-path validation unsupported for kind=%s", s.Kind)
	}
}

func (w *Watcher) expectedRefundLeaves(ctx context.Context, s *swap.Swap) (map[string]txscript.TapscriptLeafVersion, error) {
	lockedIntent, err := w.loadLockedIntent(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]txscript.TapscriptLeafVersion)

	switch s.Kind {
	case swap.KindChain:
		if len(lockedIntent.LockupDetails) == 0 {
			return nil, errors.New("missing lockup_details for chain refund validation")
		}
		var lockup boltz.ChainSwapDetails
		if err := json.Unmarshal(lockedIntent.LockupDetails, &lockup); err != nil {
			return nil, errors.New("invalid lockup_details for chain refund validation")
		}
		scriptHex := strings.ToLower(strings.TrimSpace(lockup.SwapTree.RefundLeaf.Output))
		if scriptHex == "" {
			return nil, errors.New("missing lockup_details.swapTree.refundLeaf.output")
		}
		out[scriptHex] = txscript.TapscriptLeafVersion(lockup.SwapTree.RefundLeaf.Version)
	case swap.KindReverse:
		if len(lockedIntent.ReverseDetails) == 0 {
			return nil, errors.New("missing reverse_details for reverse refund validation")
		}
		var reverse boltz.ReverseResponse
		if err := json.Unmarshal(lockedIntent.ReverseDetails, &reverse); err != nil {
			return nil, errors.New("invalid reverse_details for reverse refund validation")
		}
		scriptHex := strings.ToLower(strings.TrimSpace(reverse.SwapTree.RefundLeaf.Output))
		if scriptHex == "" {
			return nil, errors.New("missing reverse_details.swapTree.refundLeaf.output")
		}
		out[scriptHex] = txscript.TapscriptLeafVersion(reverse.SwapTree.RefundLeaf.Version)
	default:
		return nil, fmt.Errorf("refund script-path validation unsupported for kind=%s", s.Kind)
	}
	return out, nil
}

func validateTxWitnessContainsAnyRefundScript(rawHex string, expectedScriptHex []string) error {
	rawTx, err := hex.DecodeString(strings.TrimSpace(rawHex))
	if err != nil {
		return fmt.Errorf("invalid tx hex: %w", err)
	}
	var tx wire.MsgTx
	if err := tx.Deserialize(bytes.NewReader(rawTx)); err != nil {
		return fmt.Errorf("decode tx: %w", err)
	}
	if len(tx.TxIn) == 0 {
		return errors.New("refund tx has no inputs")
	}

	expected := make(map[string]struct{}, len(expectedScriptHex))
	for _, scriptHex := range expectedScriptHex {
		s := strings.ToLower(strings.TrimSpace(scriptHex))
		if s != "" {
			expected[s] = struct{}{}
		}
	}

	for _, in := range tx.TxIn {
		if len(in.Witness) < 2 {
			continue
		}
		tapscriptHex := strings.ToLower(hex.EncodeToString(in.Witness[len(in.Witness)-2]))
		if _, ok := expected[tapscriptHex]; ok {
			return nil
		}
	}
	return errors.New("no input witness matched expected refund leaf script")
}

func (w *Watcher) persistLockedIntentRefundArtifacts(ctx context.Context, swapID, strategy, source string, statusInfo *boltz.SwapStatus, refundTxid, providerTxid, rawHex string) error {
	lockedIntent, err := w.loadLockedIntent(ctx, swapID)
	if err != nil {
		return err
	}
	if lockedIntent.QuoteID == "" && lockedIntent.Kind == "" &&
		len(lockedIntent.ReverseDetails) == 0 && len(lockedIntent.LockupDetails) == 0 && len(lockedIntent.ClaimDetails) == 0 {
		return nil
	}

	if lockedIntent.Refund == nil {
		lockedIntent.Refund = &swap.LockedIntentRefund{}
	}
	if strategy != "" {
		lockedIntent.Refund.Strategy = strategy
	}
	if source != "" {
		lockedIntent.Refund.Source = source
	}
	if refundTxid != "" {
		lockedIntent.Refund.BroadcastTxID = refundTxid
	}
	if strings.TrimSpace(providerTxid) != "" {
		lockedIntent.Refund.ProviderTxID = strings.TrimSpace(providerTxid)
	}
	if strings.TrimSpace(rawHex) != "" {
		lockedIntent.Refund.RawTxHex = strings.TrimSpace(rawHex)
	}
	if statusInfo != nil && statusInfo.Transaction != nil {
		if statusInfo.Transaction.ID != "" {
			lockedIntent.Refund.ProviderTxID = statusInfo.Transaction.ID
		}
		if statusInfo.Transaction.Hex != "" {
			lockedIntent.Refund.RawTxHex = statusInfo.Transaction.Hex
		}
	}
	if lockedIntent.Refund.Template == nil && strings.TrimSpace(lockedIntent.Refund.RawTxHex) != "" {
		if tpl, tplErr := extractRefundTemplate(strings.TrimSpace(lockedIntent.Refund.RawTxHex)); tplErr == nil {
			lockedIntent.Refund.Template = tpl
		}
	}
	if lockedIntent.Refund.Template != nil {
		tpl := lockedIntent.Refund.Template
		if tpl.RefundLeafVersion == 0 {
			kind := swap.Kind(strings.TrimSpace(lockedIntent.Kind))
			if leaves, lerr := w.expectedRefundLeaves(ctx, &swap.Swap{ID: swapID, Kind: kind}); lerr == nil {
				if v, ok := leaves[strings.ToLower(strings.TrimSpace(tpl.RefundScriptHex))]; ok {
					tpl.RefundLeafVersion = int(v)
				}
			}
		}
		if w.btc != nil && tpl.PrevTxID != "" && (tpl.PrevValueSat == 0 || tpl.PrevPkScriptHex == "") {
			if prev, perr := w.btc.GetTxOut(ctx, tpl.PrevTxID, tpl.PrevVout); perr == nil && prev != nil {
				if tpl.PrevValueSat == 0 {
					tpl.PrevValueSat = prev.ValueSat
				}
				if tpl.PrevPkScriptHex == "" {
					tpl.PrevPkScriptHex = prev.ScriptHex
				}
			}
		}
	}
	lockedIntent.Refund.BroadcastAt = time.Now().UTC().Format(time.RFC3339Nano)

	updatedLockedIntent, err := lockedIntent.ToJSON()
	if err != nil {
		return err
	}
	_, err = w.db.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, string(updatedLockedIntent), swapID)
	return err
}

func extractRefundTemplate(rawHex string) (*swap.LockedIntentRefundTemplate, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(rawHex))
	if err != nil {
		return nil, err
	}
	var tx wire.MsgTx
	if err := tx.Deserialize(bytes.NewReader(raw)); err != nil {
		return nil, err
	}
	if len(tx.TxIn) != 1 || len(tx.TxOut) != 1 {
		return nil, errors.New("refund template extraction supports 1-in/1-out tx only")
	}
	in := tx.TxIn[0]
	if len(in.Witness) < 3 {
		return nil, errors.New("refund template witness too short")
	}
	refundScript := in.Witness[len(in.Witness)-2]
	controlBlock := in.Witness[len(in.Witness)-1]
	args := make([]string, 0, len(in.Witness)-3)
	for i := 1; i < len(in.Witness)-2; i++ {
		args = append(args, hex.EncodeToString(in.Witness[i]))
	}
	return &swap.LockedIntentRefundTemplate{
		TxVersion:         tx.Version,
		LockTime:          tx.LockTime,
		PrevTxID:          in.PreviousOutPoint.Hash.String(),
		PrevVout:          in.PreviousOutPoint.Index,
		Sequence:          in.Sequence,
		OutputValueSat:    tx.TxOut[0].Value,
		OutputPkScriptHex: hex.EncodeToString(tx.TxOut[0].PkScript),
		RefundScriptHex:   hex.EncodeToString(refundScript),
		ControlBlockHex:   hex.EncodeToString(controlBlock),
		WitnessArgsHex:    args,
	}, nil
}

func (w *Watcher) persistExecutionError(ctx context.Context, swapID, errorMessage string) error {
	_, err := w.db.ExecContext(ctx, `
		UPDATE swaps
		SET error_message = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, errorMessage, swapID)
	return err
}

func (w *Watcher) prepareFallbackRefundPlan(ctx context.Context, s *swap.Swap) (map[string]interface{}, error) {
	plan := map[string]interface{}{
		"kind":     string(s.Kind),
		"strategy": "script_path_fallback",
	}
	lockedIntent, err := w.loadLockedIntent(ctx, s.ID)
	if err != nil {
		return nil, err
	}

	switch s.Kind {
	case swap.KindChain:
		lockup, claim, err := w.loadChainRefundTrees(lockedIntent)
		if err != nil {
			return nil, err
		}
		plan["lockup_claim_leaf_version"] = lockup.SwapTree.ClaimLeaf.Version
		plan["lockup_refund_leaf_version"] = lockup.SwapTree.RefundLeaf.Version
		plan["claim_claim_leaf_version"] = claim.SwapTree.ClaimLeaf.Version
		plan["claim_refund_leaf_version"] = claim.SwapTree.RefundLeaf.Version
		plan["has_blinding_key"] = claim.BlindingKey != ""
		plan["claim_timeout_block"] = claim.TimeoutBlockHeight
	case swap.KindReverse:
		reverse, err := w.getPersistedReverseDetails(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		if reverse == nil || reverse.SwapTree.RefundLeaf.Output == "" {
			return nil, errors.New("reverse fallback requires reverse_details.swapTree.refundLeaf")
		}
		plan["reverse_refund_leaf_version"] = reverse.SwapTree.RefundLeaf.Version
		plan["reverse_claim_leaf_version"] = reverse.SwapTree.ClaimLeaf.Version
		plan["reverse_refund_pubkey"] = reverse.RefundPublicKey
		plan["reverse_timeout_block"] = reverse.TimeoutBlockHeight
	default:
		return nil, fmt.Errorf("fallback plan unsupported for kind=%s", s.Kind)
	}

	return plan, nil
}

func (w *Watcher) loadChainRefundTrees(intent swap.LockedIntent) (*boltz.ChainSwapDetails, *boltz.ChainSwapDetails, error) {
	if len(intent.LockupDetails) == 0 || len(intent.ClaimDetails) == 0 {
		return nil, nil, errors.New("chain fallback requires lockup_details and claim_details")
	}
	var lockup boltz.ChainSwapDetails
	if err := json.Unmarshal(intent.LockupDetails, &lockup); err != nil {
		return nil, nil, errors.New("invalid lockup_details for chain fallback")
	}
	var claim boltz.ChainSwapDetails
	if err := json.Unmarshal(intent.ClaimDetails, &claim); err != nil {
		return nil, nil, errors.New("invalid claim_details for chain fallback")
	}
	if lockup.SwapTree.RefundLeaf.Output == "" || claim.SwapTree.RefundLeaf.Output == "" {
		return nil, nil, errors.New("chain fallback requires swapTree refund leaf outputs")
	}
	return &lockup, &claim, nil
}

func (w *Watcher) loadLockedIntent(ctx context.Context, swapID string) (swap.LockedIntent, error) {
	var raw string
	if err := w.db.QueryRowContext(ctx, `SELECT COALESCE(locked_intent, '') FROM swaps WHERE id = ?`, swapID).Scan(&raw); err != nil {
		return swap.LockedIntent{}, err
	}
	if raw == "" {
		return swap.LockedIntent{}, nil
	}
	parsed, err := swap.ParseLockedIntent(raw)
	if err != nil {
		return swap.LockedIntent{}, nil
	}
	return parsed, nil
}

func (w *Watcher) getPersistedChainServerPubKey(ctx context.Context, swapID string) (string, error) {
	claim, err := w.getPersistedChainClaimDetails(ctx, swapID)
	if err != nil {
		return "", err
	}
	if claim == nil {
		return "", nil
	}
	return claim.ServerPublicKey, nil
}

func (w *Watcher) getPersistedChainClaimDetails(ctx context.Context, swapID string) (*boltz.ChainSwapDetails, error) {
	lockedIntent, err := w.loadLockedIntent(ctx, swapID)
	if err != nil {
		return nil, err
	}
	if len(lockedIntent.ClaimDetails) == 0 || string(lockedIntent.ClaimDetails) == "null" {
		return nil, nil
	}
	var claim boltz.ChainSwapDetails
	if err := json.Unmarshal(lockedIntent.ClaimDetails, &claim); err != nil {
		return nil, nil
	}
	return &claim, nil
}

func (w *Watcher) getPersistedReverseDetails(ctx context.Context, swapID string) (*boltz.ReverseResponse, error) {
	lockedIntent, err := w.loadLockedIntent(ctx, swapID)
	if err != nil {
		return nil, err
	}
	if len(lockedIntent.ReverseDetails) == 0 || string(lockedIntent.ReverseDetails) == "null" {
		return nil, nil
	}
	var reverse boltz.ReverseResponse
	if err := json.Unmarshal(lockedIntent.ReverseDetails, &reverse); err != nil {
		return nil, nil
	}
	return &reverse, nil
}

func (w *Watcher) persistLockedIntentMuSig(ctx context.Context, swapID, sessionID, localPubNonce string, aggNonce []byte, partialSig string) error {
	lockedIntent, err := w.loadLockedIntent(ctx, swapID)
	if err != nil {
		return err
	}

	// If intent has not been initialized yet, skip gracefully.
	if lockedIntent.QuoteID == "" && lockedIntent.Kind == "" && len(lockedIntent.LockupDetails) == 0 && len(lockedIntent.ClaimDetails) == 0 {
		return nil
	}

	if lockedIntent.MuSig == nil {
		lockedIntent.MuSig = &swap.LockedIntentMuSig{}
	}
	if sessionID != "" {
		lockedIntent.MuSig.SessionID = sessionID
	}
	if localPubNonce != "" {
		lockedIntent.MuSig.LocalPubNonce = localPubNonce
	}
	if len(aggNonce) > 0 {
		lockedIntent.MuSig.AggNonce = hex.EncodeToString(aggNonce)
	}
	if partialSig != "" {
		lockedIntent.MuSig.PartialSig = partialSig
	}
	lockedIntent.MuSig.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	updatedLockedIntent, err := lockedIntent.ToJSON()
	if err != nil {
		return err
	}
	_, err = w.db.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		WHERE id = ?
	`, string(updatedLockedIntent), swapID)
	return err
}

func decodeHexOrRaw(v string) []byte {
	if v == "" {
		return nil
	}
	decoded, err := hex.DecodeString(v)
	if err == nil {
		return decoded
	}
	return []byte(v)
}

func parsePubNonceHex(v string) ([musig2.PubNonceSize]byte, error) {
	var out [musig2.PubNonceSize]byte
	raw, err := hex.DecodeString(v)
	if err != nil {
		return out, err
	}
	if len(raw) != musig2.PubNonceSize {
		return out, fmt.Errorf("pub nonce size %d, expected %d", len(raw), musig2.PubNonceSize)
	}
	copy(out[:], raw)
	return out, nil
}

func parseMuSigMessage(txHash string) ([32]byte, error) {
	var msg [32]byte
	raw, err := hex.DecodeString(txHash)
	if err != nil {
		return msg, err
	}
	if len(raw) != 32 {
		return msg, fmt.Errorf("transaction hash length %d, expected 32", len(raw))
	}
	copy(msg[:], raw)
	return msg, nil
}

func encodeMusigPartialSig(sig *musig2.PartialSignature) string {
	if sig == nil || sig.S == nil {
		return ""
	}
	var scalar [32]byte
	sig.S.PutBytes(&scalar)
	return hex.EncodeToString(scalar[:])
}

func buildReconcileEventKey(s *swap.Swap, facts swap.Facts, action swap.Action, newState swap.State) string {
	return fmt.Sprintf(
		"reconcile:%s:%s->%s:tx=%s:ps=%s:h=%d:ta=%t:tr=%t:ls=%t:lc=%t",
		actionName(action),
		s.State,
		newState,
		s.LockupTxid,
		facts.ProviderStatus,
		facts.CurrentHeight,
		facts.TimeoutApproaching,
		facts.TimeoutReached,
		facts.LockupSeen,
		facts.LockupConfirmed,
	)
}

func actionName(a swap.Action) string {
	switch a {
	case swap.ActionWaitForLockup:
		return "wait_for_lockup"
	case swap.ActionTransitionToWaiting:
		return "to_waiting"
	case swap.ActionTransitionToCompleted:
		return "to_completed"
	case swap.ActionTransitionToFailed:
		return "to_failed"
	case swap.ActionPrepareRefund:
		return "prepare_refund"
	case swap.ActionBroadcastRefund:
		return "broadcast_refund"
	default:
		return "none"
	}
}

func mustJSON(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// listActiveSwaps returns all non-terminal swaps
func (w *Watcher) listActiveSwaps(ctx context.Context) ([]*swap.Swap, error) {
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, kind, env, version, state, swap_key_index,
		       COALESCE(preimage_hash_hex, ''),
		       COALESCE(boltz_id, ''), COALESCE(boltz_status, ''),
		       COALESCE(lockup_txid, ''), COALESCE(lockup_amount_sat, ''),
		       COALESCE(refund_txid, ''),
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
			&s.PreimageHashHex,
			&s.BoltzID, &s.BoltzStatus,
			&s.LockupTxid, &s.LockupAmountSat,
			&s.RefundTxid,
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
