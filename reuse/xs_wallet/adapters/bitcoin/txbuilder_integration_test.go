//go:build integration

package bitcoin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
)

func TestBuildTaprootFundingTx_Regtest(t *testing.T) {
	rpcURL := os.Getenv("BTC_RPC_URL")
	rpcUser := os.Getenv("BTC_RPC_USER")
	rpcPass := os.Getenv("BTC_RPC_PASS")
	network := os.Getenv("BTC_NETWORK")

	if rpcURL == "" || rpcUser == "" || rpcPass == "" {
		t.Skip("set BTC_RPC_URL, BTC_RPC_USER, BTC_RPC_PASS to run")
	}
	if network != "" && network != "regtest" {
		t.Skip("integration test requires regtest")
	}

	logPath := os.Getenv("TEST_LOG_PATH")
	if logPath == "" {
		logPath = filepath.Join("test_logs", "txbuilder_regtest.log")
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatalf("create log dir failed: %v", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open log file failed: %v", err)
	}
	defer logFile.Close()
	logger := log.New(logFile, "", log.LstdFlags)
	logLine := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		logger.Println(msg)
		t.Log(msg)
	}
	logLine("start TestBuildTaprootFundingTx_Regtest rpc=%s network=%s", rpcURL, network)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	baseClient := NewClient(rpcURL, rpcUser, rpcPass)
	heightPre, err := baseClient.Height(ctx)
	if err != nil {
		t.Fatalf("height pre failed: %v", err)
	}
	logLine("height_pre=%d", heightPre)

	internalKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("new private key failed: %v", err)
	}
	receiveAddr, err := taprootAddress(internalKey, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("taproot address failed: %v", err)
	}
	logLine("receive_addr=%s", receiveAddr)

	utxo, err := mineToAddressAndGetUTXO(ctx, baseClient, receiveAddr)
	if err != nil {
		t.Fatalf("mine utxo failed: %v", err)
	}
	logLine("utxo txid=%s vout=%d amount_btc=%f", utxo.TxID, utxo.Vout, utxo.Amount)

	input := FundingTxInput{
		UTXO:        utxo,
		InternalKey: internalKey,
	}

	lockupKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("new lockup key failed: %v", err)
	}
	lockupAddr, err := taprootAddress(lockupKey, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("lockup address failed: %v", err)
	}
	logLine("lockup_addr=%s", lockupAddr)

	changeKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("new change key failed: %v", err)
	}
	changeAddr, err := taprootAddress(changeKey, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("change address failed: %v", err)
	}
	logLine("change_addr=%s", changeAddr)

	inputValue := int64(utxo.Amount * 1e8)
	if inputValue <= 2000 {
		t.Fatalf("insufficient input value: %d sat", inputValue)
	}
	lockupAmount := inputValue / 2

	tb := NewTxBuilder(&chaincfg.RegressionNetParams)
	msgTx, rawHex, err := tb.BuildFundingTx([]FundingTxInput{input}, lockupAddr, lockupAmount, changeAddr, 1.0)
	if err != nil {
		t.Fatalf("build tx failed: %v", err)
	}
	logLine("raw_tx_len=%d", len(rawHex))
	if len(msgTx.TxOut) == 0 || msgTx.TxOut[0].Value != lockupAmount {
		t.Fatalf("unexpected lockup output value: got %d want %d", msgTx.TxOut[0].Value, lockupAmount)
	}

	txid, err := baseClient.BroadcastTx(ctx, rawHex)
	if err != nil {
		t.Fatalf("broadcast failed: %v", err)
	}
	if txid == "" {
		t.Fatalf("empty txid")
	}
	logLine("broadcast_txid=%s", txid)

	mempool, err := getRawMempool(ctx, baseClient)
	if err != nil {
		t.Fatalf("getrawmempool failed: %v", err)
	}
	found := false
	for _, id := range mempool {
		if id == txid {
			found = true
			break
		}
	}
	logLine("mempool_contains=%t size=%d", found, len(mempool))
	if !found {
		t.Fatalf("tx not found in mempool")
	}

	decoded, err := decodeRawTx(ctx, baseClient, rawHex)
	if err != nil {
		t.Fatalf("decode raw tx failed: %v", err)
	}
	lockupVout, ok := findVoutByAddress(decoded.Vout, lockupAddr)
	if !ok {
		t.Fatalf("lockup output not found for address=%s", lockupAddr)
	}
	gotLockup := int64(math.Round(lockupVout.Value * 1e8))
	if gotLockup != lockupAmount {
		t.Fatalf("lockup output value mismatch: got %d want %d", gotLockup, lockupAmount)
	}
	if len(msgTx.TxOut) > 1 {
		if _, ok := findVoutByAddress(decoded.Vout, changeAddr); !ok {
			t.Fatalf("change output not found for address=%s", changeAddr)
		}
	}

	// Mine a block to confirm the spend and verify it is accepted by bitcoind.
	if _, err := baseClient.GenerateToAddress(ctx, 1, receiveAddr); err != nil {
		t.Fatalf("mine confirm block failed: %v", err)
	}
	heightPost, err := baseClient.Height(ctx)
	if err != nil {
		t.Fatalf("height post failed: %v", err)
	}
	logLine("height_post=%d", heightPost)
	if heightPost <= heightPre {
		t.Fatalf("height did not advance: pre=%d post=%d", heightPre, heightPost)
	}
	confirmedTx, err := baseClient.GetTx(ctx, txid)
	if err != nil {
		t.Fatalf("get tx failed: %v", err)
	}
	if confirmedTx.Confirmations < 1 {
		t.Fatalf("tx not confirmed: confirmations=%d", confirmedTx.Confirmations)
	}
	logLine("confirmed txid=%s confirmations=%d", txid, confirmedTx.Confirmations)
	logLine("PASS TestBuildTaprootFundingTx_Regtest")
}

func taprootAddress(key *btcec.PrivateKey, params *chaincfg.Params) (string, error) {
	outputKey := txscript.ComputeTaprootKeyNoScript(key.PubKey())
	addr, err := btcutil.NewAddressTaproot(schnorr.SerializePubKey(outputKey), params)
	if err != nil {
		return "", err
	}
	return addr.EncodeAddress(), nil
}

type blockVout struct {
	Value        float64 `json:"value"`
	N            uint32  `json:"n"`
	ScriptPubKey struct {
		Address   string   `json:"address"`
		Addresses []string `json:"addresses"`
		Hex       string   `json:"hex"`
	} `json:"scriptPubKey"`
}

type blockTx struct {
	TxID string      `json:"txid"`
	Vout []blockVout `json:"vout"`
}

type blockResp struct {
	Tx []blockTx `json:"tx"`
}

type decodedVout struct {
	Value        float64 `json:"value"`
	N            uint32  `json:"n"`
	ScriptPubKey struct {
		Address   string   `json:"address"`
		Addresses []string `json:"addresses"`
	} `json:"scriptPubKey"`
}

type decodedTx struct {
	Vout []decodedVout `json:"vout"`
}

func mineToAddressAndGetUTXO(ctx context.Context, client *Client, address string) (UTXO, error) {
	hashes, err := client.GenerateToAddress(ctx, 101, address)
	if err != nil {
		return UTXO{}, err
	}
	if len(hashes) == 0 {
		return UTXO{}, fmt.Errorf("no blocks generated")
	}

	// First block's coinbase is now mature after 101 blocks.
	result, err := client.call(ctx, "getblock", hashes[0], 2)
	if err != nil {
		return UTXO{}, err
	}

	var block blockResp
	if err := json.Unmarshal(result, &block); err != nil {
		return UTXO{}, err
	}
	if len(block.Tx) == 0 {
		return UTXO{}, fmt.Errorf("empty block tx list")
	}

	coinbase := block.Tx[0]
	for _, vout := range coinbase.Vout {
		addr := vout.ScriptPubKey.Address
		if addr == "" && len(vout.ScriptPubKey.Addresses) > 0 {
			addr = vout.ScriptPubKey.Addresses[0]
		}
		if addr == address {
			return UTXO{
				TxID:        coinbase.TxID,
				Vout:        vout.N,
				Address:     address,
				Amount:      vout.Value,
				Confirmations: 101,
				Spendable:   true,
				ScriptPubKey: vout.ScriptPubKey.Hex,
			}, nil
		}
	}

	return UTXO{}, fmt.Errorf("utxo not found in coinbase")
}

func getRawMempool(ctx context.Context, client *Client) ([]string, error) {
	result, err := client.call(ctx, "getrawmempool")
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(result, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func decodeRawTx(ctx context.Context, client *Client, rawHex string) (*decodedTx, error) {
	result, err := client.call(ctx, "decoderawtransaction", rawHex)
	if err != nil {
		return nil, err
	}
	var tx decodedTx
	if err := json.Unmarshal(result, &tx); err != nil {
		return nil, err
	}
	return &tx, nil
}

func findVoutByAddress(vouts []decodedVout, address string) (decodedVout, bool) {
	for _, vout := range vouts {
		if vout.ScriptPubKey.Address == address {
			return vout, true
		}
		for _, addr := range vout.ScriptPubKey.Addresses {
			if addr == address {
				return vout, true
			}
		}
	}
	return decodedVout{}, false
}
