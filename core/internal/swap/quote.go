// Package swap - Quote service for managing swap quotes
package swap

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/xs-wallet/xscore/internal/db"
	"github.com/xs-wallet/xscore/internal/provider"
)

// QuoteService manages swap quotes
type QuoteService struct {
	db       *db.DB
	provider provider.Provider
}

// NewQuoteService creates a new quote service
func NewQuoteService(database *db.DB, prov provider.Provider) *QuoteService {
	return &QuoteService{
		db:       database,
		provider: prov,
	}
}

// CreateQuote requests a quote from the provider and persists it
func (qs *QuoteService) CreateQuote(ctx context.Context, req provider.QuoteRequest) (*provider.Quote, error) {
	// Request quote from provider
	quote, err := qs.provider.Quote(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get quote from provider: %w", err)
	}

	// Persist quote
	if err := qs.saveQuote(ctx, quote); err != nil {
		return nil, fmt.Errorf("failed to save quote: %w", err)
	}

	return quote, nil
}

// GetQuote retrieves a quote by ID
func (qs *QuoteService) GetQuote(ctx context.Context, quoteID string) (*provider.Quote, error) {
	var dataJSON string
	var expiresAt string

	err := qs.db.QueryRowContext(ctx, `
		SELECT data, expires_at FROM quotes WHERE quote_id = ?
	`, quoteID).Scan(&dataJSON, &expiresAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("quote not found: %s", quoteID)
		}
		return nil, err
	}

	var quote provider.Quote
	if err := json.Unmarshal([]byte(dataJSON), &quote); err != nil {
		return nil, err
	}

	// Check expiry
	expiry, _ := time.Parse(time.RFC3339, expiresAt)
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("quote expired")
	}

	return &quote, nil
}

// saveQuote persists a quote to the database
func (qs *QuoteService) saveQuote(ctx context.Context, quote *provider.Quote) error {
	// Ensure quotes table exists
	_, err := qs.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS quotes (
			quote_id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			amount_sat INTEGER NOT NULL,
			data TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)
	`)
	if err != nil {
		return err
	}

	// Serialize quote data
	dataJSON, err := json.Marshal(quote)
	if err != nil {
		return err
	}

	_, err = qs.db.ExecContext(ctx, `
		INSERT INTO quotes (quote_id, kind, amount_sat, data, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`, quote.QuoteID, quote.Kind, quote.AmountSat, string(dataJSON), quote.ExpiresAt.Format(time.RFC3339))

	return err
}

// ExpireQuotes removes expired quotes (background job)
func (qs *QuoteService) ExpireQuotes(ctx context.Context) error {
	_, err := qs.db.ExecContext(ctx, `
		DELETE FROM quotes WHERE expires_at < ?
	`, time.Now().Format(time.RFC3339))
	return err
}
