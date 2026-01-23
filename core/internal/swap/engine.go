// Package swap - State machine e lógica de swap
// Go Core é autoritativo para todas as transições de estado.
package swap

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xs-wallet/xscore/internal/db"
)

// State representa o estado do swap
type State string

const (
	StateOpen                     State = "open"
	StateLocked                   State = "locked"
	StateCommitStarted            State = "commit_started"
	StateWaiting                  State = "waiting"
	StateWaitingClaimDetails      State = "waiting_claim_details"
	StateSigningMusig2Partial     State = "signing_musig2_partial"
	StateSentPartialToProvider    State = "sent_partial_to_provider"
	StateWaitingProviderBroadcast State = "waiting_provider_broadcast"
	StateRefundCoopWaiting        State = "refund_coop_waiting"
	StateFallbackScriptReady      State = "fallback_script_ready"
	StateRefunding                State = "refunding"
	StateCompleted                State = "completed"
	StateFailed                   State = "failed"
	StateCanceled                 State = "canceled"
)

// Kind tipo de swap
type Kind string

const (
	KindSubmarine Kind = "submarine"
	KindReverse   Kind = "reverse"
	KindChain     Kind = "chain"
)

// Swap representa um swap
type Swap struct {
	ID           string
	Kind         Kind
	Env          string
	Version      int64
	State        State
	LockedIntent string
	SwapKeyIndex int64
	// PreimageHex removed - use encrypted_preimage in DB
	PreimageHashHex string
	ClaimPubkeyHex  string
	RefundPubkeyHex string
	BoltzID         string
	LockupTxid      string
	LockupAmountSat string
	ClaimTxid       string
	RefundTxid      string
	TimeoutBlock    int64
	ErrorMessage    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Engine gerencia swaps
type Engine struct {
	db    *db.DB
	vault interface {
		EncryptPreimage([]byte) ([]byte, error)
		DecryptPreimage([]byte) ([]byte, error)
	}
}

// NewEngine cria engine
func NewEngine(database *db.DB, v interface {
	EncryptPreimage([]byte) ([]byte, error)
	DecryptPreimage([]byte) ([]byte, error)
}) *Engine {
	return &Engine{db: database, vault: v}
}

// ValidTransitions define transições válidas da state machine
var ValidTransitions = map[State][]State{
	StateOpen:                     {StateLocked, StateCanceled},
	StateLocked:                   {StateCommitStarted, StateCanceled},
	StateCommitStarted:            {StateWaiting, StateFailed},
	StateWaiting:                  {StateWaitingClaimDetails, StateRefundCoopWaiting},
	StateWaitingClaimDetails:      {StateSigningMusig2Partial},
	StateSigningMusig2Partial:     {StateSentPartialToProvider},
	StateSentPartialToProvider:    {StateWaitingProviderBroadcast},
	StateWaitingProviderBroadcast: {StateCompleted, StateFallbackScriptReady},
	StateRefundCoopWaiting:        {StateCompleted, StateFallbackScriptReady},
	StateFallbackScriptReady:      {StateRefunding, StateCompleted},
	StateRefunding:                {StateCompleted, StateFailed},
	// Terminal states
	StateCompleted: {},
	StateFailed:    {},
	StateCanceled:  {},
}

// ErrConcurrentModification erro de CAS
var ErrConcurrentModification = errors.New("concurrent modification detected")

// ErrInvalidTransition erro de transição inválida
var ErrInvalidTransition = errors.New("invalid state transition")

// ErrVaultLocked indicates vault is required but not available
var ErrVaultLocked = errors.New("vault is locked")

// Create cria um novo swap
func (e *Engine) Create(ctx context.Context, kind Kind, env string, keyIndex int64) (*Swap, error) {
	id := uuid.New().String()

	// Gerar preimage ALEATÓRIA (CRÍTICO: NUNCA derivar de private key)
	preimage := make([]byte, 32)
	if _, err := rand.Read(preimage); err != nil {
		return nil, fmt.Errorf("failed to generate preimage: %w", err)
	}
	hash := sha256.Sum256(preimage)
	preimageHashHex := hex.EncodeToString(hash[:])

	// Encrypt preimage before storing
	if e.vault == nil {
		return nil, fmt.Errorf("vault is required to encrypt preimage")
	}
	encryptedPreimage, err := e.vault.EncryptPreimage(preimage)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt preimage: %w", err)
	}

	now := time.Now().UTC()

	swap := &Swap{
		ID:              id,
		Kind:            kind,
		Env:             env,
		Version:         0,
		State:           StateOpen,
		SwapKeyIndex:    keyIndex,
		PreimageHashHex: preimageHashHex,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// Insert
	_, err = e.db.ExecContext(ctx, `
		INSERT INTO swaps (id, kind, env, version, state, swap_key_index, encrypted_preimage, preimage_hash_hex, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, swap.ID, swap.Kind, swap.Env, swap.Version, swap.State, swap.SwapKeyIndex, encryptedPreimage, swap.PreimageHashHex, swap.CreatedAt.Format(time.RFC3339Nano), swap.UpdatedAt.Format(time.RFC3339Nano))

	if err != nil {
		return nil, fmt.Errorf("failed to insert swap: %w", err)
	}

	// Log event
	if err := e.logEvent(ctx, id, "", string(StateOpen), "create", nil); err != nil {
		return nil, err
	}

	return swap, nil
}

// Transition executa transição de estado com CAS
func (e *Engine) Transition(ctx context.Context, id string, expectedVersion int64, newState State, trigger string, details map[string]interface{}) (*Swap, error) {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get current state
	var currentState State
	var currentVersion int64
	err = tx.QueryRowContext(ctx, "SELECT state, version FROM swaps WHERE id = ?", id).Scan(&currentState, &currentVersion)
	if err != nil {
		return nil, fmt.Errorf("swap not found: %w", err)
	}

	// CAS check
	if currentVersion != expectedVersion {
		return nil, ErrConcurrentModification
	}

	// Validate transition
	validNextStates := ValidTransitions[currentState]
	valid := false
	for _, s := range validNextStates {
		if s == newState {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, currentState, newState)
	}

	// Update with CAS
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		UPDATE swaps 
		SET state = ?, version = version + 1, updated_at = ?
		WHERE id = ? AND version = ?
	`, newState, now, id, expectedVersion)
	if err != nil {
		return nil, err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return nil, ErrConcurrentModification
	}

	// Log event
	if err := e.logEventTx(ctx, tx, id, string(currentState), string(newState), trigger, details); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return e.Get(ctx, id)
}

// Get obtém swap por ID
func (e *Engine) Get(ctx context.Context, id string) (*Swap, error) {
	swap := &Swap{}
	var createdAt, updatedAt string

	err := e.db.QueryRowContext(ctx, `
		SELECT id, kind, env, version, state, swap_key_index, 
		       COALESCE(preimage_hash_hex, ''),
		       COALESCE(lockup_txid, ''), COALESCE(lockup_amount_sat, ''),
		       COALESCE(error_message, ''),
		       created_at, updated_at
		FROM swaps WHERE id = ?
	`, id).Scan(
		&swap.ID, &swap.Kind, &swap.Env, &swap.Version, &swap.State, &swap.SwapKeyIndex,
		&swap.PreimageHashHex,
		&swap.LockupTxid, &swap.LockupAmountSat,
		&swap.ErrorMessage,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	swap.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	swap.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)

	return swap, nil
}

// GetPreimage returns decrypted preimage and performs lazy migration for legacy plaintext
func (e *Engine) GetPreimage(ctx context.Context, id string) ([]byte, error) {
	if e.vault == nil {
		return nil, ErrVaultLocked
	}

	var encrypted []byte
	err := e.db.QueryRowContext(ctx, `
		SELECT COALESCE(encrypted_preimage, X'')
		FROM swaps WHERE id = ?
	`, id).Scan(&encrypted)
	if err != nil {
		return nil, err
	}

	if len(encrypted) > 0 {
		return e.vault.DecryptPreimage(encrypted)
	}

	// Legacy migration path: use preimage_hex if column exists
	hasLegacy, err := e.columnExists("swaps", "preimage_hex")
	if err != nil || !hasLegacy {
		return nil, errors.New("preimage not found")
	}

	var legacyHex sql.NullString
	err = e.db.QueryRowContext(ctx, `
		SELECT preimage_hex FROM swaps WHERE id = ?
	`, id).Scan(&legacyHex)
	if err != nil {
		return nil, err
	}
	if !legacyHex.Valid || legacyHex.String == "" {
		return nil, errors.New("preimage not found")
	}

	legacyBytes, err := hex.DecodeString(legacyHex.String)
	if err != nil {
		return nil, fmt.Errorf("invalid legacy preimage: %w", err)
	}

	encrypted, err = e.vault.EncryptPreimage(legacyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt legacy preimage: %w", err)
	}

	_, err = e.db.ExecContext(ctx, `
		UPDATE swaps SET encrypted_preimage = ?, preimage_hex = NULL WHERE id = ?
	`, encrypted, id)
	if err != nil {
		return nil, err
	}

	return legacyBytes, nil
}

func (e *Engine) columnExists(table, column string) (bool, error) {
	var count int
	err := e.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?
	`, table, column).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// logEvent registra evento de swap
func (e *Engine) logEvent(ctx context.Context, swapID, fromState, toState, trigger string, details map[string]interface{}) error {
	_, err := e.db.ExecContext(ctx, `
		INSERT INTO swap_events (swap_id, from_state, to_state, trigger, details)
		VALUES (?, ?, ?, ?, ?)
	`, swapID, fromState, toState, trigger, nil)
	return err
}

func (e *Engine) logEventTx(ctx context.Context, tx *sql.Tx, swapID, fromState, toState, trigger string, details map[string]interface{}) error {
	// Simplificado - em produção serializar details para JSON
	_, err := tx.ExecContext(ctx, `
		INSERT INTO swap_events (swap_id, from_state, to_state, trigger, details)
		VALUES (?, ?, ?, ?, ?)
	`, swapID, fromState, toState, trigger, nil)
	return err
}

// CheckIdempotency verifica se operação já foi executada
func (e *Engine) CheckIdempotency(ctx context.Context, swapID, opKey string) (bool, string, error) {
	var result string
	err := e.db.QueryRowContext(ctx, `
		SELECT result FROM swap_ops WHERE swap_id = ? AND op_key = ?
	`, swapID, opKey).Scan(&result)

	if err != nil {
		return false, "", nil // Não existe = não executada
	}
	return true, result, nil
}

// RecordOperation registra operação idempotente
func (e *Engine) RecordOperation(ctx context.Context, swapID, opKey, result string) error {
	_, err := e.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO swap_ops (swap_id, op_key, result)
		VALUES (?, ?, ?)
	`, swapID, opKey, result)
	return err
}
