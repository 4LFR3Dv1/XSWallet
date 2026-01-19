// Package boltz - Erros tipados para integração Boltz API v2
package boltz

import "errors"

// Erros de domínio
var (
	ErrQuoteExpired      = errors.New("quote expirou")
	ErrSwapNotFound      = errors.New("swap não encontrado")
	ErrPairHashMismatch  = errors.New("pair hash diferente - fees podem ter mudado")
	ErrAmountOutOfBounds = errors.New("valor fora dos limites min/max")
	ErrInvoiceMismatch   = errors.New("preimage hash da invoice não confere")
	ErrProviderUnavail   = errors.New("provider temporariamente indisponível")
	ErrWebSocketClosed   = errors.New("conexão websocket fechada")
	ErrRateLimited       = errors.New("rate limited pelo provider")
	ErrSwapExpired       = errors.New("swap expirou")
	ErrLockupFailed      = errors.New("lockup falhou - valor incorreto")
	ErrInvoiceFailedPay  = errors.New("pagamento da invoice falhou")
)

// BoltzError representa erro retornado pelo Boltz API
type BoltzError struct {
	StatusCode int
	Message    string
}

func (e *BoltzError) Error() string {
	return e.Message
}

// IsBoltzError verifica se é erro do Boltz API
func IsBoltzError(err error) (*BoltzError, bool) {
	var be *BoltzError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}
