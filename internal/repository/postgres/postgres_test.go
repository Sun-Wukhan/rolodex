package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/navid/rolodex/internal/repository"
	"github.com/navid/rolodex/internal/repository/postgres"
	"github.com/navid/rolodex/internal/repository/repositorytest"
)

// TestPostgresContract runs only when TEST_DATABASE_URL points at a disposable
// database (CI provides one via a service container). Tables are truncated
// between subtests.
func TestPostgresContract(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration tests")
	}
	repositorytest.Run(t, func(t *testing.T) repository.UserRepository {
		ctx := context.Background()
		r, err := postgres.Open(ctx, dsn)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if _, err := r.DB().ExecContext(ctx, `TRUNCATE users CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		t.Cleanup(func() { _ = r.Close() })
		return r
	})
}
