package db

import (
	"context"
	"database/sql"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, Schema)
	return err
}
