// Package bitcoin - Transaction builder for funding payouts (Taproot key-spend)
package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const (
	p2trInputVBytes  = 58.0
	p2trOutputVBytes = 43.0
	txOverheadVBytes = 10.5
	dustThresholdSat = int64(546)
)

// TxBuilder builds and signs Bitcoin transactions
// Note: This builder assumes P2TR (Taproot) key-spend inputs and outputs.
type TxBuilder struct {
	params *chaincfg.Params
}

// NewTxBuilder creates a new transaction builder
func NewTxBuilder(params *chaincfg.Params) *TxBuilder {
	return &TxBuilder{params: params}
}

// FundingTxInput represents an input for the funding transaction.
// InternalKey is the Taproot internal key (not tweaked).
type FundingTxInput struct {
	UTXO        UTXO
	InternalKey *btcec.PrivateKey
}

// OutputKey returns the Taproot output key derived from the internal key.
func (f FundingTxInput) OutputKey() (*btcec.PublicKey, error) {
	if f.InternalKey == nil {
		return nil, fmt.Errorf("missing internal key")
	}
	return txscript.ComputeTaprootKeyNoScript(f.InternalKey.PubKey()), nil
}

// BuildFundingTx builds a funding transaction for a payout
// Inputs: UTXOs to spend (P2TR key spend)
// Outputs: lockup address (P2TR), change address (P2TR if needed)
func (tb *TxBuilder) BuildFundingTx(
	inputs []FundingTxInput,
	lockupAddress string,
	lockupAmount int64,
	changeAddress string,
	feeRate float64, // sat/vB
) (*wire.MsgTx, string, error) {
	// Create transaction
	tx := wire.NewMsgTx(wire.TxVersion)

	// Add inputs
	totalInput := int64(0)
	for _, input := range inputs {
		if input.InternalKey == nil {
			return nil, "", fmt.Errorf("missing taproot internal key")
		}

		if input.UTXO.Address != "" && !IsTaprootAddress(input.UTXO.Address, tb.params) {
			return nil, "", fmt.Errorf("utxo address must be taproot (p2tr)")
		}

		hash, err := chainhash.NewHashFromStr(input.UTXO.TxID)
		if err != nil {
			return nil, "", fmt.Errorf("invalid txid: %w", err)
		}

		outPoint := wire.NewOutPoint(hash, input.UTXO.Vout)
		txIn := wire.NewTxIn(outPoint, nil, nil)
		txIn.Sequence = 0xfffffffd // Enable RBF
		tx.AddTxIn(txIn)

		totalInput += int64(input.UTXO.Amount * 1e8) // Convert BTC to sat
	}

	// Add lockup output (P2TR)
	lockupScript, err := TaprootScriptPubKey(lockupAddress, tb.params)
	if err != nil {
		return nil, "", fmt.Errorf("invalid lockup address: %w", err)
	}
	tx.AddTxOut(wire.NewTxOut(lockupAmount, lockupScript))

	// Estimate size for fee calculation
	estimatedVBytes := txOverheadVBytes + float64(len(inputs))*p2trInputVBytes + p2trOutputVBytes

	// Calculate fee
	fee := int64(estimatedVBytes * feeRate)

	// Calculate change
	change := totalInput - lockupAmount - fee

	// Add change output if significant (> dust threshold)
	if change > dustThresholdSat {
		changeScript, err := TaprootScriptPubKey(changeAddress, tb.params)
		if err != nil {
			return nil, "", fmt.Errorf("invalid change address: %w", err)
		}
		tx.AddTxOut(wire.NewTxOut(change, changeScript))

		// Recalculate fee with change output
		estimatedVBytes += p2trOutputVBytes
		fee = int64(estimatedVBytes * feeRate)
		change = totalInput - lockupAmount - fee

		// Update change amount
		tx.TxOut[1].Value = change
	} else if change < 0 {
		return nil, "", fmt.Errorf("insufficient funds: need %d sat, have %d sat", lockupAmount+fee, totalInput)
	}

	// Sign inputs (P2TR key spend)
	for i, input := range inputs {
		prevOutScript, err := createTaprootScript(input.InternalKey, tb.params)
		if err != nil {
			return nil, "", err
		}
		inputValue := int64(input.UTXO.Amount * 1e8)

		prevOutFetcher := txscript.NewCannedPrevOutputFetcher(prevOutScript, inputValue)
		sigHashes := txscript.NewTxSigHashes(tx, prevOutFetcher)

		sigHash, err := txscript.CalcTaprootSignatureHash(
			sigHashes,
			txscript.SigHashDefault,
			tx,
			i,
			prevOutFetcher,
		)
		if err != nil {
			return nil, "", fmt.Errorf("failed to calc taproot sighash: %w", err)
		}

		tweakedKey := txscript.TweakTaprootPrivKey(*input.InternalKey, nil)
		sig, err := schnorr.Sign(tweakedKey, sigHash)
		if err != nil {
			return nil, "", fmt.Errorf("failed to sign input %d: %w", i, err)
		}

		sigBytes := sig.Serialize()
		// SigHashDefault does not append sighash type
		tx.TxIn[i].Witness = wire.TxWitness{sigBytes}
	}

	// Serialize to hex
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return nil, "", err
	}
	rawHex := hex.EncodeToString(buf.Bytes())

	return tx, rawHex, nil
}

// SelectUTXOs selects UTXOs for a transaction using a simple greedy algorithm
func SelectUTXOs(utxos []UTXO, targetAmount int64, feeRate float64) ([]UTXO, error) {
	selected := []UTXO{}
	total := int64(0)

	for _, utxo := range utxos {
		if utxo.Spendable {
			selected = append(selected, utxo)
			total += int64(utxo.Amount * 1e8)

			// Estimate fee (lockup + change outputs)
			estimatedVBytes := txOverheadVBytes + float64(len(selected))*p2trInputVBytes + 2*p2trOutputVBytes
			fee := int64(estimatedVBytes * feeRate)

			if total >= targetAmount+fee {
				return selected, nil
			}
		}
	}

	return nil, fmt.Errorf("insufficient funds: need %d sat, have %d sat", targetAmount, total)
}

func createTaprootScript(internalKey *btcec.PrivateKey, params *chaincfg.Params) ([]byte, error) {
	if internalKey == nil {
		return nil, fmt.Errorf("missing internal key")
	}

	outputKey := txscript.ComputeTaprootKeyNoScript(internalKey.PubKey())
	addr, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(outputKey), params)
	if err != nil {
		return nil, err
	}
	return txscript.PayToAddrScript(addr)
}
