// Package bitcoin - Transaction builder for funding swaps
package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

// TxBuilder builds and signs Bitcoin transactions
type TxBuilder struct {
	params *chaincfg.Params
}

// NewTxBuilder creates a new transaction builder
func NewTxBuilder(params *chaincfg.Params) *TxBuilder {
	return &TxBuilder{params: params}
}

// FundingTxInput represents an input for the funding transaction
type FundingTxInput struct {
	UTXO       UTXO
	PrivateKey *btcec.PrivateKey
	PubKey     *btcec.PublicKey
}

// BuildFundingTx builds a funding transaction for a swap
// Inputs: UTXOs to spend (P2WPKH)
// Outputs: lockup address (swap), change address (if needed)
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

	// Add lockup output
	lockupAddr, err := btcutil.DecodeAddress(lockupAddress, tb.params)
	if err != nil {
		return nil, "", fmt.Errorf("invalid lockup address: %w", err)
	}
	lockupScript, err := txscript.PayToAddrScript(lockupAddr)
	if err != nil {
		return nil, "", err
	}
	tx.AddTxOut(wire.NewTxOut(lockupAmount, lockupScript))

	// Estimate size for fee calculation
	// P2WPKH input: ~68 vbytes each
	// P2WPKH output: ~31 vbytes each
	// Overhead: ~10.5 vbytes
	estimatedVBytes := 10.5 + float64(len(inputs))*68 + 31 // lockup output

	// Calculate fee
	fee := int64(estimatedVBytes * feeRate)

	// Calculate change
	change := totalInput - lockupAmount - fee

	// Add change output if significant (> dust threshold)
	dustThreshold := int64(546) // Standard dust threshold
	if change > dustThreshold {
		changeAddr, err := btcutil.DecodeAddress(changeAddress, tb.params)
		if err != nil {
			return nil, "", fmt.Errorf("invalid change address: %w", err)
		}
		changeScript, err := txscript.PayToAddrScript(changeAddr)
		if err != nil {
			return nil, "", err
		}
		tx.AddTxOut(wire.NewTxOut(change, changeScript))

		// Recalculate fee with change output
		estimatedVBytes += 31
		fee = int64(estimatedVBytes * feeRate)
		change = totalInput - lockupAmount - fee

		// Update change amount
		tx.TxOut[1].Value = change
	} else if change < 0 {
		return nil, "", fmt.Errorf("insufficient funds: need %d sat, have %d sat", lockupAmount+fee, totalInput)
	}

	// Sign inputs (P2WPKH)
	for i, input := range inputs {
		// Get the previous output script for P2WPKH
		prevOutScript := createWitnessScript(input.PubKey, tb.params)
		inputValue := int64(input.UTXO.Amount * 1e8)

		// Create signature hash
		sigHashes := txscript.NewTxSigHashes(tx, txscript.NewCannedPrevOutputFetcher(
			prevOutScript,
			inputValue,
		))

		// Create the signature
		sig, err := txscript.RawTxInWitnessSignature(
			tx,
			sigHashes,
			i,
			inputValue,
			prevOutScript,
			txscript.SigHashAll,
			input.PrivateKey,
		)
		if err != nil {
			return nil, "", fmt.Errorf("failed to sign input %d: %w", i, err)
		}

		// Build witness stack: [signature, pubkey]
		tx.TxIn[i].Witness = wire.TxWitness{
			sig,
			input.PubKey.SerializeCompressed(),
		}
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
	// Sort by amount descending (simple strategy)
	// In production, use more sophisticated coin selection
	selected := []UTXO{}
	total := int64(0)

	for _, utxo := range utxos {
		if utxo.Spendable {
			selected = append(selected, utxo)
			total += int64(utxo.Amount * 1e8)

			// Estimate fee
			estimatedVBytes := 10.5 + float64(len(selected))*68 + 31*2 // lockup + change
			fee := int64(estimatedVBytes * feeRate)

			if total >= targetAmount+fee {
				return selected, nil
			}
		}
	}

	return nil, fmt.Errorf("insufficient funds: need %d sat, have %d sat", targetAmount, total)
}

// Helper function

func createWitnessScript(pubKey *btcec.PublicKey, params *chaincfg.Params) []byte {
	// P2WPKH witness script
	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	addr, _ := btcutil.NewAddressWitnessPubKeyHash(pubKeyHash, params)
	script, _ := txscript.PayToAddrScript(addr)
	return script
}
