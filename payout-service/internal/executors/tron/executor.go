package tron

import (
	"context"
	"fmt"

	"domniwallet/payout-service/internal/store"
)

type Executor struct{}

func New() *Executor {
	return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, payout *store.Payout, utxos []store.ReservedUTXO) (string, error) {
	return "", fmt.Errorf("tron executor not implemented")
}

func (e *Executor) Confirmations(ctx context.Context, txid string) (int64, error) {
	return 0, fmt.Errorf("tron confirmations not implemented")
}

func (e *Executor) RequiredConfirmations() int64 {
	return 1
}
