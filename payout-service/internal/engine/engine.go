package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"domniwallet/payout-service/internal/store"
	"domniwallet/payout-service/internal/wallet"
)

type Executor interface {
	Execute(ctx context.Context, payout *store.Payout, utxos []store.ReservedUTXO) (string, error)
	Confirmations(ctx context.Context, txid string) (int64, error)
	RequiredConfirmations() int64
}

type Engine struct {
	store                *store.Store
	wallet               *wallet.Client
	executors            map[string]Executor
	maxAttempts          int
	interval             time.Duration
	circuitFailThreshold int
	circuitOpenDuration  time.Duration
	kick                 chan struct{}
}

func New(store *store.Store, walletClient *wallet.Client, executors map[string]Executor, maxAttempts int, interval time.Duration, failThreshold int, openDuration time.Duration) *Engine {
	return &Engine{
		store:                store,
		wallet:               walletClient,
		executors:            executors,
		maxAttempts:          maxAttempts,
		interval:             interval,
		circuitFailThreshold: failThreshold,
		circuitOpenDuration:  openDuration,
		kick:                 make(chan struct{}, 1),
	}
}

func (e *Engine) Kick() {
	select {
	case e.kick <- struct{}{}:
	default:
	}
}

func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick()
		case <-e.kick:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := e.processConfirming(ctx); err != nil {
		log.Printf("confirming error: %v", err)
	}
	if err := e.processNext(ctx); err != nil {
		log.Printf("process next error: %v", err)
	}
}

func (e *Engine) processNext(ctx context.Context) error {
	payout, err := e.store.ClaimNextPayout(ctx, time.Now().UTC())
	if err != nil || payout == nil {
		return err
	}

	exec := e.executors[payout.Network]
	if exec == nil {
		return e.failFinal(ctx, payout, fmt.Errorf("unsupported network executor: %s", payout.Network))
	}

	open, err := e.store.IsCircuitOpen(ctx, payout.Network, e.circuitOpenDuration)
	if err != nil {
		return err
	}
	if open {
		return e.failRetryable(ctx, payout, fmt.Errorf("circuit open"))
	}

	var utxos []store.ReservedUTXO
	if payout.WithdrawalID.Valid {
		utxos, err = e.store.GetReservedUTXOs(ctx, payout.WithdrawalID.Int64)
		if err != nil {
			return e.failRetryable(ctx, payout, err)
		}
	}

	txid, err := exec.Execute(ctx, payout, utxos)
	if err != nil {
		if _, openErr := e.store.RecordCircuitFailure(ctx, payout.Network, e.circuitFailThreshold); openErr != nil {
			log.Printf("circuit failure update error: %v", openErr)
		}
		return e.handleExecutionError(ctx, payout, err)
	}
	if err := e.store.RecordCircuitSuccess(ctx, payout.Network); err != nil {
		log.Printf("circuit success update error: %v", err)
	}

	if err := e.store.UpdateStatus(ctx, payout.ID, store.StatusConfirming, txid, "", nil); err != nil {
		return err
	}
	if payout.WithdrawalID.Valid {
		_ = e.wallet.UpdateWithdrawalStatus(ctx, payout.WithdrawalID.Int64, "EXECUTING", txid)
	}
	return nil
}

func (e *Engine) processConfirming(ctx context.Context) error {
	items, err := e.store.ListConfirming(ctx, 50)
	if err != nil {
		return err
	}
	for _, payout := range items {
		exec := e.executors[payout.Network]
		if exec == nil {
			_ = e.failFinal(ctx, &payout, fmt.Errorf("unsupported network executor: %s", payout.Network))
			continue
		}
		if !payout.TxID.Valid || payout.TxID.String == "" {
			_ = e.failFinal(ctx, &payout, fmt.Errorf("missing txid"))
			continue
		}
		confs, err := exec.Confirmations(ctx, payout.TxID.String)
		if err != nil {
			continue
		}
		if confs >= exec.RequiredConfirmations() {
			if err := e.store.UpdateStatus(ctx, payout.ID, store.StatusCompleted, payout.TxID.String, "", nil); err != nil {
				return err
			}
			if payout.WithdrawalID.Valid {
				_ = e.wallet.UpdateWithdrawalStatus(ctx, payout.WithdrawalID.Int64, "COMPLETED", payout.TxID.String)
			}
		}
	}
	return nil
}

func (e *Engine) handleExecutionError(ctx context.Context, payout *store.Payout, err error) error {
	if isRetryable(err) {
		return e.failRetryable(ctx, payout, err)
	}
	return e.failFinal(ctx, payout, err)
}

func (e *Engine) failRetryable(ctx context.Context, payout *store.Payout, err error) error {
	if payout.Attempts >= e.maxAttempts {
		return e.failFinal(ctx, payout, fmt.Errorf("max attempts reached: %w", err))
	}
	next := time.Now().UTC().Add(backoffForAttempt(payout.Attempts))
	return e.store.UpdateStatus(ctx, payout.ID, store.StatusFailedRetry, "", err.Error(), &next)
}

func (e *Engine) failFinal(ctx context.Context, payout *store.Payout, err error) error {
	if err := e.store.UpdateStatus(ctx, payout.ID, store.StatusFailedFinal, "", err.Error(), nil); err != nil {
		return err
	}
	return e.store.EnqueueDLQ(ctx, payout.ID, err.Error())
}

type retryableError struct {
	err error
}

func (e retryableError) Error() string { return e.err.Error() }

func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return retryableError{err: err}
}

func isRetryable(err error) bool {
	var r retryableError
	if errors.As(err, &r) {
		return true
	}
	return false
}

func backoffForAttempt(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 5 * time.Second
	case 2:
		return 15 * time.Second
	default:
		return 45 * time.Second
	}
}
