// Package migrations embeds the per-dialect SQL migrations and applies them
// with goose.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed postgres/*.sql sqlite/*.sql
var files embed.FS

// Up applies all pending migrations for the given dialect ("postgres" or
// "sqlite").
func Up(ctx context.Context, db *sql.DB, dialect string) error {
	var gd goose.Dialect
	switch dialect {
	case "postgres":
		gd = goose.DialectPostgres
	case "sqlite":
		gd = goose.DialectSQLite3
	default:
		return fmt.Errorf("migrations: unsupported dialect %q", dialect)
	}
	sub, err := fs.Sub(files, dialect)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	provider, err := goose.NewProvider(gd, db, sub)
	if err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrations: up: %w", err)
	}
	return nil
}
