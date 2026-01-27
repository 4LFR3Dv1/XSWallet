package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	httpc   *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpc: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type updateWithdrawalReq struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
	TxID   string `json:"txid,omitempty"`
}

func (c *Client) UpdateWithdrawalStatus(ctx context.Context, id int64, status, txid string) error {
	if c.baseURL == "" || id == 0 || status == "" {
		return nil
	}
	body, _ := json.Marshal(updateWithdrawalReq{ID: id, Status: status, TxID: txid})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/internal/withdrawals/status", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Internal-Token", c.token)
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("wallet update status failed: %d", resp.StatusCode)
	}
	return nil
}
