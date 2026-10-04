package postgres_test

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/internal/repository/postgres"
	"github.com/Sun-Wukhan/rolodex/internal/repository/repositorytest"
	"github.com/Sun-Wukhan/rolodex/internal/repository/split"
)

// TestPostgresContract runs only when TEST_DATABASE_URL points at a disposable
// database (CI provides one via a service container). The credentials store
// uses a sibling "<name>_credentials" database on the same server, created on
// demand. Tables are truncated between subtests.
func TestPostgresContract(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration tests")
	}
	credentialsDSN := siblingDatabase(t, dsn, "_credentials")

	repositorytest.Run(t, func(t *testing.T) repository.UserRepository {
		ctx := context.Background()
		profiles, err := postgres.OpenProfiles(ctx, dsn)
		if err != nil {
			t.Fatalf("open profiles: %v", err)
		}
		credentials, err := postgres.OpenCredentials(ctx, credentialsDSN)
		if err != nil {
			t.Fatalf("open credentials: %v", err)
		}
		if _, err := profiles.DB().ExecContext(ctx, `TRUNCATE users CASCADE`); err != nil {
			t.Fatalf("truncate profiles: %v", err)
		}
		if _, err := credentials.DB().ExecContext(ctx, `TRUNCATE user_credentials`); err != nil {
			t.Fatalf("truncate credentials: %v", err)
		}
		r := split.New(profiles, credentials)
		t.Cleanup(func() { _ = r.Close() })
		return r
	})
}

// siblingDatabase ensures a database named after dsn's plus suffix exists on
// the same server and returns its DSN.
func siblingDatabase(t *testing.T, dsn, suffix string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	name := u.Path[1:] + suffix
	u.Path = "/" + name

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		t.Fatalf("check database: %v", err)
	}
	if !exists {
		if _, err := db.ExecContext(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Fatalf("create database %s: %v", name, err)
		}
	}
	return u.String()
}
