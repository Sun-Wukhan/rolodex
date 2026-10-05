// Package postgres implements repository.ProfileStore and
// repository.CredentialStore on PostgreSQL. Each store owns its own connection
// pool and database. Because CockroachDB speaks the PostgreSQL wire protocol
// and supports this SQL subset, the same implementation can be pointed at
// CockroachDB.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"github.com/Sun-Wukhan/rolodex/internal/domain"
	"github.com/Sun-Wukhan/rolodex/migrations"
)

const uniqueViolation = "23505"

// pool opens a connection pool without connecting; label names the database
// in errors.
func pool(dsn, label string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres %s: open: %w", label, err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// connect opens a pool and verifies the server is reachable.
func connect(ctx context.Context, dsn, label string) (*sql.DB, error) {
	db, err := pool(dsn, label)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres %s: ping: %w", label, err)
	}
	return db, nil
}

// openDB connects to a primary and applies the migrations of schema.
func openDB(ctx context.Context, dsn string, schema migrations.Schema) (*sql.DB, error) {
	db, err := connect(ctx, dsn, string(schema))
	if err != nil {
		return nil, err
	}
	if err := migrations.Up(ctx, db, "postgres", schema); err != nil {
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
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return domain.ErrConflict
	}
	return fmt.Errorf("postgres: %w", err)
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
