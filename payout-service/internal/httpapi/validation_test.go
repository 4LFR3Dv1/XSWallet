package httpapi

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/google/uuid"
)

func TestValidatePayoutReq_InvalidPaymentID(t *testing.T) {
	s := &Server{btcParams: &chaincfg.RegressionNetParams}
	req := payoutReq{
		PaymentID:   "not-a-uuid",
		Network:     "btc",
		Asset:       "BTC",
		AmountSats:  1000,
		Destination: "bcrt1pdhc9lfdma6kzeuz8rd8cc9z2u94yxwt3tkh653qt4unn8yvrhweqxysz44",
		Priority:    "normal",
	}
	err := s.validatePayoutReq(context.Background(), req)
	if err == nil || err.Code != "PAYOUT_INVALID_PAYMENT_ID" {
		t.Fatalf("expected invalid payment id error, got %+v", err)
	}
}

func TestValidatePayoutReq_MixedCaseBech32(t *testing.T) {
	s := &Server{btcParams: &chaincfg.RegressionNetParams}
	req := payoutReq{
		PaymentID:   uuid.NewString(),
		Network:     "btc",
		Asset:       "BTC",
		AmountSats:  1000,
		Destination: "bcrt1Pq8abcMixed",
		Priority:    "normal",
	}
	err := s.validatePayoutReq(context.Background(), req)
	if err == nil || err.Code != "PAYOUT_MIXED_CASE_BECH32" {
		t.Fatalf("expected mixed case bech32 error, got %+v", err)
	}
}

func TestValidatePayoutReq_InvalidPriority(t *testing.T) {
	s := &Server{btcParams: &chaincfg.RegressionNetParams}
	req := payoutReq{
		PaymentID:   uuid.NewString(),
		Network:     "btc",
		Asset:       "BTC",
		AmountSats:  1000,
		Destination: "bcrt1pdhc9lfdma6kzeuz8rd8cc9z2u94yxwt3tkh653qt4unn8yvrhweqxysz44",
		Priority:    "superfast",
	}
	err := s.validatePayoutReq(context.Background(), req)
	if err == nil || err.Code != "PAYOUT_INVALID_PRIORITY" {
		t.Fatalf("expected invalid priority error, got %+v", err)
	}
}

func TestValidatePayoutReq_TaprootOK(t *testing.T) {
	s := &Server{btcParams: &chaincfg.RegressionNetParams}
	req := payoutReq{
		PaymentID:   uuid.NewString(),
		Network:     "btc",
		Asset:       "BTC",
		AmountSats:  1000,
		Destination: "bcrt1pdhc9lfdma6kzeuz8rd8cc9z2u94yxwt3tkh653qt4unn8yvrhweqxysz44",
		Priority:    "normal",
	}
	err := s.validatePayoutReq(context.Background(), req)
	if err != nil {
		t.Fatalf("expected nil error, got %+v", err)
	}
}
