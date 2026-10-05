// Package sqlite implements repository.ProfileStore and
// repository.CredentialStore on SQLite using the pure-Go modernc.org/sqlite
// driver (no cgo). Each store uses its own database file. Intended for local
// development, tests and single-node deployments.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/migrations"
)

// openDB opens (or creates) the SQLite database at path, enables foreign keys
// and WAL, and applies the migrations of schema.
func openDB(ctx context.Context, path string, schema migrations.Schema) (*sql.DB, error) {
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite %s: open: %w", schema, err)
	}
	// SQLite allows a single writer; one connection avoids SQLITE_BUSY under load.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite %s: ping: %w", schema, err)
	}
	if err := migrations.Up(ctx, db, "sqlite", schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

type scanner interface{ Scan(dest ...any) error }

func mapErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return domain.ErrConflict
		}
	}
	return fmt.Errorf("sqlite: %w", err)
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
