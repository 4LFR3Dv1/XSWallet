// Package boltz - Testes para types.go
package boltz

import (
	"encoding/json"
	"testing"
)

func TestMinerFeesAny_UnmarshalNumber(t *testing.T) {
	jsonData := `{"percentage": 0.1, "minerFees": 4379}`

	var fees PairFees
	if err := json.Unmarshal([]byte(jsonData), &fees); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if fees.MinerFees.Flat == nil {
		t.Fatal("expected Flat to be set")
	}

	if *fees.MinerFees.Flat != 4379 {
		t.Errorf("expected 4379, got %d", *fees.MinerFees.Flat)
	}

	if fees.MinerFees.Total() != 4379 {
		t.Errorf("expected Total() = 4379, got %d", fees.MinerFees.Total())
	}
}

func TestMinerFeesAny_UnmarshalObject(t *testing.T) {
	jsonData := `{"percentage": 0.5, "minerFees": {"lockup": 2772, "claim": 1998}}`

	var fees PairFees
	if err := json.Unmarshal([]byte(jsonData), &fees); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if fees.MinerFees.Detail == nil {
		t.Fatal("expected Detail to be set")
	}

	if fees.MinerFees.Detail.Lockup != 2772 {
		t.Errorf("expected lockup 2772, got %d", fees.MinerFees.Detail.Lockup)
	}

	if fees.MinerFees.Detail.Claim != 1998 {
		t.Errorf("expected claim 1998, got %d", fees.MinerFees.Detail.Claim)
	}

	if fees.MinerFees.Total() != 4770 {
		t.Errorf("expected Total() = 4770, got %d", fees.MinerFees.Total())
	}
}

func TestMinerFeesAny_Marshal(t *testing.T) {
	// Test marshal number
	flat := int64(4379)
	feesFlat := MinerFeesAny{Flat: &flat}
	data, err := json.Marshal(feesFlat)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if string(data) != "4379" {
		t.Errorf("expected '4379', got '%s'", string(data))
	}

	// Test marshal object
	feesDetail := MinerFeesAny{Detail: &MinerFees{Lockup: 2772, Claim: 1998}}
	data, err = json.Marshal(feesDetail)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	expected := `{"lockup":2772,"claim":1998}`
	if string(data) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(data))
	}
}

func TestPairInfo_Parse(t *testing.T) {
	// Real response from Boltz API
	jsonData := `{
		"BTC": {
			"BTC": {
				"hash": "90ab5c8e6ece5db52173e9423a0dd3071f5894dc8d35ed592a439ccabcdebbd5",
				"rate": 1,
				"limits": {"maximal": 25000000, "minimal": 50000, "maximalZeroConf": 0},
				"fees": {"percentage": 0.1, "minerFees": 4379}
			}
		},
		"L-BTC": {
			"BTC": {
				"hash": "b53c0ac3da051a78f67f6dd25f2ab0858492dc6881015b236d554227c85fda7d",
				"rate": 1,
				"limits": {"maximal": 25000000, "minimal": 1000, "maximalZeroConf": 100000},
				"fees": {"percentage": 0.1, "minerFees": 148}
			}
		}
	}`

	var pairs map[string]map[string]PairInfo
	if err := json.Unmarshal([]byte(jsonData), &pairs); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	btcPair, ok := pairs["BTC"]["BTC"]
	if !ok {
		t.Fatal("expected BTC->BTC pair")
	}

	if btcPair.Hash != "90ab5c8e6ece5db52173e9423a0dd3071f5894dc8d35ed592a439ccabcdebbd5" {
		t.Errorf("unexpected hash: %s", btcPair.Hash)
	}

	if btcPair.Limits.Minimal != 50000 {
		t.Errorf("expected minimal 50000, got %d", btcPair.Limits.Minimal)
	}

	if btcPair.Fees.MinerFees.Total() != 4379 {
		t.Errorf("expected minerFees 4379, got %d", btcPair.Fees.MinerFees.Total())
	}
}

func TestWSSubscribeRequest_Marshal(t *testing.T) {
	req := WSSubscribeRequest{
		Op:      "subscribe",
		Channel: "swap.update",
		Args:    []string{"abc123", "def456"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	expected := `{"op":"subscribe","channel":"swap.update","args":["abc123","def456"]}`
	if string(data) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(data))
	}
}

func TestWSMessage_Parse(t *testing.T) {
	jsonData := `{
		"event": "update",
		"channel": "swap.update",
		"args": [
			{"id": "abc123", "status": "transaction.mempool"},
			{"id": "def456", "status": "invoice.paid", "transaction": {"id": "tx123"}}
		]
	}`

	var msg WSMessage
	if err := json.Unmarshal([]byte(jsonData), &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if msg.Event != "update" {
		t.Errorf("expected event 'update', got '%s'", msg.Event)
	}

	if msg.Channel != "swap.update" {
		t.Errorf("expected channel 'swap.update', got '%s'", msg.Channel)
	}

	if len(msg.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(msg.Args))
	}

	if msg.Args[0].ID != "abc123" {
		t.Errorf("expected id 'abc123', got '%s'", msg.Args[0].ID)
	}

	if msg.Args[1].Transaction == nil {
		t.Fatal("expected transaction in second arg")
	}

	if msg.Args[1].Transaction.ID != "tx123" {
		t.Errorf("expected tx id 'tx123', got '%s'", msg.Args[1].Transaction.ID)
	}
}
