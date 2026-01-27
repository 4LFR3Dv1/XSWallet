package mocks

import (
	"context"
	"fmt"
	"sync/atomic"
)

type Executor struct {
	counter uint64
}

func (e *Executor) Send(ctx context.Context, network, address string, amountCents int64) (string, error) {
	id := atomic.AddUint64(&e.counter, 1)
	return fmt.Sprintf("mocktxid_%s_%d", network, id), nil
}
