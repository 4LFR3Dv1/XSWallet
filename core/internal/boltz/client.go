// Package boltz - HTTP client para Boltz API v2
// Implementa retry com backoff exponencial, Retry-After, e erros tipados
package boltz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	maxRetries        = 3
	initialBackoff    = 500 * time.Millisecond
	maxBackoff        = 30 * time.Second
	backoffMultiplier = 2.0
)

// Client é o cliente HTTP para a Boltz API
type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// ClientConfig configuração do client
type ClientConfig struct {
	BaseURL string        // https://api.boltz.exchange
	Timeout time.Duration // default 30s
}

// NewClient cria um novo client Boltz
func NewClient(cfg ClientConfig) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		userAgent:  "XS-Wallet/0.1.0",
	}
}

// doRequest executa HTTP request com retry, backoff, e erros tipados
func (c *Client) doRequest(ctx context.Context, method, path string, body, result interface{}) error {
	var lastErr error
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		// Prepara body
		var bodyReader io.Reader
		if body != nil {
			data, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("marshal request: %w", err)
			}
			bodyReader = bytes.NewReader(data)
		}

		// Cria request
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", c.userAgent)

		// Executa em closure para fechar body corretamente (Correção #6)
		respBody, statusCode, retryAfter, err := c.executeRequest(req)
		if err != nil {
			lastErr = err
			backoff = c.nextBackoff(backoff, 0)
			continue // Retry em erro de rede
		}

		// 5xx = retry
		if statusCode >= 500 {
			lastErr = fmt.Errorf("server error %d: %s", statusCode, string(respBody))
			backoff = c.nextBackoff(backoff, 0)
			continue
		}

		// 429 = rate limit, usar Retry-After se presente (Correção Gap #3)
		if statusCode == 429 {
			lastErr = ErrRateLimited
			backoff = c.nextBackoff(backoff, retryAfter)
			continue
		}

		// 4xx = não retry, mapear para erro tipado (Correção Gap #4)
		if statusCode >= 400 {
			return c.mapError(statusCode, respBody)
		}

		// Sucesso
		if result != nil {
			if err := json.Unmarshal(respBody, result); err != nil {
				return fmt.Errorf("unmarshal response: %w", err)
			}
		}
		return nil
	}

	return fmt.Errorf("max retries (%d) excedido: %w", maxRetries, lastErr)
}

// executeRequest executa a request e retorna body, status, retry-after header
func (c *Client) executeRequest(req *http.Request) ([]byte, int, int, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, 0, err
	}

	// Parse Retry-After header se presente
	retryAfter := 0
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if seconds, err := strconv.Atoi(ra); err == nil {
			retryAfter = seconds
		}
	}

	return body, resp.StatusCode, retryAfter, nil
}

// nextBackoff calcula próximo backoff respeitando Retry-After
func (c *Client) nextBackoff(current time.Duration, retryAfterSeconds int) time.Duration {
	if retryAfterSeconds > 0 {
		return time.Duration(retryAfterSeconds) * time.Second
	}
	next := time.Duration(float64(current) * backoffMultiplier)
	if next > maxBackoff {
		return maxBackoff
	}
	return next
}

// mapError converte erros HTTP em erros tipados
func (c *Client) mapError(statusCode int, body []byte) error {
	var errResp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(body, &errResp)

	// Mapeia status codes para erros de domínio
	switch statusCode {
	case 404:
		return ErrSwapNotFound
	case 400:
		// Parse erro específico do Boltz
		switch errResp.Error {
		case "pair hash mismatch":
			return ErrPairHashMismatch
		case "amount out of bounds":
			return ErrAmountOutOfBounds
		case "invoice hash mismatch":
			return ErrInvoiceMismatch
		}
	case 409:
		return ErrPairHashMismatch
	}

	// Retorna erro genérico do Boltz
	return &BoltzError{
		StatusCode: statusCode,
		Message:    errResp.Error,
	}
}

// =============================================================================
// PAIR INFO
// =============================================================================

// GetSubmarinePairs retorna pares disponíveis para submarine swap
func (c *Client) GetSubmarinePairs(ctx context.Context) (map[string]map[string]PairInfo, error) {
	var result map[string]map[string]PairInfo
	err := c.doRequest(ctx, "GET", "/v2/swap/submarine", nil, &result)
	return result, err
}

// GetReversePairs retorna pares disponíveis para reverse swap
func (c *Client) GetReversePairs(ctx context.Context) (map[string]map[string]PairInfo, error) {
	var result map[string]map[string]PairInfo
	err := c.doRequest(ctx, "GET", "/v2/swap/reverse", nil, &result)
	return result, err
}

// GetChainPairs retorna pares disponíveis para chain swap
func (c *Client) GetChainPairs(ctx context.Context) (map[string]map[string]PairInfo, error) {
	var result map[string]map[string]PairInfo
	err := c.doRequest(ctx, "GET", "/v2/swap/chain", nil, &result)
	return result, err
}

// =============================================================================
// CREATE SWAPS
// =============================================================================

// CreateSubmarine cria um submarine swap (on-chain → LN)
func (c *Client) CreateSubmarine(ctx context.Context, req SubmarineRequest) (*SubmarineResponse, error) {
	var result SubmarineResponse
	err := c.doRequest(ctx, "POST", "/v2/swap/submarine", req, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateReverse cria um reverse swap (LN → on-chain)
func (c *Client) CreateReverse(ctx context.Context, req ReverseRequest) (*ReverseResponse, error) {
	var result ReverseResponse
	err := c.doRequest(ctx, "POST", "/v2/swap/reverse", req, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateChain creates a chain swap (BTC <-> Liquid)
func (c *Client) CreateChain(ctx context.Context, req ChainRequest) (*ChainResponse, error) {
	var result ChainResponse
	err := c.doRequest(ctx, "POST", "/v2/swap/chain", req, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// STATUS
// =============================================================================

// GetSwapStatus retorna status atual de um swap
func (c *Client) GetSwapStatus(ctx context.Context, id string) (*SwapStatus, error) {
	var result SwapStatus
	err := c.doRequest(ctx, "GET", "/v2/swap/"+id, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// SUBMARINE CLAIM
// =============================================================================

// GetSubmarineClaimDetails obtém detalhes para cooperative claim de submarine
func (c *Client) GetSubmarineClaimDetails(ctx context.Context, id string) (*SubmarineClaimDetails, error) {
	var result SubmarineClaimDetails
	err := c.doRequest(ctx, "GET", "/v2/swap/submarine/"+id+"/claim", nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// PostSubmarineClaim envia partial signature para cooperative claim
func (c *Client) PostSubmarineClaim(ctx context.Context, id string, sig PartialSignature) error {
	return c.doRequest(ctx, "POST", "/v2/swap/submarine/"+id+"/claim", sig, nil)
}

// =============================================================================
// REVERSE CLAIM
// =============================================================================

// PostReverseClaim envia claim request para reverse swap
func (c *Client) PostReverseClaim(ctx context.Context, id string, req ReverseClaimRequest) (*ReverseClaimResponse, error) {
	var result ReverseClaimResponse
	err := c.doRequest(ctx, "POST", "/v2/swap/reverse/"+id+"/claim", req, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// =============================================================================
// CHAIN CLAIM
// =============================================================================

// GetChainClaimDetails obtém detalhes para claim de chain swap
func (c *Client) GetChainClaimDetails(ctx context.Context, id string) (*ChainClaimDetails, error) {
	var result ChainClaimDetails
	err := c.doRequest(ctx, "GET", "/v2/swap/chain/"+id+"/claim", nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// PostChainClaim envia partial signature para chain swap claim
func (c *Client) PostChainClaim(ctx context.Context, id string, sig PartialSignature, preimage string) error {
	req := struct {
		Preimage  string           `json:"preimage"`
		Signature PartialSignature `json:"signature"`
	}{preimage, sig}
	return c.doRequest(ctx, "POST", "/v2/swap/chain/"+id+"/claim", req, nil)
}

// =============================================================================
// UTILITY
// =============================================================================

// GetBlockHeight retorna altura atual do bloco
func (c *Client) GetBlockHeight(ctx context.Context, currency string) (int64, error) {
	var result struct {
		Height int64 `json:"height"`
	}
	err := c.doRequest(ctx, "GET", "/v2/chain/"+currency+"/height", nil, &result)
	return result.Height, err
}
