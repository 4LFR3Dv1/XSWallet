package watcher

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/xs-wallet/xscore/internal/adapters/bitcoin"
	"github.com/xs-wallet/xscore/internal/boltz"
	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
	"github.com/xs-wallet/xscore/internal/swap"
)

type watcherTestVault struct{}

func (watcherTestVault) EncryptPreimage(b []byte) ([]byte, error) {
	return append([]byte("enc:"), b...), nil
}

func (watcherTestVault) DecryptPreimage(b []byte) ([]byte, error) {
	return bytes.TrimPrefix(b, []byte("enc:")), nil
}

func (watcherTestVault) Seed() ([]byte, error) {
	return bytes.Repeat([]byte{0x42}, 32), nil
}

func openWatcherTestDB(t *testing.T) *db.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "watcher_test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return database
}

func TestBuildReconcileEventKeyAndIdempotency(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})

	created, err := engine.Create(ctx, swap.KindSubmarine, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := database.ExecContext(ctx, `UPDATE swaps SET locked_intent = ?, lockup_txid = ? WHERE id = ?`, "{}", txid, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit-start swap: %v", err)
	}

	facts := swap.ComputeFacts(ctx, commitStarted, 100, nil, "")
	action, newState := swap.NextAction(commitStarted, facts)
	if action != swap.ActionTransitionToWaiting || newState != swap.StateWaiting {
		t.Fatalf("unexpected action/state: %v %v", action, newState)
	}

	eventKey := buildReconcileEventKey(commitStarted, facts, action, newState)
	if eventKey == "" {
		t.Fatalf("expected non-empty event key")
	}

	done, _, err := engine.CheckIdempotency(ctx, commitStarted.ID, eventKey)
	if err != nil {
		t.Fatalf("check idempotency (before): %v", err)
	}
	if done {
		t.Fatalf("event should not be marked as processed yet")
	}

	if err := engine.RecordOperation(ctx, commitStarted.ID, eventKey, string(newState)); err != nil {
		t.Fatalf("record operation: %v", err)
	}

	done, result, err := engine.CheckIdempotency(ctx, commitStarted.ID, eventKey)
	if err != nil {
		t.Fatalf("check idempotency (after): %v", err)
	}
	if !done {
		t.Fatalf("event should be marked as processed")
	}
	if result != string(newState) {
		t.Fatalf("unexpected idempotency result: %s", result)
	}
}

func TestApplyProviderStatusState_ChainTransitionsToSigningPath(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})
	w := &Watcher{db: database, engine: engine}

	created, err := engine.Create(ctx, swap.KindChain, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?, boltz_id = ?, boltz_status = ?
		WHERE id = ?
	`, `{"lockup_details":{"lockupAddress":"tb1q"}, "claim_details":{"lockupAddress":"tlq1","serverPublicKey":"02aa"}}`, "boltz-chain-1", provider.StatusCreated, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit-start swap: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("waiting swap: %v", err)
	}

	transitioned, err := w.applyProviderStatusState(ctx, waiting, "transaction.server.confirmed")
	if err != nil {
		t.Fatalf("apply provider status: %v", err)
	}
	if !transitioned {
		t.Fatalf("expected transition to happen")
	}

	updated, err := engine.Get(ctx, waiting.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}
	if updated.State != swap.StateSigningMusig2Partial {
		t.Fatalf("expected state %s, got %s", swap.StateSigningMusig2Partial, updated.State)
	}
}

func TestApplyProviderStatusState_WaitingProviderBroadcastFailureGoesToRefunding(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})
	w := &Watcher{db: database, engine: engine}

	created, err := engine.Create(ctx, swap.KindChain, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?, boltz_id = ?, boltz_status = ?
		WHERE id = ?
	`, `{"version":1,"quote_id":"q-fallback","kind":"chain","lockup_details":{"lockupAddress":"tb1q","swapTree":{"claimLeaf":{"version":192,"output":"51"},"refundLeaf":{"version":192,"output":"52"}}},"claim_details":{"lockupAddress":"tlq1","serverPublicKey":"02aa","swapTree":{"claimLeaf":{"version":196,"output":"53"},"refundLeaf":{"version":196,"output":"54"}}}}`, "boltz-chain-fallback", provider.StatusCreated, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit-start swap: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("waiting swap: %v", err)
	}
	waitClaim, err := engine.Transition(ctx, waiting.ID, waiting.Version, swap.StateWaitingClaimDetails, "provider_status", nil)
	if err != nil {
		t.Fatalf("waiting claim details: %v", err)
	}
	signing, err := engine.Transition(ctx, waitClaim.ID, waitClaim.Version, swap.StateSigningMusig2Partial, "provider_status", nil)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	sent, err := engine.Transition(ctx, signing.ID, signing.Version, swap.StateSentPartialToProvider, "musig_claim_sent", nil)
	if err != nil {
		t.Fatalf("sent partial: %v", err)
	}
	waitBroadcast, err := engine.Transition(ctx, sent.ID, sent.Version, swap.StateWaitingProviderBroadcast, "musig_claim_sent", nil)
	if err != nil {
		t.Fatalf("waiting provider broadcast: %v", err)
	}

	transitioned, err := w.applyProviderStatusState(ctx, waitBroadcast, "swap.expired")
	if err != nil {
		t.Fatalf("apply provider status: %v", err)
	}
	if !transitioned {
		t.Fatalf("expected transition to fallback path")
	}

	updated, err := engine.Get(ctx, waitBroadcast.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}
	if updated.State != swap.StateRefunding {
		t.Fatalf("expected state %s, got %s", swap.StateRefunding, updated.State)
	}
}

func TestApplyProviderStatusState_WaitingProviderBroadcastSuccessGoesToCompleted(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})
	w := &Watcher{db: database, engine: engine}

	created, err := engine.Create(ctx, swap.KindReverse, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?, boltz_id = ?, boltz_status = ?
		WHERE id = ?
	`, `{"version":1,"quote_id":"q-complete","kind":"reverse","reverse_details":{"id":"boltz-rev-complete"}}`, "boltz-rev-complete", provider.StatusCreated, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit-start swap: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("waiting swap: %v", err)
	}
	waitClaim, err := engine.Transition(ctx, waiting.ID, waiting.Version, swap.StateWaitingClaimDetails, "provider_status", nil)
	if err != nil {
		t.Fatalf("waiting claim details: %v", err)
	}
	signing, err := engine.Transition(ctx, waitClaim.ID, waitClaim.Version, swap.StateSigningMusig2Partial, "provider_status", nil)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}
	sent, err := engine.Transition(ctx, signing.ID, signing.Version, swap.StateSentPartialToProvider, "musig_claim_sent", nil)
	if err != nil {
		t.Fatalf("sent partial: %v", err)
	}
	waitBroadcast, err := engine.Transition(ctx, sent.ID, sent.Version, swap.StateWaitingProviderBroadcast, "musig_claim_sent", nil)
	if err != nil {
		t.Fatalf("waiting provider broadcast: %v", err)
	}

	transitioned, err := w.applyProviderStatusState(ctx, waitBroadcast, "invoice.settled")
	if err != nil {
		t.Fatalf("apply provider status: %v", err)
	}
	if !transitioned {
		t.Fatalf("expected transition to happen")
	}

	updated, err := engine.Get(ctx, waitBroadcast.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}
	if updated.State != swap.StateCompleted {
		t.Fatalf("expected state %s, got %s", swap.StateCompleted, updated.State)
	}
}

type watcherReverseClaimProvider struct{}

func (watcherReverseClaimProvider) Quote(context.Context, provider.QuoteRequest) (*provider.Quote, error) {
	return nil, nil
}

func (watcherReverseClaimProvider) Create(context.Context, provider.CreateRequest) (*provider.CreateResponse, error) {
	return nil, nil
}

func (watcherReverseClaimProvider) Subscribe(context.Context, string) (<-chan provider.Update, func(), error) {
	ch := make(chan provider.Update)
	cancel := func() { close(ch) }
	return ch, cancel, nil
}

func (watcherReverseClaimProvider) GetSwapStatus(context.Context, string) (string, error) {
	return "transaction.confirmed", nil
}

func (watcherReverseClaimProvider) PostReverseClaim(_ context.Context, _ string, req boltz.ReverseClaimRequest) (*boltz.ReverseClaimResponse, error) {
	return &boltz.ReverseClaimResponse{
		PubNonce:         req.PubNonce,
		PartialSignature: "abababababababababababababababababababababababababababababababababababababababababababababababababababababababababababababababab",
	}, nil
}

func TestExecuteMuSig2Claim_ReverseAdvancesToWaitingProviderBroadcast(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})
	w := &Watcher{db: database, engine: engine, prov: watcherReverseClaimProvider{}, seedSource: watcherTestVault{}, network: "regtest"}

	created, err := engine.Create(ctx, swap.KindReverse, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?, boltz_id = ?, boltz_status = ?
		WHERE id = ?
	`, `{"version":1,"quote_id":"q-1","kind":"reverse","reverse_details":{"id":"boltz-rev-1","invoice":"lntb1test","lockupAddress":"tb1qreverse","timeoutBlockHeight":555000,"onchainAmount":123456}}`, "boltz-rev-1", "transaction.confirmed", created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit-start swap: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("waiting swap: %v", err)
	}
	waitClaim, err := engine.Transition(ctx, waiting.ID, waiting.Version, swap.StateWaitingClaimDetails, "provider_status", nil)
	if err != nil {
		t.Fatalf("waiting claim details: %v", err)
	}
	signing, err := engine.Transition(ctx, waitClaim.ID, waitClaim.Version, swap.StateSigningMusig2Partial, "provider_status", nil)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	if err := w.executeMuSig2Claim(ctx, signing); err != nil {
		t.Fatalf("execute claim: %v", err)
	}

	updated, err := engine.Get(ctx, signing.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}
	if updated.State != swap.StateWaitingProviderBroadcast {
		t.Fatalf("expected state %s, got %s", swap.StateWaitingProviderBroadcast, updated.State)
	}

	var lockedIntentRaw string
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(locked_intent, '') FROM swaps WHERE id = ?`, signing.ID).Scan(&lockedIntentRaw); err != nil {
		t.Fatalf("query locked_intent: %v", err)
	}
	lockedIntent, err := swap.ParseLockedIntent(lockedIntentRaw)
	if err != nil {
		t.Fatalf("parse locked_intent: %v", err)
	}
	if lockedIntent.MuSig == nil {
		t.Fatalf("expected musig metadata persisted on locked_intent")
	}
	if lockedIntent.MuSig.SessionID == "" || lockedIntent.MuSig.LocalPubNonce == "" || lockedIntent.MuSig.PartialSig == "" {
		t.Fatalf("expected musig session/pubnonce/partial on locked_intent, got %+v", lockedIntent.MuSig)
	}

	var reverseEventDetails sql.NullString
	if err := database.QueryRowContext(ctx, `
		SELECT details
		FROM swap_events
		WHERE swap_id = ? AND trigger = 'musig_claim_sent' AND to_state = 'waiting_provider_broadcast'
		ORDER BY seq DESC LIMIT 1
	`, signing.ID).Scan(&reverseEventDetails); err != nil {
		t.Fatalf("query musig event details: %v", err)
	}
	if !reverseEventDetails.Valid || reverseEventDetails.String == "" {
		t.Fatalf("expected swap_events.details to be persisted")
	}
	if !strings.Contains(reverseEventDetails.String, "reverse_onchain_amount") {
		t.Fatalf("expected reverse details in swap_events.details, got: %s", reverseEventDetails.String)
	}
	if !strings.Contains(reverseEventDetails.String, "has_invoice") {
		t.Fatalf("expected reverse invoice flag in swap_events.details, got: %s", reverseEventDetails.String)
	}
}

type watcherChainClaimProvider struct {
	pubNonceHex  string
	txHashHex    string
	lastPartial  boltz.PartialSignature
	lastPreimage string
}

func (p *watcherChainClaimProvider) Quote(context.Context, provider.QuoteRequest) (*provider.Quote, error) {
	return nil, nil
}

func (p *watcherChainClaimProvider) Create(context.Context, provider.CreateRequest) (*provider.CreateResponse, error) {
	return nil, nil
}

func (p *watcherChainClaimProvider) Subscribe(context.Context, string) (<-chan provider.Update, func(), error) {
	ch := make(chan provider.Update)
	cancel := func() { close(ch) }
	return ch, cancel, nil
}

func (p *watcherChainClaimProvider) GetSwapStatus(context.Context, string) (string, error) {
	return "transaction.server.confirmed", nil
}

func (p *watcherChainClaimProvider) GetChainClaimDetails(context.Context, string) (*boltz.ChainClaimDetails, error) {
	return &boltz.ChainClaimDetails{
		PubNonce:        p.pubNonceHex,
		PublicKey:       "",
		TransactionHash: p.txHashHex,
	}, nil
}

func (p *watcherChainClaimProvider) PostChainClaim(_ context.Context, _ string, sig boltz.PartialSignature, preimage string) error {
	p.lastPartial = sig
	p.lastPreimage = preimage
	return nil
}

func TestExecuteMuSig2Claim_ChainUsesPersistedServerPubKeyFallback(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})

	txHashHex := strings.Repeat("12", 32)
	msgHashBytes, err := hex.DecodeString(txHashHex)
	if err != nil {
		t.Fatalf("decode tx hash: %v", err)
	}
	var msgHash [32]byte
	copy(msgHash[:], msgHashBytes)

	remotePriv, remotePub := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{0x24}, 32))
	remoteNonces, err := musig2.GenNonces(
		musig2.WithPublicKey(remotePub),
		musig2.WithNonceSecretKeyAux(remotePriv),
		musig2.WithNonceMessageAux(msgHash),
	)
	if err != nil {
		t.Fatalf("gen remote nonce: %v", err)
	}

	claimProvider := &watcherChainClaimProvider{
		pubNonceHex: hex.EncodeToString(remoteNonces.PubNonce[:]),
		txHashHex:   txHashHex,
	}
	w := &Watcher{
		db:         database,
		engine:     engine,
		prov:       claimProvider,
		seedSource: watcherTestVault{},
		network:    "regtest",
	}

	created, err := engine.Create(ctx, swap.KindChain, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	serverPubHex := fmt.Sprintf("%x", remotePub.SerializeCompressed())
	lockedIntent := fmt.Sprintf(`{
		"version":1,
		"quote_id":"q-chain-1",
		"kind":"chain",
		"lockup_details":{"lockupAddress":"tb1qchainlock","timeoutBlockHeight":111},
		"claim_details":{
			"lockupAddress":"tlq1chainclaim",
			"timeoutBlockHeight":222,
			"blindingKey":"abcdef",
			"serverPublicKey":"%s",
			"swapTree":{
				"claimLeaf":{"version":196,"output":"51"},
				"refundLeaf":{"version":197,"output":"52"}
			}
		}
	}`, serverPubHex)
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?, boltz_id = ?, boltz_status = ?
		WHERE id = ?
	`, lockedIntent, "boltz-chain-1", "transaction.server.confirmed", created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}

	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock swap: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit-start swap: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("waiting swap: %v", err)
	}
	waitClaim, err := engine.Transition(ctx, waiting.ID, waiting.Version, swap.StateWaitingClaimDetails, "provider_status", nil)
	if err != nil {
		t.Fatalf("waiting claim details: %v", err)
	}
	signing, err := engine.Transition(ctx, waitClaim.ID, waitClaim.Version, swap.StateSigningMusig2Partial, "provider_status", nil)
	if err != nil {
		t.Fatalf("signing: %v", err)
	}

	if err := w.executeMuSig2Claim(ctx, signing); err != nil {
		t.Fatalf("execute chain claim: %v", err)
	}

	updated, err := engine.Get(ctx, signing.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}
	if updated.State != swap.StateWaitingProviderBroadcast {
		t.Fatalf("expected state %s, got %s", swap.StateWaitingProviderBroadcast, updated.State)
	}
	if claimProvider.lastPartial.PartialSignature == "" {
		t.Fatalf("expected partial signature to be posted")
	}
	if claimProvider.lastPreimage == "" {
		t.Fatalf("expected preimage to be posted")
	}

	var lockedIntentRaw string
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(locked_intent, '') FROM swaps WHERE id = ?`, signing.ID).Scan(&lockedIntentRaw); err != nil {
		t.Fatalf("query locked_intent: %v", err)
	}
	parsedIntent, err := swap.ParseLockedIntent(lockedIntentRaw)
	if err != nil {
		t.Fatalf("parse locked_intent: %v", err)
	}
	if parsedIntent.MuSig == nil {
		t.Fatalf("expected musig metadata persisted on locked_intent")
	}
	if parsedIntent.MuSig.AggNonce == "" {
		t.Fatalf("expected aggregate nonce persisted on locked_intent.musig")
	}
	if parsedIntent.MuSig.PartialSig == "" {
		t.Fatalf("expected partial signature persisted on locked_intent.musig")
	}
	if parsedIntent.MuSig.UpdatedAt == "" {
		t.Fatalf("expected musig updated_at persisted on locked_intent.musig")
	}
	var check map[string]interface{}
	if err := json.Unmarshal([]byte(lockedIntentRaw), &check); err != nil {
		t.Fatalf("locked_intent should remain valid JSON: %v", err)
	}

	var chainEventDetails string
	if err := database.QueryRowContext(ctx, `
		SELECT COALESCE(details, '')
		FROM swap_events
		WHERE swap_id = ? AND trigger = 'musig_claim_sent' AND to_state = 'waiting_provider_broadcast'
		ORDER BY seq DESC LIMIT 1
	`, signing.ID).Scan(&chainEventDetails); err != nil {
		t.Fatalf("query musig event details: %v", err)
	}
	if !strings.Contains(chainEventDetails, "await_status") {
		t.Fatalf("expected enriched details_json in musig event, got: %s", chainEventDetails)
	}
	if !strings.Contains(chainEventDetails, "has_blinding_key") {
		t.Fatalf("expected claim_details fields in details_json, got: %s", chainEventDetails)
	}
	if !strings.Contains(chainEventDetails, "claim_lockup_address") {
		t.Fatalf("expected claim lockup address in details_json, got: %s", chainEventDetails)
	}
}

func TestValidateTxWitnessContainsAnyRefundScript_Match(t *testing.T) {
	scriptHex := "20fc7e8a775782ffb79acb5daf290cf913728c6fc20a4f289a6054dfd0e90b3a00ad0360e249b1"
	rawHex := buildTestTaprootWitnessTxHex(t, scriptHex)
	if err := validateTxWitnessContainsAnyRefundScript(rawHex, []string{scriptHex}); err != nil {
		t.Fatalf("expected script match, got error: %v", err)
	}
}

func TestValidateTxWitnessContainsAnyRefundScript_Mismatch(t *testing.T) {
	scriptHex := "20fc7e8a775782ffb79acb5daf290cf913728c6fc20a4f289a6054dfd0e90b3a00ad0360e249b1"
	otherScript := "201111111111111111111111111111111111111111111111111111111111111111ad0360e249b1"
	rawHex := buildTestTaprootWitnessTxHex(t, scriptHex)
	if err := validateTxWitnessContainsAnyRefundScript(rawHex, []string{otherScript}); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func buildTestTaprootWitnessTxHex(t *testing.T, scriptHex string) string {
	t.Helper()
	script, err := hex.DecodeString(scriptHex)
	if err != nil {
		t.Fatalf("decode script hex: %v", err)
	}
	tx := wire.NewMsgTx(2)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: 0,
		},
		Sequence: wire.MaxTxInSequenceNum,
		Witness: wire.TxWitness{
			{0x00},       // stack item placeholder
			script,       // tapscript (second-last witness element)
			{0xc0, 0x01}, // control block placeholder
		},
	})
	tx.AddTxOut(&wire.TxOut{Value: 1000, PkScript: []byte{0x51}})
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		t.Fatalf("serialize tx: %v", err)
	}
	return hex.EncodeToString(buf.Bytes())
}

func TestLoadPersistedRefundRawTx(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})
	w := &Watcher{db: database, engine: engine}

	created, err := engine.Create(ctx, swap.KindChain, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	raw := `{
		"version":1,
		"quote_id":"q-refund",
		"kind":"chain",
		"refund":{
			"source":"provider_tx_hex",
			"provider_txid":"abc123",
			"raw_tx_hex":"deadbeef"
		}
	}`
	if _, err := database.ExecContext(ctx, `UPDATE swaps SET locked_intent = ? WHERE id = ?`, raw, created.ID); err != nil {
		t.Fatalf("set locked_intent: %v", err)
	}

	hexTx, txid := w.loadPersistedRefundRawTx(ctx, created.ID)
	if hexTx != "deadbeef" {
		t.Fatalf("expected deadbeef raw tx, got %s", hexTx)
	}
	if txid != "abc123" {
		t.Fatalf("expected abc123 txid, got %s", txid)
	}
}

func TestExtractRefundTemplate(t *testing.T) {
	scriptHex := "20fc7e8a775782ffb79acb5daf290cf913728c6fc20a4f289a6054dfd0e90b3a00ad0360e249b1"
	rawHex := buildTestTaprootWitnessTxHex(t, scriptHex)
	tpl, err := extractRefundTemplate(rawHex)
	if err != nil {
		t.Fatalf("extract template: %v", err)
	}
	if tpl == nil {
		t.Fatalf("expected template")
	}
	if tpl.RefundScriptHex != scriptHex {
		t.Fatalf("expected refund script %s, got %s", scriptHex, tpl.RefundScriptHex)
	}
	if tpl.PrevTxID == "" {
		t.Fatalf("expected prev txid")
	}
	if tpl.OutputPkScriptHex == "" {
		t.Fatalf("expected output pkscript")
	}
}

type watcherRefundStatusProvider struct {
	status string
	info   *boltz.SwapStatus
}

func (p watcherRefundStatusProvider) Quote(context.Context, provider.QuoteRequest) (*provider.Quote, error) {
	return nil, nil
}
func (p watcherRefundStatusProvider) Create(context.Context, provider.CreateRequest) (*provider.CreateResponse, error) {
	return nil, nil
}
func (p watcherRefundStatusProvider) Subscribe(context.Context, string) (<-chan provider.Update, func(), error) {
	ch := make(chan provider.Update)
	cancel := func() { close(ch) }
	return ch, cancel, nil
}
func (p watcherRefundStatusProvider) GetSwapStatus(context.Context, string) (string, error) {
	return p.status, nil
}
func (p watcherRefundStatusProvider) GetSwapStatusInfo(context.Context, string) (*boltz.SwapStatus, error) {
	return p.info, nil
}

func TestBroadcastFallbackRefund_FetchesRawHexByTxIDWhenProviderHexMissing(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("environment does not allow local listener: %v", err)
	}
	_ = l.Close()

	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})

	scriptHex := "20fc7e8a775782ffb79acb5daf290cf913728c6fc20a4f289a6054dfd0e90b3a00ad0360e249b1"
	rawTxHex := buildTestTaprootWitnessTxHex(t, scriptHex)
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "getrawtransaction":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":"%s","error":null,"id":"xscore"}`, rawTxHex)))
		case "sendrawtransaction":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":"%s","error":null,"id":"xscore"}`, txid)))
		default:
			_, _ = w.Write([]byte(`{"result":0,"error":null,"id":"xscore"}`))
		}
	}))
	defer rpcServer.Close()

	prov := watcherRefundStatusProvider{
		status: boltz.StatusSwapExpired,
		info: &boltz.SwapStatus{
			Status: boltz.StatusSwapExpired,
			Transaction: &boltz.TxInfo{
				ID:  txid,
				Hex: "",
			},
		},
	}

	w := &Watcher{
		db:     database,
		engine: engine,
		btc:    bitcoin.NewClient(rpcServer.URL, "", ""),
		prov:   prov,
	}

	created, err := engine.Create(ctx, swap.KindChain, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	lockedIntent := fmt.Sprintf(`{
		"version":1,
		"quote_id":"q-refund-rpc",
		"kind":"chain",
		"lockup_details":{"swapTree":{"refundLeaf":{"version":192,"output":"%s"}}}
	}`, scriptHex)
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps SET locked_intent = ?, boltz_id = ? WHERE id = ?
	`, lockedIntent, "boltz-refund-1", created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	s, err := engine.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}
	s.BoltzID = "boltz-refund-1"

	broadcastedTxid, _, err := w.broadcastFallbackRefund(ctx, s)
	if err != nil {
		t.Fatalf("broadcastFallbackRefund: %v", err)
	}
	if broadcastedTxid != txid {
		t.Fatalf("expected txid %s, got %s", txid, broadcastedTxid)
	}

	var raw string
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(locked_intent, '') FROM swaps WHERE id = ?`, s.ID).Scan(&raw); err != nil {
		t.Fatalf("query locked_intent: %v", err)
	}
	intent, err := swap.ParseLockedIntent(raw)
	if err != nil {
		t.Fatalf("parse locked_intent: %v", err)
	}
	if intent.Refund == nil {
		t.Fatalf("expected refund metadata")
	}
	if intent.Refund.RawTxHex == "" {
		t.Fatalf("expected raw tx hex persisted")
	}
	if intent.Refund.ProviderTxID != txid {
		t.Fatalf("expected provider txid %s, got %s", txid, intent.Refund.ProviderTxID)
	}
}

func TestBroadcastFallbackRefund_BuildsFromArtifactsWithoutProvider(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("environment does not allow local listener: %v", err)
	}
	_ = l.Close()

	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})

	lockupAddress := "bcrt1q2vfxp4fmgf8kuy0a8248yk6cehwwlry2a66f9h"
	lockupScriptHex := "5120e8f32e723decf4051aefac8e2c93c9c5b214313a8fe23095ef58f6f8c04d8d83"
	refundScriptHex := "51"
	controlBlockHex := "c001"
	prevTxID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	broadcastTxID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	rpcServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "scantxoutset":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"success":true,"height":100,"txouts":1,"unspents":[{"txid":"%s","vout":0,"address":"%s","amount":0.01,"height":99,"scriptPubKey":"%s"}]},"error":null,"id":"xscore"}`, prevTxID, lockupAddress, lockupScriptHex)))
		case "gettxout":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"value":0.01,"scriptPubKey":{"hex":"%s"}},"error":null,"id":"xscore"}`, lockupScriptHex)))
		case "sendrawtransaction":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":"%s","error":null,"id":"xscore"}`, broadcastTxID)))
		default:
			_, _ = w.Write([]byte(`{"result":0,"error":null,"id":"xscore"}`))
		}
	}))
	rpcServer.Listener, _ = net.Listen("tcp", "127.0.0.1:0")
	rpcServer.Start()
	defer rpcServer.Close()

	w := &Watcher{
		db:         database,
		engine:     engine,
		btc:        bitcoin.NewClient(rpcServer.URL, "", ""),
		seedSource: watcherTestVault{},
		network:    "regtest",
	}

	created, err := engine.Create(ctx, swap.KindChain, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	lockedIntent := fmt.Sprintf(`{
		"version":1,
		"quote_id":"q-refund-local",
		"kind":"chain",
		"lockup_address":"%s",
		"lockup_details":{"lockupAddress":"%s","timeoutBlockHeight":200,"swapTree":{"refundLeaf":{"version":192,"output":"%s"}}},
		"refund":{"template":{"control_block_hex":"%s"}}
	}`, lockupAddress, lockupAddress, refundScriptHex, controlBlockHex)
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?
		WHERE id = ?
	`, lockedIntent, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}
	s, err := engine.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}

	txid, _, err := w.broadcastFallbackRefund(ctx, s)
	if err != nil {
		t.Fatalf("broadcastFallbackRefund: %v", err)
	}
	if txid != broadcastTxID {
		t.Fatalf("expected txid %s, got %s", broadcastTxID, txid)
	}

	var raw string
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(locked_intent, '') FROM swaps WHERE id = ?`, s.ID).Scan(&raw); err != nil {
		t.Fatalf("query locked_intent: %v", err)
	}
	intent, err := swap.ParseLockedIntent(raw)
	if err != nil {
		t.Fatalf("parse locked_intent: %v", err)
	}
	if intent.Refund == nil {
		t.Fatalf("expected refund metadata")
	}
	if intent.Refund.Source != "local_zero_builder" {
		t.Fatalf("expected source local_zero_builder, got %s", intent.Refund.Source)
	}
	if intent.Refund.RawTxHex == "" {
		t.Fatalf("expected raw tx hex persisted")
	}
	if intent.Refund.BroadcastTxID != broadcastTxID {
		t.Fatalf("expected broadcast txid %s, got %s", broadcastTxID, intent.Refund.BroadcastTxID)
	}
}

func TestReconcileRefundingRequiresOnchainEvidenceNotProviderStatus(t *testing.T) {
	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})
	w := &Watcher{db: database, engine: engine}

	created, err := engine.Create(ctx, swap.KindReverse, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET boltz_status = ?, refund_txid = ?, locked_intent = ?
		WHERE id = ?
	`, "transaction.refunded", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		`{"version":1,"quote_id":"q-refund-evidence","kind":"reverse"}`, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}

	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit_started: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("waiting: %v", err)
	}
	refundCoop, err := engine.Transition(ctx, waiting.ID, waiting.Version, swap.StateRefundCoopWaiting, "timeout_approaching", nil)
	if err != nil {
		t.Fatalf("refund_coop_waiting: %v", err)
	}
	fallbackReady, err := engine.Transition(ctx, refundCoop.ID, refundCoop.Version, swap.StateFallbackScriptReady, "coop_failed", nil)
	if err != nil {
		t.Fatalf("fallback_script_ready: %v", err)
	}
	refunding, err := engine.Transition(ctx, fallbackReady.ID, fallbackReady.Version, swap.StateRefunding, "timeout_reached", nil)
	if err != nil {
		t.Fatalf("refunding: %v", err)
	}

	if err := w.reconcileSwap(ctx, refunding); err != nil {
		t.Fatalf("reconcile swap: %v", err)
	}
	updated, err := engine.Get(ctx, refunding.ID)
	if err != nil {
		t.Fatalf("get swap: %v", err)
	}
	if updated.State != swap.StateRefunding {
		t.Fatalf("expected state to remain %s without on-chain evidence, got %s", swap.StateRefunding, updated.State)
	}
}

func TestReconcileLocalZeroBuilderRestartIdempotent(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("environment does not allow local listener: %v", err)
	}
	_ = l.Close()

	ctx := context.Background()
	database := openWatcherTestDB(t)
	engine := swap.NewEngine(database, watcherTestVault{})
	vault := watcherTestVault{}

	lockupAddress := "bcrt1q2vfxp4fmgf8kuy0a8248yk6cehwwlry2a66f9h"
	lockupScriptHex := "5120e8f32e723decf4051aefac8e2c93c9c5b214313a8fe23095ef58f6f8c04d8d83"
	refundScriptHex := "51"
	controlBlockHex := "c001"
	prevTxID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	broadcastTxID := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	var sendRawCount int32
	rpcServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")

		switch req.Method {
		case "scantxoutset":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"success":true,"height":100,"txouts":1,"unspents":[{"txid":"%s","vout":0,"address":"%s","amount":0.01,"height":99,"scriptPubKey":"%s"}]},"error":null,"id":"xscore"}`, prevTxID, lockupAddress, lockupScriptHex)))
		case "gettxout":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"value":0.01,"scriptPubKey":{"hex":"%s"}},"error":null,"id":"xscore"}`, lockupScriptHex)))
		case "estimatesmartfee":
			_, _ = w.Write([]byte(`{"result":{"feerate":0.00001000},"error":null,"id":"xscore"}`))
		case "sendrawtransaction":
			atomic.AddInt32(&sendRawCount, 1)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":"%s","error":null,"id":"xscore"}`, broadcastTxID)))
		case "getrawtransaction":
			// Reconcile completion should use objective chain evidence (mempool/confirmed).
			_, _ = w.Write([]byte(fmt.Sprintf(`{"result":{"txid":"%s","hash":"%s","size":120,"vsize":110,"confirmations":1,"hex":"00"},"error":null,"id":"xscore"}`, broadcastTxID, broadcastTxID)))
		default:
			_, _ = w.Write([]byte(`{"result":0,"error":null,"id":"xscore"}`))
		}
	}))
	rpcServer.Listener, _ = net.Listen("tcp", "127.0.0.1:0")
	rpcServer.Start()
	defer rpcServer.Close()

	created, err := engine.Create(ctx, swap.KindChain, "regtest", 0)
	if err != nil {
		t.Fatalf("create swap: %v", err)
	}
	lockedIntent := fmt.Sprintf(`{
		"version":1,
		"quote_id":"q-restart-local-zero",
		"kind":"chain",
		"lockup_address":"%s",
		"lockup_details":{"lockupAddress":"%s","timeoutBlockHeight":1,"swapTree":{"claimLeaf":{"version":192,"output":"51"},"refundLeaf":{"version":192,"output":"%s"}}},
		"claim_details":{"lockupAddress":"tlq1qqtest","timeoutBlockHeight":1,"swapTree":{"claimLeaf":{"version":196,"output":"53"},"refundLeaf":{"version":196,"output":"54"}}},
		"refund":{"template":{"control_block_hex":"%s"}}
	}`, lockupAddress, lockupAddress, refundScriptHex, controlBlockHex)
	if _, err := database.ExecContext(ctx, `
		UPDATE swaps
		SET locked_intent = ?, timeout_block_height = ?, boltz_status = ''
		WHERE id = ?
	`, lockedIntent, 1, created.ID); err != nil {
		t.Fatalf("prepare swap: %v", err)
	}

	locked, err := engine.Transition(ctx, created.ID, created.Version, swap.StateLocked, "lock", nil)
	if err != nil {
		t.Fatalf("lock: %v", err)
	}
	commitStarted, err := engine.Transition(ctx, locked.ID, locked.Version, swap.StateCommitStarted, "commit", nil)
	if err != nil {
		t.Fatalf("commit_started: %v", err)
	}
	waiting, err := engine.Transition(ctx, commitStarted.ID, commitStarted.Version, swap.StateWaiting, "provider_create", nil)
	if err != nil {
		t.Fatalf("waiting: %v", err)
	}
	refundCoop, err := engine.Transition(ctx, waiting.ID, waiting.Version, swap.StateRefundCoopWaiting, "timeout_approaching", nil)
	if err != nil {
		t.Fatalf("refund_coop_waiting: %v", err)
	}
	fallbackReady, err := engine.Transition(ctx, refundCoop.ID, refundCoop.Version, swap.StateFallbackScriptReady, "coop_failed", nil)
	if err != nil {
		t.Fatalf("fallback_script_ready: %v", err)
	}
	var timeoutCheck int64
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(timeout_block_height, 0) FROM swaps WHERE id = ?`, fallbackReady.ID).Scan(&timeoutCheck); err != nil {
		t.Fatalf("query timeout_block_height: %v", err)
	}
	if timeoutCheck <= 0 {
		t.Fatalf("expected timeout_block_height > 0, got %d", timeoutCheck)
	}

	w1 := &Watcher{
		db:         database,
		engine:     engine,
		btc:        bitcoin.NewClient(rpcServer.URL, "", ""),
		seedSource: vault,
		network:    "regtest",
	}
	w1.mu.Lock()
	w1.currentHeight = 100
	w1.mu.Unlock()
	if got := w1.CurrentHeight(); got != 100 {
		t.Fatalf("expected watcher height=100, got %d", got)
	}
	activeBefore, err := w1.listActiveSwaps(ctx)
	if err != nil {
		t.Fatalf("list active before reconcile: %v", err)
	}
	if len(activeBefore) != 1 {
		t.Fatalf("expected 1 active swap before reconcile, got %d", len(activeBefore))
	}
	if activeBefore[0].TimeoutBlock <= 0 {
		t.Fatalf("expected TimeoutBlock > 0 in active swap, got %d", activeBefore[0].TimeoutBlock)
	}
	if err := w1.ReconcileAllActiveSwaps(ctx); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	afterFirst, err := engine.Get(ctx, fallbackReady.ID)
	if err != nil {
		t.Fatalf("get after first reconcile: %v", err)
	}
	if afterFirst.State != swap.StateRefunding && afterFirst.State != swap.StateFallbackScriptReady {
		t.Fatalf("expected state %s or %s after first reconcile, got %s", swap.StateRefunding, swap.StateFallbackScriptReady, afterFirst.State)
	}

	// Depending on reconcile timing and idempotency key progression, the first pass may
	// stop at fallback_script_ready and require one extra cycle for broadcast.
	if afterFirst.State == swap.StateFallbackScriptReady {
		if err := w1.ReconcileAllActiveSwaps(ctx); err != nil {
			t.Fatalf("first reconcile (second cycle): %v", err)
		}
		afterFirst, err = engine.Get(ctx, fallbackReady.ID)
		if err != nil {
			t.Fatalf("get after first reconcile second cycle: %v", err)
		}
		if afterFirst.State != swap.StateRefunding {
			t.Fatalf("expected state %s after second cycle, got %s", swap.StateRefunding, afterFirst.State)
		}
	}
	if strings.TrimSpace(afterFirst.RefundTxid) == "" {
		t.Fatalf("expected refund_txid persisted after first reconcile")
	}
	if got := atomic.LoadInt32(&sendRawCount); got != 1 {
		t.Fatalf("expected one broadcast in first reconcile, got %d", got)
	}

	// Simulate process restart with a new watcher instance.
	w2 := &Watcher{
		db:         database,
		engine:     engine,
		btc:        bitcoin.NewClient(rpcServer.URL, "", ""),
		seedSource: vault,
		network:    "regtest",
	}
	w2.mu.Lock()
	w2.currentHeight = 100
	w2.mu.Unlock()
	if err := w2.ReconcileAllActiveSwaps(ctx); err != nil {
		t.Fatalf("second reconcile after restart: %v", err)
	}
	afterSecond, err := engine.Get(ctx, fallbackReady.ID)
	if err != nil {
		t.Fatalf("get after second reconcile: %v", err)
	}
	if afterSecond.State != swap.StateCompleted {
		t.Fatalf("expected state %s after objective on-chain evidence, got %s", swap.StateCompleted, afterSecond.State)
	}
	if got := atomic.LoadInt32(&sendRawCount); got != 1 {
		t.Fatalf("expected no duplicate broadcast after restart, got sendrawtransaction=%d", got)
	}

	var raw string
	if err := database.QueryRowContext(ctx, `SELECT COALESCE(locked_intent, '') FROM swaps WHERE id = ?`, afterSecond.ID).Scan(&raw); err != nil {
		t.Fatalf("query locked_intent: %v", err)
	}
	intent, err := swap.ParseLockedIntent(raw)
	if err != nil {
		t.Fatalf("parse locked_intent: %v", err)
	}
	if intent.Refund == nil || intent.Refund.Source != "local_zero_builder" {
		t.Fatalf("expected refund source local_zero_builder, got %+v", intent.Refund)
	}
}
