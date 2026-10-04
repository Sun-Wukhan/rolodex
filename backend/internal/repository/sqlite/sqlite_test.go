package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Sun-Wukhan/rolodex/internal/repository"
	"github.com/Sun-Wukhan/rolodex/internal/repository/repositorytest"
	"github.com/Sun-Wukhan/rolodex/internal/repository/split"
	"github.com/Sun-Wukhan/rolodex/internal/repository/sqlite"
)

func TestSQLiteContract(t *testing.T) {
	repositorytest.Run(t, func(t *testing.T) repository.UserRepository {
		ctx := context.Background()
		dir := t.TempDir()
		profiles, err := sqlite.OpenProfiles(ctx, filepath.Join(dir, "profiles.db"))
		if err != nil {
			t.Fatalf("open profiles: %v", err)
		}
		credentials, err := sqlite.OpenCredentials(ctx, filepath.Join(dir, "credentials.db"))
		if err != nil {
			t.Fatalf("open credentials: %v", err)
		}
		r := split.New(profiles, credentials)
		t.Cleanup(func() { _ = r.Close() })
		return r
	})
}
