package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/navid/rolodex/internal/repository"
	"github.com/navid/rolodex/internal/repository/repositorytest"
	"github.com/navid/rolodex/internal/repository/sqlite"
)

func TestSQLiteContract(t *testing.T) {
	repositorytest.Run(t, func(t *testing.T) repository.UserRepository {
		r, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = r.Close() })
		return r
	})
}
