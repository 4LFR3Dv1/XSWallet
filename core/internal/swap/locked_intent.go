package swap

import (
	"encoding/json"
	"fmt"
)

// LockedIntentVersion is the current serialized locked_intent schema version.
const LockedIntentVersion = 1

// LockedIntent captures immutable swap creation/execution data persisted in swaps.locked_intent.
type LockedIntent struct {
	Version        int                 `json:"version,omitempty"`
	QuoteID        string              `json:"quote_id"`
	Kind           string              `json:"kind"`
	FromChain      string              `json:"from_chain"`
	ToChain        string              `json:"to_chain"`
	AmountSat      int64               `json:"amount_sat"`
	Invoice        string              `json:"invoice,omitempty"`
	PayoutAddress  string              `json:"payout_address,omitempty"`
	LockupAddress  string              `json:"lockup_address,omitempty"`
	ClaimAddress   string              `json:"claim_address,omitempty"`
	TimeoutBlocks  int64               `json:"timeout_blocks,omitempty"`
	BoltzID        string              `json:"boltz_id,omitempty"`
	ReverseDetails json.RawMessage     `json:"reverse_details,omitempty"`
	LockupDetails  json.RawMessage     `json:"lockup_details,omitempty"`
	ClaimDetails   json.RawMessage     `json:"claim_details,omitempty"`
	MuSig          *LockedIntentMuSig  `json:"musig,omitempty"`
	Refund         *LockedIntentRefund `json:"refund,omitempty"`
}

// LockedIntentMuSig stores resumable MuSig2 execution metadata.
type LockedIntentMuSig struct {
	SessionID     string `json:"session_id,omitempty"`
	LocalPubNonce string `json:"local_pubnonce,omitempty"`
	AggNonce      string `json:"agg_nonce,omitempty"`
	PartialSig    string `json:"partial_sig,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// LockedIntentRefund stores resumable fallback refund execution metadata.
type LockedIntentRefund struct {
	Strategy      string                      `json:"strategy,omitempty"`
	Source        string                      `json:"source,omitempty"`
	ProviderTxID  string                      `json:"provider_txid,omitempty"`
	BroadcastTxID string                      `json:"broadcast_txid,omitempty"`
	RawTxHex      string                      `json:"raw_tx_hex,omitempty"`
	BroadcastAt   string                      `json:"broadcast_at,omitempty"`
	Template      *LockedIntentRefundTemplate `json:"template,omitempty"`
}

// LockedIntentRefundTemplate stores minimal data to rebuild/sign refund tx locally.
type LockedIntentRefundTemplate struct {
	TxVersion         int32    `json:"tx_version,omitempty"`
	LockTime          uint32   `json:"lock_time,omitempty"`
	PrevTxID          string   `json:"prev_txid,omitempty"`
	PrevVout          uint32   `json:"prev_vout,omitempty"`
	PrevValueSat      int64    `json:"prev_value_sat,omitempty"`
	PrevPkScriptHex   string   `json:"prev_pkscript_hex,omitempty"`
	Sequence          uint32   `json:"sequence,omitempty"`
	OutputValueSat    int64    `json:"output_value_sat,omitempty"`
	OutputPkScriptHex string   `json:"output_pkscript_hex,omitempty"`
	RefundLeafVersion int      `json:"refund_leaf_version,omitempty"`
	RefundScriptHex   string   `json:"refund_script_hex,omitempty"`
	ControlBlockHex   string   `json:"control_block_hex,omitempty"`
	WitnessArgsHex    []string `json:"witness_args_hex,omitempty"`
}

// ParseLockedIntent parses and validates a locked intent payload.
func ParseLockedIntent(raw string) (LockedIntent, error) {
	var payload LockedIntent
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return LockedIntent{}, fmt.Errorf("invalid locked_intent JSON: %w", err)
	}
	if payload.Version == 0 {
		payload.Version = LockedIntentVersion
	}
	return payload, nil
}

// ToJSON normalizes and serializes the payload to persist in DB.
func (p LockedIntent) ToJSON() ([]byte, error) {
	if p.Version == 0 {
		p.Version = LockedIntentVersion
	}
	return json.Marshal(p)
}

// HasChainExecutionDetails reports if both lockup_details and claim_details are present.
func (p LockedIntent) HasChainExecutionDetails() bool {
	return len(p.LockupDetails) > 0 &&
		string(p.LockupDetails) != "null" &&
		len(p.ClaimDetails) > 0 &&
		string(p.ClaimDetails) != "null"
}
